// Package xmltree is an in-memory tree of an XML document, or of one element of it, that can be changed and
// written back.
//
// A tree is built from any source of tokens: a whole document with Parse or ReadDocument, or one element in the
// middle of a stream with ReadElement, which leaves the stream right after the end tag of the element. It is
// written through an xmlwriter.Writer with WriteTo, and NewReader gives its tokens back, so a tree fits anywhere
// in a chain of readers, filters and writers:
//
//	r := xmlreader.New(in, xmlreader.Options{})
//	w := xmlwriter.New(out, xmlwriter.Options{})
//	for {
//		t, err := r.Next()
//		if err == io.EOF {
//			break
//		}
//		if err != nil {
//			return err
//		}
//		if t.Kind == xmlreader.KindStartElement && t.Name.Local == "Row" {
//			row, err := xmltree.ReadElement(r, t, xmltree.Options{}) // only this element is in memory
//			if err != nil {
//				return err
//			}
//			row.SetAttr(xmlreader.Name{Local: "checked"}, "yes")
//			if err := row.WriteTo(w); err != nil {
//				return err
//			}
//			continue
//		}
//		if err := w.WriteToken(t); err != nil {
//			return err
//		}
//	}
//	return w.Close()
//
// # What a tree keeps
//
// A tree keeps what the tokens carry: names with their prefixes, attributes in document order (with the type and
// the Specified flag from the DTD), the namespace declarations of every element, text as it was split by the
// source, CDATA sections, comments, processing instructions, the form of an empty element (<a/> or <a></a>) and
// the line and column of every node. A document that was not changed is written with the same bytes as the
// stream it was built from. A DOCTYPE declaration is not kept: the writer does not write one.
//
// # Changing a tree
//
// A Node is read and changed only through its methods. The links between nodes are therefore always consistent:
// a node has at most one parent, a node cannot be put inside itself, and adding, inserting and removing a child
// take constant time. The data of a node (name, attributes, namespace declarations, text) may be set to any
// value; what cannot be written as XML is an error with a fixed code, from the method where that is cheap to see
// (a second root element, text outside the root element) and from the writer otherwise (a name that is not a
// name, an attribute given twice, a character that XML does not allow).
//
// # Namespaces
//
// An element keeps its own declarations (Namespaces) and LookupNamespace walks up through the parents. An element
// that is taken out of its place (Remove, Clone, ReadElement) remembers the namespaces that were in scope there.
// When it is written in a place where they are not, WriteTo declares on the element what is missing: the
// remembered namespaces, and the prefix of every element and attribute name that is not bound to the namespace of
// the name. The expanded names read back are those of the tree, and prefixes used in text keep their meaning.
//
// # Memory
//
// The nodes, attributes and text of a tree built from a source are allocated in blocks, not one by one. A block
// is freed when nothing in it is used any more, so a node taken out of a large tree and kept keeps a part of the
// memory of that tree; Clone gives a copy that does not.
//
// # Limits
//
// A tree holds a whole document in memory, so the number of nodes is limited (Options.MaxNodes, 2 000 000 by
// default). The limits of the reader apply as well.
package xmltree
