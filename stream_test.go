package goxmlgo

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/iotest"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmltree"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

// tokenDump zapisuje strumień tokenów i błąd końcowy w postaci tekstu (z pozycjami: do porównań między przebiegami
// tego samego wejścia).
func tokenDump(src io.Reader, opts xmlreader.Options) string {
	var b strings.Builder
	r := xmlreader.New(src, opts)
	for {
		t, err := r.Next()
		if err != nil {
			fmt.Fprintf(&b, "end %v\n", err)
			return b.String()
		}
		fmt.Fprintf(&b, "%d %+v %q %v %v %v %d:%d %+v %+v\n", t.Kind, t.Name, t.Data, t.CDATA, t.Whitespace, t.SelfClosing,
			t.Line, t.Column, t.Attrs, t.Namespaces)
	}
}

// passThrough przepuszcza dokument przez czytnik i pisarza; step, jeśli nie jest nil, dostaje każdy token i mówi,
// czy sam go obsłużył (wtedy token nie jest zapisywany).
func passThrough(src io.Reader, dst io.Writer, step func(r *xmlreader.Reader, w *xmlwriter.Writer, t xmlreader.Token) (handled bool, err error)) error {
	r := xmlreader.New(src, xmlreader.Options{})
	w := xmlwriter.New(dst, xmlwriter.Options{})
	d, err := r.Declaration()
	if err != nil {
		return err
	}
	if d.Version != "" {
		if err := w.Declaration(d.Standalone); err != nil {
			return err
		}
	}
	for {
		t, err := r.Next()
		if err == io.EOF {
			return w.Close()
		}
		if err != nil {
			return err
		}
		if step != nil {
			handled, err := step(r, w, t)
			if err != nil {
				return err
			}
			if handled {
				continue
			}
		}
		if err := w.WriteToken(t); err != nil {
			return err
		}
	}
}

func corpus(t testing.TB) map[string][]byte {
	files, _ := filepath.Glob("testdata/corpus/*.xml")
	out := map[string][]byte{}
	for _, f := range files {
		if strings.HasSuffix(f, ".out.xml") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(f)] = data
	}
	if len(out) < 5 {
		t.Fatalf("only %d corpus documents", len(out))
	}
	return out
}

// A filter between a reader and a writer: one element is dropped with Skip, an attribute value and a text are
// changed, and an element is added. The document is never held in memory.
func Example_filter() {
	const src = `<orders><order id="1" state="new"><note>draft</note><internal>cost 7</internal></order><order id="2" state="new"/></orders>`

	r := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	inNote := false
	for {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		switch {
		case t.Kind == xmlreader.KindStartElement && t.Name.Local == "internal":
			if err := r.Skip(); err != nil { // the element and everything in it is passed over
				fmt.Println(err)
				return
			}
			continue
		case t.Kind == xmlreader.KindStartElement && t.Name.Local == "order":
			for i := range t.Attrs {
				if t.Attrs[i].Name.Local == "state" {
					t.Attrs[i].Value = "sent"
				}
			}
		case t.Kind == xmlreader.KindStartElement && t.Name.Local == "note":
			inNote = true
		case t.Kind == xmlreader.KindEndElement && t.Name.Local == "note":
			inNote = false
		case t.Kind == xmlreader.KindText && inNote:
			t.Data = bytes.ToUpper(t.Data)
		case t.Kind == xmlreader.KindEndElement && t.Name.Local == "orders":
			w.Start(xmlwriter.Name{Local: "count"}) // added before the end tag of the root
			w.Text("2")
			w.End()
		}
		if err := w.WriteToken(t); err != nil {
			fmt.Println(err)
			return
		}
	}
	if err := w.Close(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <orders><order id="1" state="sent"><note>DRAFT</note></order><order id="2" state="sent"/><count>2</count></orders>
}

// An island: the document flows from the reader to the writer, and only the element to change is built as a
// tree. The memory needed is that of the largest island, not of the document.
func Example_island() {
	const src = `<list xmlns:p="urn:example:p"><p:item n="1"><p:v>10</p:v></p:item><other/><p:item n="2"><p:v>20</p:v></p:item></list>`

	r := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	for {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		if t.Kind == xmlreader.KindStartElement && t.Name.Local == "item" {
			item, err := xmltree.ReadElement(r, t, xmltree.Options{})
			if err != nil {
				fmt.Println(err)
				return
			}
			v := item.FirstChild()
			sum := xmltree.NewElement(xmltree.Name{Space: "urn:example:p", Local: "double", Prefix: "p"})
			n, _ := strconv.Atoi(v.Text())
			sum.SetText(strconv.Itoa(2 * n))
			item.Append(sum)
			if err := item.WriteTo(w); err != nil {
				fmt.Println(err)
				return
			}
			continue
		}
		if err := w.WriteToken(t); err != nil {
			fmt.Println(err)
			return
		}
	}
	if err := w.Close(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <list xmlns:p="urn:example:p"><p:item n="1"><p:v>10</p:v><p:double>20</p:double></p:item><other/><p:item n="2"><p:v>20</p:v><p:double>40</p:double></p:item></list>
}

// TestFilter: usunięcie elementu przez Skip, zmiana wartości atrybutu i tekstu oraz dopisanie elementów w strumieniu
// dają dokument dobrze uformowany, w którym poza zmianami strumień tokenów jest strumieniem wejścia. Filtr działa
// na każdym dokumencie korpusu: usuwa co trzeci element, a resztę przepuszcza.
// REQ: STR-004
func TestFilter(t *testing.T) {
	var out bytes.Buffer
	src := `<a xmlns:p="urn:p"><drop x="1"><deep>t</deep></drop><p:keep v="old">text<drop/></p:keep><!--c--></a>`
	err := passThrough(strings.NewReader(src), &out, func(r *xmlreader.Reader, w *xmlwriter.Writer, tok xmlreader.Token) (bool, error) {
		switch {
		case tok.Kind == xmlreader.KindStartElement && tok.Name.Local == "drop":
			return true, r.Skip()
		case tok.Kind == xmlreader.KindStartElement && tok.Name.Local == "keep":
			tok.Attrs[0].Value = `new "value" <&>`
			if err := w.WriteToken(tok); err != nil {
				return true, err
			}
			return true, seqErr(w.Start(xmlwriter.Name{Space: "urn:p", Local: "added", Prefix: "p"}), w.Attr(xmlwriter.Name{Local: "k"}, "v"), w.EndEmpty())
		case tok.Kind == xmlreader.KindText:
			return true, w.Text(strings.ToUpper(string(tok.Data)))
		}
		return false, nil
	})
	want := `<a xmlns:p="urn:p"><p:keep v="new &quot;value&quot; &lt;&amp;>"><p:added k="v"/>TEXT</p:keep><!--c--></a>`
	if err != nil || out.String() != want {
		t.Errorf("filter: %v\n got  %s\n want %s", err, out.String(), want)
	}
	if d := tokenDump(bytes.NewReader(out.Bytes()), xmlreader.Options{}); !strings.HasSuffix(d, "end EOF\n") {
		t.Errorf("the output of the filter is not well-formed: %s", d)
	}
	// filtr na korpusie: co trzeci element (poza głównym) znika razem z zawartością; wynik jest dobrze uformowany
	// i ma dokładnie tyle elementów mniej, ile filtr pominął
	for name, data := range corpus(t) {
		count := func(d []byte) (n int) {
			err := passThrough(bytes.NewReader(d), io.Discard, func(_ *xmlreader.Reader, _ *xmlwriter.Writer, tok xmlreader.Token) (bool, error) {
				if tok.Kind == xmlreader.KindStartElement {
					n++
				}
				return false, nil
			})
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			return n
		}
		var out bytes.Buffer
		seen, skips := 0, 0
		err := passThrough(bytes.NewReader(data), &out, func(r *xmlreader.Reader, _ *xmlwriter.Writer, tok xmlreader.Token) (bool, error) {
			if tok.Kind != xmlreader.KindStartElement {
				return false, nil
			}
			if seen++; seen > 1 && seen%3 == 0 {
				skips++
				return true, r.Skip()
			}
			return false, nil
		})
		if err != nil {
			t.Errorf("%s: filter: %v", name, err)
			continue
		}
		// wynik czyta się bez błędu (count) i ma mniej elementów o co najmniej tyle, ile razy filtr wołał Skip
		if got, all := count(out.Bytes()), count(data); got > all-skips {
			t.Errorf("%s: %d elements in, %d out after %d calls of Skip", name, all, got, skips)
		}
	}
}

func seqErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// heapPeak wykonuje f i zwraca szczyt sterty ponad stan sprzed wywołania (próbkowanie co 200 µs).
func heapPeak(f func()) uint64 {
	_, peak := measure(f)
	return peak
}

// TestIsland: element zmaterializowany w środku przelotu, zmieniony i zapisany daje dokument, w którym poza
// zmienionymi poddrzewami strumień tokenów jest strumieniem wejścia, a pamięć zależy od największego poddrzewa,
// nie od rozmiaru dokumentu: dokument cztery razy większy nie podnosi szczytu sterty.
// REQ: STR-005 DET-006
func TestIsland(t *testing.T) {
	island := func(rows int, dst io.Writer, change bool) error {
		return passThrough(testkit.GenJPK(rows), dst, func(r *xmlreader.Reader, w *xmlwriter.Writer, tok xmlreader.Token) (bool, error) {
			if tok.Kind != xmlreader.KindStartElement || tok.Name.Local != "SprzedazWiersz" {
				return false, nil
			}
			row, err := xmltree.ReadElement(r, tok, xmltree.Options{})
			if err != nil {
				return true, err
			}
			if change {
				if err := row.SetAttr(xmltree.Name{Local: "checked"}, "yes"); err != nil {
					return true, err
				}
			}
			return true, row.WriteTo(w)
		})
	}
	// bez zmiany: te same bajty co zwykły przelot
	var plain, same, changed bytes.Buffer
	if err := seqErr(passThrough(testkit.GenJPK(300), &plain, nil), island(300, &same, false), island(300, &changed, true)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plain.Bytes(), same.Bytes()) {
		t.Error("islands that change nothing change the output")
	}
	// ze zmianą: po zdjęciu dopisanego atrybutu strumień jest strumieniem wejścia
	var reverted bytes.Buffer
	err := passThrough(bytes.NewReader(changed.Bytes()), &reverted, func(_ *xmlreader.Reader, w *xmlwriter.Writer, tok xmlreader.Token) (bool, error) {
		if tok.Kind == xmlreader.KindStartElement && tok.Name.Local == "SprzedazWiersz" {
			if len(tok.Attrs) != 1 || tok.Attrs[0].Name.Local != "checked" {
				return true, fmt.Errorf("row without the added attribute: %+v", tok.Attrs)
			}
			tok.Attrs = tok.Attrs[:0]
			return true, w.WriteToken(tok)
		}
		return false, nil
	})
	if err != nil || !bytes.Equal(reverted.Bytes(), plain.Bytes()) {
		t.Errorf("outside the changed attribute the stream differs from the input (%v)", err)
	}
	// pamięć: 20 tys. i 80 tys. wierszy (ok. 8,8 i 35 MB)
	small := heapPeak(func() { err = island(20_000, io.Discard, true) })
	if err != nil {
		t.Fatal(err)
	}
	large := heapPeak(func() { err = island(80_000, io.Discard, true) })
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("peak heap with an island per row: %.1f MB for 20 000 rows, %.1f MB for 80 000 rows", float64(small)/(1<<20), float64(large)/(1<<20))
	if !raceEnabled && (large > 8<<20 || large > 2*small+(1<<20)) {
		t.Errorf("peak heap grows with the document: %d bytes for 20 000 rows, %d for 80 000", small, large)
	}
}

// TestStreamingMemory: przelot czytnik → pisarz działa w pamięci niezależnej od rozmiaru dokumentu. Domyślnie
// dokument ma 100 MB; zmienna GOXMLGO_STREAM_MB ustawia inny rozmiar (1024 dla pomiaru K7: plik 1 GB w < 64 MB
// pamięci procesu). Dokument powstaje w generatorze i nie jest trzymany w pamięci. Szczyt sterty musi być
// poniżej 8 MB; przy rozmiarze podanym przez zmienną sprawdzana jest też pamięć pobrana od systemu przez cały
// proces testu (uruchom wtedy tylko ten test: go test . -run TestStreamingMemory -v).
// REQ: DET-006 STR-003
func TestStreamingMemory(t *testing.T) {
	mb, dedicated := 100, false
	if v := os.Getenv("GOXMLGO_STREAM_MB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("GOXMLGO_STREAM_MB=%q", v)
		}
		mb, dedicated = n, true
	}
	if testing.Short() || (raceEnabled && !dedicated) {
		mb = 8
	}
	rows := mb << 20 / 439 // jeden wiersz GenJPK to ok. 439 bajtów
	var in, out countingWriter
	var err error
	peak := heapPeak(func() {
		err = passThrough(io.TeeReader(testkit.GenJPK(rows), &in), &out, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("pass-through of %d MB (%d rows): %d bytes written, peak heap %.2f MB, memory obtained from the system by the test process %.1f MB",
		in.n>>20, rows, out.n, float64(peak)/(1<<20), float64(m.Sys)/(1<<20))
	if in.n < int64(mb)<<20*9/10 || out.n < in.n*9/10 {
		t.Errorf("the document has %d bytes, the output %d: not the size asked for", in.n, out.n)
	}
	if !raceEnabled && peak > 8<<20 {
		t.Errorf("peak heap %d bytes for a document of %d MB", peak, mb)
	}
	if dedicated && !raceEnabled && m.Sys > 64<<20 {
		t.Errorf("the process obtained %d MB from the system, want under 64 MB (K7)", m.Sys>>20)
	}
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// TestDeterministic: ten sam dokument i te same opcje dają ten sam strumień tokenów i błędów, ten sam zapis i to
// samo drzewo w każdym z 10 przebiegów (mapy w Go mają losową kolejność iteracji, więc zależność od niej wyszłaby
// tutaj), także dla wejść z błędem; wynik nie zależy od podziału wejścia na odczyty: po jednym bajcie, po 7
// bajtów i w całości.
// REQ: DET-001 DET-003
func TestDeterministic(t *testing.T) {
	inputs := corpus(t)
	inputs["error: mismatched tag"] = []byte("<a>\n <b x='1'></c></a>")
	inputs["error: undeclared prefix"] = []byte("<a><p:b/></a>")
	inputs["many attributes"] = []byte("<a" + strings.Repeat(" ", 1) + manyAttrs(200) + "/>")
	type result struct{ tokens, written, tree string }
	run := func(data []byte, wrap func(io.Reader) io.Reader) result {
		var res result
		res.tokens = tokenDump(wrap(bytes.NewReader(data)), xmlreader.Options{})
		var out bytes.Buffer
		if err := passThrough(wrap(bytes.NewReader(data)), &out, nil); err != nil {
			res.written = "error: " + err.Error()
		} else {
			res.written = out.String()
		}
		doc, err := xmltree.Parse(wrap(bytes.NewReader(data)), xmltree.Options{IDs: true})
		if err != nil {
			res.tree = "error: " + err.Error()
			return res
		}
		out.Reset()
		w := xmlwriter.New(&out, xmlwriter.Options{AutoNamespaces: true, Indent: " "})
		if err := seqErr(doc.WriteTo(w), w.Close()); err != nil {
			res.tree = "error: " + err.Error()
		} else {
			res.tree = out.String() + fmt.Sprint(doc.Root().InScopeNamespaces(), doc.IDs())
		}
		return res
	}
	whole := func(r io.Reader) io.Reader { return r }
	for name, data := range inputs {
		first := run(data, whole)
		for i := 1; i < 10; i++ {
			if again := run(data, whole); again != first {
				t.Errorf("%s: run %d differs from the first run", name, i+1)
				break
			}
		}
		for how, wrap := range map[string]func(io.Reader) io.Reader{
			"one byte at a time": iotest.OneByteReader,
			"7 bytes at a time":  func(r io.Reader) io.Reader { return &chunked{r: r, n: 7} },
			"half reads":         iotest.HalfReader,
			"data with errors":   iotest.DataErrReader,
		} {
			if got := run(data, wrap); got != first {
				t.Errorf("%s, input read %s: the result differs from reading it whole", name, how)
			}
		}
	}
}

func manyAttrs(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, " a%d='%d'", i, i)
	}
	return b.String()
}

type chunked struct {
	r io.Reader
	n int
}

func (c *chunked) Read(p []byte) (int, error) {
	if len(p) > c.n {
		p = p[:c.n]
	}
	return c.r.Read(p)
}

// TestParallelUse: różne egzemplarze czytnika, pisarza i drzewa wolno używać równolegle: pakiety nie mają stanu
// globalnego zmienianego po inicjalizacji. Osiem gorutyn przetwarza te same dokumenty; każda dostaje ten sam
// wynik, a detektor wyścigów (go test -race) niczego nie zgłasza.
// REQ: DET-005
func TestParallelUse(t *testing.T) {
	inputs := corpus(t)
	work := func(data []byte) string {
		var out bytes.Buffer
		if err := passThrough(bytes.NewReader(data), &out, nil); err != nil {
			return "error: " + err.Error()
		}
		doc, err := xmltree.Parse(bytes.NewReader(data), xmltree.Options{})
		if err != nil {
			return "error: " + err.Error()
		}
		w := xmlwriter.New(&out, xmlwriter.Options{AutoNamespaces: true})
		if err := seqErr(doc.Root().Clone().WriteTo(w), w.Close()); err != nil {
			return "error: " + err.Error()
		}
		return out.String() + tokenDump(bytes.NewReader(data), xmlreader.Options{})
	}
	want := map[string]string{}
	for name, data := range inputs {
		want[name] = work(data)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < 3; round++ {
				for name, data := range inputs {
					if got := work(data); got != want[name] {
						t.Errorf("%s: a goroutine got another result", name)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}
