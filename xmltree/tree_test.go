package xmltree

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

func parse(t testing.TB, src string) *Document {
	t.Helper()
	d, err := Parse(strings.NewReader(src), Options{})
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return d
}

// str zapisuje dokument albo węzeł (jako element główny) pisarzem domyślnym; błąd zapisu kończy test.
func str(t testing.TB, x interface {
	WriteTo(*xmlwriter.Writer) error
}) string {
	t.Helper()
	var out bytes.Buffer
	w := xmlwriter.New(&out, xmlwriter.Options{})
	if err := x.WriteTo(w); err != nil {
		t.Fatalf("WriteTo: %v (written: %s)", err, out.String())
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return out.String()
}

func writeErr(x interface {
	WriteTo(*xmlwriter.Writer) error
}) error {
	w := xmlwriter.New(io.Discard, xmlwriter.Options{})
	if err := x.WriteTo(w); err != nil {
		return err
	}
	return w.Close()
}

func code(err error) string {
	var te *Error
	var we *xmlwriter.Error
	var re *xmlreader.Error
	switch {
	case err == nil:
		return ""
	case errors.As(err, &te):
		return te.Code
	case errors.As(err, &we):
		return we.Code
	case errors.As(err, &re):
		return re.Code
	}
	return "other: " + err.Error()
}

func el(local string) *Node { return NewElement(Name{Local: local}) }

func child(t testing.TB, n *Node, i int) *Node {
	t.Helper()
	c := n.FirstChild()
	for ; i > 0 && c != nil; i-- {
		c = c.NextSibling()
	}
	if c == nil {
		t.Fatalf("no such child")
	}
	return c
}

// checkLinks sprawdza spójność drzewa: każde dziecko wskazuje swojego rodzica i żaden węzeł nie występuje dwa razy.
func checkLinks(t testing.TB, root *Node) {
	t.Helper()
	seen := map[*Node]bool{}
	var visit func(n *Node)
	visit = func(n *Node) {
		if seen[n] {
			t.Fatalf("node %v is in the tree twice", n.Name())
		}
		seen[n] = true
		for _, c := range n.Children() {
			if c.Parent() != n {
				t.Fatalf("child %q of %q has parent %v", c.Name().Local+c.Data(), n.Name().Local, c.Parent())
			}
			visit(c)
		}
	}
	visit(root)
}

// TestDocumentShape: dokument ma węzły przed korzeniem i po nim, korzeń i treść deklaracji XML; węzły mają dane
// ze źródła, a dane nie zależą od bufora czytnika.
// REQ: TR-001 TR-003
func TestDocumentShape(t *testing.T) {
	src := "<?xml version=\"1.0\" standalone=\"yes\"?>\n<!--before--><?pi  data ?>\n" +
		"<!DOCTYPE r [<!ATTLIST r d CDATA \"dflt\" i ID #IMPLIED>]>\n" +
		"<r xmlns=\"urn:d\" xmlns:p=\"urn:p\" b=\"2\" p:a=\"1\" i=\"x\">t1<![CDATA[c<d]]>t2<e/><f></f><!--in--><?q?></r>\n<!--after-->\n"
	d := parse(t, src)
	if !d.XMLDeclaration || d.Standalone != "yes" || !d.Doctype {
		t.Errorf("declaration %v standalone %q doctype %v", d.XMLDeclaration, d.Standalone, d.Doctype)
	}
	var kinds []Kind
	for _, c := range d.Node().Children() {
		kinds = append(kinds, c.Kind())
		if c.Parent() != d.Node() {
			t.Errorf("a top-level node has parent %v", c.Parent())
		}
	}
	// białych znaków poza elementem głównym czytnik nie daje, więc nie ma ich też w drzewie
	want := []Kind{KindComment, KindProcInst, KindElement, KindComment}
	if len(kinds) != len(want) {
		t.Fatalf("top-level kinds %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("top-level kinds %v, want %v", kinds, want)
		}
	}
	if d.Node().Kind() != KindDocument || d.Node().Parent() != nil {
		t.Error("the document node")
	}
	pi := child(t, d.Node(), 1)
	if pi.Name().Local != "pi" || pi.Data() != "data " {
		t.Errorf("pi %q %q", pi.Name().Local, pi.Data())
	}
	r := d.Root()
	if r.Name() != (Name{Space: "urn:d", Local: "r"}) {
		t.Errorf("root name %+v", r.Name())
	}
	if ns := r.Namespaces(); len(ns) != 2 || ns[0] != (Binding{URI: "urn:d"}) || ns[1] != (Binding{Prefix: "p", URI: "urn:p"}) {
		t.Errorf("namespaces %+v", ns)
	}
	var attrs []string
	for _, a := range r.Attrs() {
		s := a.Name.Local + "=" + a.Value
		if !a.Specified {
			s += " default"
		}
		if a.Type != "" {
			s += " " + a.Type
		}
		attrs = append(attrs, s)
	}
	if got := strings.Join(attrs, "|"); got != "b=2|a=1|i=x ID|d=dflt default CDATA" {
		t.Errorf("attributes %s", got)
	}
	if v, ok := r.Attr("urn:p", "a"); !ok || v != "1" {
		t.Errorf("Attr(urn:p, a) = %q, %v", v, ok)
	}
	if _, ok := r.Attr("", "a"); ok {
		t.Error("Attr finds p:a without its namespace")
	}
	var content []string
	for _, c := range r.Children() {
		switch c.Kind() {
		case KindText:
			if c.IsCDATA() {
				content = append(content, "cdata:"+c.Data())
			} else {
				content = append(content, "text:"+c.Data())
			}
		case KindElement:
			content = append(content, "element:"+c.Name().Local+map[bool]string{true: "/", false: ""}[c.SelfClosing()])
		case KindComment:
			content = append(content, "comment:"+c.Data())
		case KindProcInst:
			content = append(content, "pi:"+c.Name().Local)
		}
	}
	if got := strings.Join(content, "|"); got != "text:t1|cdata:c<d|text:t2|element:e/|element:f|comment:in|pi:q" {
		t.Errorf("content %s", got)
	}
	if got := r.Text(); got != "t1c<dt2" {
		t.Errorf("Text %q", got)
	}
	checkLinks(t, d.Node())

	// dane są kopiami: drugi dokument czytany tym samym czytnikiem nie zmienia pierwszego
	rd := xmlreader.New(strings.NewReader(`<a x="1">first</a>`), xmlreader.Options{})
	d1, err := ReadDocument(rd, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rd.Reset(strings.NewReader(`<b y="2">other</b>`), xmlreader.Options{})
	if _, err := ReadDocument(rd, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := str(t, d1); got != `<a x="1">first</a>` {
		t.Errorf("the first document after the reader was reused: %s", got)
	}
}

// TestSourceErrors: błąd źródła wraca bez zmiany kodu i pozycji; źródło, które nie daje dokumentu, to błąd drzewa.
// REQ: TR-001
func TestSourceErrors(t *testing.T) {
	_, err := Parse(strings.NewReader("<a>\n  <b></c></a>"), Options{})
	var re *xmlreader.Error
	if !errors.As(err, &re) || re.Code != xmlreader.CodeElementTypeMatch || re.Line != 2 {
		t.Errorf("error of the reader: %v", err)
	}
	if _, direct := err.(*xmlreader.Error); !direct {
		t.Errorf("the error of the reader is wrapped: %T", err)
	}
	_, err = Parse(strings.NewReader("<a/>"), Options{Reader: xmlreader.Options{Limits: xmlreader.Limits{MaxDepth: -1, MaxNameLength: 1 << 20}}})
	if err != nil {
		t.Errorf("reader options: %v", err)
	}
	tok := func(kind xmlreader.Kind, local string) xmlreader.Token {
		return xmlreader.Token{Kind: kind, Name: Name{Local: local}, Data: []byte(local)}
	}
	for name, tc := range map[string]struct {
		tokens []xmlreader.Token
		want   string
	}{
		"no root":            {nil, CodeNoRoot},
		"only a comment":     {[]xmlreader.Token{tok(xmlreader.KindComment, "c")}, CodeNoRoot},
		"end without start":  {[]xmlreader.Token{tok(xmlreader.KindEndElement, "a")}, CodeSource},
		"eof in an element":  {[]xmlreader.Token{tok(xmlreader.KindStartElement, "a")}, CodeSource},
		"unknown kind":       {[]xmlreader.Token{tok(99, "a")}, CodeSource},
		"two roots":          {[]xmlreader.Token{tok(xmlreader.KindStartElement, "a"), tok(xmlreader.KindEndElement, "a"), tok(xmlreader.KindStartElement, "b")}, CodeOutsideRoot},
		"text outside root":  {[]xmlreader.Token{tok(xmlreader.KindText, "x")}, CodeOutsideRoot},
		"cdata outside root": {[]xmlreader.Token{{Kind: xmlreader.KindText, CDATA: true}}, CodeOutsideRoot},
	} {
		if _, err := ReadDocument(&sliceSource{tokens: tc.tokens}, Options{}); code(err) != tc.want {
			t.Errorf("%s: %v, want %s", name, err, tc.want)
		}
	}
	if _, err := ReadElement(&sliceSource{}, tok(xmlreader.KindText, "x"), Options{}); code(err) != CodeSource {
		t.Errorf("ReadElement with a text token: %v", err)
	}
	if _, err := ReadElement(&sliceSource{}, tok(xmlreader.KindStartElement, "a"), Options{}); code(err) != CodeSource {
		t.Errorf("ReadElement on a source that ends: %v", err)
	}
	boom := errors.New("boom")
	if _, err := ReadDocument(&sliceSource{err: boom}, Options{}); err != boom {
		t.Errorf("an error of the source: %v", err)
	}
}

type sliceSource struct {
	tokens []xmlreader.Token
	err    error
}

func (s *sliceSource) Next() (xmlreader.Token, error) {
	if len(s.tokens) == 0 {
		if s.err != nil {
			return xmlreader.Token{}, s.err
		}
		return xmlreader.Token{}, io.EOF
	}
	t := s.tokens[0]
	s.tokens = s.tokens[1:]
	return t, nil
}

// TestPositions: węzeł i atrybut ze źródła mają wiersz i kolumnę swojego tokenu, węzeł utworzony w pamięci zera.
// REQ: TR-004
func TestPositions(t *testing.T) {
	d := parse(t, "<a>\n  <b x='1'\n     y='2'>text</b>\n  <!--c--><?p?>\n</a>")
	b := child(t, d.Root(), 1)
	pos := func(n *Node) [2]int { return [2]int{n.Line(), n.Column()} }
	if pos(d.Root()) != [2]int{1, 1} || pos(b) != [2]int{2, 3} || pos(child(t, b, 0)) != [2]int{3, 12} {
		t.Errorf("positions: root %v, b %v, text %v", pos(d.Root()), pos(b), pos(child(t, b, 0)))
	}
	if a := b.Attrs()[1]; a.Line != 3 || a.Column != 6 {
		t.Errorf("attribute y at %d:%d", a.Line, a.Column)
	}
	if c := child(t, d.Root(), 3); pos(c) != [2]int{4, 3} || pos(child(t, d.Root(), 4)) != [2]int{4, 11} {
		t.Errorf("comment at %v, pi at %v", pos(c), pos(child(t, d.Root(), 4)))
	}
	for _, n := range []*Node{el("a"), NewText("t"), NewCDATA("c"), NewComment("c"), NewProcInst("p", "d")} {
		if n.Line() != 0 || n.Column() != 0 {
			t.Errorf("a node made in memory has a position: %d:%d", n.Line(), n.Column())
		}
	}
	// kopia zachowuje pozycję źródła, a strumień drzewa ją oddaje
	if c := b.Clone(); pos(c) != pos(b) {
		t.Errorf("clone at %v", pos(c))
	}
	r := NewReader(b)
	if tok, _ := r.Next(); tok.Line != 2 || tok.Column != 3 {
		t.Errorf("token of the tree reader at %d:%d", tok.Line, tok.Column)
	}
}

// TestLookupNamespace: LookupNamespace i InScopeNamespaces na każdym elemencie dają to samo co czytnik w tym
// miejscu dokumentu; poddrzewo z ReadElement widzi deklaracje przodków spoza poddrzewa.
// REQ: TR-005
func TestLookupNamespace(t *testing.T) {
	src := `<a xmlns="urn:d" xmlns:p="urn:p1" xmlns:q="urn:q"><p:b xmlns:p="urn:p2"><c xmlns=""><d xmlns="urn:d2" xmlns:r="urn:r"/></c></p:b><e/></a>`
	prefixes := []string{"", "p", "q", "r", "xml", "none"}
	// czytnik: zakres każdego elementu w kolejności dokumentu
	type scope struct {
		lookups []string
		inScope []Binding
	}
	var want []scope
	rd := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
	for {
		tok, err := rd.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if tok.Kind != xmlreader.KindStartElement {
			continue
		}
		var s scope
		for _, p := range prefixes {
			uri, ok := rd.LookupNamespace(p)
			s.lookups = append(s.lookups, uri+map[bool]string{true: " bound", false: " unbound"}[ok])
		}
		s.inScope = rd.InScopeNamespaces()
		want = append(want, s)
	}
	same := func(n *Node, s scope, what string) {
		t.Helper()
		for i, p := range prefixes {
			uri, ok := n.LookupNamespace(p)
			if got := uri + map[bool]string{true: " bound", false: " unbound"}[ok]; got != s.lookups[i] {
				t.Errorf("%s <%s>: LookupNamespace(%q) = %s, the reader says %s", what, n.Name().Local, p, got, s.lookups[i])
			}
		}
		got := n.InScopeNamespaces()
		if len(got) != len(s.inScope) {
			t.Errorf("%s <%s>: InScopeNamespaces %v, the reader says %v", what, n.Name().Local, got, s.inScope)
			return
		}
		for i := range got {
			if got[i] != s.inScope[i] {
				t.Errorf("%s <%s>: InScopeNamespaces %v, the reader says %v", what, n.Name().Local, got, s.inScope)
			}
		}
	}
	d := parse(t, src)
	i := 0
	for it := newWalk(d.Root()); ; {
		n, enter := it.next()
		if n == nil {
			break
		}
		if enter && n.Kind() == KindElement {
			same(n, want[i], "document")
			i++
		}
	}
	if i != len(want) || i != 5 {
		t.Fatalf("%d elements, reader %d", i, len(want))
	}
	if got := d.Root().InScopeNamespaces(); len(got) != 3 || got[0].Prefix != "" || got[1].Prefix != "p" || got[2].Prefix != "q" {
		t.Errorf("InScopeNamespaces is not sorted by prefix: %v", got)
	}
	// tekst pyta swojego elementu
	if uri, ok := child(t, parse(t, `<a xmlns:p="urn:p">t</a>`).Root(), 0).LookupNamespace("p"); !ok || uri != "urn:p" {
		t.Errorf("LookupNamespace on a text node: %q %v", uri, ok)
	}

	// poddrzewo każdego elementu, wczytane w środku strumienia: ten sam zakres w każdym jego elemencie
	for k := 0; k < len(want); k++ {
		rd := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
		for n := 0; ; {
			tok, err := rd.Next()
			if err != nil {
				t.Fatal(err)
			}
			if tok.Kind != xmlreader.KindStartElement {
				continue
			}
			if n == k {
				sub, err := ReadElement(rd, tok, Options{})
				if err != nil {
					t.Fatal(err)
				}
				if sub.Parent() != nil {
					t.Error("a subtree has a parent")
				}
				j := k
				for it := newWalk(sub); ; {
					m, enter := it.next()
					if m == nil {
						break
					}
					if enter && m.Kind() == KindElement {
						same(m, want[j], "subtree")
						j++
					}
				}
				break
			}
			n++
		}
	}
}

// TestNodeLimit: limit węzłów obejmuje elementy, teksty, komentarze i PI; przekroczenie daje limit-nodes z pozycją
// węzła ponad limit i żadnego drzewa. Zero to wartość domyślna, wartość ujemna brak limitu.
// REQ: TR-006
func TestNodeLimit(t *testing.T) {
	src := "<a><b/>text<!--c--><?p?>\n<e/></a>" // 7 węzłów: a, b, text, komentarz, PI, tekst, e
	if d, err := Parse(strings.NewReader(src), Options{MaxNodes: 7}); err != nil || d == nil {
		t.Errorf("7 nodes with a limit of 7: %v", err)
	}
	for limit, at := range map[int]int{6: 1, 5: 25, 4: 20, 3: 12, 2: 8, 1: 4} {
		d, err := Parse(strings.NewReader(src), Options{MaxNodes: limit})
		var te *Error
		if !errors.As(err, &te) || te.Code != CodeLimitNodes || d != nil {
			t.Errorf("limit %d: %v, document %v", limit, err, d)
			continue
		}
		if line := map[bool]int{true: 2, false: 1}[limit == 6]; te.Line != line || te.Column != at {
			t.Errorf("limit %d: error at %d:%d, want %d:%d", limit, te.Line, te.Column, line, at)
		}
	}
	if _, err := Parse(strings.NewReader(src), Options{MaxNodes: -1}); err != nil {
		t.Errorf("no limit: %v", err)
	}
	if DefaultMaxNodes != 2_000_000 || maxNodes(Options{}) != DefaultMaxNodes {
		t.Errorf("default limit %d", maxNodes(Options{}))
	}
	// ReadElement liczy węzły swojego poddrzewa
	rd := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
	tok, _ := rd.Next()
	if n, err := ReadElement(rd, tok, Options{MaxNodes: 3}); code(err) != CodeLimitNodes || n != nil {
		t.Errorf("ReadElement over the limit: %v", err)
	}
}

// TestChanges: dodanie na końcu, wstawienie przed dzieckiem, usunięcie, zastąpienie, atrybuty, tekst i tworzenie
// węzłów każdego rodzaju; po każdej operacji rodzice i kolejność dzieci są spójne.
// REQ: TR-007
func TestChanges(t *testing.T) {
	d := parse(t, `<a><b/><c/></a>`)
	a := d.Root()
	b, c := child(t, a, 0), child(t, a, 1)
	step := func(want string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%v", err)
		}
		checkLinks(t, d.Node())
		if got := str(t, d); got != want {
			t.Fatalf("got  %s\nwant %s", got, want)
		}
	}
	x := el("x")
	step(`<a><b/><c/><x/></a>`, a.Append(x))
	if x.Parent() != a {
		t.Error("parent after Append")
	}
	step(`<a><b/>text<c/><x/></a>`, a.InsertBefore(NewText("text"), c))
	step(`<a><!--first--><b/>text<c/><x/></a>`, a.InsertBefore(NewComment("first"), b))
	step(`<a><!--first--><b/>text<c/><x/><?pi d?></a>`, a.InsertBefore(NewProcInst("pi", "d"), nil))
	step(`<a><!--first--><b/>text<c/><x><![CDATA[<raw>]]></x><?pi d?></a>`, x.Append(NewCDATA("<raw>")))
	step(`<a><!--first--><b/>text<x><![CDATA[<raw>]]></x><?pi d?></a>`, a.Remove(c))
	if c.Parent() != nil {
		t.Error("a removed node has a parent")
	}
	step(`<a><!--first--><c/>text<x><![CDATA[<raw>]]></x><?pi d?></a>`, a.Replace(b, c))
	if b.Parent() != nil || c.Parent() != a {
		t.Error("parents after Replace")
	}
	step(`<a><!--first--><c/>text<x><![CDATA[<raw>]]></x><?pi d?></a>`, a.Replace(c, c))
	// usunięty węzeł wraca w inne miejsce razem ze swoimi dziećmi
	step(`<a><!--first--><c/>text<?pi d?></a>`, a.Remove(x))
	step(`<a><!--first--><c><x><![CDATA[<raw>]]></x></c>text<?pi d?></a>`, c.Append(x))

	// atrybuty: nowy na końcu, istniejący zachowuje miejsce i prefiks
	c.SetAttr(Name{Local: "k"}, "1")
	c.SetAttr(Name{Space: "urn:p", Local: "m", Prefix: "p"}, "2")
	c.SetAttr(Name{Local: "z"}, "3")
	step(`<a><!--first--><c xmlns:p="urn:p" k="1" p:m="2" z="3"><x><![CDATA[<raw>]]></x></c>text<?pi d?></a>`, nil)
	c.SetAttr(Name{Local: "k"}, "<new>")
	c.SetAttr(Name{Space: "urn:p", Local: "m", Prefix: "other"}, "22")
	step(`<a><!--first--><c xmlns:p="urn:p" k="&lt;new>" p:m="22" z="3"><x><![CDATA[<raw>]]></x></c>text<?pi d?></a>`, nil)
	if !c.RemoveAttr("", "k") || c.RemoveAttr("", "k") || c.RemoveAttr("", "m") || !c.RemoveAttr("urn:p", "m") {
		t.Error("RemoveAttr")
	}
	step(`<a><!--first--><c z="3"><x><![CDATA[<raw>]]></x></c>text<?pi d?></a>`, nil)
	if v, ok := c.Attr("", "z"); !ok || v != "3" || len(c.Attrs()) != 1 || !c.Attrs()[0].Specified {
		t.Errorf("attributes after the changes: %+v", c.Attrs())
	}

	// tekst elementu
	step(`<a><!--first--><c z="3">a &lt; b</c>text<?pi d?></a>`, c.SetText("a < b"))
	if x.Parent() != nil || c.Text() != "a < b" || len(c.Children()) != 1 {
		t.Error("SetText left the old children")
	}
	step(`<a><!--first--><c z="3"/>text<?pi d?></a>`, c.SetText(""))
	if got := a.Text(); got != "text" {
		t.Errorf("Text %q", got)
	}

	// dokument budowany od zera, z węzłami na najwyższym poziomie
	n := NewDocument()
	n.XMLDeclaration = true
	root := el("root")
	if err := errors.Join(n.Node().Append(NewComment("c")), n.Node().Append(NewText("\n")), n.Node().Append(root),
		root.Append(el("empty")), root.Append(NewText("t")), n.Node().Append(NewProcInst("after", ""))); err != nil {
		t.Fatal(err)
	}
	if n.Root() != root || root.Parent() != n.Node() {
		t.Error("Root of a new document")
	}
	checkLinks(t, n.Node())
	if got, want := str(t, n), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!--c-->\n<root><empty/>t</root><?after?>"; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	// element bez dzieci: <a/> z NewElement, <a></a> po wyłączeniu SelfClosing, a z dziećmi zawsze znacznik końca
	e := el("e")
	e.SetSelfClosing(false)
	if got := str(t, e); got != "<e></e>" {
		t.Errorf("SelfClosing false: %s", got)
	}
}

// TestOneParent: węzeł ma najwyżej jednego rodzica, nie da się go wstawić do własnego potomka, a kopia głęboka
// nie ma rodzica i nie dzieli niczego z oryginałem.
// REQ: TR-008
func TestOneParent(t *testing.T) {
	d := parse(t, `<a><b><c x="1" xmlns:p="urn:p">t</c></b><d/></a>`)
	a := d.Root()
	b, dd := child(t, a, 0), child(t, a, 1)
	c := child(t, b, 0)
	before := str(t, d)
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"append a node that has a parent":  {dd.Append(c), CodeHasParent},
		"insert a node that has a parent":  {a.InsertBefore(c, dd), CodeHasParent},
		"replace by a node with a parent":  {a.Replace(dd, c), CodeHasParent},
		"a node into itself":               {a.Append(a), CodeHasParent}, // a ma rodzica: dokument
		"the root into its descendant":     {c.Append(a), CodeHasParent},
		"a child into a text node":         {child(t, c, 0).Append(el("x")), CodeNodeKind},
		"a nil child":                      {a.Append(nil), CodeNodeKind},
		"a document node as a child":       {a.Append(NewDocument().Node()), CodeNodeKind},
		"the zero Node":                    {a.Append(&Node{}), CodeNodeKind},
		"a reference that is not a child":  {a.InsertBefore(el("x"), c), CodeNotChild},
		"remove what is not a child":       {a.Remove(c), CodeNotChild},
		"remove a node without a parent":   {a.Remove(el("x")), CodeNotChild},
		"replace what is not a child":      {a.Replace(c, el("x")), CodeNotChild},
		"SetText on a text node":           {child(t, c, 0).SetText("x"), CodeNodeKind},
		"SetText on a document":            {d.Node().SetText("x"), CodeNodeKind},
		"a second root element":            {d.Node().Append(el("x")), CodeOutsideRoot},
		"text outside the root element":    {d.Node().Append(NewText("x")), CodeOutsideRoot},
		"CDATA outside the root element":   {d.Node().Append(NewCDATA(" ")), CodeOutsideRoot},
		"a second root by InsertBefore":    {d.Node().InsertBefore(el("x"), a), CodeOutsideRoot},
		"white space outside the root":     {d.Node().Append(NewText("\n")), ""},
		"a comment outside the root":       {d.Node().Append(NewComment("c")), ""},
		"the root replaced by another one": {d.Node().Replace(a, a), ""},
	} {
		if code(tc.err) != tc.want {
			t.Errorf("%s: %v, want %q", name, tc.err, tc.want)
		}
	}
	checkLinks(t, d.Node())
	if got := str(t, d); got != before+"\n<!--c-->" {
		t.Errorf("refused changes changed the tree:\n got  %s\n want %s", got, before+"\n<!--c-->")
	}
	// cykl: węzeł bez rodzica do własnego potomka
	if err := a.Remove(b); err != nil {
		t.Fatal(err)
	}
	if err := c.Append(b); code(err) != CodeCycle {
		t.Errorf("a node into its descendant: %v", err)
	}
	if err := b.Append(b); code(err) != CodeCycle {
		t.Errorf("a node into itself: %v", err)
	}
	if err := a.InsertBefore(b, dd); err != nil {
		t.Fatal(err)
	}
	// zastąpienie korzenia innym elementem
	other := el("other")
	if err := d.Node().Replace(a, other); err != nil || d.Root() != other || a.Parent() != nil {
		t.Errorf("replacing the root: %v", err)
	}
	if err := d.Node().Replace(other, a); err != nil {
		t.Fatal(err)
	}

	// kopia głęboka
	cp := b.Clone()
	if cp.Parent() != nil || cp == b || child(t, cp, 0) == c || child(t, cp, 0).Parent() != cp {
		t.Error("links of the clone")
	}
	checkLinks(t, cp)
	if got := str(t, cp); got != `<b><c xmlns:p="urn:p" x="1">t</c></b>` {
		t.Errorf("clone: %s", got)
	}
	cc := child(t, cp, 0)
	cc.SetAttr(Name{Local: "x"}, "changed")
	cc.DeclareNamespace("p", "urn:changed")
	child(t, cc, 0).SetData("changed")
	if got := str(t, d); got != before+"\n<!--c-->" {
		t.Errorf("a change of the clone changed the original: %s", got)
	}
	if err := dd.Append(cp); err != nil {
		t.Errorf("a clone can be put into the tree: %v", err)
	}
	checkLinks(t, d.Node())
}

// TestIllFormedChanges: zmiana, która dałaby dokument źle uformowany, jest błędem ze stałym kodem najpóźniej przy
// zapisie.
// REQ: TR-009
func TestIllFormedChanges(t *testing.T) {
	for name, tc := range map[string]struct {
		change func(d *Document)
		want   string
	}{
		"a name that is not a name":     {func(d *Document) { d.Root().SetName(Name{Local: "1a"}) }, xmlwriter.CodeName},
		"an empty name":                 {func(d *Document) { d.Root().Append(el("")) }, xmlwriter.CodeName},
		"an attribute name with space":  {func(d *Document) { d.Root().SetAttr(Name{Local: "a b"}, "") }, xmlwriter.CodeName},
		"an attribute given twice":      {func(d *Document) { d.Root().SetAttrs(append(d.Root().Attrs(), Attr{Name: Name{Local: "x"}})) }, xmlwriter.CodeUniqueAttSpec},
		"a character XML does not have": {func(d *Document) { d.Root().SetText("\x01") }, xmlwriter.CodeChar},
		"a comment with two hyphens":    {func(d *Document) { d.Root().Append(NewComment("a--b")) }, xmlwriter.CodeComment},
		"a CDATA section with its end":  {func(d *Document) { d.Root().Append(NewCDATA("]]>")) }, xmlwriter.CodeCDATAEnd},
		"a PI named xml":                {func(d *Document) { d.Root().Append(NewProcInst("XML", "")) }, xmlwriter.CodeReservedPITarget},
		"a document without a root":     {func(d *Document) { d.Node().Remove(d.Root()) }, xmlwriter.CodeNoRoot},
		"text made non-white outside":   {func(d *Document) { d.Node().FirstChild().SetData("x") }, xmlwriter.CodeOutsideRoot},
		"a carriage return outside":     {func(d *Document) { d.Node().FirstChild().SetData("\r") }, xmlwriter.CodeLineBreak},
		"a name against a declaration":  {func(d *Document) { d.Root().SetName(Name{Space: "urn:other", Local: "a", Prefix: "p"}) }, xmlwriter.CodeNamespaceName},
		"a prefix without a namespace":  {func(d *Document) { d.Root().SetName(Name{Local: "a", Prefix: "q"}) }, xmlwriter.CodePrefixDeclared},
		"a standalone that is not one":  {func(d *Document) { d.XMLDeclaration, d.Standalone = true, "maybe" }, xmlwriter.CodeXMLDecl},
	} {
		d := parse(t, "<a x='1' xmlns:p='urn:p'/>")
		if err := d.Node().InsertBefore(NewText("\n"), d.Root()); err != nil {
			t.Fatal(err)
		}
		tc.change(d)
		if got := code(writeErr(d)); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

// TestMovedNamespaces: węzeł przeniesiony albo skopiowany w miejsce o innych deklaracjach zapisuje się poprawnie:
// brakujące deklaracje są dopisane w miejscu użycia, nazwy rozwinięte po odczycie są takie same, prefiksy z treści
// zachowują znaczenie, a wynik jest ten sam w każdym przebiegu.
// REQ: TR-011 DET-002
func TestMovedNamespaces(t *testing.T) {
	src := `<a xmlns="urn:d" xmlns:p="urn:p" xmlns:q="urn:q"><p:b p:x="1" type="q:T"><c/></p:b><plain xmlns="" xmlns:p="urn:other"><keep/></plain></a>`
	d := parse(t, src)
	a := d.Root()
	b, plain := child(t, a, 0), child(t, a, 1)
	if err := a.Remove(b); err != nil {
		t.Fatal(err)
	}
	// wyjęty element pamięta zakres: nazwy rozwiązuje jak w źródle
	for prefix, want := range map[string]string{"": "urn:d", "p": "urn:p", "q": "urn:q"} {
		if uri, ok := b.LookupNamespace(prefix); !ok || uri != want {
			t.Errorf("a removed element: LookupNamespace(%q) = %q, %v", prefix, uri, ok)
		}
	}
	// zapisany sam: deklaruje wszystko, co miał w zakresie (także q, użyte tylko w wartości atrybutu)
	if got, want := str(t, b), `<p:b xmlns="urn:d" xmlns:p="urn:p" xmlns:q="urn:q" p:x="1" type="q:T"><c/></p:b>`; got != want {
		t.Errorf("alone:\n got  %s\n want %s", got, want)
	}
	// wstawiony tam, gdzie p znaczy co innego, a przestrzeń domyślna jest zniesiona
	if err := plain.Append(b); err != nil {
		t.Fatal(err)
	}
	want := `<a xmlns="urn:d" xmlns:p="urn:p" xmlns:q="urn:q"><plain xmlns="" xmlns:p="urn:other"><keep/>` +
		`<p:b xmlns="urn:d" xmlns:p="urn:p" p:x="1" type="q:T"><c/></p:b></plain></a>`
	got := str(t, d)
	if got != want {
		t.Errorf("moved:\n got  %s\n want %s", got, want)
	}
	for i := 0; i < 10; i++ {
		if again := str(t, d); again != got {
			t.Fatalf("run %d gives other bytes", i)
		}
	}
	// nazwy rozwinięte po odczycie: takie jak w drzewie
	back := parse(t, got)
	moved := child(t, child(t, back.Root(), 0), 1)
	if moved.Name() != b.Name() || moved.Attrs()[0].Name != b.Attrs()[0].Name || child(t, moved, 0).Name() != (Name{Space: "urn:d", Local: "c"}) {
		t.Errorf("expanded names after the move: %+v %+v %+v", moved.Name(), moved.Attrs()[0].Name, child(t, moved, 0).Name())
	}
	if uri, _ := moved.LookupNamespace("q"); uri != "urn:q" {
		t.Errorf("the prefix used in content is bound to %q", uri)
	}
	// LookupNamespace w drzewie odpowiada temu, co zostanie zapisane
	for _, prefix := range []string{"", "p", "q"} {
		inTree, _ := b.LookupNamespace(prefix)
		written, _ := moved.LookupNamespace(prefix)
		if inTree != written {
			t.Errorf("LookupNamespace(%q): %q in the tree, %q after writing", prefix, inTree, written)
		}
	}
	// strumień drzewa niesie te same deklaracje co zapis
	var out bytes.Buffer
	w := xmlwriter.New(&out, xmlwriter.Options{})
	for r := NewReader(d.Node()); ; {
		tok, err := r.Next()
		if err == io.EOF {
			break
		}
		if err == nil {
			err = w.WriteToken(tok)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil || out.String() != want {
		t.Errorf("NewReader → writer:\n got  %s (%v)\n want %s", out.String(), err, want)
	}

	// element bez przestrzeni nazw wstawiony pod przestrzeń domyślną dostaje xmlns=""
	d = parse(t, `<a xmlns="urn:d"/>`)
	d.Root().Append(el("n"))
	if got, want := str(t, d), `<a xmlns="urn:d"><n xmlns=""/></a>`; got != want {
		t.Errorf("no namespace under a default namespace:\n got  %s\n want %s", got, want)
	}
	// element i atrybuty utworzone w pamięci: deklaracje w miejscu pierwszego użycia, nie wyżej i nie dwa razy
	d = parse(t, `<a/>`)
	n1 := NewElement(Name{Space: "urn:x", Local: "n", Prefix: "x"})
	n1.SetAttr(Name{Space: "urn:y", Local: "k", Prefix: "y"}, "v")
	n1.SetAttr(Name{Space: "urn:x", Local: "m", Prefix: "x"}, "w")
	n2 := NewElement(Name{Space: "urn:x", Local: "inner", Prefix: "x"})
	n3 := NewElement(Name{Space: "urn:dflt", Local: "d"})
	if err := errors.Join(d.Root().Append(n1), n1.Append(n2), n2.Append(n3), n3.Append(NewElement(Name{Space: "urn:dflt", Local: "e"}))); err != nil {
		t.Fatal(err)
	}
	if got, want := str(t, d), `<a><x:n xmlns:x="urn:x" xmlns:y="urn:y" y:k="v" x:m="w"><x:inner><d xmlns="urn:dflt"><e/></d></x:inner></x:n></a>`; got != want {
		t.Errorf("made in memory:\n got  %s\n want %s", got, want)
	}
	// kopia zapisuje się poprawnie w innym dokumencie
	src2 := parse(t, `<r xmlns:p="urn:p"><p:item p:a="1"/></r>`)
	dst := parse(t, `<list/>`)
	item := child(t, src2.Root(), 0)
	dst.Root().Append(item.Clone())
	dst.Root().Append(item.Clone())
	if got, want := str(t, dst), `<list><p:item xmlns:p="urn:p" p:a="1"/><p:item xmlns:p="urn:p" p:a="1"/></list>`; got != want {
		t.Errorf("clones in another document:\n got  %s\n want %s", got, want)
	}
	// węzeł wyjęty i włożony w to samo miejsce nie dostaje niczego
	d = parse(t, src)
	a, b = d.Root(), child(t, d.Root(), 0)
	a.Remove(b)
	a.InsertBefore(b, child(t, a, 0))
	if got := str(t, d); got != src {
		t.Errorf("removed and put back:\n got  %s\n want %s", got, src)
	}
	// wyspa zapisana w innym miejscu: poddrzewo z ReadElement niesie deklaracje przodków
	rd := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
	rd.Next()
	tok, _ := rd.Next()
	sub, err := ReadElement(rd, tok, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := str(t, sub), `<p:b xmlns="urn:d" xmlns:p="urn:p" xmlns:q="urn:q" p:x="1" type="q:T"><c/></p:b>`; got != want {
		t.Errorf("a subtree from a stream written alone:\n got  %s\n want %s", got, want)
	}
	// tryb automatyczny pisarza naprawia także nazwę sprzeczną z deklaracją własną elementu
	bad := NewElement(Name{Space: "urn:one", Local: "n", Prefix: "p"})
	bad.SetNamespaces([]Binding{{Prefix: "p", URI: "urn:two"}})
	if code(writeErr(bad)) != xmlwriter.CodeNamespaceName {
		t.Errorf("a name against its own declaration: %v", writeErr(bad))
	}
	var auto bytes.Buffer
	aw := xmlwriter.New(&auto, xmlwriter.Options{AutoNamespaces: true})
	if err := errors.Join(bad.WriteTo(aw), aw.Close()); err != nil || auto.String() != `<ns1:n xmlns:p="urn:two" xmlns:ns1="urn:one"/>` {
		t.Errorf("AutoNamespaces: %s (%v)", auto.String(), err)
	}
}

// TestIDs: rejestr identyfikatorów z atrybutów typu ID, xml:id i nazw z opcji; powtórzenie to błąd ze stałym
// kodem i pozycjami obu wystąpień; po zmianach rejestr przebudowuje się jawnie.
// REQ: TR-013
func TestIDs(t *testing.T) {
	src := "<!DOCTYPE r [<!ATTLIST e key ID #IMPLIED>]>\n" +
		"<r Id='root'>\n <e key='k1'/>\n <e xml:id='  x1  '/>\n <e Id='i1' other='o1'/>\n <e Id=''/>\n <e xml:id='same' Id='same'/>\n</r>"
	d, err := Parse(strings.NewReader(src), Options{IDAttributes: []string{"Id"}})
	if err != nil {
		t.Fatal(err)
	}
	if d.IDs() != 5 {
		t.Errorf("%d identifiers", d.IDs())
	}
	for id, line := range map[string]int{"root": 2, "k1": 3, "x1": 4, "i1": 5, "same": 7} {
		if n, ok := d.ByID(id); !ok || n.Line() != line {
			t.Errorf("ByID(%q) = %v, %v", id, n, ok)
		}
	}
	for _, id := range []string{"", "o1", "none", "  x1  "} {
		if _, ok := d.ByID(id); ok {
			t.Errorf("ByID(%q) finds an element", id)
		}
	}
	// bez nazw z opcji: tylko typ ID i xml:id; bez opcji: bez rejestru
	if d, _ := Parse(strings.NewReader(src), Options{IDs: true}); d.IDs() != 3 {
		t.Errorf("IDs without names: %d identifiers", d.IDs())
	}
	plain := parse(t, src)
	if _, ok := plain.ByID("k1"); ok || plain.IDs() != 0 {
		t.Error("an index without the option")
	}
	// powtórzenie
	dup := "<r>\n <a Id='x'/>\n <b>\n   <c Id='x'/></b></r>"
	_, err = Parse(strings.NewReader(dup), Options{IDAttributes: []string{"Id"}})
	var te *Error
	if !errors.As(err, &te) || te.Code != CodeIDDuplicate || te.Line != 4 || te.Column != 7 || te.FirstLine != 2 || te.FirstColumn != 5 {
		t.Errorf("duplicate: %+v", err)
	}
	if !strings.HasPrefix(te.Error(), "4:7: id-duplicate: ") {
		t.Errorf("message %q", te.Error())
	}
	// po zmianach: jawna przebudowa
	e := el("new")
	e.SetAttr(Name{Local: "Id"}, "added")
	d.Root().Append(e)
	if _, ok := d.ByID("added"); ok {
		t.Error("the index follows changes by itself")
	}
	if err := d.IndexIDs("Id"); err != nil {
		t.Fatal(err)
	}
	if n, ok := d.ByID("added"); !ok || n != e || d.IDs() != 6 {
		t.Errorf("after IndexIDs: %v %v, %d identifiers", n, ok, d.IDs())
	}
	e2 := el("new")
	e2.SetAttr(Name{Local: "Id"}, "added")
	d.Root().Append(e2)
	if err := d.IndexIDs("Id"); code(err) != CodeIDDuplicate || d.IDs() != 0 {
		t.Errorf("a duplicate made by a change: %v, %d identifiers", err, d.IDs())
	}
	if (&Error{Code: CodeIDDuplicate, Message: "m"}).Error() != "id-duplicate: m" {
		t.Error("message of an error without a position")
	}
}

// TestTreeReader: strumień drzewa zmienionego w pamięci czytany przez pisarza daje to samo co WriteTo, pusty
// tekst nie daje tokenu, a Whitespace i SelfClosing są takie, jakie dałby czytnik po zapisie.
// REQ: TR-012 STR-007
func TestTreeReader(t *testing.T) {
	d := parse(t, `<a><b/></a>`)
	a := d.Root()
	a.Append(NewText(""))
	a.Append(NewText(" \n"))
	a.Append(NewCDATA(""))
	e := el("e")
	a.Append(e)
	e.Append(NewText("x"))
	child(t, a, 0).SetSelfClosing(false)
	got, err := dump(NewReader(d.Node()))
	if err != nil {
		t.Fatal(err)
	}
	want, err := dumpBytes([]byte(str(t, d)), false)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("the tree reader and the reader of the written tree differ\n%s\n got:\n%s", firstDiff(got, want), got)
	}
	r := NewReader(e)
	var kinds []xmlreader.Kind
	for {
		tok, err := r.Next()
		if err == io.EOF {
			break
		}
		kinds = append(kinds, tok.Kind)
	}
	if len(kinds) != 3 || kinds[0] != xmlreader.KindStartElement || kinds[2] != xmlreader.KindEndElement {
		t.Errorf("tokens of a subtree: %v", kinds)
	}
	if _, err := r.Next(); err != io.EOF {
		t.Errorf("after the end: %v", err)
	}
	if tok, err := NewReader(NewText("t")).Next(); err != nil || tok.Kind != xmlreader.KindText || string(tok.Data) != "t" {
		t.Errorf("a reader of a text node: %+v %v", tok, err)
	}
	if _, err := NewReader(&Node{}).Next(); code(err) != CodeNodeKind {
		t.Errorf("the zero Node: %v", err)
	}
	// drzewo jako źródło drzewa
	again, err := ReadDocument(NewReader(d.Node()), Options{})
	if err != nil || str(t, again) != str(t, d) {
		t.Errorf("a tree built from a tree reader: %v", err)
	}
}

// TestAccessors: odczyt i zmiana danych węzła idą przez metody; metoda niepasująca do rodzaju węzła jest błędem
// i niczego nie zmienia, a rodzaj węzła nie zmienia się nigdy. Attrs oddaje kopię, Namespaces widok o pojemności
// równej długości: zmiana ani dopisanie do nich nie zmienia drzewa ani sąsiedniego elementu.
// REQ: TR-003 TR-007 TR-015
func TestAccessors(t *testing.T) {
	d := parse(t, `<a x="1" xmlns:p="urn:p"><b y="2" xmlns:q="urn:q"/>text<![CDATA[c]]><!--k--><?pi d?><e/></a>`)
	a := d.Root()
	b, text, cdata, comment, pi, e := child(t, a, 0), child(t, a, 1), child(t, a, 2), child(t, a, 3), child(t, a, 4), child(t, a, 5)
	// rodzeństwo i dzieci
	if a.FirstChild() != b || a.LastChild() != e || b.NextSibling() != text || text.PrevSibling() != b ||
		b.PrevSibling() != nil || e.NextSibling() != nil || text.FirstChild() != nil || text.LastChild() != nil {
		t.Error("sibling links")
	}
	kids := a.Children()
	if len(kids) != 6 || kids[0] != b || kids[5] != e {
		t.Fatalf("Children: %d nodes", len(kids))
	}
	kids[0], kids[5] = kids[5], nil // kopia: zmiana wycinka nie zmienia drzewa
	if a.FirstChild() != b || a.LastChild() != e || len(text.Children()) != 0 {
		t.Error("Children is not a copy")
	}
	// rodzaje i dane
	for n, want := range map[*Node]Kind{d.Node(): KindDocument, a: KindElement, text: KindText, cdata: KindText, comment: KindComment, pi: KindProcInst} {
		if n.Kind() != want {
			t.Errorf("Kind %d, want %d", n.Kind(), want)
		}
	}
	if !cdata.IsCDATA() || text.IsCDATA() || !b.SelfClosing() || a.SelfClosing() || pi.Name().Local != "pi" ||
		pi.Data() != "d" || comment.Data() != "k" || text.Name() != (Name{}) || len(text.Attrs()) != 0 || len(text.Namespaces()) != 0 {
		t.Error("data of the nodes")
	}
	// metody niepasujące do rodzaju
	for name, err := range map[string]error{
		"SetName on text":            text.SetName(Name{Local: "x"}),
		"SetName on a comment":       comment.SetName(Name{Local: "x"}),
		"SetName on a document":      d.Node().SetName(Name{Local: "x"}),
		"SetData on an element":      a.SetData("x"),
		"SetData on a document":      d.Node().SetData("x"),
		"SetCDATA on a comment":      comment.SetCDATA(true),
		"SetCDATA on an element":     a.SetCDATA(true),
		"SetSelfClosing on text":     text.SetSelfClosing(true),
		"SetAttr on text":            text.SetAttr(Name{Local: "x"}, "1"),
		"SetAttr on a PI":            pi.SetAttr(Name{Local: "x"}, "1"),
		"SetAttrs on a comment":      comment.SetAttrs(nil),
		"SetNamespaces on text":      text.SetNamespaces(nil),
		"DeclareNamespace on a PI":   pi.DeclareNamespace("p", "urn:p"),
		"DeclareNamespace on a doc":  d.Node().DeclareNamespace("p", "urn:p"),
		"SetAttr on the zero Node":   (&Node{}).SetAttr(Name{Local: "x"}, "1"),
		"SetText on the zero Node":   (&Node{}).SetText("x"),
		"Append to the zero Node":    (&Node{}).Append(el("x")),
		"SetName on a nil node":      (*Node)(nil).SetName(Name{Local: "x"}),
		"Remove on a nil node":       (*Node)(nil).Remove(a),
		"Remove of a nil child":      a.Remove(nil),
		"Replace of a nil child":     a.Replace(nil, el("x")),
		"InsertBefore on a nil node": (*Node)(nil).InsertBefore(el("x"), b),
	} {
		var te *Error
		if !errors.As(err, &te) || (te.Code != CodeNodeKind && te.Code != CodeNotChild) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if text.RemoveAttr("", "x") || (&Node{}).Kind() != 0 || (&Node{}).FirstChild() != nil || (&Node{}).Text() != "" {
		t.Error("reads of nodes without the data")
	}
	if code(writeErr(&Node{})) != CodeNodeKind {
		t.Error("the zero Node can be written")
	}
	// zmiany danych
	if err := errors.Join(a.SetName(Name{Local: "root"}), text.SetData("T"), cdata.SetCDATA(false), comment.SetData("K"),
		pi.SetName(Name{Local: "target"}), pi.SetData("D"), b.SetSelfClosing(false), e.DeclareNamespace("", "urn:e"),
		e.SetName(Name{Space: "urn:e", Local: "e"}), a.DeclareNamespace("p", "urn:p2"), a.DeclareNamespace("r", "urn:r"),
		b.SetAttrs([]Attr{{Name: Name{Local: "z"}, Value: "26"}, {Name: Name{Local: "y"}, Value: "25"}})); err != nil {
		t.Fatal(err)
	}
	want := `<root xmlns:p="urn:p2" xmlns:r="urn:r" x="1"><b xmlns:q="urn:q" z="26" y="25"></b>Tc<!--K--><?target D?><e xmlns="urn:e"/></root>`
	if got := str(t, d); got != want {
		t.Errorf("after the changes:\n got  %s\n want %s", got, want)
	}
	// SetAttrs i SetNamespaces kopiują argument
	attrs := []Attr{{Name: Name{Local: "k"}, Value: "v"}}
	decls := []Binding{{Prefix: "s", URI: "urn:s"}}
	e.SetAttrs(attrs)
	e.SetNamespaces(decls)
	attrs[0].Value, decls[0].URI = "changed", "urn:changed"
	if v, _ := e.Attr("", "k"); v != "v" || e.Namespaces()[0].URI != "urn:s" {
		t.Error("SetAttrs or SetNamespaces keeps the slice of the caller")
	}

	// atrybuty: Attrs oddaje kopię, NumAttrs i AttrAt czytają bez alokacji i oddają to samo co Attrs
	d = parse(t, "<!DOCTYPE r [<!ATTLIST r i ID #IMPLIED d CDATA 'dflt'>]>\n<r xmlns:p='urn:p' i='x'\n   p:a='1'/>")
	r := d.Root()
	list := r.Attrs()
	if r.NumAttrs() != 3 || len(list) != 3 {
		t.Fatalf("%d attributes, Attrs gives %d", r.NumAttrs(), len(list))
	}
	for i, want := range []Attr{
		{Name: Name{Local: "i"}, Value: "x", Type: xmlreader.AttrTypeID, Specified: true, Line: 2, Column: 20},
		{Name: Name{Space: "urn:p", Local: "a", Prefix: "p"}, Value: "1", Specified: true, Line: 3, Column: 4},
		{Name: Name{Local: "d"}, Value: "dflt", Type: xmlreader.AttrTypeCDATA},
	} {
		got := r.AttrAt(i)
		if got.Name != want.Name || got.Value != want.Value || got.Type != want.Type || got.Specified != want.Specified ||
			(want.Line != 0 && (got.Line != want.Line || got.Column != want.Column)) || got != list[i] {
			t.Errorf("attribute %d: %+v, want %+v (Attrs gives %+v)", i, got, want, list[i])
		}
	}
	if r.AttrAt(-1) != (Attr{}) || r.AttrAt(3) != (Attr{}) || text.NumAttrs() != 0 || text.AttrAt(0) != (Attr{}) {
		t.Error("AttrAt outside the attributes")
	}
	list[0].Value, list[0].Name.Local = "changed", "changed"
	if v, ok := r.Attr("", "i"); !ok || v != "x" {
		t.Error("a change of the slice from Attrs changed the tree")
	}
	if a := testing.AllocsPerRun(20, func() {
		for i := 0; i < r.NumAttrs(); i++ {
			_ = r.AttrAt(i)
		}
		_, _ = r.Attr("urn:p", "a")
	}); a != 0 {
		t.Errorf("NumAttrs, AttrAt and Attr allocate %v times", a)
	}
	// typ spoza stałych czytnika i nazwy ustawione przez SetAttrs wracają bez zmian
	custom := []Attr{{Name: Name{Local: "k"}, Value: "v", Type: "MY-TYPE", Line: 7, Column: 9}}
	if err := r.SetAttrs(custom); err != nil || r.AttrAt(0) != custom[0] {
		t.Errorf("SetAttrs: %+v (%v)", r.AttrAt(0), err)
	}

	// widoki: elementy sąsiadujące w pamięci (jedna porcja atrybutów i deklaracji) nie widzą dopisań do widoku
	d = parse(t, `<r><a x="1" xmlns:p="urn:p"/><b y="2" xmlns:q="urn:q"/></r>`)
	first, second := child(t, d.Root(), 0), child(t, d.Root(), 1)
	_ = append(first.Attrs(), Attr{Name: Name{Local: "evil"}, Value: "!"})
	_ = append(first.Namespaces(), Binding{Prefix: "evil", URI: "urn:evil"})
	if err := errors.Join(first.SetAttr(Name{Local: "new"}, "n"), first.DeclareNamespace("n", "urn:n")); err != nil {
		t.Fatal(err)
	}
	if got, want := str(t, d), `<r><a xmlns:p="urn:p" xmlns:n="urn:n" x="1" new="n"/><b xmlns:q="urn:q" y="2"/></r>`; got != want {
		t.Errorf("appending to a view or to the first element changed its neighbour:\n got  %s\n want %s", got, want)
	}
	if a := second.Attrs(); len(a) != 1 || a[0].Name.Local != "y" || cap(first.Namespaces()) != len(first.Namespaces()) {
		t.Errorf("attributes of the second element: %+v", a)
	}
}

// TestConstantTimeChanges: dodanie, wstawienie i usunięcie dziecka nie zależą od liczby dzieci. Element z 200 000
// dzieci jest opróżniany po jednym dziecku od początku, od końca i ze środka; przy koszcie liniowym na operację
// byłoby to 10¹⁰ kroków.
// REQ: TR-014
func TestConstantTimeChanges(t *testing.T) {
	const n = 200_000
	root := el("r")
	kids := make([]*Node, n)
	for i := range kids {
		kids[i] = el("k")
		if err := root.Append(kids[i]); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	for i := 0; i < n/2; i += 2 { // od początku i od końca na przemian
		if err := errors.Join(root.Remove(kids[i]), root.Remove(kids[n-1-i])); err != nil {
			t.Fatal(err)
		}
	}
	mid := kids[n/2]
	for i := 0; i < n/4; i++ { // wstawianie w środek i usuwanie stamtąd
		x := el("x")
		if err := errors.Join(root.InsertBefore(x, mid), root.Remove(x)); err != nil {
			t.Fatal(err)
		}
	}
	for root.FirstChild() != nil {
		if err := root.Remove(root.LastChild()); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("%d changes of a child list of %d took %v: not constant time per change", n+n/2, n, d)
	}
	if root.FirstChild() != nil || root.LastChild() != nil || kids[0].Parent() != nil {
		t.Error("links after emptying the element")
	}
}

// TestConcurrentReads: metody odczytu nie zmieniają drzewa, więc drzewo, którego nikt nie zmienia, wolno czytać
// z wielu gorutyn naraz (sprawdza to detektor wyścigów: go test -race).
// REQ: TR-015
func TestConcurrentReads(t *testing.T) {
	d := parse(t, `<a xmlns="urn:d" xmlns:p="urn:p" Id="r"><p:b p:x="1">t<c Id="k"/></p:b><!--c--><d>u</d></a>`)
	if err := d.IndexIDs("Id"); err != nil {
		t.Fatal(err)
	}
	want := str(t, d)
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				var out bytes.Buffer
				w := xmlwriter.New(&out, xmlwriter.Options{})
				if err := errors.Join(d.WriteTo(w), w.Close()); err != nil || out.String() != want {
					t.Errorf("concurrent WriteTo: %v", err)
					return
				}
				if s, err := dump(NewReader(d.Node())); err != nil || s == "" {
					t.Errorf("concurrent NewReader: %v", err)
				}
				n, _ := d.ByID("k")
				if uri, _ := n.LookupNamespace("p"); uri != "urn:p" || len(n.InScopeNamespaces()) != 2 || d.Root().Text() != "tu" {
					t.Error("concurrent reads")
				}
				_ = d.Root().Clone()
			}
		}()
	}
	wg.Wait()
}
