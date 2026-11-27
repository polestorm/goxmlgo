package xmlwriter

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// TestExplicitNamespaces: tryb jawny. Deklaracje podaje wywołujący; są zapisywane w podanej kolejności przed
// atrybutami; nazwa, której prefiks nie jest związany z jej przestrzenią, jest błędem.
// REQ: WR-011
func TestExplicitNamespaces(t *testing.T) {
	got, err := write(Options{}, func(w *Writer) error {
		return seq(
			w.Start(qn("urn:d", "root", ""), bind("", "urn:d"), bind("b", "urn:b"), bind("a", "urn:a"), bind("xml", nsXML)),
			w.Attr(n("plain"), "1"), w.Attr(qn("urn:a", "x", "a"), "2"),
			w.Start(qn("urn:d", "inherits", "")), w.EndEmpty(),
			w.Start(qn("urn:b", "e", "b")), w.EndEmpty(),
			w.Start(n("none"), bind("", "")), w.Attr(qn("urn:b", "x", "b"), "3"), // xmlns="" wyłącza przestrzeń domyślną
			w.Start(qn("urn:z", "shadow", "a"), bind("a", "urn:z")), w.EndEmpty(), // przesłonięcie prefiksu
			w.End(),
			w.Start(qn("urn:a", "back", "a")), w.EndEmpty(), // po zamknięciu wraca wiązanie zewnętrzne
			w.End())
	})
	want := `<root xmlns="urn:d" xmlns:b="urn:b" xmlns:a="urn:a" xmlns:xml="http://www.w3.org/XML/1998/namespace" plain="1" a:x="2">` +
		`<inherits/><b:e/><none xmlns="" b:x="3"><a:shadow xmlns:a="urn:z"/></none><a:back/></root>`
	if err != nil || got != want {
		t.Errorf("%v\n got  %s\n want %s", err, got, want)
	}
	if _, err := streamDump([]byte(got), false); err != nil {
		t.Errorf("the reader rejects it: %v", err)
	}
	for _, c := range []struct {
		code string
		fn   func(w *Writer) error
	}{
		{CodePrefixDeclared, func(w *Writer) error { return w.Start(qn("urn:p", "a", "p")) }},
		{CodePrefixDeclared, func(w *Writer) error { return w.Attr(qn("urn:p", "a", "p"), "") }},
		{CodePrefixDeclared, func(w *Writer) error { return w.Start(qn("", "a", "p")) }},
		{CodeNamespaceName, func(w *Writer) error { return w.Start(qn("urn:other", "a", "p"), bind("p", "urn:p")) }},
		{CodeNamespaceName, func(w *Writer) error { return w.Start(qn("", "a", "p"), bind("p", "urn:p")) }},
		{CodeNamespaceName, func(w *Writer) error { return w.Start(qn("urn:d", "a", "")) }},      // przestrzeń bez deklaracji domyślnej
		{CodeNamespaceName, func(w *Writer) error { return w.Start(n("a"), bind("", "urn:d")) }}, // element bez przestrzeni pod domyślną
		{CodeNamespaceName, func(w *Writer) error { return w.Attr(qn("urn:d", "a", ""), "") }},   // atrybut w przestrzeni bez prefiksu
		{CodeNamespaceName, func(w *Writer) error { return w.Attr(qn("", "a", "xml"), "") }},
		{CodeNamespaceName, func(w *Writer) error { return w.Attr(qn(nsXML, "lang", "x"), "") }},
		{CodeUniqueAttSpec, func(w *Writer) error { return w.Start(n("a"), bind("p", "urn:1"), bind("p", "urn:2")) }},
		{CodeUniqueAttSpec, func(w *Writer) error { return w.Start(n("a"), bind("", ""), bind("", "")) }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Start(n("a"), bind("xmlns", "urn:x")) }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Start(n("a"), bind("xml", "urn:x")) }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Start(n("a"), bind("p", nsXML)) }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Start(n("a"), bind("p", nsXMLNS)) }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Start(n("a"), bind("", nsXML)) }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Start(qn("urn:x", "a", "xmlns")) }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Attr(n("xmlns"), "urn:x") }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Attr(qn(nsXMLNS, "p", "xmlns"), "urn:x") }},
		{CodeNoPrefixUndeclaring, func(w *Writer) error { return w.Start(n("a"), bind("p", "")) }},
		{CodeName, func(w *Writer) error { return w.Start(n("a"), bind("1", "urn:x")) }},
		{CodeQName, func(w *Writer) error { return w.Start(n("a"), bind("p:q", "urn:x")) }},
	} {
		expectCode(t, Options{}, c.code, c.fn)
	}
	// nieudany Start nie zostawia swoich deklaracji w zakresie
	_, err = write(Options{}, func(w *Writer) error {
		w.Start(n("r"))
		w.Start(n("a b"), bind("p", "urn:p"))
		return w.Start(qn("urn:p", "x", "p"))
	})
	if code(err) != CodePrefixDeclared {
		t.Errorf("declarations of a failed Start stay in scope: %v", err)
	}
}

// TestAutoNamespaces: tryb automatyczny. Pisarz dopisuje brakujące deklaracje w miejscu użycia: z prefiksem
// z nazwy, gdy jest wolny, inaczej z prefiksem już związanym albo z kolejnym z ciągu ns1, ns2, …
// REQ: WR-012
func TestAutoNamespaces(t *testing.T) {
	auto := Options{AutoNamespaces: true}
	for _, c := range []struct {
		name  string
		calls func(w *Writer) error
		want  string
	}{
		{"default namespace from the element name", func(w *Writer) error {
			return seq(w.Start(qn("urn:d", "a", "")), w.Start(qn("urn:d", "b", "")), w.EndEmpty(), w.End())
		}, `<a xmlns="urn:d"><b/></a>`},
		{"prefix from the name", func(w *Writer) error {
			return seq(w.Start(qn("urn:p", "a", "p")), w.Start(qn("urn:p", "b", "p")), w.EndEmpty(), w.End())
		}, `<p:a xmlns:p="urn:p"><p:b/></p:a>`},
		{"no namespace under a default namespace", func(w *Writer) error {
			return seq(w.Start(qn("urn:d", "a", "")), w.Start(n("b")), w.Start(qn("urn:d", "c", "")), w.EndEmpty(), w.End(), w.End())
		}, `<a xmlns="urn:d"><b xmlns=""><c xmlns="urn:d"/></b></a>`},
		{"prefix shadowed for an element", func(w *Writer) error {
			return seq(w.Start(qn("urn:1", "a", "p")), w.Start(qn("urn:2", "b", "p")), w.EndEmpty(), w.Start(qn("urn:1", "c", "p")), w.EndEmpty(), w.End())
		}, `<p:a xmlns:p="urn:1"><p:b xmlns:p="urn:2"/><p:c/></p:a>`},
		{"attribute: prefix from the name", func(w *Writer) error {
			return seq(w.Start(n("a")), w.Attr(qn("urn:p", "x", "p"), "1"), w.Attr(qn("urn:p", "y", "p"), "2"), w.EndEmpty())
		}, `<a xmlns:p="urn:p" p:x="1" p:y="2"/>`},
		{"attribute: generated prefix, never the default namespace", func(w *Writer) error {
			return seq(w.Start(qn("urn:d", "a", "")), w.Attr(qn("urn:d", "x", ""), "1"), w.Attr(qn("urn:e", "y", ""), "2"),
				w.Attr(qn("urn:d", "z", ""), "3"), w.EndEmpty())
		}, `<a xmlns="urn:d" xmlns:ns1="urn:d" ns1:x="1" xmlns:ns2="urn:e" ns2:y="2" ns1:z="3"/>`},
		{"attribute: prefix taken by another namespace", func(w *Writer) error {
			return seq(w.Start(qn("urn:1", "a", "p")), w.Attr(qn("urn:2", "x", "p"), "1"), w.EndEmpty())
		}, `<p:a xmlns:p="urn:1" xmlns:ns1="urn:2" ns1:x="1"/>`},
		{"attribute: a prefix already bound is reused", func(w *Writer) error {
			return seq(w.Start(n("a"), bind("q", "urn:q")), w.Start(n("b")), w.Attr(qn("urn:q", "x", ""), "1"), w.Attr(qn("urn:q", "y", "other"), "2"), w.EndEmpty(), w.End())
		}, `<a xmlns:q="urn:q"><b q:x="1" xmlns:other="urn:q" other:y="2"/></a>`},
		{"generated prefixes skip the ones in scope", func(w *Writer) error {
			return seq(w.Start(n("a"), bind("ns1", "urn:1"), bind("ns3", "urn:3")), w.Attr(qn("urn:x", "x", ""), ""), w.Attr(qn("urn:y", "y", ""), ""),
				w.Start(n("b")), w.Attr(qn("urn:z", "z", ""), ""), w.EndEmpty(), w.End())
		}, `<a xmlns:ns1="urn:1" xmlns:ns3="urn:3" xmlns:ns2="urn:x" ns2:x="" xmlns:ns4="urn:y" ns4:y=""><b xmlns:ns5="urn:z" ns5:z=""/></a>`},
		{"explicit declaration wins over the prefix of the name", func(w *Writer) error {
			return seq(w.Start(qn("urn:2", "a", "p"), bind("p", "urn:1")), w.EndEmpty())
		}, `<ns1:a xmlns:p="urn:1" xmlns:ns1="urn:2"/>`},
		{"xml namespace", func(w *Writer) error {
			return seq(w.Start(n("a")), w.Attr(qn(nsXML, "space", ""), "preserve"), w.Attr(qn(nsXML, "lang", "xml"), "pl"), w.EndEmpty())
		}, `<a xml:space="preserve" xml:lang="pl"/>`},
		{"moved subtree keeps its meaning", func(w *Writer) error {
			// element z prefiksem w, który w nowym miejscu znaczy co innego
			return seq(w.Start(qn("urn:new", "host", "w")), w.Start(qn("urn:old", "p", "w")), w.Attr(qn("urn:old", "val", "w"), "1"),
				w.Start(qn("urn:old", "r", "w")), w.EndEmpty(), w.End(), w.End())
		}, `<w:host xmlns:w="urn:new"><w:p xmlns:w="urn:old" w:val="1"><w:r/></w:p></w:host>`},
	} {
		got, err := write(auto, c.calls)
		if err != nil || got != c.want {
			t.Errorf("%s: %v\n got  %s\n want %s", c.name, err, got, c.want)
			continue
		}
		if _, err := streamDump([]byte(got), false); err != nil {
			t.Errorf("%s: the reader rejects it: %v", c.name, err)
		}
		again, _ := write(auto, c.calls)
		if again != got {
			t.Errorf("%s: not deterministic", c.name)
		}
	}
	for _, c := range []struct {
		code string
		fn   func(w *Writer) error
	}{
		{CodeNamespaceName, func(w *Writer) error { return w.Start(qn("", "a", "p")) }},
		{CodeNamespaceName, func(w *Writer) error { return w.Attr(qn("", "a", "p"), "") }},
		{CodeNamespaceName, func(w *Writer) error { return w.Start(n("a"), bind("", "urn:d")) }},
		{CodeNamespaceName, func(w *Writer) error { return w.Start(qn("urn:x", "a", "xml")) }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Start(qn(nsXMLNS, "a", "")) }},
		// przestrzeń nazw, którą pisarz deklaruje sam, jest sprawdzana jak podana jawnie (znalezione przez FuzzWriter)
		{CodeChar, func(w *Writer) error { return w.Start(qn("urn:\x0f", "a", "")) }},
		{CodeChar, func(w *Writer) error { return w.Attr(qn("urn:\x0f", "a", "p"), "") }},
		{CodeEncodingInvalid, func(w *Writer) error { return w.Attr(qn("urn:\xff", "a", ""), "") }},
		{CodeReservedPrefixes, func(w *Writer) error { return w.Attr(qn(nsXMLNS, "a", "p"), "") }},
		{CodeAttributesUnique, func(w *Writer) error {
			return seq(w.Attr(qn("urn:p", "x", "p"), ""), w.Attr(qn("urn:p", "x", "q"), ""))
		}},
		{CodeUniqueAttSpec, func(w *Writer) error {
			return seq(w.Attr(qn("urn:p", "x", "p"), ""), w.Attr(qn("urn:p", "x", ""), ""))
		}},
	} {
		if c.code == CodeUniqueAttSpec || c.code == CodeAttributesUnique {
			expectLastOpts(t, auto, c.code, c.fn)
			continue
		}
		expectCode(t, auto, c.code, c.fn)
	}
	if got := code(New(&bytes.Buffer{}, Options{AutoNamespaces: true, NoNamespaces: true}).Start(n("a"))); got != CodeUsageOptions {
		t.Errorf("AutoNamespaces with NoNamespaces: %q", got)
	}

	// Zapis w trybie automatycznym strumienia czytnika bez deklaracji (jak węzły drzewa przeniesione w inne
	// miejsce) daje dokument o tych samych nazwach rozwiniętych i wartościach.
	for _, file := range corpusFiles(t) {
		src := readFile(t, file)
		var b bytes.Buffer
		w := New(&b, auto)
		r := xmlreader.New(bytes.NewReader(src), xmlreader.Options{})
		for {
			tok, err := r.Next()
			if err != nil {
				break
			}
			tok.Namespaces = nil
			if err := w.WriteToken(tok); err != nil {
				t.Fatalf("%s: %v", file, err)
			}
		}
		if err := w.Close(); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if got, want := expandedDump(t, b.Bytes()), expandedDump(t, src); got != want {
			t.Errorf("%s: expanded names or values changed\n%s", file, firstDiff(got, want))
		}
	}
}

// expandedDump to strumień bez prefiksów i deklaracji: nazwy rozwinięte, wartości, dane.
func expandedDump(t *testing.T, src []byte) string {
	var b strings.Builder
	r := xmlreader.New(bytes.NewReader(src), xmlreader.Options{})
	for {
		tok, err := r.Next()
		if err != nil {
			if e, ok := err.(*xmlreader.Error); ok {
				t.Fatalf("reader: %v", e)
			}
			return b.String()
		}
		switch tok.Kind {
		case xmlreader.KindStartElement:
			fmt.Fprintf(&b, "start {%s}%s\n", tok.Name.Space, tok.Name.Local)
			for _, a := range tok.Attrs {
				fmt.Fprintf(&b, "  {%s}%s=%q\n", a.Name.Space, a.Name.Local, a.Value)
			}
		case xmlreader.KindEndElement:
			fmt.Fprintf(&b, "end {%s}%s\n", tok.Name.Space, tok.Name.Local)
		case xmlreader.KindDoctype:
		default:
			fmt.Fprintf(&b, "%d %q %q %v\n", tok.Kind, tok.Name.Local, tok.Data, tok.CDATA)
		}
	}
}

// TestErrorState: błąd pisarza to *Error ze stałym kodem, ścieżką otwartych elementów i numerem wywołania; po
// błędzie pisarz jest w stanie sprzed wywołania. Ciąg wywołań z wplecionymi wywołaniami błędnymi daje te same
// bajty co ten sam ciąg bez nich.
// REQ: WR-014
func TestErrorState(t *testing.T) {
	bad := []func(w *Writer) error{
		func(w *Writer) error { return w.Start(n("a b")) },
		func(w *Writer) error { return w.Start(qn("urn:nope", "a", "nope")) },
		func(w *Writer) error { return w.Start(n("a"), bind("p", "urn:p"), bind("p", "urn:q")) },
		func(w *Writer) error { return w.Start(n("a"), bind("zz", "urn:zz"), bind("bad prefix", "urn:q")) },
		func(w *Writer) error { return w.Attr(n("a b"), "") },
		func(w *Writer) error { return w.Attr(n("ok"), "\x00") },
		func(w *Writer) error { return w.Attr(qn("urn:zz", "a", "zz"), "") }, // prefiks z nieudanego Start nie jest w zakresie
		func(w *Writer) error { return w.Text("\x01") },
		func(w *Writer) error { return w.CDATA("]]>") },
		func(w *Writer) error { return w.Comment("--") },
		func(w *Writer) error { return w.ProcInst("xml", "") },
		func(w *Writer) error { return w.WriteToken(Token{Kind: xmlreader.KindEndElement, Name: n("not-open")}) },
		func(w *Writer) error {
			return w.WriteToken(Token{Kind: xmlreader.KindStartElement, Name: n("t"), Namespaces: []Binding{bind("zz", "urn:zz")},
				Attrs: []Attr{{Name: n("a"), Value: "1"}, {Name: qn("urn:zz", "b", "zz"), Value: "2"}, {Name: n("a"), Value: "3"}}})
		},
		func(w *Writer) error { return w.WriteToken(Token{}) },
	}
	good := func(w *Writer, between func()) error {
		step := func(err error) error { between(); return err }
		return seq(
			step(w.Start(n("root"), bind("p", "urn:p"))), step(w.Attr(n("dup"), "1")), step(w.Attr(qn("urn:p", "x", "p"), "2")),
			step(w.Text("t")), step(w.Start(qn("urn:p", "in", "p"))), step(w.Attr(n("dup"), "3")), step(w.EndEmpty()),
			step(w.Start(n("e"))), step(w.Attr(n("dup"), "4")), step(w.Comment("c")), step(w.ProcInst("pi", "d")), step(w.CDATA("x")), step(w.End()),
			step(w.Start(n("last"))), step(w.Attr(n("dup"), "5")), step(w.End()), step(w.End()),
		)
	}
	want, err := write(Options{}, func(w *Writer) error { return good(w, func() {}) })
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range bad {
		calls := 0
		got, err := write(Options{}, func(w *Writer) error {
			return good(w, func() {
				calls++
				err := b(w)
				e, ok := err.(*Error)
				if !ok {
					t.Errorf("bad call %d at step %d: %v, want an *Error", i, calls, err)
					return
				}
				if e.Call != w.calls || e.Code == "" || e.Message == "" || !strings.HasPrefix(e.Path, "/") {
					t.Errorf("bad call %d at step %d: %+v (the Writer counts %d calls)", i, calls, e, w.calls)
				}
				if !strings.Contains(e.Error(), e.Code) || !strings.Contains(e.Error(), e.Path) {
					t.Errorf("Error() %q lacks the code or the path", e.Error())
				}
			})
		})
		if err != nil || got != want {
			t.Errorf("bad call %d woven in: %v\n got  %s\n want %s", i, err, got, want)
		}
	}
	// ścieżka
	var buf bytes.Buffer
	w := New(&buf, Options{})
	seq(w.Start(n("a")), w.Start(qn("urn:p", "b", "p"), bind("p", "urn:p")), w.Start(n("c")))
	if e := w.Text("\x00").(*Error); e.Path != "/a/p:b/c" || e.Call != 4 {
		t.Errorf("path %q call %d, want /a/p:b/c call 4", e.Path, e.Call)
	}
	if e := New(&buf, Options{}).Text("x").(*Error); e.Path != "/" || e.Call != 1 {
		t.Errorf("path %q call %d at the document level", e.Path, e.Call)
	}
}

// expectLastOpts jak expectLast, z opcjami.
func expectLastOpts(t *testing.T, opts Options, want string, fn func(w *Writer) error) {
	t.Helper()
	w := New(&bytes.Buffer{}, opts)
	w.Start(n("r"))
	if got := code(fn(w)); got != want {
		t.Errorf("code %q, want %q", got, want)
	}
}

// TestIndent: wcięcia tylko między znacznikami, nigdy w elemencie, w którym zapisano tekst, ani w jego potomkach.
// REQ: WR-016
func TestIndent(t *testing.T) {
	doc := func(w *Writer) error {
		return seq(w.Declaration(""), w.Comment("top"), w.Start(n("a")), w.Attr(n("x"), "1"),
			w.Start(n("b")), w.Start(n("c")), w.EndEmpty(), w.Start(n("d")), w.End(), w.End(),
			w.Start(n("text")), w.Text("mixed "), w.Start(n("i")), w.Start(n("deep")), w.EndEmpty(), w.End(), w.Text(" content"), w.End(),
			w.Comment("k"), w.ProcInst("p", "d"),
			w.Start(n("cdata")), w.CDATA("x"), w.Start(n("e")), w.EndEmpty(), w.End(),
			w.End(), w.Comment("end"))
	}
	got, err := write(Options{Indent: "  "}, doc)
	want := `<?xml version="1.0" encoding="UTF-8"?>
<!--top-->
<a x="1">
  <b>
    <c/>
    <d></d>
  </b>
  <text>mixed <i><deep/></i> content</text>
  <!--k-->
  <?p d?>
  <cdata><![CDATA[x]]><e/></cdata>
</a>
<!--end-->
`
	if err != nil || got != want {
		t.Errorf("%v\n got:\n%s\n want:\n%s", err, got, want)
	}
	// wcięcia nie zmieniają tekstu ani struktury: strumień bez tokenów białych znaków jest taki sam
	plain, _ := write(Options{}, doc)
	strip := func(s string) string {
		d, err := streamDump([]byte(s), false)
		if err != nil {
			t.Fatal(err)
		}
		var keep []string
		for _, l := range strings.Split(d, "\n") {
			if !strings.HasSuffix(l, " whitespace") {
				keep = append(keep, l)
			}
		}
		return strings.Join(keep, "\n")
	}
	if strip(got) != strip(plain) {
		t.Errorf("indentation changed more than white space between tags:\n%s", firstDiff(strip(got), strip(plain)))
	}
	// dokument już wcięty przechodzi bez zmian: białe znaki ze źródła są tekstem, więc pisarz niczego nie dodaje
	again, err := rewrite([]byte(got), false, Options{Indent: "\t"})
	if err != nil || string(again) != got {
		t.Errorf("an indented document written with Indent: %v\n%s", err, again)
	}
	if got, _ := write(Options{Indent: "\t"}, func(w *Writer) error { return seq(w.Start(n("a")), w.Start(n("b")), w.EndEmpty(), w.End()) }); got != "<a>\n\t<b/>\n</a>\n" {
		t.Errorf("tab indentation: %q", got)
	}
	if got := code(New(&bytes.Buffer{}, Options{Indent: "\n"}).Start(n("a"))); got != CodeUsageOptions {
		t.Errorf("Indent with a line feed: %q", got)
	}
}

// TestIndentDeterministic: wynik z wcięciami zależy tylko od ciągu wywołań i wartości Indent: te same bajty
// w kolejnych przebiegach, z Flush po każdym wywołaniu, przy zapisie po 3 bajty i po Reset.
// REQ: WR-016
// REQ: DET-004
func TestIndentDeterministic(t *testing.T) {
	doc := func(w *Writer, flush func()) error {
		return seq(w.Declaration(""), do(flush), w.Start(n("a")), do(flush), w.Attr(n("x"), "1"), do(flush),
			w.Start(n("b")), do(flush), w.Start(n("c")), w.EndEmpty(), do(flush), w.End(), do(flush),
			w.Start(n("t")), w.Text("x"), do(flush), w.Start(n("i")), w.EndEmpty(), w.End(), do(flush),
			w.Comment("k"), do(flush), w.ProcInst("p", "d"), w.End(), do(flush), w.Comment("end"))
	}
	opts := Options{Indent: "  "}
	want, err := write(opts, func(w *Writer) error { return doc(w, func() {}) })
	if err != nil {
		t.Fatal(err)
	}
	cw := &chunkWriter{size: 3}
	w := New(cw, opts)
	for i := 0; i < 10; i++ {
		cw.b.Reset()
		w.Reset(cw, opts)
		if err := seq(doc(w, func() { w.Flush() }), w.Close()); err != nil {
			t.Fatal(err)
		}
		if got := cw.b.String(); got != want {
			t.Fatalf("run %d:\n got  %q\n want %q", i, got, want)
		}
	}
}

// TestLimitsMatchReader: limity pisarza mają te same wartości domyślne i kody co limity czytnika. Dokument
// zapisany na granicy limitu czytnik z tymi samymi limitami czyta; o jeden więcej pisarz odrzuca, tak jak czytnik.
// REQ: WR-017
func TestLimitsMatchReader(t *testing.T) {
	d, rd := DefaultLimits(), xmlreader.DefaultLimits()
	if d.MaxDepth != rd.MaxDepth || d.MaxNameLength != rd.MaxNameLength || d.MaxAttributes != rd.MaxAttributes || d.MaxTextSize != rd.MaxTextSize {
		t.Errorf("default limits %+v differ from the reader's %+v", d, rd)
	}
	if z := (Limits{}).withDefaults(); z != d {
		t.Errorf("zero Limits give %+v, want the defaults", z)
	}
	lim := Limits{MaxDepth: 5, MaxNameLength: 10, MaxAttributes: 4, MaxTextSize: 20}
	rlim := xmlreader.Limits{MaxDepth: 5, MaxNameLength: 10, MaxAttributes: 4, MaxTextSize: 20}
	nested := func(k int) func(w *Writer) error {
		return func(w *Writer) error {
			for i := 0; i < k; i++ {
				if err := w.Start(n("a")); err != nil {
					return err
				}
			}
			for i := 0; i < k; i++ {
				w.End()
			}
			return nil
		}
	}
	in := func(calls func(w *Writer) error) func(w *Writer) error {
		return func(w *Writer) error {
			if err := w.Start(n("r")); err != nil {
				return err
			}
			if err := calls(w); err != nil {
				return err
			}
			return w.End()
		}
	}
	attrs := func(decls, k int) func(w *Writer) error {
		return func(w *Writer) error {
			var ds []Binding
			for i := 0; i < decls; i++ {
				ds = append(ds, bind(fmt.Sprintf("p%d", i), "urn:p"))
			}
			if err := w.Start(n("a"), ds...); err != nil {
				return err
			}
			for i := 0; i < k; i++ {
				if err := w.Attr(n(fmt.Sprintf("a%d", i)), ""); err != nil {
					return err
				}
			}
			return w.EndEmpty()
		}
	}
	x := func(k int) string { return strings.Repeat("x", k) }
	for _, c := range []struct {
		name     string
		code     string
		at, over func(w *Writer) error
	}{
		{"depth", CodeLimitDepth, nested(5), nested(6)},
		{"element name", CodeLimitNameLength, in(func(w *Writer) error { return seq(w.Start(n(x(10))), w.End()) }), func(w *Writer) error { return w.Start(n(x(11))) }},
		{"qualified name", CodeLimitNameLength,
			func(w *Writer) error { return seq(w.Start(qn("urn:p", x(7), "pp"), bind("pp", "urn:p")), w.End()) },
			func(w *Writer) error { return w.Start(qn("urn:p", x(8), "pp"), bind("pp", "urn:p")) }},
		{"attribute name", CodeLimitNameLength,
			in(func(w *Writer) error { return w.Attr(n(x(10)), "") }), in(func(w *Writer) error { return w.Attr(n(x(11)), "") })},
		{"PI target", CodeLimitNameLength,
			in(func(w *Writer) error { return w.ProcInst(x(10), "") }), in(func(w *Writer) error { return w.ProcInst(x(11), "") })},
		{"attributes", CodeLimitAttributes, in(attrs(0, 4)), in(attrs(0, 5))},
		{"attributes with declarations", CodeLimitAttributes, in(attrs(2, 2)), in(attrs(2, 3))},
		{"declarations", CodeLimitAttributes, in(attrs(4, 0)), in(attrs(5, 0))},
		{"text", CodeLimitTextSize, in(func(w *Writer) error { return w.Text(x(20)) }), in(func(w *Writer) error { return w.Text(x(21)) })},
		{"text in several calls", CodeLimitTextSize,
			in(func(w *Writer) error { return seq(w.Text(x(10)), w.Text(x(10))) }), in(func(w *Writer) error { return seq(w.Text(x(10)), w.Text(x(11))) })},
		{"escaped text counts as data", CodeLimitTextSize,
			in(func(w *Writer) error { return w.Text(strings.Repeat("&", 20)) }), in(func(w *Writer) error { return w.Text(strings.Repeat("&", 21)) })},
		{"text after markup starts again", "",
			in(func(w *Writer) error {
				return seq(w.Text(x(20)), w.Comment(""), w.Text(x(20)), w.CDATA(x(20)), w.Text(x(20)))
			}), nil},
		{"CDATA", CodeLimitTextSize, in(func(w *Writer) error { return w.CDATA(x(20)) }), in(func(w *Writer) error { return w.CDATA(x(21)) })},
		{"comment", CodeLimitTextSize, in(func(w *Writer) error { return w.Comment(x(20)) }), in(func(w *Writer) error { return w.Comment(x(21)) })},
		{"PI data", CodeLimitTextSize, in(func(w *Writer) error { return w.ProcInst("p", x(20)) }), in(func(w *Writer) error { return w.ProcInst("p", x(21)) })},
		{"attribute value", CodeLimitTextSize,
			in(func(w *Writer) error { return w.Attr(n("a"), x(20)) }), in(func(w *Writer) error { return w.Attr(n("a"), x(21)) })},
	} {
		out, err := write(Options{Limits: lim}, c.at)
		if err != nil {
			t.Errorf("%s at the limit: %v", c.name, err)
			continue
		}
		r := xmlreader.New(strings.NewReader(out), xmlreader.Options{Limits: rlim})
		for err = nil; err == nil; {
			_, err = r.Next()
		}
		if _, isErr := err.(*xmlreader.Error); isErr {
			t.Errorf("%s at the limit: the reader with the same limits rejects it: %v\n%s", c.name, err, out)
		}
		if c.over == nil {
			continue
		}
		if _, err := write(Options{Limits: lim}, c.over); code(err) != c.code {
			t.Errorf("%s over the limit: %v, want %s", c.name, err, c.code)
		}
		// bez limitów pisarz zapisuje, a czytnik z limitami odrzuca tym samym kodem
		out, err = write(Options{Limits: Limits{MaxDepth: -1, MaxNameLength: -1, MaxAttributes: -1, MaxTextSize: -1}}, func(w *Writer) error {
			err := c.over(w)
			for len(w.stack) > 0 {
				w.End()
			}
			return err
		})
		if err != nil {
			t.Errorf("%s without limits: %v", c.name, err)
			continue
		}
		r = xmlreader.New(strings.NewReader(out), xmlreader.Options{Limits: rlim})
		for err = nil; err == nil; {
			_, err = r.Next()
		}
		if e, _ := err.(*xmlreader.Error); e == nil || e.Code != c.code {
			t.Errorf("%s over the limit: the reader says %v, the writer %s\n%s", c.name, err, c.code, out)
		}
	}
	// ustawienia domyślne: dokument na granicy głębokości i liczby atrybutów czytnik domyślny czyta
	out, err := write(Options{}, in(func(w *Writer) error { return seq(nested(d.MaxDepth-1)(w), attrs(0, d.MaxAttributes)(w)) }))
	if err != nil {
		t.Fatalf("document at the default limits: %v", err)
	}
	if _, err := streamDump([]byte(out), false); err != nil {
		t.Errorf("the default reader rejects a document at the default limits of the writer: %v", err)
	}
}
