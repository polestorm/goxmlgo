package goxmlgo

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmltree"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

// TestNoIOImports: pakiety biblioteki nie mają jak sięgnąć do plików, sieci, procesów ani pamięci poza kontrolą
// języka: jedyne wejście i wyjście to io.Reader i io.Writer wywołującego.
// REQ: SAFE-001
func TestNoIOImports(t *testing.T) {
	forbidden := map[string]bool{"os": true, "net": true, "net/http": true, "os/exec": true, "syscall": true,
		"unsafe": true, "plugin": true, "io/ioutil": true, "os/signal": true, "runtime/cgo": true, "C": true}
	checked := 0
	for _, pkg := range append([]string{"."}, publicPackages...) {
		files, err := filepath.Glob(filepath.Join(pkg, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			checked++
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if forbidden[path] {
					t.Errorf("%s imports %q", file, path)
				}
				if !strings.Contains(path, ".") || strings.HasPrefix(path, modulePath+"/") {
					continue // biblioteka standardowa albo ten moduł
				}
				t.Errorf("%s imports %q: the library has no dependencies", file, path)
			}
		}
	}
	if checked < 10 {
		t.Fatalf("only %d source files checked", checked)
	}
}

// measure wykonuje f i zwraca czas oraz szczyt pamięci sterty ponad stan sprzed wywołania. Szczyt mierzy osobna
// gorutyna co 200 µs, przy odśmiecaniu ustawionym tak, żeby sterta trzymała się blisko pamięci żywej; pomiar czasu
// jest przez to zawyżony, nie zaniżony.
func measure(f func()) (time.Duration, uint64) {
	defer debug.SetGCPercent(debug.SetGCPercent(10))
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	base, peak := m.HeapAlloc, m.HeapAlloc
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		var s runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-time.After(200 * time.Microsecond):
				runtime.ReadMemStats(&s)
				if s.HeapAlloc > peak {
					peak = s.HeapAlloc
				}
			}
		}
	}()
	start := time.Now()
	f()
	d := time.Since(start)
	runtime.ReadMemStats(&m)
	close(stop)
	<-done
	if m.HeapAlloc > peak {
		peak = m.HeapAlloc
	}
	return d, peak - base
}

// repeat to io.Reader, który oddaje head, potem n razy unit, potem tail, bez trzymania całości w pamięci.
func repeat(head, unit string, n int, tail string) io.Reader {
	return io.MultiReader(strings.NewReader(head), &repeater{unit: unit, n: n}, strings.NewReader(tail))
}

type repeater struct {
	unit string
	n    int
	off  int
}

func (r *repeater) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		if r.n == 0 {
			if written == 0 {
				return 0, io.EOF
			}
			break
		}
		k := copy(p[written:], r.unit[r.off:])
		written += k
		if r.off += k; r.off == len(r.unit) {
			r.off = 0
			r.n--
		}
	}
	return written, nil
}

func readAll(src io.Reader, opts xmlreader.Options) (tokens int, err error) {
	r := xmlreader.New(src, opts)
	for {
		if _, err := r.Next(); err != nil {
			if err == io.EOF {
				return tokens, nil
			}
			return tokens, err
		}
		tokens++
	}
}

func errCode(err error) string {
	var re *xmlreader.Error
	var we *xmlwriter.Error
	var te *xmltree.Error
	switch {
	case errors.As(err, &re):
		return re.Code
	case errors.As(err, &we):
		return we.Code
	case errors.As(err, &te):
		return te.Code
	case err == nil:
		return "no error"
	}
	return "not a library error: " + err.Error()
}

const (
	hostileTime  = time.Second
	hostileBytes = 64 << 20
)

// TestHostileInputs: wejścia zbudowane po to, żeby zużyć pamięć albo czas, są odrzucane błędem ze stałym kodem,
// każde w czasie poniżej sekundy i przy szczycie pamięci poniżej 64 MB (bez samego wejścia), przy ustawieniach domyślnych:
// przez czytnik, przez budowę drzewa i, dla ciągów wywołań, przez pisarza. Encja zewnętrzna nie jest czytana.
// REQ: SAFE-003 SAFE-005 SAFE-006 WR-020
func TestHostileInputs(t *testing.T) {
	laughs := "<!DOCTYPE a [<!ENTITY e0 'ha'>"
	for i := 1; i <= 12; i++ {
		laughs += fmt.Sprintf("<!ENTITY e%d '%s'>", i, strings.Repeat(fmt.Sprintf("&e%d;", i-1), 10))
	}
	laughs += "]><a>&e12;</a>"
	chain := "<!DOCTYPE a [<!ENTITY e0 'x'>"
	for i := 1; i <= 40; i++ {
		chain += fmt.Sprintf("<!ENTITY e%d '&e%d;'>", i, i-1)
	}
	chain += "]><a>&e40;</a>"
	big := 17 << 20 // ponad domyślny limit tekstu 16 MiB
	cases := []struct {
		name string
		src  func() io.Reader
		code string // "" oznacza dowolny kod z rodzin limit-, wfc- i unsupported-
	}{
		{"exponential entity expansion", func() io.Reader { return strings.NewReader(laughs) }, xmlreader.CodeLimitEntityExpand},
		{"quadratic entity expansion", func() io.Reader {
			return repeat("<!DOCTYPE a [<!ENTITY e '"+strings.Repeat("x", 50_000)+"'>]><a>", "&e;", 200, "</a>")
		}, xmlreader.CodeLimitEntityExpand},
		{"entity that refers to itself", func() io.Reader {
			return strings.NewReader("<!DOCTYPE a [<!ENTITY e '&f;'><!ENTITY f '&e;'>]><a>&e;</a>")
		}, xmlreader.CodeNoRecursion},
		{"chain of 40 nested entities", func() io.Reader { return strings.NewReader(chain) }, xmlreader.CodeLimitEntityDepth},
		{"external entity", func() io.Reader {
			return strings.NewReader(`<!DOCTYPE a [<!ENTITY e SYSTEM "file:///etc/passwd">]><a>&e;</a>`)
		}, ""},
		{"external parameter entity", func() io.Reader {
			return strings.NewReader(`<!DOCTYPE a [<!ENTITY % p SYSTEM "http://127.0.0.1:9/x.dtd">%p;]><a>&undeclared;</a>`)
		}, ""},
		{"a million nested elements", func() io.Reader { return repeat("", "<a>", 1_000_000, "") }, xmlreader.CodeLimitDepth},
		{"a name of 1 MB", func() io.Reader { return repeat("<", "abcdefgh", 1<<17, "/>") }, xmlreader.CodeLimitNameLength},
		{"an attribute name of 1 MB", func() io.Reader { return repeat("<a ", "abcdefgh", 1<<17, "='1'/>") }, xmlreader.CodeLimitNameLength},
		{"an attribute value of 17 MB", func() io.Reader { return repeat("<a x='", "0123456789abcdef", big/16, "'/>") }, xmlreader.CodeLimitTextSize},
		{"a million attributes", func() io.Reader {
			var b strings.Builder
			b.WriteString("<a")
			for i := 0; i < 1_000_000; i++ {
				b.WriteString(" a")
				b.WriteString(strconv.Itoa(i))
				b.WriteString("=''")
			}
			return strings.NewReader(b.String() + "/>")
		}, xmlreader.CodeLimitAttributes},
		{"a million namespace declarations in one tag", func() io.Reader {
			var b strings.Builder
			b.WriteString("<a")
			for i := 0; i < 1_000_000; i++ {
				b.WriteString(" xmlns:p")
				b.WriteString(strconv.Itoa(i))
				b.WriteString("='u'")
			}
			return strings.NewReader(b.String() + "/>")
		}, ""},
		{"namespace declarations piled up in nested elements", func() io.Reader {
			var b strings.Builder
			for level := 0; level < 500; level++ {
				b.WriteString("<a")
				for i := 0; i < 600; i++ {
					fmt.Fprintf(&b, " xmlns:p%d_%d='u'", level, i)
				}
				b.WriteString(">")
			}
			return repeat(b.String(), "<p0_0:b/>", 1_000_000, "")
		}, xmlreader.CodeLimitNamespaces},
		{"text of 17 MB", func() io.Reader { return repeat("<a>", "0123456789abcdef", big/16, "</a>") }, xmlreader.CodeLimitTextSize},
		{"a CDATA section of 17 MB", func() io.Reader { return repeat("<a><![CDATA[", "0123456789abcdef", big/16, "]]></a>") }, xmlreader.CodeLimitTextSize},
		{"a comment of 17 MB", func() io.Reader { return repeat("<a><!--", "0123456789abcdef", big/16, "--></a>") }, xmlreader.CodeLimitTextSize},
		{"a processing instruction of 17 MB", func() io.Reader { return repeat("<a><?p ", "0123456789abcdef", big/16, "?></a>") }, xmlreader.CodeLimitTextSize},
		{"a DOCTYPE of 2 MB", func() io.Reader {
			return repeat("<!DOCTYPE a [", "<!ELEMENT abcdefgh ANY>", 100_000, "]><a/>")
		}, xmlreader.CodeLimitDTDSize},
	}
	check := func(name, what, want string, d time.Duration, alloc uint64, err error) {
		t.Helper()
		got := errCode(err)
		ok := got == want
		if want == "" {
			ok = strings.HasPrefix(got, "limit-") || strings.HasPrefix(got, "wfc-") || strings.HasPrefix(got, "unsupported-")
		}
		if !ok {
			t.Errorf("%s, %s: %v, want code %q", name, what, err, want)
		}
		if d > hostileTime || alloc > hostileBytes {
			t.Errorf("%s, %s: %v and %d MB, want under %v and %d MB", name, what, d, alloc>>20, hostileTime, hostileBytes>>20)
		}
		if testing.Verbose() {
			t.Logf("%-52s %-7s %-32s %8.1f ms %6.1f MB", name, what, got, float64(d.Microseconds())/1000, float64(alloc)/(1<<20))
		}
	}
	for _, tc := range cases {
		var err error
		src := tc.src() // wejście powstaje przed pomiarem: mierzymy bibliotekę, nie generator
		d, alloc := measure(func() { _, err = readAll(src, xmlreader.Options{}) })
		check(tc.name, "reader", tc.code, d, alloc, err)
		var doc *xmltree.Document
		src = tc.src()
		d, alloc = measure(func() { doc, err = xmltree.Parse(src, xmltree.Options{}) })
		check(tc.name, "tree", tc.code, d, alloc, err)
		if doc != nil {
			t.Errorf("%s: Parse returned a document with the error", tc.name)
		}
	}

	// drzewo: limit węzłów, także dla poddrzewa w środku strumienia
	flat := func() io.Reader { return repeat("<r>", "<a/>", 1_000_000, "</r>") }
	var err error
	d, alloc := measure(func() { _, err = xmltree.Parse(flat(), xmltree.Options{MaxNodes: 100_000}) })
	check("a million elements, MaxNodes 100 000", "tree", xmltree.CodeLimitNodes, d, alloc, err)
	d, alloc = measure(func() {
		r := xmlreader.New(flat(), xmlreader.Options{})
		var tok xmlreader.Token
		if tok, err = r.Next(); err == nil {
			_, err = xmltree.ReadElement(r, tok, xmltree.Options{MaxNodes: 100_000})
		}
	})
	check("a million elements, ReadElement, MaxNodes 100 000", "tree", xmltree.CodeLimitNodes, d, alloc, err)

	// pisarz: ciągi wywołań zamiast wejścia
	calls := []struct {
		name string
		run  func(w *xmlwriter.Writer) error
		code string
	}{
		{"a million nested Start", func(w *xmlwriter.Writer) error {
			for i := 0; i < 1_000_000; i++ {
				if err := w.Start(xmlwriter.Name{Local: "a"}); err != nil {
					return err
				}
			}
			return nil
		}, xmlwriter.CodeLimitDepth},
		{"a million Attr", func(w *xmlwriter.Writer) error {
			w.Start(xmlwriter.Name{Local: "a"})
			for i := 0; i < 1_000_000; i++ {
				if err := w.Attr(xmlwriter.Name{Local: "a" + strconv.Itoa(i)}, ""); err != nil {
					return err
				}
			}
			return nil
		}, xmlwriter.CodeLimitAttributes},
		{"a name of 1 MB", func(w *xmlwriter.Writer) error {
			return w.Start(xmlwriter.Name{Local: strings.Repeat("abcdefgh", 1<<17)})
		}, xmlwriter.CodeLimitNameLength},
		{"text of 17 MB in calls of 64 KiB", func(w *xmlwriter.Writer) error {
			w.Start(xmlwriter.Name{Local: "a"})
			chunk := strings.Repeat("x", 64<<10)
			for i := 0; i < big/len(chunk)+1; i++ {
				if err := w.Text(chunk); err != nil {
					return err
				}
			}
			return nil
		}, xmlwriter.CodeLimitTextSize},
		{"namespace declarations piled up in nested Start", func(w *xmlwriter.Writer) error {
			for level := 0; level < 500; level++ {
				decls := make([]xmlwriter.Binding, 600)
				for i := range decls {
					decls[i] = xmlwriter.Binding{Prefix: fmt.Sprintf("p%d_%d", level, i), URI: "u"}
				}
				if err := w.Start(xmlwriter.Name{Local: "a"}, decls...); err != nil {
					return err
				}
			}
			return nil
		}, xmlwriter.CodeLimitNamespaces},
		{"a million calls after Close", func(w *xmlwriter.Writer) error {
			w.Start(xmlwriter.Name{Local: "a"})
			w.End()
			w.Close()
			var last error
			for i := 0; i < 1_000_000; i++ {
				last = w.Text("x")
			}
			return last
		}, xmlwriter.CodeUsageClosed},
	}
	for _, tc := range calls {
		d, alloc := measure(func() { err = tc.run(xmlwriter.New(io.Discard, xmlwriter.Options{})) })
		check(tc.name, "writer", tc.code, d, alloc, err)
	}
}

// TestSafeByDefault: zerowe Options włączają limity o wartościach z dokumentacji; wyłączenie limitu wymaga jawnej
// wartości ujemnej.
// REQ: SAFE-002 XML-031
func TestSafeByDefault(t *testing.T) {
	rl := xmlreader.DefaultLimits()
	for name, v := range map[string]int{"MaxDepth": rl.MaxDepth, "MaxNameLength": rl.MaxNameLength, "MaxAttributes": rl.MaxAttributes,
		"MaxTextSize": rl.MaxTextSize, "MaxEntityExpansion": rl.MaxEntityExpansion, "MaxEntityDepth": rl.MaxEntityDepth,
		"MaxDTDSize": rl.MaxDTDSize, "MaxNamespaces": rl.MaxNamespaces} {
		if v <= 0 {
			t.Errorf("xmlreader.DefaultLimits().%s = %d: the limit is off by default", name, v)
		}
	}
	wl := xmlwriter.DefaultLimits()
	if wl.MaxDepth != rl.MaxDepth || wl.MaxNameLength != rl.MaxNameLength || wl.MaxAttributes != rl.MaxAttributes ||
		wl.MaxTextSize != rl.MaxTextSize || wl.MaxNamespaces != rl.MaxNamespaces {
		t.Errorf("limits of the writer %+v differ from those of the reader %+v", wl, rl)
	}
	if xmltree.DefaultMaxNodes <= 0 {
		t.Error("the tree has no default limit of nodes")
	}
	nested := func(depth int) io.Reader { return repeat("", "<a>", depth, strings.Repeat("</a>", depth)) }
	// czytnik i drzewo: zero to limit domyślny, wartość ujemna go wyłącza
	if _, err := readAll(nested(rl.MaxDepth), xmlreader.Options{}); err != nil {
		t.Errorf("depth at the limit: %v", err)
	}
	if _, err := readAll(nested(rl.MaxDepth+1), xmlreader.Options{}); errCode(err) != xmlreader.CodeLimitDepth {
		t.Errorf("depth over the default limit with zero Options: %v", err)
	}
	if _, err := xmltree.Parse(nested(rl.MaxDepth+1), xmltree.Options{}); errCode(err) != xmlreader.CodeLimitDepth {
		t.Errorf("tree, depth over the default limit with zero Options: %v", err)
	}
	if _, err := readAll(nested(rl.MaxDepth+1), xmlreader.Options{Limits: xmlreader.Limits{MaxDepth: -1}}); err != nil {
		t.Errorf("depth with the limit turned off: %v", err)
	}
	// deklaracje w zakresie: tyle, ile wynosi limit, przechodzi; o jedną więcej nie
	scope := func(n int) io.Reader {
		var b strings.Builder
		b.WriteString("<a")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, " xmlns:p%d='u'", i)
		}
		return strings.NewReader(b.String() + "><b/></a>")
	}
	if _, err := readAll(scope(rl.MaxNamespaces), xmlreader.Options{}); err != nil {
		t.Errorf("namespace declarations at the limit: %v", err)
	}
	if _, err := readAll(scope(rl.MaxNamespaces+1), xmlreader.Options{}); errCode(err) != xmlreader.CodeLimitNamespaces {
		t.Errorf("namespace declarations over the limit: %v", err)
	}
	if _, err := readAll(scope(rl.MaxNamespaces+1), xmlreader.Options{Limits: xmlreader.Limits{MaxNamespaces: -1}}); err != nil {
		t.Errorf("namespace declarations with the limit turned off: %v", err)
	}
	// pisarz
	deep := func(opts xmlwriter.Options, depth int) error {
		w := xmlwriter.New(io.Discard, opts)
		for i := 0; i < depth; i++ {
			if err := w.Start(xmlwriter.Name{Local: "a"}); err != nil {
				return err
			}
		}
		return nil
	}
	if err := deep(xmlwriter.Options{}, wl.MaxDepth); err != nil {
		t.Errorf("writer, depth at the limit: %v", err)
	}
	if err := deep(xmlwriter.Options{}, wl.MaxDepth+1); errCode(err) != xmlwriter.CodeLimitDepth {
		t.Errorf("writer, depth over the default limit with zero Options: %v", err)
	}
	if err := deep(xmlwriter.Options{Limits: xmlwriter.Limits{MaxDepth: -1}}, wl.MaxDepth+1); err != nil {
		t.Errorf("writer, depth with the limit turned off: %v", err)
	}
	// drzewo: limit węzłów ujemny wyłącza, dodatni działa
	small := func() io.Reader { return repeat("<r>", "<a/>", 1000, "</r>") }
	if _, err := xmltree.Parse(small(), xmltree.Options{MaxNodes: 1000}); errCode(err) != xmltree.CodeLimitNodes {
		t.Errorf("tree, 1001 nodes with MaxNodes 1000: %v", err)
	}
	if _, err := xmltree.Parse(small(), xmltree.Options{MaxNodes: -1}); err != nil {
		t.Errorf("tree with the limit turned off: %v", err)
	}
}

// TestPathological: czas liniowy na wejściach patologicznych. Dla każdej rodziny podwojenie rozmiaru zwiększa czas
// najwyżej 2,5 raza (najmniejszy z siedmiu przebiegów). Limity, które same odrzuciłyby wejście, są wyłączone, żeby
// mierzyć algorytm, a nie limit; rodzina z deklaracjami przestrzeni nazw działa na limicie domyślnym, bo to on
// ogranicza koszt wyszukania prefiksu.
// REQ: SAFE-004
func TestPathological(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	off := xmlreader.Limits{MaxDepth: -1, MaxAttributes: -1, MaxNameLength: -1, MaxTextSize: -1, MaxEntityExpansion: -1}
	woff := xmlwriter.Limits{MaxDepth: -1, MaxAttributes: -1, MaxNameLength: -1, MaxTextSize: -1}
	read := func(src string) func() {
		return func() {
			if _, err := readAll(strings.NewReader(src), xmlreader.Options{Limits: off}); err != nil {
				panic(err)
			}
		}
	}
	many := func(n int, unit func(i int) string) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteString(unit(i))
		}
		return b.String()
	}
	scope := "<r" + many(255, func(i int) string { return " xmlns:p" + strconv.Itoa(i) + "='u'" }) + " xmlns:z='u'>"
	families := []struct {
		name string
		size int
		make func(n int) func()
	}{
		{"reader: attributes of one element", 60_000, func(n int) func() {
			return read("<a" + many(n, func(i int) string { return " a" + strconv.Itoa(i) + "='v'" }) + "/>")
		}},
		{"reader: attributes with namespaces", 60_000, func(n int) func() {
			return read("<a xmlns:p='u'" + many(n, func(i int) string { return " p:a" + strconv.Itoa(i) + "='v'" }) + "/>")
		}},
		{"reader: nested elements", 200_000, func(n int) func() {
			return read(strings.Repeat("<a>", n) + strings.Repeat("</a>", n))
		}},
		{"reader: distinct element names", 100_000, func(n int) func() {
			return read("<r>" + many(n, func(i int) string { return "<e" + strconv.Itoa(i) + "/>" }) + "</r>")
		}},
		{"reader: prefix declared first among 256 in scope", 20_000, func(n int) func() {
			return read(scope + strings.Repeat("<p0:a/>", n) + "</r>")
		}},
		{"reader: entity references", 200_000, func(n int) func() {
			return read("<!DOCTYPE a [<!ENTITY e 'xy'>]><a>" + strings.Repeat("&e;", n) + "</a>")
		}},
		{"reader: text full of brackets and references", 400_000, func(n int) func() {
			return read("<a>" + strings.Repeat("]]&amp;&#10;\r\n", n) + "</a>")
		}},
		{"reader: attribute value full of references", 400_000, func(n int) func() {
			return read("<a x='" + strings.Repeat("&lt;&#9; \r\n", n) + "'/>")
		}},
		{"reader: Skip over a subtree", 200_000, func(n int) func() {
			src := "<r><big>" + strings.Repeat("<a x='1'>t</a>", n) + "</big></r>"
			return func() {
				r := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
				r.Next()
				r.Next()
				if err := r.Skip(); err != nil {
					panic(err)
				}
			}
		}},
		{"writer: attributes of one element", 150_000, func(n int) func() {
			names := make([]string, n)
			for i := range names {
				names[i] = "a" + strconv.Itoa(i)
			}
			return func() {
				w := xmlwriter.New(io.Discard, xmlwriter.Options{Limits: woff})
				w.Start(xmlwriter.Name{Local: "a"})
				for _, name := range names {
					if err := w.Attr(xmlwriter.Name{Local: name}, "v"); err != nil {
						panic(err)
					}
				}
			}
		}},
		{"writer: nested elements", 200_000, func(n int) func() {
			return func() {
				w := xmlwriter.New(io.Discard, xmlwriter.Options{Limits: woff})
				for i := 0; i < n; i++ {
					if err := w.Start(xmlwriter.Name{Local: "a"}); err != nil {
						panic(err)
					}
				}
				for i := 0; i < n; i++ {
					w.End()
				}
			}
		}},
		{"writer: elements with 100 automatic prefixes each", 500, func(n int) func() {
			spaces := make([]string, 100)
			for i := range spaces {
				spaces[i] = "urn:" + strconv.Itoa(i)
			}
			return func() {
				w := xmlwriter.New(io.Discard, xmlwriter.Options{AutoNamespaces: true})
				w.Start(xmlwriter.Name{Local: "r"})
				for e := 0; e < n; e++ {
					w.Start(xmlwriter.Name{Local: "e"})
					for _, space := range spaces {
						if err := w.Attr(xmlwriter.Name{Space: space, Local: "a"}, ""); err != nil {
							panic(err)
						}
					}
					w.End()
				}
			}
		}},
		{"tree: build and write", 100_000, func(n int) func() {
			src := "<r>" + strings.Repeat("<a x='1'>t<b/></a>", n) + "</r>"
			return func() {
				doc, err := xmltree.Parse(strings.NewReader(src), xmltree.Options{MaxNodes: -1})
				if err != nil {
					panic(err)
				}
				w := xmlwriter.New(io.Discard, xmlwriter.Options{})
				if err := errors.Join(doc.WriteTo(w), w.Close()); err != nil {
					panic(err)
				}
			}
		}},
		{"tree: deep document, build, clone and write", 100_000, func(n int) func() {
			src := strings.Repeat("<a>", n) + strings.Repeat("</a>", n)
			return func() {
				doc, err := xmltree.Parse(strings.NewReader(src), xmltree.Options{MaxNodes: -1, Reader: xmlreader.Options{Limits: off}})
				if err != nil {
					panic(err)
				}
				w := xmlwriter.New(io.Discard, xmlwriter.Options{Limits: woff})
				if err := errors.Join(doc.Root().Clone().WriteTo(w), w.Close()); err != nil {
					panic(err)
				}
			}
		}},
		{"tree: children removed one by one", 200_000, func(n int) func() {
			return func() {
				root := xmltree.NewElement(xmltree.Name{Local: "r"})
				for i := 0; i < n; i++ {
					root.Append(xmltree.NewElement(xmltree.Name{Local: "k"}))
				}
				for c := root.FirstChild(); c != nil; c = root.FirstChild() {
					root.Remove(c)
				}
			}
		}},
	}
	best := func(f func()) time.Duration {
		min := time.Duration(1 << 62)
		for i := 0; i < 7; i++ {
			runtime.GC() // odśmiecanie po poprzednim przebiegu nie wchodzi do pomiaru następnego
			start := time.Now()
			f()
			if d := time.Since(start); d < min {
				min = d
			}
		}
		return min
	}
	for _, fam := range families {
		small, large := best(fam.make(fam.size)), best(fam.make(2*fam.size))
		ratio := float64(large) / float64(small)
		if ratio > 2.5 { // jeden pomiar ponad progiem może być szumem maszyny: powtórka rozstrzyga
			small, large = best(fam.make(fam.size)), best(fam.make(2*fam.size))
			ratio = float64(large) / float64(small)
		}
		if ratio > 2.5 {
			t.Errorf("%s: %v for %d, %v for %d: ratio %.2f, want at most 2.5", fam.name, small, fam.size, large, 2*fam.size, ratio)
		}
		if testing.Verbose() {
			t.Logf("%-52s %9.2f ms → %9.2f ms  ×%.2f", fam.name, float64(small.Microseconds())/1000, float64(large.Microseconds())/1000, ratio)
		}
	}
}
