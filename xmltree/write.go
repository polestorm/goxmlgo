package xmltree

import (
	"io"

	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

// WriteTo writes the document through w: the XML declaration when the document has one, then the nodes. It does
// not call w.Close. See Node.WriteTo.
func (d *Document) WriteTo(w *xmlwriter.Writer) error {
	if d.XMLDeclaration {
		if err := w.Declaration(d.Standalone); err != nil {
			return err
		}
	}
	return d.node.WriteTo(w)
}

// WriteTo writes the node and everything below it through w, in the place where w is: an element as a child of
// the element open in w, or as the root element.
//
// An element is written with its own namespace declarations. What the place of writing lacks is declared on the
// element as well: the namespaces the element remembers from where it was taken, and the prefix of the name of
// the element and of each attribute when it is not bound to the namespace of the name. A name that disagrees
// with a declaration of its own element is not repaired: the writer reports it (unless it is in the mode
// AutoNamespaces, which chooses another prefix).
//
// The error is that of the writer (*xmlwriter.Error, or the error of its io.Writer), or an *Error with
// CodeNodeKind for a node of an unknown kind. After an error a part of the subtree may have been written.
func (n *Node) WriteTo(w *xmlwriter.Writer) error {
	fix := fixer{lookup: w.LookupNamespace}
	var all []Binding
	for it := newWalk(n); ; {
		c, enter := it.next()
		if c == nil {
			return nil
		}
		var err error
		switch c.kind {
		case KindDocument:
		case KindElement:
			if !enter {
				err = w.End()
				break
			}
			if extra := fix.element(c); len(extra) == 0 {
				err = w.Start(c.el.nm(), c.el.decls()...)
			} else {
				all = append(append(all[:0], c.el.decls()...), extra...)
				err = w.Start(c.el.nm(), all...)
			}
			for i := 0; err == nil && i < len(c.el.attrs); i++ {
				err = w.Attr(c.el.attrs[i].Name, c.el.attrs[i].Value)
			}
			if err == nil && c.el.first == nil {
				if c.selfClosing {
					err = w.EndEmpty()
				} else {
					err = w.End()
				}
			}
		case KindText:
			if c.cdata {
				err = w.CDATA(c.data)
			} else {
				err = w.Text(c.data)
			}
		case KindComment:
			err = w.Comment(c.data)
		case KindProcInst:
			err = w.ProcInst(c.el.nm().Local, c.data)
		default:
			err = nodeErr(CodeNodeKind, c, "unknown kind of node")
		}
		if err != nil {
			return err
		}
	}
}

// Reader gives the tokens of a tree; it is an xmlreader.TokenReader. Create one with NewReader.
type Reader struct {
	it      walk
	scope   xmlreader.NamespaceScope
	fix     fixer
	all     []Binding
	data    []byte
	pending *Node // element bez dzieci, którego token końca jest następny
}

// NewReader returns a source of the tokens of n and of everything below it, in document order: for a tree that
// was not changed, the tokens of the source it was built from (without the DOCTYPE token, and without the end
// positions). The tree must not be changed while it is read.
//
// The tokens are those of the subtree in its place: declarations of the ancestors of n, and the namespaces n
// remembers, are taken as being in scope and are not repeated. Inside the subtree a start tag token carries, as
// in WriteTo, the declarations of the element and those it needs beyond them. As with xmlreader.Reader,
// Token.Data and the slices of a token are valid until the next call of Next. As the reader gives no token for
// white space outside the root element, a text node at the top level of a document gives none either (WriteTo
// does write it).
func NewReader(n *Node) *Reader {
	r := &Reader{it: newWalk(n)}
	r.scope.Push(n.scopeAboveFor())
	r.fix.lookup = r.scope.Lookup
	return r
}

// scopeAboveFor: zakres nad węzłem dla czytnika drzewa; dla dokumentu pusty.
func (n *Node) scopeAboveFor() []Binding {
	if n.kind == KindDocument {
		return nil
	}
	return n.scopeAbove()
}

// Next returns the next token, or io.EOF after the last one.
func (r *Reader) Next() (xmlreader.Token, error) {
	if n := r.pending; n != nil {
		r.pending = nil
		r.scope.Pop()
		return xmlreader.Token{Kind: xmlreader.KindEndElement, Name: n.el.nm()}, nil
	}
	for {
		n, enter := r.it.next()
		if n == nil {
			return xmlreader.Token{}, io.EOF
		}
		switch n.kind {
		case KindElement:
			if !enter {
				r.scope.Pop()
				return xmlreader.Token{Kind: xmlreader.KindEndElement, Name: n.el.nm()}, nil
			}
			decls := n.el.decls()
			if extra := r.fix.element(n); len(extra) > 0 {
				r.all = append(append(r.all[:0], decls...), extra...)
				decls = r.all
			}
			r.scope.Push(decls)
			empty := n.el.first == nil
			if empty {
				r.pending = n
			}
			return xmlreader.Token{Kind: xmlreader.KindStartElement, Name: n.el.nm(), Attrs: n.el.attrs, Namespaces: decls,
				SelfClosing: empty && n.selfClosing, Line: n.line, Column: n.column}, nil
		case KindText:
			if (n.data == "" && !n.cdata) || (n.parent != nil && n.parent.kind == KindDocument) {
				continue // czytnik nie daje pustego tokenu tekstu ani białych znaków poza elementem głównym
			}
			r.data = append(r.data[:0], n.data...)
			return xmlreader.Token{Kind: xmlreader.KindText, Data: r.data, CDATA: n.cdata, Whitespace: isSpace(n.data),
				Line: n.line, Column: n.column}, nil
		case KindComment:
			r.data = append(r.data[:0], n.data...)
			return xmlreader.Token{Kind: xmlreader.KindComment, Data: r.data, Line: n.line, Column: n.column}, nil
		case KindProcInst:
			r.data = append(r.data[:0], n.data...)
			return xmlreader.Token{Kind: xmlreader.KindProcInst, Name: Name{Local: n.el.nm().Local}, Data: r.data,
				Line: n.line, Column: n.column}, nil
		case KindDocument:
		default:
			return xmlreader.Token{}, nodeErr(CodeNodeKind, n, "unknown kind of node")
		}
	}
}
