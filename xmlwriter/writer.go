// Package xmlwriter is a streaming XML 1.0 writer in pure Go, the counterpart of xmlreader: it takes calls
// (Start, Attr, Text, End) or the tokens of a reader (WriteToken) and writes a document in UTF-8.
//
// What the writer wrote without an error is a well-formed document, and reading it gives back the names and
// values the caller passed. The writer never repairs its input: a character that XML does not allow, a bad name,
// a prefix without a declaration, a second root element are errors with a fixed code (*Error), not something
// that is dropped or replaced. After such an error nothing of the call was written and the Writer can be used
// further.
//
// Memory depends on the depth of the document, not on its size: output goes through a buffer of fixed size, and
// after warm-up a call does not allocate.
//
// Namespaces are written as the caller gives them (Start takes the declarations of the element, WriteToken those
// of the token); a name whose prefix is not bound to its namespace is an error. With Options.AutoNamespaces the
// writer adds the missing declarations itself.
//
// A Writer is not safe for use by several goroutines; different Writers are independent.
package xmlwriter

import (
	"io"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// The types of names, attributes, namespace declarations and tokens are those of xmlreader; the aliases let code
// that only writes XML use them without importing the reader.
type (
	Name    = xmlreader.Name
	Attr    = xmlreader.Attr
	Binding = xmlreader.Binding
	Token   = xmlreader.Token
)

const (
	nsXML   = "http://www.w3.org/XML/1998/namespace"
	nsXMLNS = "http://www.w3.org/2000/xmlns/"

	bufSize = 32 << 10
)

// Limits are the limits of the writer. Zero means the default (DefaultLimits), a negative value no limit.
// Exceeding a limit is an *Error with a limit-* code. The defaults are those of xmlreader, so that a document
// written with default limits can be read with default limits.
type Limits struct {
	MaxDepth      int // nesting of elements
	MaxNameLength int // length of a qualified name or a PI target in UTF-8 bytes
	MaxAttributes int // attributes of one element, namespace declarations included
	// MaxTextSize is the size in bytes of what a reader returns as one token or value: a run of text between
	// two tags (several Text calls in a row add up), a CDATA section, a comment, the data of a PI, an attribute
	// value.
	MaxTextSize int
	// MaxNamespaces is the number of namespace declarations in scope at one time, as in the reader.
	MaxNamespaces int
}

// DefaultLimits returns the default limits: depth 512, names 16 KiB, 4096 attributes, text 16 MiB.
func DefaultLimits() Limits {
	d := xmlreader.DefaultLimits()
	return Limits{MaxDepth: d.MaxDepth, MaxNameLength: d.MaxNameLength, MaxAttributes: d.MaxAttributes, MaxTextSize: d.MaxTextSize,
		MaxNamespaces: d.MaxNamespaces}
}

func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	pick := func(v, def int) int {
		if v == 0 {
			v = def
		}
		if v < 0 {
			return int(^uint(0) >> 1)
		}
		return v
	}
	return Limits{
		MaxDepth:      pick(l.MaxDepth, d.MaxDepth),
		MaxNameLength: pick(l.MaxNameLength, d.MaxNameLength),
		MaxAttributes: pick(l.MaxAttributes, d.MaxAttributes),
		MaxTextSize:   pick(l.MaxTextSize, d.MaxTextSize),
		MaxNamespaces: pick(l.MaxNamespaces, d.MaxNamespaces),
	}
}

// Options configure a Writer. The zero value is the default: namespaces as given, no indentation, DOCTYPE
// tokens skipped, default limits.
type Options struct {
	Limits Limits
	// NoNamespaces turns off namespace processing, as in xmlreader: Name.Local holds the whole name, colons have
	// no meaning, Name.Space and Name.Prefix must be empty, and namespace declarations are ordinary attributes.
	NoNamespaces bool
	// AutoNamespaces makes the writer declare namespaces itself. When the namespace of an element or attribute
	// name is not bound to its prefix in scope, the writer adds a declaration to the tag: with the prefix of
	// the name where that prefix is free, otherwise with a prefix already bound to the namespace or with the
	// first free one of ns1, ns2, ... An attribute in a namespace never gets the empty prefix. The result
	// depends only on the sequence of calls.
	AutoNamespaces bool
	// Indent, when not empty, puts every start tag, end tag, comment and PI on a line of its own, indented by
	// one Indent (spaces and tabs only) per level. It changes the content of the document: white space is added
	// between tags. Nothing is added inside an element once text or a CDATA section has been written in it or
	// in one of its ancestors, so text is never changed; in an element that gets text after child elements,
	// the white space written before those children stays.
	Indent string
	// RejectDoctype makes WriteToken return an error for a DOCTYPE token. By default the token is skipped: the
	// writer does not write DOCTYPE declarations, and what the declaration gave the document (default attribute
	// values, expanded entities) is in the other tokens.
	RejectDoctype bool
}

// elem to otwarty element.
type elem struct {
	name     int  // początek nazwy kwalifikowanej w Writer.names
	mark     int  // długość Writer.bindings przed deklaracjami tego elementu
	noIndent bool // w elemencie albo jego przodku zapisano tekst: wcięć już nie dodajemy
	children bool // w elemencie zapisano znacznik z wcięciem: znacznik końca też je dostaje
}

// attrPlan to wynik sprawdzenia atrybutu: prefiks, z jakim zostanie zapisany, czy przed nim trzeba zapisać
// deklarację tego prefiksu (tryb automatyczny) i indeks pierwszego znaku wartości zapisywanego odwołaniem.
type attrPlan struct {
	prefix  string
	declare bool
	first   int
}

// A Writer writes one XML document to an io.Writer. Create it with New; the zero value is not usable.
type Writer struct {
	out io.Writer
	buf []byte
	err error // błąd io.Writer: od tej chwili zwraca go każde wywołanie

	lim           Limits
	noNS, auto    bool
	rejectDoctype bool
	indent        string
	optErr        string

	calls     int
	closed    bool
	wrote     bool // cokolwiek zapisano (deklaracja XML musi być pierwsza)
	seenRoot  bool
	topItems  bool // na poziomie dokumentu zapisano już znacznik (wcięcia)
	tagOpen   bool // znacznik początku nie ma jeszcze zamykającego '>'
	emptyHint bool // otwarty znacznik pochodzi z tokenu SelfClosing: bez treści zostanie zapisany jako <a/>
	run       int  // bajty bieżącego ciągu tekstu (limit MaxTextSize)

	names    []byte // nazwy kwalifikowane otwartych elementów, jedna za drugą
	stack    []elem
	bindings []Binding // deklaracje przestrzeni nazw otwartych elementów, od zewnętrznego

	cur, next *attrSet // atrybuty otwartego znacznika i znacznika właśnie sprawdzanego
	plan      []attrPlan
}

// New returns a Writer that writes to w.
func New(w io.Writer, opts Options) *Writer {
	x := &Writer{buf: make([]byte, 0, bufSize), cur: &attrSet{}, next: &attrSet{}}
	x.Reset(w, opts)
	return x
}

// Reset makes the Writer start a new document on w, as New does, and keeps its buffers. What the previous
// document left in the buffer is dropped: call Close or Flush first.
func (w *Writer) Reset(out io.Writer, opts Options) {
	w.out, w.buf, w.err = out, w.buf[:0], nil
	w.lim = opts.Limits.withDefaults()
	w.noNS, w.auto, w.rejectDoctype, w.indent = opts.NoNamespaces, opts.AutoNamespaces, opts.RejectDoctype, opts.Indent
	w.optErr = ""
	for i := 0; i < len(w.indent); i++ {
		if w.indent[i] != ' ' && w.indent[i] != '\t' {
			w.optErr = "Options.Indent may hold only spaces and tabs"
		}
	}
	if w.noNS && w.auto {
		w.optErr = "Options.AutoNamespaces needs namespace processing, NoNamespaces turns it off"
	}
	w.calls, w.closed, w.wrote, w.seenRoot, w.topItems, w.tagOpen, w.emptyHint, w.run = 0, false, false, false, false, false, false, 0
	w.names, w.stack, w.bindings, w.plan = w.names[:0], w.stack[:0], w.bindings[:0], w.plan[:0]
	w.cur.reset()
	w.next.reset()
}

// Declaration writes the XML declaration: version 1.0, encoding UTF-8 and, when standalone is "yes" or "no",
// the standalone declaration, followed by a line feed. It must be the first call that writes anything.
func (w *Writer) Declaration(standalone string) error {
	if err := w.enter(); err != nil {
		return err
	}
	if w.wrote {
		return w.fail(CodeXMLDecl, "the XML declaration must be the first thing written")
	}
	if standalone != "" && standalone != "yes" && standalone != "no" {
		return w.fail(CodeXMLDecl, `standalone must be "yes", "no" or empty`)
	}
	put(w, `<?xml version="1.0" encoding="UTF-8"`)
	if standalone != "" {
		put(w, ` standalone="`)
		put(w, standalone)
		w.putByte('"')
	}
	put(w, "?>\n")
	w.wrote = true
	return w.err
}

// Start writes the start tag of an element with its namespace declarations, in the order given. The tag stays
// open for Attr until the next call of another kind.
//
// With namespace processing Name.Local and Name.Prefix are the two parts of the qualified name and Name.Space
// must be the namespace the prefix is bound to in scope (the empty prefix: the default namespace); the
// declarations passed here are in scope for the name of the element itself.
func (w *Writer) Start(name Name, namespaces ...Binding) error {
	if err := w.enter(); err != nil {
		return err
	}
	return w.done(w.start(name, namespaces, nil, false))
}

// Attr adds an attribute to the start tag just written. Attributes are written in the order of the calls.
// Namespace declarations are not attributes here: they are given to Start.
func (w *Writer) Attr(name Name, value string) error {
	if err := w.enter(); err != nil {
		return err
	}
	if !w.tagOpen {
		return w.fail(CodeUsageAttr, "Attr needs an open start tag: call it right after Start")
	}
	p, err := w.prepareAttr(w.cur, name, value)
	if err != nil {
		return err
	}
	w.putAttr(p, name, value)
	return w.err
}

// End writes the end tag of the innermost open element. An element without content is written as <a></a>; use
// EndEmpty for <a/>.
func (w *Writer) End() error {
	if err := w.enter(); err != nil {
		return err
	}
	return w.done(w.end(Name{}, false))
}

// EndEmpty closes the element whose start tag was just written as an empty-element tag, <a/>. After any content
// it is an error.
func (w *Writer) EndEmpty() error {
	if err := w.enter(); err != nil {
		return err
	}
	if !w.tagOpen {
		return w.fail(CodeUsageEnd, "EndEmpty needs an element without content: call it right after Start and Attr")
	}
	return w.done(w.end(Name{}, true))
}

// Text writes character data: "&", "<", ">" and carriage returns are written as references, everything else as
// it is. Outside the root element only spaces, tabs and line feeds are allowed. Empty data writes nothing.
func (w *Writer) Text(data string) error {
	if err := w.enter(); err != nil {
		return err
	}
	return w.done(text(w, data))
}

// CDATA writes data as one CDATA section. Data that contains "]]>" is an error: the writer does not split
// sections.
func (w *Writer) CDATA(data string) error {
	if err := w.enter(); err != nil {
		return err
	}
	return w.done(cdata(w, data))
}

// Comment writes a comment. Data that contains "--" or ends with "-" is an error.
func (w *Writer) Comment(data string) error {
	if err := w.enter(); err != nil {
		return err
	}
	return w.done(comment(w, data))
}

// ProcInst writes a processing instruction. Data that contains "?>" or starts with white space is an error (a
// reader does not return white space after the target).
func (w *Writer) ProcInst(target, data string) error {
	if err := w.enter(); err != nil {
		return err
	}
	return w.done(procInst(w, target, data))
}

// WriteToken writes a token of a reader: a start tag with its namespace declarations and attributes (those
// from default values included), an end tag, text (as a CDATA section when Token.CDATA is set), a comment or a
// PI. An element whose start token has SelfClosing is written as <a/> when nothing is written between its start
// and end tokens. An end token with a name must name the open element. A DOCTYPE token is skipped, or an error
// with Options.RejectDoctype. Positions are ignored, and nothing of the token is kept after the call.
func (w *Writer) WriteToken(t Token) error {
	if err := w.enter(); err != nil {
		return err
	}
	switch t.Kind {
	case xmlreader.KindStartElement:
		return w.done(w.start(t.Name, t.Namespaces, t.Attrs, t.SelfClosing))
	case xmlreader.KindEndElement:
		return w.done(w.end(t.Name, false))
	case xmlreader.KindText:
		if t.CDATA {
			return w.done(cdata(w, t.Data))
		}
		return w.done(text(w, t.Data))
	case xmlreader.KindComment:
		return w.done(comment(w, t.Data))
	case xmlreader.KindProcInst:
		return w.done(procInst(w, t.Name.Local, t.Data))
	case xmlreader.KindDoctype:
		if w.rejectDoctype {
			return w.fail(CodeDoctype, "DOCTYPE declarations are not written")
		}
		return nil
	}
	return w.fail(CodeUsageToken, "unknown token kind")
}

// LookupNamespace returns the namespace bound to prefix in the scope of the innermost open element, from the
// declarations written so far. The empty prefix is the default namespace; it is not bound (false) outside any
// declaration and after xmlns="". The prefix xml is always bound. With Options.NoNamespaces nothing is bound.
func (w *Writer) LookupNamespace(prefix string) (string, bool) {
	if w.noNS {
		return "", false
	}
	return w.lookup(prefix)
}

// Flush writes what is in the buffer to the io.Writer. It does not change what the document will be.
func (w *Writer) Flush() error {
	if err := w.enter(); err != nil {
		return err
	}
	w.flush()
	return w.err
}

// Close checks that the document is complete (a root element, no element open) and flushes the buffer. After a
// successful Close every call is an error, until Reset. Close does not close the io.Writer.
func (w *Writer) Close() error {
	if err := w.enter(); err != nil {
		return err
	}
	if n := len(w.stack); n > 0 {
		return w.fail(CodeEOF, "element <"+string(w.names[w.stack[n-1].name:])+"> is not closed")
	}
	if !w.seenRoot {
		return w.fail(CodeNoRoot, "the document has no root element")
	}
	if w.indent != "" {
		w.putByte('\n')
	}
	w.flush()
	if w.err == nil {
		w.closed = true
	}
	return w.err
}

// enter zaczyna wywołanie publiczne.
func (w *Writer) enter() error {
	w.calls++
	switch {
	case w.err != nil:
		return w.err
	case w.optErr != "":
		return w.fail(CodeUsageOptions, w.optErr)
	case w.closed:
		return w.fail(CodeUsageClosed, "the Writer is closed")
	}
	return nil
}

// done kończy wywołanie: błąd wywołania albo, gdy go nie ma, błąd zapisu.
func (w *Writer) done(err error) error {
	if err != nil {
		return err
	}
	return w.err
}

func (w *Writer) fail(code, msg string) error {
	path := make([]byte, 0, len(w.names)+len(w.stack)+1)
	for i, e := range w.stack {
		end := len(w.names)
		if i+1 < len(w.stack) {
			end = w.stack[i+1].name
		}
		path = append(append(path, '/'), w.names[e.name:end]...)
	}
	if len(path) == 0 {
		path = append(path, '/')
	}
	return &Error{Code: code, Path: string(path), Call: w.calls, Message: msg}
}

// dataErr zamienia kod z check na błąd z opisem, czego dotyczy.
func (w *Writer) dataErr(code, what string) error {
	switch code {
	case CodeEncodingInvalid:
		return w.fail(code, what+" is not valid UTF-8")
	case CodeLineBreak:
		return w.fail(code, "a carriage return in "+what+" cannot be written so that a reader returns it")
	}
	return w.fail(code, what+" has a character that XML does not allow")
}

// Zapis do bufora.

func (w *Writer) flush() {
	if w.err == nil && len(w.buf) > 0 {
		_, w.err = w.out.Write(w.buf)
	}
	w.buf = w.buf[:0]
}

func (w *Writer) putByte(b byte) {
	if len(w.buf) == cap(w.buf) {
		w.flush()
	}
	w.buf = append(w.buf, b)
}

func put[T bytestring](w *Writer, s T) {
	if len(s) <= cap(w.buf)-len(w.buf) { // zwykły przypadek: mieści się w buforze
		w.buf = append(w.buf, s...)
		return
	}
	for len(s) > 0 {
		if len(w.buf) == cap(w.buf) {
			w.flush()
		}
		n := copy(w.buf[len(w.buf):cap(w.buf)], s)
		w.buf = w.buf[:len(w.buf)+n]
		s = s[n:]
	}
}

// closePending zamyka otwarty znacznik początku, bo zaczyna się treść elementu.
func (w *Writer) closePending() {
	if w.tagOpen {
		w.putByte('>')
		w.tagOpen = false
	}
}

func (w *Writer) newline(depth int) {
	w.putByte('\n')
	for ; depth > 0; depth-- {
		put(w, w.indent)
	}
}

// breakBefore zapisuje wcięcie przed znacznikiem początku, komentarzem albo PI (Options.Indent).
func (w *Writer) breakBefore() {
	if w.indent == "" {
		return
	}
	n := len(w.stack)
	if n == 0 {
		if w.topItems {
			w.putByte('\n')
		}
		w.topItems = true
		return
	}
	if e := &w.stack[n-1]; !e.noIndent {
		e.children = true
		w.newline(n)
	}
}

// Elementy.

func (w *Writer) start(name Name, decls []Binding, attrs []Attr, hint bool) error {
	depth := len(w.stack)
	if depth == 0 && w.seenRoot {
		return w.fail(CodeOutsideRoot, "a document has one root element")
	}
	if depth >= w.lim.MaxDepth {
		return w.fail(CodeLimitDepth, "elements nested deeper than the limit")
	}
	mark := len(w.bindings)
	prefix, err := w.startName(name, decls)
	if err != nil {
		w.bindings = w.bindings[:mark]
		return err
	}
	declEnd := len(w.bindings)
	w.next.reset()
	w.next.n = declEnd - mark
	if declEnd > w.lim.MaxNamespaces {
		w.bindings = w.bindings[:mark]
		return w.fail(CodeLimitNamespaces, "more namespace declarations in scope than the limit")
	}
	if w.next.n > w.lim.MaxAttributes {
		w.bindings = w.bindings[:mark]
		return w.fail(CodeLimitAttributes, "more namespace declarations than the limit of attributes")
	}
	w.plan = w.plan[:0]
	for i := range attrs {
		p, err := w.prepareAttr(w.next, attrs[i].Name, attrs[i].Value)
		if err != nil {
			w.bindings = w.bindings[:mark]
			return err
		}
		w.plan = append(w.plan, p)
	}

	w.closePending()
	w.breakBefore()
	w.putByte('<')
	off := len(w.names)
	if prefix != "" {
		w.names = append(append(w.names, prefix...), ':')
	}
	w.names = append(w.names, name.Local...)
	put(w, w.names[off:])
	for _, b := range w.bindings[mark:declEnd] {
		w.putDecl(b)
	}
	for i := range attrs {
		w.putAttr(w.plan[i], attrs[i].Name, attrs[i].Value)
	}
	w.stack = append(w.stack, elem{name: off, mark: mark, noIndent: depth > 0 && w.stack[depth-1].noIndent})
	w.cur, w.next = w.next, w.cur
	w.tagOpen, w.emptyHint, w.seenRoot, w.wrote, w.run = true, hint, true, true, 0
	return nil
}

// startName sprawdza nazwę elementu i jego deklaracje przestrzeni nazw, dopisuje deklaracje do w.bindings
// i zwraca prefiks, z jakim nazwa zostanie zapisana. Po błędzie wywołujący obcina w.bindings.
func (w *Writer) startName(name Name, decls []Binding) (string, error) {
	if w.noNS {
		if len(decls) > 0 {
			return "", w.fail(CodeUsageOptions, "namespace declarations given to a Writer with NoNamespaces")
		}
		return "", w.plainName(name, "element")
	}
	if err := w.qname(name, "element"); err != nil {
		return "", err
	}
	mark := len(w.bindings)
	for _, d := range decls {
		if err := w.checkDecl(d, mark); err != nil {
			return "", err
		}
		w.bindings = append(w.bindings, d)
	}
	prefix, declare, err := w.elemPrefix(name, mark)
	if err != nil {
		return "", err
	}
	if declare {
		if err := w.checkSpace(name.Space); err != nil {
			return "", err
		}
		w.bindings = append(w.bindings, Binding{Prefix: prefix, URI: name.Space})
	}
	if nameLen(prefix, name.Local) > w.lim.MaxNameLength {
		return "", w.fail(CodeLimitNameLength, "element name longer than the limit")
	}
	return prefix, nil
}

func nameLen(prefix, local string) int {
	if prefix == "" {
		return len(local)
	}
	return len(prefix) + 1 + len(local)
}

// plainName sprawdza nazwę w trybie bez przestrzeni nazw: [5] Name w polu Local, reszta pusta.
func (w *Writer) plainName(name Name, what string) error {
	if name.Space != "" || name.Prefix != "" {
		return w.fail(CodeUsageOptions, what+" name with a namespace or a prefix given to a Writer with NoNamespaces")
	}
	if nameKind(name.Local) == nameBad {
		return w.fail(CodeName, what+" name is not a Name")
	}
	if len(name.Local) > w.lim.MaxNameLength {
		return w.fail(CodeLimitNameLength, what+" name longer than the limit")
	}
	return nil
}

// qname sprawdza składnię nazwy kwalifikowanej: części Local i Prefix to NCName (Namespaces in XML 1.0, [7]–[11]).
func (w *Writer) qname(name Name, what string) error {
	switch nameKind(name.Local) {
	case nameBad:
		return w.fail(CodeName, "local part of the "+what+" name is not a Name")
	case nameColon:
		return w.fail(CodeQName, "local part of the "+what+" name contains a colon")
	}
	if name.Prefix != "" {
		switch nameKind(name.Prefix) {
		case nameBad:
			return w.fail(CodeName, "prefix of the "+what+" name is not a Name")
		case nameColon:
			return w.fail(CodeQName, "prefix of the "+what+" name contains a colon")
		}
	}
	if nameLen(name.Prefix, name.Local) > w.lim.MaxNameLength {
		return w.fail(CodeLimitNameLength, what+" name longer than the limit")
	}
	return nil
}

func (w *Writer) end(name Name, empty bool) error {
	n := len(w.stack)
	if n == 0 {
		return w.fail(CodeUsageEnd, "no element is open")
	}
	e := w.stack[n-1]
	qname := w.names[e.name:]
	if name.Local != "" && !w.sameName(qname, name) {
		return w.fail(CodeElementTypeMatch, "end token names another element than the open <"+string(qname)+">")
	}
	if w.tagOpen && (empty || w.emptyHint) {
		put(w, "/>")
	} else {
		if w.tagOpen {
			w.putByte('>')
		} else if w.indent != "" && e.children && !e.noIndent {
			w.newline(n - 1)
		}
		put(w, "</")
		put(w, qname)
		w.putByte('>')
	}
	w.names, w.bindings, w.stack = w.names[:e.name], w.bindings[:e.mark], w.stack[:n-1]
	w.tagOpen, w.emptyHint, w.run = false, false, 0
	return nil
}

// sameName porównuje nazwę z tokenu końca z nazwą zapisaną w znaczniku początku. W trybie automatycznym pisarz
// mógł nadać inny prefiks, więc porównuje się tylko część lokalną.
func (w *Writer) sameName(qname []byte, name Name) bool {
	l := len(name.Local)
	if len(qname) < l || string(qname[len(qname)-l:]) != name.Local {
		return false
	}
	rest := qname[:len(qname)-l]
	switch {
	case w.noNS:
		return len(rest) == 0 && name.Prefix == ""
	case w.auto:
		return len(rest) == 0 || rest[len(rest)-1] == ':'
	case name.Prefix == "":
		return len(rest) == 0
	}
	return len(rest) == len(name.Prefix)+1 && string(rest[:len(rest)-1]) == name.Prefix
}

// Atrybuty.

// prepareAttr sprawdza atrybut i dopisuje go do zbioru set; niczego nie zapisuje. Po błędzie stan pisarza jest
// taki jak przed wywołaniem.
func (w *Writer) prepareAttr(set *attrSet, name Name, value string) (attrPlan, error) {
	var p attrPlan
	if w.noNS {
		if err := w.plainName(name, "attribute"); err != nil {
			return p, err
		}
	} else {
		if err := w.qname(name, "attribute"); err != nil {
			return p, err
		}
		if name.Prefix == "xmlns" || (name.Prefix == "" && name.Local == "xmlns") {
			return p, w.fail(CodeReservedPrefixes, "namespace declarations are not attributes: pass them to Start")
		}
		var err error
		if p.prefix, p.declare, err = w.attrPrefix(name); err != nil {
			return p, err
		}
		if nameLen(p.prefix, name.Local) > w.lim.MaxNameLength {
			return p, w.fail(CodeLimitNameLength, "attribute name longer than the limit")
		}
		if p.declare {
			if err := w.checkSpace(name.Space); err != nil {
				return p, err
			}
		}
	}
	if len(value) > w.lim.MaxTextSize {
		return p, w.fail(CodeLimitTextSize, "attribute value longer than the limit")
	}
	first, code := check(value, cAttr, 0)
	if code != "" {
		return p, w.dataErr(code, "an attribute value")
	}
	p.first = first
	need := 1
	if p.declare {
		need = 2
	}
	if set.n+need > w.lim.MaxAttributes {
		return p, w.fail(CodeLimitAttributes, "more attributes than the limit")
	}
	if p.declare && len(w.bindings) >= w.lim.MaxNamespaces {
		return p, w.fail(CodeLimitNamespaces, "more namespace declarations in scope than the limit")
	}
	key := attrKey{name.Space, name.Local}
	if prev, dup := set.find(key); dup {
		if prev == p.prefix {
			return p, w.fail(CodeUniqueAttSpec, "attribute "+name.Local+" given twice")
		}
		return p, w.fail(CodeAttributesUnique, "two attributes with the local name "+name.Local+" in the same namespace")
	}
	set.add(key, p.prefix)
	if p.declare {
		set.n++
		w.bindings = append(w.bindings, Binding{Prefix: p.prefix, URI: name.Space})
	}
	return p, nil
}

func (w *Writer) putAttr(p attrPlan, name Name, value string) {
	if p.declare {
		w.putDecl(Binding{Prefix: p.prefix, URI: name.Space})
	}
	w.putByte(' ')
	if p.prefix != "" {
		put(w, p.prefix)
		w.putByte(':')
	}
	put(w, name.Local)
	put(w, `="`)
	w.putAttrValue(value, p.first)
	w.putByte('"')
}

func (w *Writer) putDecl(b Binding) {
	put(w, " xmlns")
	if b.Prefix != "" {
		w.putByte(':')
		put(w, b.Prefix)
	}
	put(w, `="`)
	first := 0
	for first < len(b.URI) && class[b.URI[first]]&cAttr == 0 {
		first++
	}
	w.putAttrValue(b.URI, first)
	w.putByte('"')
}

// putAttrValue zapisuje wartość atrybutu: "&", "<", cudzysłów oraz TAB, LF i CR jako odwołania, żeby
// normalizacja przy odczycie (XML 1.0, 3.3.3) oddała tę samą wartość. first to indeks pierwszego takiego znaku.
func (w *Writer) putAttrValue(s string, first int) {
	put(w, s[:first])
	last := first
	for i := first; i < len(s); i++ {
		if class[s[i]]&cAttr == 0 {
			continue
		}
		put(w, s[last:i])
		switch s[i] {
		case '&':
			put(w, "&amp;")
		case '<':
			put(w, "&lt;")
		case '"':
			put(w, "&quot;")
		case '\t':
			put(w, "&#9;")
		case '\n':
			put(w, "&#10;")
		default:
			put(w, "&#13;")
		}
		last = i + 1
	}
	put(w, s[last:])
}

// Treść.

func text[T bytestring](w *Writer, s T) error {
	if len(s) == 0 {
		return nil
	}
	n := len(w.stack)
	if n == 0 {
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case ' ', '\t', '\n':
			case '\r':
				return w.dataErr(CodeLineBreak, "white space outside the root element")
			default:
				return w.fail(CodeOutsideRoot, "text outside the root element")
			}
		}
		put(w, s)
		w.wrote = true
		return nil
	}
	if len(s) > w.lim.MaxTextSize-w.run {
		return w.fail(CodeLimitTextSize, "text longer than the limit")
	}
	first, code := check(s, cText, 0)
	if code != "" {
		return w.dataErr(code, "text")
	}
	w.closePending()
	w.stack[n-1].noIndent = true
	put(w, s[:first])
	last := first
	for i := first; i < len(s); i++ {
		if class[s[i]]&cText == 0 {
			continue
		}
		put(w, s[last:i])
		switch s[i] {
		case '&':
			put(w, "&amp;")
		case '<':
			put(w, "&lt;")
		case '>':
			put(w, "&gt;")
		default:
			put(w, "&#13;")
		}
		last = i + 1
	}
	put(w, s[last:])
	w.run += len(s)
	return nil
}

func cdata[T bytestring](w *Writer, s T) error {
	n := len(w.stack)
	if n == 0 {
		return w.fail(CodeOutsideRoot, "CDATA section outside the root element")
	}
	if len(s) > w.lim.MaxTextSize {
		return w.fail(CodeLimitTextSize, "CDATA section longer than the limit")
	}
	if _, code := check(s, 0, cCR); code != "" {
		return w.dataErr(code, "a CDATA section")
	}
	if contains(s, "]]>") {
		return w.fail(CodeCDATAEnd, `"]]>" in the data of a CDATA section`)
	}
	w.closePending()
	w.stack[n-1].noIndent = true
	put(w, "<![CDATA[")
	put(w, s)
	put(w, "]]>")
	w.run = 0
	return nil
}

func comment[T bytestring](w *Writer, s T) error {
	if len(s) > w.lim.MaxTextSize {
		return w.fail(CodeLimitTextSize, "comment longer than the limit")
	}
	if _, code := check(s, 0, cCR); code != "" {
		return w.dataErr(code, "a comment")
	}
	if contains(s, "--") || (len(s) > 0 && s[len(s)-1] == '-') {
		return w.fail(CodeComment, `"--" in a comment, or "-" at its end`)
	}
	w.closePending()
	w.breakBefore()
	put(w, "<!--")
	put(w, s)
	put(w, "-->")
	w.wrote, w.run = true, 0
	return nil
}

func procInst[T bytestring](w *Writer, target string, s T) error {
	switch nameKind(target) {
	case nameBad:
		return w.fail(CodeName, "processing instruction target is not a Name")
	case nameColon:
		if !w.noNS {
			return w.fail(CodeQName, "processing instruction target contains a colon")
		}
	}
	if len(target) > w.lim.MaxNameLength {
		return w.fail(CodeLimitNameLength, "processing instruction target longer than the limit")
	}
	if len(target) == 3 && target[0]|0x20 == 'x' && target[1]|0x20 == 'm' && target[2]|0x20 == 'l' {
		return w.fail(CodeReservedPITarget, "processing instruction target "+target+" is reserved")
	}
	if len(s) > w.lim.MaxTextSize {
		return w.fail(CodeLimitTextSize, "processing instruction longer than the limit")
	}
	if _, code := check(s, 0, cCR); code != "" {
		return w.dataErr(code, "the data of a processing instruction")
	}
	if len(s) > 0 && class[s[0]]&cSpace != 0 {
		return w.fail(CodePIData, "the data of a processing instruction starts with white space")
	}
	if contains(s, "?>") {
		return w.fail(CodePIData, `"?>" in the data of a processing instruction`)
	}
	w.closePending()
	w.breakBefore()
	put(w, "<?")
	put(w, target)
	if len(s) > 0 {
		w.putByte(' ')
		put(w, s)
	}
	put(w, "?>")
	w.wrote, w.run = true, 0
	return nil
}
