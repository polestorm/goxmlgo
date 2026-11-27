package xmltree

import (
	"strconv"
	"strings"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// IndexIDs builds the index of identifiers from the tree as it is now, replacing the index built before. An
// attribute is an identifier when its type in the DTD is ID, when it is xml:id (its value is normalized as the
// type ID is: white space trimmed and collapsed), or when it has no namespace and its local name is one of
// attributes. An empty value identifies nothing. Two elements with the same identifier give CodeIDDuplicate with
// the places of both attributes, and the index is left empty.
//
// The index is not updated when the tree changes: call IndexIDs again after changes.
func (d *Document) IndexIDs(attributes ...string) error {
	d.ids = map[string]*Node{}
	var where map[string][2]int // identyfikator → wiersz i kolumna pierwszego atrybutu
	for it := newWalk(&d.node); ; {
		n, enter := it.next()
		if n == nil {
			return nil
		}
		if !enter || n.kind != KindElement {
			continue
		}
		for i := range n.el.attrs {
			a := n.el.attrs[i].public()
			v, ok := idValue(&a, attributes)
			if !ok || v == "" {
				continue
			}
			prev, dup := d.ids[v]
			if dup && prev != n {
				first := where[v]
				d.ids = nil
				return &Error{Code: CodeIDDuplicate, Line: a.Line, Column: a.Column,
					FirstLine: first[0], FirstColumn: first[1],
					Message: "the identifier " + strconv.Quote(v) + " is used by two elements"}
			}
			if !dup {
				if where == nil {
					where = map[string][2]int{}
				}
				d.ids[v], where[v] = n, [2]int{a.Line, a.Column}
			}
		}
	}
}

func idValue(a *Attr, names []string) (string, bool) {
	switch {
	case a.Type == xmlreader.AttrTypeID:
		return a.Value, true
	case a.Name.Space == nsXML && a.Name.Local == "id":
		return strings.Join(strings.Fields(a.Value), " "), true
	case a.Name.Space == "":
		for _, n := range names {
			if n == a.Name.Local {
				return a.Value, true
			}
		}
	}
	return "", false
}

// ByID returns the element with the identifier id, from the index built by Parse or ReadDocument (Options.IDs,
// Options.IDAttributes) or by IndexIDs. Without an index it finds nothing.
func (d *Document) ByID(id string) (*Node, bool) {
	n, ok := d.ids[id]
	return n, ok
}

// IDs returns the number of identifiers in the index.
func (d *Document) IDs() int { return len(d.ids) }
