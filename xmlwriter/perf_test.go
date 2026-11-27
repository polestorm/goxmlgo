package xmlwriter

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlreader"
)

// passThrough to przelot czytnik → pisarz na gotowych egzemplarzach (Reset), bez deklaracji XML.
func passThrough(r *xmlreader.Reader, w *Writer, src io.Reader, out io.Writer) error {
	r.Reset(src, xmlreader.Options{})
	w.Reset(out, Options{})
	for {
		t, err := r.Next()
		if err == io.EOF {
			return w.Close()
		}
		if err != nil {
			return err
		}
		if err := w.WriteToken(t); err != nil {
			return err
		}
	}
}

// TestWriterAllocations: po rozgrzaniu pisarz nie alokuje pamięci na wywołanie (dwa rozmiary dokumentu różniące
// się 10×), a przelot czytnik → pisarz nie alokuje więcej niż sam czytnik na tym samym dokumencie.
// REQ: WR-015 STR-003 DET-007
func TestWriterAllocations(t *testing.T) {
	// wywołania: dokument z przestrzeniami nazw, atrybutami, tekstem do escapowania, komentarzem, PI i CDATA
	rows := func(w *Writer, k int) {
		w.Reset(io.Discard, Options{})
		ns := "urn:example:doc"
		w.Declaration("")
		w.Start(Name{Space: ns, Local: "doc", Prefix: "d"}, Binding{Prefix: "d", URI: ns})
		for i := 0; i < k; i++ {
			w.Start(Name{Space: ns, Local: "row", Prefix: "d"})
			w.Attr(Name{Local: "id"}, "identyfikator")
			w.Attr(Name{Space: ns, Local: "kind", Prefix: "d"}, "a<b&c\"d\n")
			w.Start(Name{Local: "cell"}, Binding{Prefix: "", URI: ""})
			w.Text("zażółć gęślą jaźń & <reszta>\r\n")
			w.End()
			w.Start(Name{Local: "empty"})
			w.EndEmpty()
			w.Comment("komentarz")
			w.ProcInst("target", "dane")
			w.CDATA("<surowe>")
			w.End()
		}
		w.End()
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
	}
	w := New(io.Discard, Options{})
	rows(w, 10) // rozgrzanie: stos elementów, tablica deklaracji
	for _, k := range []int{100, 1000} {
		if a := testing.AllocsPerRun(5, func() { rows(w, k) }); a != 0 {
			t.Errorf("calls, %d rows: %v allocations per document, want 0", k, a)
		}
	}
	auto := New(io.Discard, Options{AutoNamespaces: true})
	gen := func(k int) {
		auto.Reset(io.Discard, Options{AutoNamespaces: true})
		auto.Start(Name{Space: "urn:a", Local: "doc"})
		for i := 0; i < k; i++ {
			auto.Start(Name{Space: "urn:b", Local: "row", Prefix: "b"})
			auto.Attr(Name{Space: "urn:c", Local: "x"}, "1")
			auto.EndEmpty()
		}
		auto.End()
		if err := auto.Close(); err != nil {
			t.Fatal(err)
		}
	}
	gen(10)
	if a := testing.AllocsPerRun(5, func() { gen(1000) }); a != 0 {
		t.Errorf("AutoNamespaces, 1000 rows: %v allocations per document, want 0", a)
	}

	// przelot: te same egzemplarze czytnika i pisarza, dwa rozmiary
	read := func(g io.Reader) []byte {
		b, err := io.ReadAll(g)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, doc := range []struct {
		name         string
		small, large []byte
	}{
		{"GenJPK", read(testkit.GenJPK(200)), read(testkit.GenJPK(2000))},
		{"GenWordML", read(testkit.GenWordML(200)), read(testkit.GenWordML(2000))},
	} {
		r := xmlreader.New(nil, xmlreader.Options{})
		var src bytes.Reader
		pass := func(data []byte) float64 {
			return testing.AllocsPerRun(5, func() {
				src.Reset(data)
				if err := passThrough(r, w, &src, io.Discard); err != nil {
					t.Fatal(err)
				}
			})
		}
		// sam czytnik: wartości atrybutów są napisami, więc niepowtarzalna wartość to alokacja czytnika
		readOnly := func(data []byte) float64 {
			return testing.AllocsPerRun(5, func() {
				src.Reset(data)
				r.Reset(&src, xmlreader.Options{})
				for {
					if _, err := r.Next(); err != nil {
						break
					}
				}
			})
		}
		pass(doc.large) // rozgrzanie
		for _, data := range [][]byte{doc.small, doc.large} {
			p, ro := pass(data), readOnly(data)
			t.Logf("%s, %d bytes: %v allocations for the pass-through, %v for the reader alone", doc.name, len(data), p, ro)
			if p > ro {
				t.Errorf("%s, %d bytes: the writer adds %v allocations to the %v of the reader", doc.name, len(data), p-ro, ro)
			}
		}
	}
}

// stdEvents czyta dokument przez encoding/xml i zapisuje nazwy rozwinięte, wartości i dane; tekst sąsiednich
// zdarzeń CharData jest łączony, bo encoding/xml nie odróżnia sekcji CDATA od tekstu.
func stdEvents(doc []byte) (string, error) {
	var b strings.Builder
	var text []byte
	flush := func(depth int) {
		if depth > 0 && len(text) > 0 {
			fmt.Fprintf(&b, "text %q\n", text)
		}
		text = text[:0]
	}
	dec := xml.NewDecoder(bytes.NewReader(doc))
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return b.String(), nil
		}
		if err != nil {
			return "", err
		}
		switch e := tok.(type) {
		case xml.StartElement:
			flush(depth)
			depth++
			fmt.Fprintf(&b, "start {%s}%s\n", e.Name.Space, e.Name.Local)
			for _, a := range e.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				fmt.Fprintf(&b, "  {%s}%s=%q\n", a.Name.Space, a.Name.Local, a.Value)
			}
		case xml.EndElement:
			flush(depth)
			depth--
			fmt.Fprintf(&b, "end {%s}%s\n", e.Name.Space, e.Name.Local)
		case xml.CharData:
			text = append(text, e...)
		case xml.Comment:
			flush(depth)
			fmt.Fprintf(&b, "comment %q\n", []byte(e))
		case xml.ProcInst:
			flush(depth)
			if e.Target != "xml" {
				fmt.Fprintf(&b, "pi %q %q\n", e.Target, e.Inst)
			}
		}
	}
}

// ourEvents to ten sam zapis ze strumienia czytnika goxmlgo.
func ourEvents(doc []byte) (string, error) {
	var b strings.Builder
	var text []byte
	flush := func() {
		if len(text) > 0 {
			fmt.Fprintf(&b, "text %q\n", text)
		}
		text = text[:0]
	}
	r := xmlreader.New(bytes.NewReader(doc), xmlreader.Options{})
	for {
		t, err := r.Next()
		if err == io.EOF {
			return b.String(), nil
		}
		if err != nil {
			return "", err
		}
		switch t.Kind {
		case xmlreader.KindStartElement:
			flush()
			fmt.Fprintf(&b, "start {%s}%s\n", t.Name.Space, t.Name.Local)
			for _, a := range t.Attrs {
				fmt.Fprintf(&b, "  {%s}%s=%q\n", a.Name.Space, a.Name.Local, a.Value)
			}
		case xmlreader.KindEndElement:
			flush()
			fmt.Fprintf(&b, "end {%s}%s\n", t.Name.Space, t.Name.Local)
		case xmlreader.KindText:
			text = append(text, t.Data...)
		case xmlreader.KindComment:
			flush()
			fmt.Fprintf(&b, "comment %q\n", t.Data)
		case xmlreader.KindProcInst:
			flush()
			fmt.Fprintf(&b, "pi %q %q\n", t.Name.Local, t.Data)
		}
	}
}

// TestStdlibWriter: encoding/xml jako czarna skrzynka czyta wyjście pisarza i daje te same nazwy rozwinięte,
// wartości atrybutów, tekst, komentarze i PI co czytnik goxmlgo na dokumencie źródłowym. Wsad: korpus oraz
// dokumenty zestawu W3C, które czytnik przyjmuje z przestrzeniami nazw. Dokument, którego encoding/xml nie
// czyta (np. nazwy z piątej edycji XML 1.0), jest liczony osobno i nie jest porażką pisarza.
func TestStdlibWriter(t *testing.T) {
	type doc struct {
		id  string
		src []byte
	}
	var docs []doc
	for _, f := range corpusFiles(t) {
		docs = append(docs, doc{path.Base(f), readFile(t, f)})
	}
	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err == nil {
		for _, ct := range testkit.LoadXMLConf(t, xmlconfRoot) {
			if ct.Excluded == "" && ct.Type != "not-wf" {
				docs = append(docs, doc{ct.Group + "/" + ct.ID, readFile(t, path.Join(xmlconfRoot, ct.Dir, ct.URI))})
			}
		}
	}
	same, stdRejects, notNS := 0, 0, 0
	for _, d := range docs {
		want, err := ourEvents(d.src)
		if err != nil {
			notNS++ // dokument XML 1.0 z nazwami, które nie są QName, albo porażka K1
			continue
		}
		out, err := rewrite(d.src, false, Options{})
		if err != nil {
			t.Errorf("%s: %v", d.id, err)
			continue
		}
		got, err := stdEvents(out)
		if err != nil {
			stdRejects++
			if testing.Verbose() {
				t.Logf("%s: encoding/xml rejects the output: %v", d.id, err)
			}
			continue
		}
		if got != want {
			t.Errorf("%s: encoding/xml reads the output differently\n%s", d.id, firstDiff(got, want))
			continue
		}
		same++
	}
	t.Logf("encoding/xml on the output of the writer: %d documents, %d the same, %d rejected by encoding/xml, %d not read with namespaces",
		len(docs), same, stdRejects, notNS)
	if same < len(docs)/2 {
		t.Errorf("only %d of %d documents compared", same, len(docs))
	}
}

func benchDoc(b *testing.B, g io.Reader) []byte {
	data, err := io.ReadAll(g)
	if err != nil {
		b.Fatal(err)
	}
	return data
}

// BenchmarkPassThrough: czytnik → pisarz na dokumentach wzorcowych (internal/testkit), wynik do io.Discard.
func BenchmarkPassThrough(b *testing.B) {
	for _, doc := range []struct {
		name string
		gen  io.Reader
	}{{"JPK", testkit.GenJPK(20000)}, {"WordML", testkit.GenWordML(30000)}} {
		data := benchDoc(b, doc.gen)
		b.Run(doc.name, func(b *testing.B) {
			r := xmlreader.New(nil, xmlreader.Options{})
			w := New(io.Discard, Options{})
			var src bytes.Reader
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				src.Reset(data)
				if err := passThrough(r, w, &src, io.Discard); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkReadOnly: sam czytnik na tych samych dokumentach, dla porównania z przelotem.
func BenchmarkReadOnly(b *testing.B) {
	for _, doc := range []struct {
		name string
		gen  io.Reader
	}{{"JPK", testkit.GenJPK(20000)}, {"WordML", testkit.GenWordML(30000)}} {
		data := benchDoc(b, doc.gen)
		b.Run(doc.name, func(b *testing.B) {
			r := xmlreader.New(nil, xmlreader.Options{})
			var src bytes.Reader
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				src.Reset(data)
				r.Reset(&src, xmlreader.Options{})
				for {
					if _, err := r.Next(); err != nil {
						break
					}
				}
			}
		})
	}
}

// TestStdlibEncoderDifferences utrwala zachowania encoding/xml (Encoder.EncodeToken), które wymienia tabela
// „Pisarz” w docs/differences.md, obok zachowania pisarza goxmlgo dla tych samych danych. Gdy biblioteka
// standardowa zmieni któreś z nich, test to pokaże i tabelę trzeba poprawić.
func TestStdlibEncoderDifferences(t *testing.T) {
	std := func(toks ...xml.Token) string {
		var b bytes.Buffer
		e := xml.NewEncoder(&b)
		for _, tok := range toks {
			if err := e.EncodeToken(tok); err != nil {
				return "error: " + err.Error()
			}
		}
		if err := e.Flush(); err != nil {
			return "error: " + err.Error()
		}
		return b.String()
	}
	a, b := xml.Name{Local: "a"}, xml.Name{Local: "b"}
	for _, c := range []struct {
		what, std, wantStd string
		ours               func(w *Writer) error
		wantCode           string
	}{
		{"character outside Char and bytes that are not UTF-8",
			std(xml.StartElement{Name: a}, xml.CharData("x\x01y\xffz"), xml.EndElement{Name: a}), "<a>x�y�z</a>",
			func(w *Writer) error { return seq(w.Start(n("a")), w.Text("x\x01y")) }, CodeChar},
		{"second root element",
			std(xml.StartElement{Name: a}, xml.EndElement{Name: a}, xml.StartElement{Name: b}, xml.EndElement{Name: b}), "<a></a><b></b>",
			func(w *Writer) error { return seq(w.Start(n("a")), w.End(), w.Start(n("b"))) }, CodeOutsideRoot},
		{"text outside the root element",
			std(xml.CharData("text"), xml.StartElement{Name: a}, xml.EndElement{Name: a}), "text<a></a>",
			func(w *Writer) error { return w.Text("text") }, CodeOutsideRoot},
		{"attribute given twice",
			std(xml.StartElement{Name: a, Attr: []xml.Attr{{Name: xml.Name{Local: "x"}, Value: "1"}, {Name: xml.Name{Local: "x"}, Value: "2"}}}, xml.EndElement{Name: a}),
			`<a x="1" x="2"></a>`,
			func(w *Writer) error { return seq(w.Start(n("a")), w.Attr(n("x"), "1"), w.Attr(n("x"), "2")) }, CodeUniqueAttSpec},
	} {
		if c.std != c.wantStd {
			t.Errorf("%s: encoding/xml now writes %q (docs/differences.md says %q)", c.what, c.std, c.wantStd)
		}
		if _, err := write(Options{}, c.ours); code(err) != c.wantCode {
			t.Errorf("%s: xmlwriter gives %v, want %s", c.what, err, c.wantCode)
		}
	}
	// przelot: prefiksy, znacznik elementu pustego i sekcja CDATA
	const doc = `<p:a xmlns:p="urn:p" p:x="1"><b/><![CDATA[c<d]]></p:a>`
	var out bytes.Buffer
	d, e := xml.NewDecoder(strings.NewReader(doc)), xml.NewEncoder(&out)
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		e.EncodeToken(tok)
	}
	e.Flush()
	if s := out.String(); strings.Contains(s, "<p:a") || strings.Contains(s, "<b/>") || strings.Contains(s, "CDATA") {
		t.Errorf("encoding/xml now keeps prefixes, empty-element tags or CDATA sections: %s", s)
	}
	ours, err := rewrite([]byte(doc), false, Options{})
	if err != nil || string(ours) != doc {
		t.Errorf("xmlwriter pass-through: %s, %v", ours, err)
	}
}
