package xmltree

import (
	"strings"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// The types shared with the reader and the writer.
type (
	Name    = xmlreader.Name
	Attr    = xmlreader.Attr
	Binding = xmlreader.Binding
)

const nsXML = "http://www.w3.org/XML/1998/namespace"

// Kind is the kind of a node.
type Kind uint8

const (
	// KindDocument is the node of a Document: its children are the nodes at the top level.
	KindDocument Kind = iota + 1
	KindElement
	// KindText is character data or, with Node.IsCDATA, one CDATA section.
	KindText
	KindComment
	KindProcInst
)

// Node is a node of a tree. Nodes are made by the functions that build a tree (Parse, ReadDocument, ReadElement)
// and by NewElement, NewText, NewCDATA, NewComment and NewProcInst; the zero Node is not a node of any kind and
// every operation refuses it. The kind of a node never changes.
//
// All access is through methods, so the links between nodes are always consistent: a node has at most one
// parent, the children of a node are a list without repetitions, and no node is inside itself. The methods that
// read do not change the tree, so a tree that nobody changes may be read from several goroutines.
type Node struct {
	parent, prev, next *Node
	// el: dane, które ma tylko element (i dokument: dzieci; PI: nazwa). Węzeł tekstu i komentarz go nie mają,
	// więc zajmują trzecią część pamięci elementu.
	el           *elem
	data         string
	line, column int
	kind         Kind
	cdata        bool
	selfClosing  bool
}

type elem struct {
	first, last *Node
	// name wskazuje nazwę przechowywaną raz na budowę (elementy o tej samej nazwie dzielą jeden zapis); nazwa pod
	// wskaźnikiem nigdy nie jest zmieniana w miejscu, SetName podstawia nowy wskaźnik.
	name  *Name
	attrs []Attr
	// ns: deklaracje przestrzeni nazw i zapamiętany kontekst; nil u elementów, które nie mają ani jednego,
	// ani drugiego (czyli u większości).
	ns *nsData
}

type nsData struct {
	decls []Binding
	// context: przestrzenie nazw, które były w zakresie nad węzłem w miejscu, z którego go wyjęto albo wczytano
	// (ReadElement, Remove, Clone), posortowane według prefiksu, z wpisem {"", ""}, gdy przestrzeń domyślna nie
	// była związana. nil: węzeł nie ma zapamiętanego miejsca. Wycinek nigdy nie jest zmieniany w miejscu.
	context []Binding
}

func (e *elem) nm() Name {
	if e.name == nil {
		return Name{}
	}
	return *e.name
}

func (e *elem) decls() []Binding {
	if e.ns == nil {
		return nil
	}
	return e.ns.decls
}

func (e *elem) ctx() []Binding {
	if e.ns == nil {
		return nil
	}
	return e.ns.context
}

func (e *elem) setDecls(d []Binding) {
	if e.ns == nil {
		if len(d) == 0 {
			return
		}
		e.ns = &nsData{}
	}
	e.ns.decls = d
}

func (e *elem) setCtx(c []Binding) {
	if e.ns == nil {
		e.ns = &nsData{}
	}
	e.ns.context = c
}

// withElem to węzeł razem z danymi elementu w jednym przydziale pamięci.
type withElem struct {
	n    Node
	e    elem
	name Name
}

func newNode(kind Kind) *Node {
	if kind == KindText || kind == KindComment {
		return &Node{kind: kind}
	}
	x := &withElem{}
	x.n.kind, x.n.el = kind, &x.e
	return &x.n
}

// newNamed tworzy element albo PI z nazwą we własnym przydziale węzła.
func newNamed(kind Kind, name Name) *Node {
	x := &withElem{name: name}
	x.n.kind, x.n.el, x.e.name = kind, &x.e, &x.name
	return &x.n
}

// setName podstawia węzłowi nowy zapis nazwy: dotychczasowy może być wspólny z innymi elementami tej samej budowy
// albo z kopią węzła, więc nie wolno go zmieniać w miejscu.
func (n *Node) setName(name Name) {
	n.el.name = &Name{Space: name.Space, Local: name.Local, Prefix: name.Prefix}
}

// Odczyt danych elementu z węzła dowolnego rodzaju: węzeł bez nich daje wartości puste.

func (n *Node) firstChild() *Node {
	if n.el == nil {
		return nil
	}
	return n.el.first
}

func (n *Node) attrList() []Attr {
	if n.el == nil {
		return nil
	}
	return n.el.attrs
}

func (n *Node) declared() []Binding {
	if n.el == nil {
		return nil
	}
	return n.el.decls()
}

func (n *Node) remembered() []Binding {
	if n.el == nil {
		return nil
	}
	return n.el.ctx()
}

// NewElement returns an element without a parent. It is written as <a/> until it gets children (SelfClosing).
func NewElement(name Name) *Node {
	n := newNamed(KindElement, name)
	n.selfClosing = true
	return n
}

// NewText returns a text node.
func NewText(data string) *Node { return &Node{kind: KindText, data: data} }

// NewCDATA returns a text node that is written as a CDATA section.
func NewCDATA(data string) *Node { return &Node{kind: KindText, data: data, cdata: true} }

// NewComment returns a comment.
func NewComment(data string) *Node { return &Node{kind: KindComment, data: data} }

// NewProcInst returns a processing instruction.
func NewProcInst(target, data string) *Node {
	n := newNamed(KindProcInst, Name{Local: target})
	n.data = data
	return n
}

// Kind returns the kind of the node.
func (n *Node) Kind() Kind { return n.kind }

// Name returns the name of an element; for a processing instruction Local is its target. Other nodes have no name.
func (n *Node) Name() Name {
	if n.el == nil {
		return Name{}
	}
	return n.el.nm()
}

// Data returns the text of a text node, the content of a comment or the data of a processing instruction.
func (n *Node) Data() string { return n.data }

// IsCDATA reports whether a text node is written as a CDATA section.
func (n *Node) IsCDATA() bool { return n.cdata }

// SelfClosing reports whether an element without children is written as <a/> and not as <a></a>. It comes from
// the source; NewElement sets it.
func (n *Node) SelfClosing() bool { return n.selfClosing }

// Line returns the line of the node in the source, from 1; zero for a node that was not read from a source.
func (n *Node) Line() int { return n.line }

// Column returns the column of the node in the source, from 1, in characters; zero as for Line.
func (n *Node) Column() int { return n.column }

// Attrs returns the attributes of an element, in document order. Namespace declarations are not attributes. The
// slice is a view of the tree, valid until the attributes of the node are changed; change attributes with SetAttr,
// RemoveAttr and SetAttrs.
func (n *Node) Attrs() []Attr {
	a := n.attrList()
	return a[:len(a):len(a)]
}

// Namespaces returns the namespace declarations of an element, in document order; an empty URI with an empty
// prefix is xmlns="". The slice is a view of the tree, valid until the declarations of the node are changed.
func (n *Node) Namespaces() []Binding {
	d := n.declared()
	return d[:len(d):len(d)]
}

// Parent returns the parent of the node: an element, the node of the document, or nil.
func (n *Node) Parent() *Node { return n.parent }

// FirstChild returns the first child of an element or of a document node, or nil.
func (n *Node) FirstChild() *Node { return n.firstChild() }

// LastChild returns the last child, or nil.
func (n *Node) LastChild() *Node {
	if n.el == nil {
		return nil
	}
	return n.el.last
}

// NextSibling returns the node after n among the children of its parent, or nil.
func (n *Node) NextSibling() *Node { return n.next }

// PrevSibling returns the node before n among the children of its parent, or nil.
func (n *Node) PrevSibling() *Node { return n.prev }

// Children returns the children in document order, as a new slice. To walk the children without allocating use
// FirstChild and NextSibling.
func (n *Node) Children() []*Node {
	var out []*Node
	for c := n.firstChild(); c != nil; c = c.next {
		out = append(out, c)
	}
	return out
}

// SetName sets the name of an element or, with Local, the target of a processing instruction. CodeNodeKind for
// other nodes.
func (n *Node) SetName(name Name) error {
	if n == nil || (n.kind != KindElement && n.kind != KindProcInst) {
		return nodeErr(CodeNodeKind, n, "only an element and a processing instruction have a name")
	}
	n.setName(name)
	return nil
}

// SetData sets the text of a text node, the content of a comment or the data of a processing instruction.
// CodeNodeKind for other nodes (for an element use SetText).
func (n *Node) SetData(data string) error {
	if n == nil || (n.kind != KindText && n.kind != KindComment && n.kind != KindProcInst) {
		return nodeErr(CodeNodeKind, n, "only text, a comment and a processing instruction have data")
	}
	n.data = data
	return nil
}

// SetCDATA makes a text node a CDATA section or plain text. CodeNodeKind for other nodes.
func (n *Node) SetCDATA(cdata bool) error {
	if n == nil || n.kind != KindText {
		return nodeErr(CodeNodeKind, n, "only a text node can be a CDATA section")
	}
	n.cdata = cdata
	return nil
}

// SetSelfClosing chooses how an element without children is written: <a/> or <a></a>. CodeNodeKind for other
// nodes.
func (n *Node) SetSelfClosing(selfClosing bool) error {
	if n == nil || n.kind != KindElement {
		return nodeErr(CodeNodeKind, n, "only an element has an empty-element tag")
	}
	n.selfClosing = selfClosing
	return nil
}

// Append adds child as the last child of n. See InsertBefore for the errors.
func (n *Node) Append(child *Node) error { return n.InsertBefore(child, nil) }

// InsertBefore adds child to the children of n before ref, or as the last child when ref is nil.
//
// Errors (*Error): CodeNodeKind when n is not an element or a document node, or child is nil, a document node or
// the zero Node; CodeHasParent when child has a parent (remove or clone it first); CodeCycle when child is n or
// an ancestor of n; CodeNotChild when ref is not a child of n; CodeOutsideRoot when n is a document node and
// child would be a second root element or text other than white space. A refused call changes nothing.
func (n *Node) InsertBefore(child, ref *Node) error {
	if ref != nil && (n == nil || ref.parent != n) {
		return nodeErr(CodeNotChild, ref, "the reference node is not a child of this node")
	}
	if err := n.accepts(child, nil); err != nil {
		return err
	}
	n.link(child, ref)
	return nil
}

// Remove takes child out of the children of n. The child keeps its own children and remembers the namespaces
// that were in scope, so it can be put somewhere else. CodeNotChild when child is not a child of n.
func (n *Node) Remove(child *Node) error {
	if n == nil || child == nil || child.parent != n {
		return nodeErr(CodeNotChild, child, "the node is not a child of this node")
	}
	n.unlink(child)
	return nil
}

// Replace puts child in the place of old, a child of n; old is taken out as by Remove. The errors are those of
// InsertBefore, with old in the role of ref.
func (n *Node) Replace(old, child *Node) error {
	if n == nil || old == nil || old.parent != n {
		return nodeErr(CodeNotChild, old, "the node to replace is not a child of this node")
	}
	if old == child {
		return nil
	}
	if err := n.accepts(child, old); err != nil {
		return err
	}
	after := old.next
	n.unlink(old)
	n.link(child, after)
	return nil
}

// SetText replaces the children of an element with one text node, or with nothing when data is empty.
// CodeNodeKind when n is not an element.
func (n *Node) SetText(data string) error {
	if n == nil || n.kind != KindElement {
		return nodeErr(CodeNodeKind, n, "only an element has text to set")
	}
	for n.el.first != nil {
		n.unlink(n.el.first)
	}
	if data != "" {
		n.link(NewText(data), nil)
	}
	return nil
}

// Text returns the character data of the node: Data of a text node, and for an element or a document the text of
// all text nodes below it, in document order (comments and processing instructions are left out).
func (n *Node) Text() string {
	switch {
	case n.kind == KindText:
		return n.data
	case n.firstChild() == nil:
		return ""
	case n.el.first == n.el.last && n.el.first.kind == KindText:
		return n.el.first.data
	}
	var b strings.Builder
	for it := newWalk(n); ; {
		c, enter := it.next()
		if c == nil {
			return b.String()
		}
		if enter && c.kind == KindText {
			b.WriteString(c.data)
		}
	}
}

// Attr returns the value of the attribute with the namespace name space and the local name local.
func (n *Node) Attr(space, local string) (string, bool) {
	attrs := n.attrList()
	for i := range attrs {
		if a := &attrs[i]; a.Name.Local == local && a.Name.Space == space {
			return a.Value, true
		}
	}
	return "", false
}

// SetAttr sets the value of an attribute of an element. An attribute with the same namespace name and local name
// keeps its place and its prefix; a new attribute is added at the end. CodeNodeKind when n is not an element.
func (n *Node) SetAttr(name Name, value string) error {
	if n == nil || n.kind != KindElement {
		return nodeErr(CodeNodeKind, n, "only an element has attributes")
	}
	e := n.el
	for i := range e.attrs {
		if a := &e.attrs[i]; a.Name.Local == name.Local && a.Name.Space == name.Space {
			a.Value, a.Specified = value, true
			return nil
		}
	}
	// wycinek mógł powstać z porcji wspólnej z innymi elementami: dopisanie zawsze kopiuje (pojemność = długość)
	e.attrs = append(e.attrs[:len(e.attrs):len(e.attrs)], Attr{Name: name, Value: value, Specified: true})
	return nil
}

// RemoveAttr removes an attribute and reports whether it was there.
func (n *Node) RemoveAttr(space, local string) bool {
	attrs := n.attrList()
	for i := range attrs {
		if a := &attrs[i]; a.Name.Local == local && a.Name.Space == space {
			out := make([]Attr, 0, len(attrs)-1)
			n.el.attrs = append(append(out, attrs[:i]...), attrs[i+1:]...)
			return true
		}
	}
	return false
}

// SetAttrs replaces all attributes of an element with a copy of attrs, in that order. CodeNodeKind when n is not
// an element. An attribute given twice is reported by the writer.
func (n *Node) SetAttrs(attrs []Attr) error {
	if n == nil || n.kind != KindElement {
		return nodeErr(CodeNodeKind, n, "only an element has attributes")
	}
	n.el.attrs = append([]Attr(nil), attrs...)
	return nil
}

// SetNamespaces replaces the namespace declarations of an element with a copy of declarations, in that order.
// CodeNodeKind when n is not an element. What is not a legal declaration is reported by the writer.
func (n *Node) SetNamespaces(declarations []Binding) error {
	if n == nil || n.kind != KindElement {
		return nodeErr(CodeNodeKind, n, "only an element has namespace declarations")
	}
	n.el.setDecls(append([]Binding(nil), declarations...))
	return nil
}

// DeclareNamespace adds the declaration of prefix to an element, or changes the one it has. An empty prefix is
// the default namespace; an empty uri with an empty prefix is xmlns="". CodeNodeKind when n is not an element.
func (n *Node) DeclareNamespace(prefix, uri string) error {
	if n == nil || n.kind != KindElement {
		return nodeErr(CodeNodeKind, n, "only an element has namespace declarations")
	}
	e := n.el
	d := e.decls()
	for i := range d {
		if d[i].Prefix == prefix {
			d[i].URI = uri
			return nil
		}
	}
	e.setDecls(append(d[:len(d):len(d)], Binding{Prefix: prefix, URI: uri}))
	return nil
}

// Clone returns a deep copy of the node and of everything below it. The copy has no parent, shares nothing with
// the original and remembers the namespaces in scope of the original, so it is written correctly anywhere.
func (n *Node) Clone() *Node {
	root := n.shallow()
	if n.kind == KindElement {
		root.el.setCtx(n.scopeAbove())
	}
	src, dst := n, root
	for {
		// w dół: pierwsze dziecko
		if f := src.firstChild(); f != nil {
			src = f
			c := src.shallow()
			dst.link(c, nil)
			dst = c
			continue
		}
		// w bok albo w górę
		for src != n && src.next == nil {
			src, dst = src.parent, dst.parent
		}
		if src == n {
			return root
		}
		src = src.next
		c := src.shallow()
		dst.parent.link(c, nil)
		dst = c
	}
}

// shallow kopiuje dane węzła bez powiązań.
func (n *Node) shallow() *Node {
	c := newNode(n.kind)
	c.data, c.line, c.column, c.cdata, c.selfClosing = n.data, n.line, n.column, n.cdata, n.selfClosing
	if n.el != nil && c.el != nil {
		c.el.name = n.el.name // nazwa pod wskaźnikiem jest niezmienna, więc kopia może ją dzielić
		if len(n.el.attrs) > 0 {
			c.el.attrs = append([]Attr(nil), n.el.attrs...)
		}
		if n.el.ns != nil {
			c.el.ns = &nsData{context: n.el.ns.context}
			if len(n.el.ns.decls) > 0 {
				c.el.ns.decls = append([]Binding(nil), n.el.ns.decls...)
			}
		}
	}
	return c
}

// accepts sprawdza, czy child może zostać dzieckiem n; replaced to dziecko, które child zastąpi (albo nil).
func (n *Node) accepts(child, replaced *Node) error {
	switch {
	case n == nil || (n.kind != KindElement && n.kind != KindDocument):
		return nodeErr(CodeNodeKind, n, "only an element and a document have children")
	case child == nil:
		return nodeErr(CodeNodeKind, n, "the child is nil")
	case child.kind == KindDocument:
		return nodeErr(CodeNodeKind, child, "a document node cannot be a child")
	case child.kind < KindElement || child.kind > KindProcInst:
		return nodeErr(CodeNodeKind, child, "the zero Node is not a node: use NewElement, NewText and the like")
	case child.parent != nil:
		return nodeErr(CodeHasParent, child, "the node has a parent: remove it or clone it first")
	}
	for a := n; a != nil; a = a.parent {
		if a == child {
			return nodeErr(CodeCycle, child, "a node cannot be put inside itself")
		}
	}
	if n.kind == KindDocument {
		switch child.kind {
		case KindElement:
			for c := n.el.first; c != nil; c = c.next {
				if c.kind == KindElement && c != replaced {
					return nodeErr(CodeOutsideRoot, child, "a document has one root element")
				}
			}
		case KindText:
			if child.cdata || !isSpace(child.data) {
				return nodeErr(CodeOutsideRoot, child, "text outside the root element")
			}
		}
	}
	return nil
}

// link wstawia child (bez rodzica) do dzieci n przed before albo na koniec. Jedyne miejsce, w którym węzeł
// dostaje rodzica.
func (n *Node) link(child, before *Node) {
	child.parent = n
	e := n.el
	if before == nil {
		child.prev = e.last
		if e.last != nil {
			e.last.next = child
		} else {
			e.first = child
		}
		e.last = child
		return
	}
	child.next, child.prev = before, before.prev
	if before.prev != nil {
		before.prev.next = child
	} else {
		e.first = child
	}
	before.prev = child
}

// unlink odłącza dziecko od n i zapamiętuje w nim przestrzenie nazw z zakresu. Jedyne miejsce, w którym węzeł
// traci rodzica.
func (n *Node) unlink(child *Node) {
	if child.kind == KindElement {
		child.el.setCtx(child.scopeAbove())
	}
	if child.prev != nil {
		child.prev.next = child.next
	} else {
		n.el.first = child.next
	}
	if child.next != nil {
		child.next.prev = child.prev
	} else {
		n.el.last = child.prev
	}
	child.parent, child.prev, child.next = nil, nil, nil
}

func isSpace[T string | []byte](s T) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return true
}

// walk przechodzi poddrzewo w kolejności dokumentu bez stosu i bez alokacji: każdy węzeł raz przy wejściu
// (enter), a węzeł z dziećmi drugi raz przy wyjściu. Drzewa nie wolno zmieniać w trakcie.
type walk struct {
	root, cur *Node
	begun     bool
	done      bool // cur jest załatwiony: liść po wejściu albo węzeł po wyjściu
}

func newWalk(root *Node) walk { return walk{root: root} }

func (w *walk) next() (n *Node, enter bool) {
	switch {
	case !w.begun:
		w.begun, w.cur = true, w.root
	case w.cur == nil:
		return nil, false
	case !w.done:
		w.cur = w.cur.el.first
	case w.cur == w.root:
		w.cur = nil
		return nil, false
	case w.cur.next != nil:
		w.cur = w.cur.next
	default:
		w.cur = w.cur.parent
		return w.cur, false // done zostaje: po wyjściu z węzła idziemy w bok albo wyżej
	}
	w.done = w.cur.firstChild() == nil
	return w.cur, true
}
