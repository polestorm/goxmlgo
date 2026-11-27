package xmlreader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/polestorm/goxmlgo/internal/testkit"
)

// Testy rozszerzeń czytnika z etapu 2 (wymagania XML-019…029).

// testDocs zwraca dokumenty do testów równoważności: korpus i, gdy pobrany, przypadki W3C w zakresie (dobrze
// i źle uformowane) z opcjami, z jakimi czyta je TestXMLConformance.
type testDoc struct {
	id   string
	data []byte
	opts Options
}

func testDocs(t testing.TB) []testDoc {
	var docs []testDoc
	files, _ := filepath.Glob(filepath.Join(corpusDir, "*.xml"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, testDoc{id: filepath.Base(f), data: b})
	}
	for _, s := range []string{
		`<a><b x="1" x="2"/></a>`,
		`<a><b><c></b></a>`,
		`<a><p:b/></a>`,
		`<a><b xmlns:p="u" p:x="1" xmlns:q="u" q:x="2"/></a>`,
		`<!DOCTYPE a [<!ENTITY e "<x>t</x>"><!ENTITY bad "<x>">]><a><b>&e;&e;</b><c>&bad;</c></a>`,
		`<a><b>` + strings.Repeat("<c>", 600) + `</b></a>`,
		"<a><b>\x01</b></a>",
		`<a><b>text`,
		`<a><b y="&undefined;"/></a>`,
		`<a><b y="<"/></a>`,
	} {
		docs = append(docs, testDoc{id: fmt.Sprintf("inline-%d", len(docs)), data: []byte(s)})
	}
	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err == nil {
		for _, ct := range loadConformance(t.(*testing.T)) {
			if ct.Excluded != "" {
				continue
			}
			b, err := os.ReadFile(path.Join(xmlconfRoot, ct.Dir, ct.URI))
			if err != nil {
				t.Fatal(err)
			}
			docs = append(docs, testDoc{id: ct.Group + "/" + ct.ID, data: b,
				opts: Options{NoNamespaces: !strings.HasPrefix(ct.Recommendation, "NS")}})
		}
	}
	return docs
}

func describe(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return fmt.Sprintf("%s %d:%d", e.Code, e.Line, e.Column)
	}
	return fmt.Sprint(err)
}

// outcome czyta dokument do końca i zwraca liczbę tokenów oraz opis końca ("EOF" albo kod i miejsce błędu).
func outcome(r *Reader) (int, string) {
	for n := 0; ; n++ {
		if _, err := r.Next(); err != nil {
			return n, describe(err)
		}
	}
}

// TestSkip: Skip po początku elementu pomija resztę elementu, także pustego; w innym stanie zwraca usage-skip
// i niczego nie zmienia.
// REQ: XML-019
func TestSkip(t *testing.T) {
	const doc = `<a><skip x="1"><in>t<![CDATA[c]]><!--c--><?p d?><deep><e/></deep></in></skip><empty/><b>after</b></a>`
	r := New(strings.NewReader(doc), Options{})
	if err := r.Skip(); describe(err) != "usage-skip 1:1" {
		t.Fatalf("Skip before the first token: %v", err)
	}
	var got []string
	for {
		tok, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch tok.Kind {
		case KindStartElement:
			got = append(got, "<"+tok.Name.Local)
			if tok.Name.Local == "skip" || tok.Name.Local == "empty" {
				if err := r.Skip(); err != nil {
					t.Fatal(err)
				}
				// drugi Skip z rzędu nie następuje po początku elementu
				if err := r.Skip(); describe(err)[:10] != "usage-skip" {
					t.Fatalf("second Skip: %v", err)
				}
			}
		case KindEndElement:
			got = append(got, ">"+tok.Name.Local)
			if err := r.Skip(); describe(err)[:10] != "usage-skip" {
				t.Fatalf("Skip after an end tag: %v", err)
			}
		case KindText:
			got = append(got, string(tok.Data))
		}
	}
	if want := "<a <skip <empty <b after >b >a"; strings.Join(got, " ") != want {
		t.Errorf("tokens %q, want %q", strings.Join(got, " "), want)
	}
	if err := r.Skip(); err != io.EOF {
		t.Errorf("Skip after the end of the document: %v, want io.EOF", err)
	}

	// Skip korzenia: następny Next to koniec dokumentu albo to, co stoi za korzeniem.
	r = New(strings.NewReader(`<a><b/></a><!--tail-->`), Options{})
	r.Next()
	if err := r.Skip(); err != nil {
		t.Fatal(err)
	}
	if tok, err := r.Next(); err != nil || tok.Kind != KindComment {
		t.Errorf("after skipping the root: %v %v", tok.Kind, err)
	}

	// Zakres przestrzeni nazw po Skip jest zakresem rodzica.
	r = New(strings.NewReader(`<a xmlns:p="outer"><b xmlns:p="inner" xmlns:q="q"><p:c/></b><p:d/></a>`), Options{})
	r.Next()
	r.Next()
	if err := r.Skip(); err != nil {
		t.Fatal(err)
	}
	if uri, _ := r.LookupNamespace("p"); uri != "outer" {
		t.Errorf("after Skip p is bound to %q", uri)
	}
	if _, ok := r.LookupNamespace("q"); ok {
		t.Error("after Skip q is still bound")
	}
	if tok, _ := r.Next(); tok.Name.Space != "outer" || tok.Name.Local != "d" {
		t.Errorf("token after Skip: %+v", tok.Name)
	}
}

// TestSkipEquivalent: dla każdego dokumentu i każdego elementu, na którym wywołano Skip, wynik jest taki, jak przy
// czytaniu token po tokenie: te same tokeny poza pominiętymi i ten sam koniec (EOF albo ten sam błąd w tym samym
// miejscu, także gdy błąd leży w pomijanej treści).
// REQ: XML-020
func TestSkipEquivalent(t *testing.T) {
	skips := 0
	for _, d := range testDocs(t) {
		// Przebieg wzorcowy: zrzut każdego tokenu i głębokość, na której stoi.
		type item struct {
			dump  string
			depth int
			start bool
		}
		var ref []item
		r := New(bytes.NewReader(d.data), d.opts)
		depth := 0
		var end string
		for {
			tok, err := r.Next()
			if err != nil {
				end = describe(err)
				break
			}
			it := item{dump: fmt.Sprintf("%d %+v %q %d:%d", tok.Kind, tok.Name, tok.Data, tok.Line, tok.Column), depth: depth}
			switch tok.Kind {
			case KindStartElement:
				it.start = true
				depth++
			case KindEndElement:
				depth--
				it.depth = depth
			}
			ref = append(ref, it)
		}
		// Dla wybranych początków elementów: czytaj do niego, Skip, czytaj dalej; porównaj z wzorcem bez środka.
		starts := 0
		for i, it := range ref {
			if !it.start {
				continue
			}
			starts++
			if starts > 6 && starts%17 != 0 {
				continue // w dużych dokumentach próbka
			}
			skips++
			r := New(bytes.NewReader(d.data), d.opts)
			for j := 0; j <= i; j++ {
				if _, err := r.Next(); err != nil {
					t.Fatalf("%s: %v", d.id, err)
				}
			}
			// koniec pomijanego elementu we wzorcu: pierwszy token końca na tej samej głębokości
			k := i + 1
			for k < len(ref) && !(ref[k].depth == it.depth && !ref[k].start) {
				k++
			}
			err := r.Skip()
			if k >= len(ref) {
				// element nie kończy się we wzorcu: błąd leży w pomijanej treści
				if describe(err) != end {
					t.Errorf("%s: Skip at token %d: %s, reading gives %s", d.id, i, describe(err), end)
				}
				if _, again := r.Next(); again != err {
					t.Errorf("%s: Next after a failed Skip: %v, want the same error", d.id, again)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s: Skip at token %d: %v", d.id, i, err)
				continue
			}
			for j := k + 1; ; j++ {
				tok, err := r.Next()
				if err != nil {
					if j != len(ref) || describe(err) != end {
						t.Errorf("%s: after Skip at token %d the stream ends at %d with %s, want %d with %s", d.id, i, j, describe(err), len(ref), end)
					}
					break
				}
				if j >= len(ref) {
					t.Errorf("%s: after Skip at token %d: more tokens than reading gives", d.id, i)
					break
				}
				if got := fmt.Sprintf("%d %+v %q %d:%d", tok.Kind, tok.Name, tok.Data, tok.Line, tok.Column); got != ref[j].dump {
					t.Errorf("%s: after Skip at token %d, token %d: %s, want %s", d.id, i, j, got, ref[j].dump)
					break
				}
			}
		}
	}
	t.Logf("%d Skip calls compared with reading", skips)
}

// TestSkipAllocations: Skip nie alokuje zależnie od liczby pomijanych elementów ani atrybutów.
// REQ: XML-021
func TestSkipAllocations(t *testing.T) {
	doc := func(n int) []byte {
		var b bytes.Buffer
		b.WriteString(`<a xmlns:p="urn:p"><skip>`)
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, `<p:row id="r%d" p:kind="k%d">tekst %d<cell a="%d"/></p:row>`, i, i, i, i)
		}
		b.WriteString(`</skip></a>`)
		return b.Bytes()
	}
	measure := func(n int) float64 {
		data := doc(n)
		return testing.AllocsPerRun(5, func() {
			r := New(bytes.NewReader(data), Options{})
			r.Next()
			r.Next()
			if err := r.Skip(); err != nil {
				t.Fatal(err)
			}
			if tok, err := r.Next(); err != nil || tok.Kind != KindEndElement {
				t.Fatal(tok.Kind, err)
			}
		})
	}
	small, large := measure(1000), measure(10000)
	if large > small+8 {
		t.Errorf("allocations grow with the skipped content: %v for 1000 rows, %v for 10000", small, large)
	}
	// dla porównania: czytanie tego samego tworzy napis dla każdej wartości atrybutu
	data := doc(10000)
	reading := testing.AllocsPerRun(2, func() { outcome(New(bytes.NewReader(data), Options{})) })
	t.Logf("allocations: Skip %v (1000 rows), %v (10000 rows); reading 10000 rows %v", small, large, reading)
}

// TestReset: po Reset strumień tokenów i błędów jest taki sam jak z nowego czytnika, niezależnie od tego, co
// czytnik czytał wcześniej, czy skończył, czy został przerwany w środku i z jakimi opcjami pracował.
// REQ: XML-022
func TestReset(t *testing.T) {
	docs := testDocs(t)
	reused := New(strings.NewReader(""), Options{})
	var zero Reader
	for i, d := range docs {
		want := dumpTokensOpts(bytes.NewReader(d.data), d.opts)
		for name, r := range map[string]*Reader{"reused": reused, "zero": &zero} {
			r.Reset(bytes.NewReader(d.data), d.opts)
			if got := dumpReader(r); got != want {
				t.Fatalf("%s (%s reader, after %d documents): stream differs from a new reader\n%s", d.id, name, i, firstDiff(got, want))
			}
		}
		// co trzeci dokument zostawiamy przerwany w środku, żeby następny Reset zaczynał z niedokończonego stanu
		if i%3 == 0 {
			reused.Reset(bytes.NewReader(d.data), d.opts)
			for j := 0; j < 3; j++ {
				reused.Next()
			}
		}
	}
	// Deklaracja XML i limity pochodzą z nowych opcji, nie z poprzedniego dokumentu.
	reused.Reset(strings.NewReader(`<?xml version="1.0" standalone="yes"?><a><b/></a>`), Options{Limits: Limits{MaxElements: 1}})
	if _, end := outcome(reused); end != "limit-elements 1:42" {
		t.Errorf("limit of the new options: %s", end)
	}
	reused.Reset(strings.NewReader(`<a><b/></a>`), Options{})
	if d, err := reused.Declaration(); err != nil || d.Version != "" || d.Standalone != "" {
		t.Errorf("declaration left over from the previous document: %+v %v", d, err)
	}
	if _, end := outcome(reused); end != "EOF" {
		t.Errorf("limit left over from the previous options: %s", end)
	}
}

// dumpReader to dumpTokens dla istniejącego czytnika.
func dumpReader(r *Reader) string {
	saved := newReaderForDump
	newReaderForDump = func(io.Reader, Options) *Reader { return r }
	defer func() { newReaderForDump = saved }()
	return dumpTokensOpts(nil, Options{})
}

// BenchmarkSmallDocuments: wiele małych dokumentów nowym czytnikiem i jednym czytnikiem z Reset.
func BenchmarkSmallDocuments(b *testing.B) {
	doc, err := os.ReadFile(filepath.Join(corpusDir, "style-wordml.xml"))
	if err != nil {
		b.Fatal(err)
	}
	b.Run("New", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(doc)))
		for i := 0; i < b.N; i++ {
			if _, end := outcome(New(bytes.NewReader(doc), Options{})); end != "EOF" {
				b.Fatal(end)
			}
		}
	})
	b.Run("Reset", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(doc)))
		r := New(bytes.NewReader(doc), Options{})
		src := bytes.NewReader(doc)
		for i := 0; i < b.N; i++ {
			src.Reset(doc)
			r.Reset(src, Options{})
			if _, end := outcome(r); end != "EOF" {
				b.Fatal(end)
			}
		}
	})
}

// TestMaxElements: limit łącznej liczby elementów, domyślnie wyłączony; elementy pomijane przez Skip są liczone.
// REQ: XML-023
func TestMaxElements(t *testing.T) {
	const doc = `<a><b/><c><d/></c>` + "\n" + `<e/></a>`
	for _, c := range []struct {
		max  int
		want string
	}{
		{0, "EOF"}, {-1, "EOF"}, {5, "EOF"}, {4, "limit-elements 2:1"}, {3, "limit-elements 1:11"}, {1, "limit-elements 1:4"},
	} {
		if _, end := outcome(New(strings.NewReader(doc), Options{Limits: Limits{MaxElements: c.max}})); end != c.want {
			t.Errorf("MaxElements %d: %s, want %s", c.max, end, c.want)
		}
	}
	if DefaultLimits().MaxElements != 0 {
		t.Error("the default is no limit on elements")
	}
	// Skip liczy elementy pomijane: <c> i <d> zużywają limit, <e> go przekracza.
	r := New(strings.NewReader(doc), Options{Limits: Limits{MaxElements: 4}})
	for {
		tok, err := r.Next()
		if err != nil {
			if describe(err) != "limit-elements 2:1" {
				t.Errorf("with Skip: %s", describe(err))
			}
			break
		}
		if tok.Kind == KindStartElement && tok.Name.Local == "c" {
			if err := r.Skip(); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Limit przekroczony w pomijanej treści jest błędem Skip.
	r = New(strings.NewReader(doc), Options{Limits: Limits{MaxElements: 3}})
	r.Next()
	r.Next()
	r.Next()
	r.Next() // <c>
	if err := r.Skip(); describe(err) != "limit-elements 1:11" {
		t.Errorf("limit inside skipped content: %s", describe(err))
	}
}

// TestTokenReader: *Reader spełnia TokenReader; EOF po ostatnim tokenie, po błędzie stale ten sam błąd.
// REQ: XML-024
func TestTokenReader(t *testing.T) {
	var src TokenReader = New(strings.NewReader(`<a/>`), Options{})
	n := 0
	for {
		_, err := src.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if _, err := src.Next(); n != 2 || err != io.EOF {
		t.Errorf("%d tokens, then %v", n, err)
	}
	src = New(strings.NewReader(`<a><b></a>`), Options{})
	var first error
	for first == nil {
		_, first = src.Next()
	}
	for i := 0; i < 3; i++ {
		if _, err := src.Next(); err != first {
			t.Errorf("after an error Next returns %v, want the same error %v", err, first)
		}
	}
	if err := src.(*Reader).Skip(); err != first {
		t.Errorf("after an error Skip returns %v", err)
	}
}

// TestTokenNamespaces: token początku elementu niesie deklaracje przestrzeni nazw elementu w kolejności
// z dokumentu, także z wartości domyślnych ATTLIST; to te same deklaracje, które zwraca DeclaredNamespaces.
// REQ: XML-025
func TestTokenNamespaces(t *testing.T) {
	const doc = `<!DOCTYPE a [<!ATTLIST b xmlns:d CDATA "urn:default" xmlns CDATA "urn:defns">]>
<a xmlns:z="urn:z" xmlns="urn:a" xmlns:y="urn:y" x="1"><b xmlns:q="urn:q"/><c xmlns=""/><z:d/></a>`
	want := map[string]string{
		"a": "z=urn:z =urn:a y=urn:y",
		"b": "q=urn:q d=urn:default =urn:defns",
		"c": "=",
		"d": "",
	}
	r := New(strings.NewReader(doc), Options{})
	seen := 0
	for {
		tok, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if tok.Kind != KindStartElement {
			if tok.Namespaces != nil {
				t.Errorf("token of kind %d has namespaces", tok.Kind)
			}
			continue
		}
		seen++
		var parts []string
		for _, b := range tok.Namespaces {
			parts = append(parts, b.Prefix+"="+b.URI)
		}
		if got := strings.Join(parts, " "); got != want[tok.Name.Local] {
			t.Errorf("<%s>: %q, want %q", tok.Name.Local, got, want[tok.Name.Local])
		}
		declared := r.DeclaredNamespaces()
		if len(declared) != len(tok.Namespaces) {
			t.Errorf("<%s>: DeclaredNamespaces has %d, the token %d", tok.Name.Local, len(declared), len(tok.Namespaces))
		}
		for i := range declared {
			if declared[i] != tok.Namespaces[i] {
				t.Errorf("<%s>: declaration %d differs", tok.Name.Local, i)
			}
		}
		if tok.Name.Local == "d" && tok.Namespaces != nil {
			t.Error("an element without declarations has a non-nil slice")
		}
	}
	if seen != 4 {
		t.Fatalf("%d elements", seen)
	}
	// Bez przetwarzania przestrzeni nazw deklaracje są zwykłymi atrybutami.
	r = New(strings.NewReader(`<a xmlns:p="u"/>`), Options{NoNamespaces: true})
	if tok, _ := r.Next(); tok.Namespaces != nil || len(tok.Attrs) != 1 {
		t.Errorf("NoNamespaces: %+v", tok)
	}
	// Bez alokacji na token: bufor deklaracji jest używany ponownie.
	data := []byte(`<a xmlns:p="u">` + strings.Repeat(`<p:b xmlns:q="v" xmlns="w"/>`, 2000) + `</a>`)
	small := testing.AllocsPerRun(5, func() { outcome(New(bytes.NewReader(data[:len(data)/2+3]), Options{})) })
	large := testing.AllocsPerRun(5, func() { outcome(New(bytes.NewReader(data), Options{})) })
	if large > small+8 {
		t.Errorf("allocations grow with the number of declarations: %v, %v", small, large)
	}
}

// TestNamespaceScope: typ śledzący zakres, zasilany tokenami, odpowiada dla każdego tokenu tak samo jak
// Reader.LookupNamespace w tym samym miejscu dokumentu.
// REQ: XML-026
func TestNamespaceScope(t *testing.T) {
	prefixes := []string{"", "xml", "xmlns", "a", "b", "p", "q", "x", "z", "w", "mc", "w14", "r", "tns", "etd", "xsi", "xlink", "undeclared"}
	checked := 0
	for _, d := range testDocs(t) {
		if d.opts.NoNamespaces {
			continue
		}
		r := New(bytes.NewReader(d.data), d.opts)
		var scope NamespaceScope
		for {
			tok, err := r.Next()
			if err != nil {
				break
			}
			scope.Update(tok)
			for _, b := range tok.Namespaces {
				prefixes = appendUnique(prefixes, b.Prefix)
			}
			for _, p := range prefixes {
				wantURI, wantOK := r.LookupNamespace(p)
				gotURI, gotOK := scope.Lookup(p)
				if gotURI != wantURI || gotOK != wantOK {
					t.Fatalf("%s at %d:%d, prefix %q: scope gives %q %v, the reader %q %v", d.id, tok.Line, tok.Column, p, gotURI, gotOK, wantURI, wantOK)
				}
				checked++
			}
		}
		prefixes = prefixes[:18]
	}
	t.Logf("%d lookups compared", checked)

	var s NamespaceScope
	s.Pop() // bez otwartego zakresu: nic
	decls := []Binding{{Prefix: "p", URI: "one"}}
	s.Push(decls)
	decls[0].URI = "changed" // zakres trzyma kopię
	s.Push(nil)
	if uri, ok := s.Lookup("p"); uri != "one" || !ok || s.Depth() != 2 {
		t.Errorf("Lookup %q %v, depth %d", uri, ok, s.Depth())
	}
	s.Reset()
	if _, ok := s.Lookup("p"); ok || s.Depth() != 0 {
		t.Error("Reset left a binding")
	}
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// TestDeclaration: treść deklaracji XML i kodowanie wejścia, przed pierwszym Next i po nim.
// REQ: XML-027
func TestDeclaration(t *testing.T) {
	utf16 := func(s string, be bool) string {
		var b []byte
		if be {
			b = append(b, 0xFE, 0xFF)
		} else {
			b = append(b, 0xFF, 0xFE)
		}
		for _, c := range s {
			if be {
				b = append(b, byte(c>>8), byte(c))
			} else {
				b = append(b, byte(c), byte(c>>8))
			}
		}
		return string(b)
	}
	for _, c := range []struct {
		doc  string
		want Declaration
	}{
		{`<a/>`, Declaration{InputEncoding: "UTF-8"}},
		{"\xef\xbb\xbf<a/>", Declaration{InputEncoding: "UTF-8", BOM: true}},
		{`<?xml version="1.0"?><a/>`, Declaration{Version: "1.0", InputEncoding: "UTF-8"}},
		{`<?xml version='1.1' encoding='utf-8' standalone='no'?><a/>`, Declaration{Version: "1.1", Encoding: "utf-8", Standalone: "no", InputEncoding: "UTF-8"}},
		{`<?xml version="1.0" standalone="yes"?><a/>`, Declaration{Version: "1.0", Standalone: "yes", InputEncoding: "UTF-8"}},
		{"<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><a>\xe9</a>", Declaration{Version: "1.0", Encoding: "ISO-8859-1", InputEncoding: "ISO-8859-1"}},
		{`<?xml version="1.0" encoding="US-ASCII"?><a/>`, Declaration{Version: "1.0", Encoding: "US-ASCII", InputEncoding: "US-ASCII"}},
		{utf16(`<?xml version="1.0" encoding="UTF-16"?><a/>`, false), Declaration{Version: "1.0", Encoding: "UTF-16", InputEncoding: "UTF-16LE", BOM: true}},
		{utf16(`<a/>`, true), Declaration{InputEncoding: "UTF-16BE", BOM: true}},
	} {
		// przed pierwszym Next
		r := New(strings.NewReader(c.doc), Options{})
		got, err := r.Declaration()
		if err != nil || got != c.want {
			t.Errorf("%q before Next: %+v %v, want %+v", c.doc, got, err, c.want)
		}
		// wywołanie przed Next nie zmienia strumienia tokenów
		if a, b := dumpReader(r), dumpTokens(strings.NewReader(c.doc)); a != b {
			t.Errorf("%q: Declaration before Next changes the stream", c.doc)
		}
		// po przeczytaniu dokumentu
		r = New(iotest.OneByteReader(strings.NewReader(c.doc)), Options{})
		outcome(r)
		if got, err := r.Declaration(); err != nil || got != c.want {
			t.Errorf("%q after reading: %+v %v", c.doc, got, err)
		}
	}
	// Błąd deklaracji albo kodowania wraca z Declaration i potem z każdego Next.
	for doc, want := range map[string]string{
		`<?xml version="2.0"?><a/>`:                   "xml-decl 1:20",
		`<?xml version="1.0" encoding="KOI8-R"?><a/>`: "xml-encoding-unsupported 1:38",
		"<\x00a\x00/\x00>\x00":                        "xml-encoding-invalid 1:1",
	} {
		r := New(strings.NewReader(doc), Options{})
		_, err := r.Declaration()
		_, fromNext := New(strings.NewReader(doc), Options{}).Next()
		if describe(err) != want || describe(fromNext) != want {
			t.Errorf("%q: Declaration gives %s, Next alone %s, want %s", doc, describe(err), describe(fromNext), want)
		}
		if _, again := r.Next(); again != err {
			t.Errorf("%q: Next after a failed Declaration: %v", doc, again)
		}
		if _, again := r.Declaration(); again != err {
			t.Errorf("%q: second Declaration: %v", doc, again)
		}
	}
}

// TestStringsStayValid: napisy z tokenów (nazwy, wartości i typy atrybutów, deklaracje przestrzeni nazw)
// zatrzymane w trakcie czytania dużego dokumentu są po jego zakończeniu takie same, a wycinki Data, Attrs
// i Namespaces wolno zatrzymać po skopiowaniu.
// REQ: XML-028
func TestStringsStayValid(t *testing.T) {
	const rows = 30000 // ok. 13 MB: bufory czytnika są wielokrotnie nadpisywane
	type kept struct {
		row         int
		names       []Name
		values      []string
		bindings    []Binding
		text        string
		wantText    string
		wantVal     string
		wantBinding string
	}
	var all []kept
	var b bytes.Buffer
	b.WriteString(`<!DOCTYPE t [<!ATTLIST row kind (x|y) "x">]><t>`)
	for i := 0; i < rows; i++ {
		fmt.Fprintf(&b, `<row id="id-%d" xmlns:n%d="urn:ns:%d"><n%d:cell v="val-%d">text-%d</n%d:cell></row>`, i, i%5000, i, i%5000, i, i, i%5000)
	}
	b.WriteString(`</t>`)
	r := New(&b, Options{})
	row := -1
	var cur *kept
	for {
		tok, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case tok.Kind == KindStartElement && tok.Name.Local == "row":
			row++
			all = append(all, kept{row: row})
			cur = &all[len(all)-1]
			cur.names = append(cur.names, tok.Name, tok.Attrs[0].Name, tok.Attrs[1].Name)
			cur.values = append(cur.values, tok.Attrs[0].Value, tok.Attrs[1].Value, tok.Attrs[1].Type)
			cur.bindings = append(cur.bindings, tok.Namespaces...)
		case tok.Kind == KindStartElement && tok.Name.Local == "cell":
			cur.names = append(cur.names, tok.Name)
			cur.values = append(cur.values, tok.Attrs[0].Value)
		case tok.Kind == KindText:
			cur.text = string(tok.Data)
		}
	}
	if len(all) != rows {
		t.Fatalf("%d rows", len(all))
	}
	for _, k := range all {
		i := k.row
		p := fmt.Sprintf("n%d", i%5000)
		ok := k.names[0] == (Name{Local: "row"}) && k.names[1] == (Name{Local: "id"}) && k.names[2] == (Name{Local: "kind"}) &&
			k.names[3] == (Name{Space: fmt.Sprintf("urn:ns:%d", i), Local: "cell", Prefix: p}) &&
			k.values[0] == fmt.Sprintf("id-%d", i) && k.values[1] == "x" && k.values[2] == AttrTypeEnumeration &&
			k.values[3] == fmt.Sprintf("val-%d", i) &&
			len(k.bindings) == 1 && k.bindings[0] == (Binding{Prefix: p, URI: fmt.Sprintf("urn:ns:%d", i)}) &&
			k.text == fmt.Sprintf("text-%d", i)
		if !ok {
			t.Fatalf("row %d: kept values changed: %+v", i, k)
		}
	}
}

var _ = testkit.FetchHint

// TestInScopeNamespaces: InScopeNamespaces daje po jednym wiązaniu na związany prefiks, zgodnie z LookupNamespace
// w tym samym miejscu (po tokenie początku: ten element, po tokenie końca: jego rodzic), posortowane według
// prefiksu, bez prefiksu xml i bez przestrzeni domyślnej zniesionej przez xmlns=""; wynik jest kopią.
// REQ: XML-030
func TestInScopeNamespaces(t *testing.T) {
	src := `<a xmlns="urn:d" xmlns:q="urn:q" xmlns:p="urn:p1"><p:b xmlns:p="urn:p2"><c xmlns=""><d xmlns:r="urn:r"/></c></p:b><e/></a>`
	want := []string{
		"start a: =urn:d p=urn:p1 q=urn:q",
		"start b: =urn:d p=urn:p2 q=urn:q",
		"start c: p=urn:p2 q=urn:q",
		"start d: p=urn:p2 q=urn:q r=urn:r",
		"end d: p=urn:p2 q=urn:q",
		"end c: =urn:d p=urn:p2 q=urn:q",
		"end b: =urn:d p=urn:p1 q=urn:q",
		"start e: =urn:d p=urn:p1 q=urn:q",
		"end e: =urn:d p=urn:p1 q=urn:q",
		"end a:",
	}
	r := New(strings.NewReader(src), Options{})
	if got := r.InScopeNamespaces(); len(got) != 0 {
		t.Errorf("before the first token: %v", got)
	}
	var got []string
	var kept []Binding
	for {
		tok, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		in := r.InScopeNamespaces()
		line := map[Kind]string{KindStartElement: "start ", KindEndElement: "end "}[tok.Kind] + tok.Name.Local + ":"
		for _, b := range in {
			line += " " + b.Prefix + "=" + b.URI
			if uri, ok := r.LookupNamespace(b.Prefix); !ok || uri != b.URI {
				t.Errorf("%s: %q is bound to %q in the result, LookupNamespace says %q, %v", line, b.Prefix, b.URI, uri, ok)
			}
		}
		for _, p := range []string{"", "p", "q", "r"} {
			if _, ok := r.LookupNamespace(p); ok != strings.Contains(line, " "+p+"=") {
				t.Errorf("%s: prefix %q bound: %v", line, p, ok)
			}
		}
		got = append(got, line)
		if kept == nil {
			kept = in
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(kept) != 3 || kept[0] != (Binding{URI: "urn:d"}) || kept[2] != (Binding{Prefix: "q", URI: "urn:q"}) {
		t.Errorf("the result of the first call changed: %v", kept)
	}
	nr := New(strings.NewReader(`<a xmlns="urn:d" xmlns:p="urn:p"/>`), Options{NoNamespaces: true})
	nr.Next()
	if got := nr.InScopeNamespaces(); len(got) != 0 {
		t.Errorf("without namespace processing: %v", got)
	}
}
