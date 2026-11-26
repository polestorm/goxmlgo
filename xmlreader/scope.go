package xmlreader

// NamespaceScope tracks the namespaces in scope from the tokens of a stream, for code that sees tokens but not
// the Reader they come from: a writer, a tree, a filter. Fed every token of a stream with Update, it answers
// Lookup as Reader.LookupNamespace does at the same place in the document. The zero value is ready to use.
type NamespaceScope struct {
	bindings []Binding
	marks    []int // dla każdego otwartego elementu: długość bindings przed jego deklaracjami
}

// Update takes the next token of the stream: a KindStartElement opens a scope with the declarations of the
// element (Token.Namespaces), a KindEndElement closes the innermost scope; other tokens change nothing.
func (s *NamespaceScope) Update(t Token) {
	switch t.Kind {
	case KindStartElement:
		s.Push(t.Namespaces)
	case KindEndElement:
		s.Pop()
	}
}

// Push opens the scope of an element with its namespace declarations. The declarations are copied.
func (s *NamespaceScope) Push(declarations []Binding) {
	s.marks = append(s.marks, len(s.bindings))
	s.bindings = append(s.bindings, declarations...)
}

// Pop closes the innermost scope. Without an open scope it does nothing.
func (s *NamespaceScope) Pop() {
	if n := len(s.marks); n > 0 {
		s.bindings = s.bindings[:s.marks[n-1]]
		s.marks = s.marks[:n-1]
	}
}

// Depth returns the number of open scopes, that is, of open elements.
func (s *NamespaceScope) Depth() int { return len(s.marks) }

// Lookup returns the namespace bound to prefix in the innermost open scope. The empty prefix is the default
// namespace; it is not bound (false) outside any declaration and after xmlns="". The prefix xml is always bound
// to the XML namespace.
func (s *NamespaceScope) Lookup(prefix string) (string, bool) {
	if prefix == "xml" {
		return nsXML, true
	}
	for i := len(s.bindings) - 1; i >= 0; i-- {
		if s.bindings[i].Prefix == prefix {
			return s.bindings[i].URI, s.bindings[i].URI != "" || prefix != ""
		}
	}
	return "", false
}

// Reset closes all scopes and keeps the memory for reuse.
func (s *NamespaceScope) Reset() {
	s.bindings, s.marks = s.bindings[:0], s.marks[:0]
}
