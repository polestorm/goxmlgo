package xmltree

import (
	"io"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// DefaultMaxNodes is the default limit of nodes in a tree built by Parse, ReadDocument and ReadElement.
const DefaultMaxNodes = 2_000_000

// Options configure Parse, ReadDocument and ReadElement. The zero value is the default.
type Options struct {
	// Reader are the options of the reader that Parse creates (limits, NoNamespaces).
	Reader xmlreader.Options
	// MaxNodes limits the number of nodes (elements, text nodes, comments, processing instructions) built by
	// one call; zero means DefaultMaxNodes, a negative value no limit. A source with more nodes gives
	// CodeLimitNodes and no tree. The limit bounds the memory of a tree only through the number of nodes: an
	// element takes about 200 bytes and an attribute about 110, whatever the size of the input, so a tree at the
	// default limit can take several hundred megabytes. Set a lower limit for input that is not trusted.
	MaxNodes int
	// IDs makes Parse and ReadDocument build the index of identifiers (Document.ByID) from the attributes of
	// type ID declared in the DTD and from xml:id.
	IDs bool
	// IDAttributes are local names of attributes without a namespace that are identifiers as well, such as
	// "Id". A name here turns the index on.
	IDAttributes []string
}

// Document is a whole document. A Document must not be copied: its nodes point at it.
type Document struct {
	// XMLDeclaration: the document has an XML declaration (WriteTo writes one). Standalone is the value of
	// standalone in it: "yes", "no" or empty.
	XMLDeclaration bool
	Standalone     string
	// Doctype: the source had a DOCTYPE declaration. It is not kept and not written; attributes it defaults are
	// in the tree (Attr.Specified is false) with the types it declares (Attr.Type).
	Doctype bool

	node Node
	top  elem
	ids  map[string]*Node
}

// NewDocument returns an empty document.
func NewDocument() *Document {
	d := &Document{}
	d.node.kind, d.node.el = KindDocument, &d.top
	return d
}

// Node returns the node of the document. Its children are the nodes at the top level: the root element, and
// comments, processing instructions and white space before and after it. It is the parent of those nodes.
func (d *Document) Node() *Node { return &d.node }

// Root returns the root element, or nil when the document has none yet.
func (d *Document) Root() *Node {
	for c := d.top.first; c != nil; c = c.next {
		if c.kind == KindElement {
			return c
		}
	}
	return nil
}

// Parse reads a whole document from r. An error of the reader is returned as it is (*xmlreader.Error, or the
// error of r); an error of the tree is an *Error.
func Parse(r io.Reader, opts Options) (*Document, error) {
	return ReadDocument(xmlreader.New(r, opts.Reader), opts)
}

// ReadDocument builds a document from all tokens of src, up to io.EOF. When src has the method Declaration of
// xmlreader.Reader, the XML declaration is taken from it. An error of src is returned as it is.
func ReadDocument(src xmlreader.TokenReader, opts Options) (*Document, error) {
	d := NewDocument()
	if ds, ok := src.(interface {
		Declaration() (xmlreader.Declaration, error)
	}); ok {
		decl, err := ds.Declaration()
		if err != nil {
			return nil, err
		}
		d.XMLDeclaration, d.Standalone = decl.Version != "", decl.Standalone
	}
	b := builder{src: src, max: maxNodes(opts), doc: d}
	if err := b.run(&d.node); err != nil {
		return nil, err
	}
	if d.Root() == nil {
		return nil, &Error{Code: CodeNoRoot, Message: "the source gave no root element"}
	}
	if opts.IDs || len(opts.IDAttributes) > 0 {
		if err := d.IndexIDs(opts.IDAttributes...); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// ReadElement builds the tree of one element in the middle of a stream. start is the KindStartElement token that
// src has just returned; ReadElement reads src up to the end tag of that element, so the next token of src is
// the one after it, as after Reader.Skip. The element has no parent. When src has the method InScopeNamespaces
// of xmlreader.Reader, the element remembers the namespaces in scope around it (LookupNamespace sees them, and
// WriteTo declares them where they are missing).
func ReadElement(src xmlreader.TokenReader, start xmlreader.Token, opts Options) (*Node, error) {
	if start.Kind != xmlreader.KindStartElement {
		return nil, &Error{Code: CodeSource, Line: start.Line, Column: start.Column,
			Message: "ReadElement needs the start tag token that the source has just returned"}
	}
	b := builder{src: src, max: maxNodes(opts)}
	n, err := b.element(&start)
	if err != nil {
		return nil, err
	}
	if sc, ok := src.(interface{ InScopeNamespaces() []Binding }); ok {
		n.el.setCtx(contextFrom(sc.InScopeNamespaces(), n.el.decls()))
	}
	if err := b.run(n); err != nil {
		return nil, err
	}
	return n, nil
}

// contextFrom buduje kontekst elementu z wiązań w zakresie elementu (razem z jego własnymi deklaracjami own):
// zostają te, których element sam nie deklaruje.
func contextFrom(inScope, own []Binding) []Binding {
	out := make([]Binding, 0, len(inScope)+1)
	hasDefault := false
next:
	for _, b := range inScope {
		for i := range own {
			if own[i].Prefix == b.Prefix {
				continue next
			}
		}
		if b.Prefix == "" {
			hasDefault = true
		}
		out = append(out, b)
	}
	if !hasDefault {
		out = append(out, Binding{})
	}
	if len(out) == 1 && out[0] == (Binding{}) {
		return unboundOnly
	}
	sortBindings(out)
	return out
}

func maxNodes(opts Options) int {
	if opts.MaxNodes == 0 {
		return DefaultMaxNodes
	}
	return opts.MaxNodes
}

// Pamięć drzewa budowanego ze źródła jest przydzielana porcjami: węzły, atrybuty i deklaracje z tablic, a dane
// tekstowe z jednego napisu na porcję (węzły dostają jego wycinki). Porcje węzłów rosną od małych, żeby poddrzewo
// kilku węzłów (ReadElement) nie rezerwowało pamięci na setki. Ceną jest to, że porcja żyje tak długo, jak
// którykolwiek jej węzeł albo wycinek.
const (
	firstNodeSlab = 8
	maxNodeSlab   = 512
	attrSlab      = 256
	bindingSlab   = 64
	textChunk     = 64 << 10
)

type builder struct {
	src   xmlreader.TokenReader
	max   int
	count int
	doc   *Document

	nodes    []Node
	elems    []elem
	nextSlab int
	attrs    []attr
	bindings []Binding
	// names: nazwy przechowywane raz. recent to mała tablica ostatnio użytych, pytana przed mapą: czytnik oddaje
	// te same nazwy tymi samymi napisami, więc porównanie trafionej pozycji kosztuje kilka porównań wskaźników.
	names    map[Name]*Name
	recent   [64]*Name
	nameSlab []Name
	text     []byte    // dane węzłów czekających na napis porcji
	pending  []textRef // węzły, których dane leżą w text
}

type textRef struct {
	n          *Node
	start, end int
}

func (b *builder) node(kind Kind, line, column int) (*Node, error) {
	b.count++
	if b.max > 0 && b.count > b.max {
		return nil, &Error{Code: CodeLimitNodes, Line: line, Column: column, Message: "more nodes than the limit"}
	}
	if len(b.nodes) == 0 {
		switch {
		case b.nextSlab == 0:
			b.nextSlab = firstNodeSlab
		case b.nextSlab < maxNodeSlab:
			b.nextSlab *= 2
		}
		b.nodes = make([]Node, b.nextSlab)
	}
	n := &b.nodes[0]
	b.nodes = b.nodes[1:]
	n.kind, n.line, n.column = kind, line, column
	if kind == KindElement || kind == KindProcInst {
		if len(b.elems) == 0 {
			b.elems = make([]elem, max(b.nextSlab/2, firstNodeSlab))
		}
		n.el = &b.elems[0]
		b.elems = b.elems[1:]
	}
	return n, nil
}

func (b *builder) element(t *xmlreader.Token) (*Node, error) {
	n, err := b.node(KindElement, t.Line, t.Column)
	if err != nil {
		return nil, err
	}
	e := n.el
	e.name, n.selfClosing = b.name(&t.Name), t.SelfClosing
	if k := len(t.Attrs); k > 0 {
		if k > len(b.attrs) {
			b.attrs = make([]attr, max(k, attrSlab))
		}
		e.attrs = b.attrs[:k:k] // pojemność równa długości: dopisanie atrybutu nie sięgnie do sąsiada
		b.attrs = b.attrs[k:]
		for i := range t.Attrs {
			e.attrs[i] = compact(&t.Attrs[i], b.name(&t.Attrs[i].Name))
		}
	}
	if k := len(t.Namespaces); k > 0 {
		if k > len(b.bindings) {
			b.bindings = make([]Binding, max(k, bindingSlab))
		}
		d := b.bindings[:k:k]
		b.bindings = b.bindings[k:]
		copy(d, t.Namespaces)
		e.ns = &nsData{decls: d}
	}
	return n, nil
}

// name zwraca wskaźnik na zapis nazwy wspólny dla wszystkich węzłów tej budowy o tej nazwie.
func (b *builder) name(n *Name) *Name {
	slot := (len(n.Local)*31 + len(n.Prefix)*7 + len(n.Space)) & 63
	if len(n.Local) > 0 {
		slot = (slot + int(n.Local[0])*13 + int(n.Local[len(n.Local)-1])*5) & 63
	}
	if p := b.recent[slot]; p != nil && *p == *n {
		return p
	}
	p, ok := b.names[*n]
	if !ok {
		if b.names == nil {
			b.names = map[Name]*Name{}
		}
		if len(b.nameSlab) == 0 {
			b.nameSlab = make([]Name, 32)
		}
		p = &b.nameSlab[0]
		b.nameSlab = b.nameSlab[1:]
		*p = *n
		b.names[*n] = p
	}
	b.recent[slot] = p
	return p
}

// setData zapisuje dane węzła: krótkie do wspólnej porcji tekstu, długie od razu jako własny napis.
func (b *builder) setData(n *Node, data []byte) {
	switch {
	case len(data) == 0:
	case len(data) >= textChunk/8:
		n.data = string(data)
	default:
		if len(b.text)+len(data) > textChunk {
			b.seal()
		}
		start := len(b.text)
		b.text = append(b.text, data...)
		b.pending = append(b.pending, textRef{n: n, start: start, end: len(b.text)})
	}
}

// seal zamienia porcję tekstu w jeden napis i rozdaje jego wycinki węzłom.
func (b *builder) seal() {
	if len(b.pending) == 0 {
		return
	}
	s := string(b.text)
	for _, r := range b.pending {
		r.n.data = s[r.start:r.end]
	}
	b.text, b.pending = b.text[:0], b.pending[:0]
}

// run czyta treść węzła top (elementu albo dokumentu) do jego znacznika końca albo, dla dokumentu, do io.EOF.
func (b *builder) run(top *Node) error {
	err := b.read(top)
	b.seal()
	return err
}

func (b *builder) read(top *Node) error {
	cur := top
	hasRoot := false
	for {
		t, err := b.src.Next()
		if err == io.EOF {
			if cur == top && top.kind == KindDocument {
				return nil
			}
			return &Error{Code: CodeSource, Line: cur.line, Column: cur.column,
				Message: "the source ended inside element " + cur.Name().Local}
		}
		if err != nil {
			return err
		}
		var n *Node
		switch t.Kind {
		case xmlreader.KindStartElement:
			if cur.kind == KindDocument {
				if hasRoot {
					return &Error{Code: CodeOutsideRoot, Line: t.Line, Column: t.Column, Message: "a document has one root element"}
				}
				hasRoot = true
			}
			if n, err = b.element(&t); err != nil {
				return err
			}
			cur.link(n, nil)
			cur = n
			continue
		case xmlreader.KindEndElement:
			if cur == top {
				if top.kind == KindDocument {
					return &Error{Code: CodeSource, Line: t.Line, Column: t.Column, Message: "an end tag without a start tag"}
				}
				return nil
			}
			cur = cur.parent
			continue
		case xmlreader.KindText:
			if cur.kind == KindDocument && (t.CDATA || !isSpace(t.Data)) {
				return &Error{Code: CodeOutsideRoot, Line: t.Line, Column: t.Column, Message: "text outside the root element"}
			}
			if n, err = b.node(KindText, t.Line, t.Column); err != nil {
				return err
			}
			n.cdata = t.CDATA
			b.setData(n, t.Data)
		case xmlreader.KindComment:
			if n, err = b.node(KindComment, t.Line, t.Column); err != nil {
				return err
			}
			b.setData(n, t.Data)
		case xmlreader.KindProcInst:
			if n, err = b.node(KindProcInst, t.Line, t.Column); err != nil {
				return err
			}
			n.el.name = b.name(&Name{Local: t.Name.Local})
			b.setData(n, t.Data)
		case xmlreader.KindDoctype:
			if b.doc != nil {
				b.doc.Doctype = true
			}
			continue
		default:
			return &Error{Code: CodeSource, Line: t.Line, Column: t.Column, Message: "unknown kind of token"}
		}
		cur.link(n, nil)
	}
}
