package xmlwriter

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// write wykonuje calls na nowym pisarzu i zwraca wyjście oraz pierwszy błąd (także z Close).
func write(opts Options, calls func(w *Writer) error) (string, error) {
	var b bytes.Buffer
	w := New(&b, opts)
	err := calls(w)
	if err == nil {
		err = w.Close()
	} else {
		w.Flush()
	}
	return b.String(), err
}

// seq wykonuje kroki po kolei i kończy na pierwszym błędzie.
func seq(steps ...error) error {
	for _, err := range steps {
		if err != nil {
			return err
		}
	}
	return nil
}

func n(local string) Name { return Name{Local: local} }

// qn to nazwa w przestrzeni nazw: przestrzeń, nazwa lokalna, prefiks.
func qn(space, local, prefix string) Name { return Name{Space: space, Local: local, Prefix: prefix} }

func bind(prefix, uri string) Binding { return Binding{Prefix: prefix, URI: uri} }

func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err == nil {
		return ""
	}
	return "not an *Error: " + err.Error()
}

// expectCode sprawdza, że wywołanie fn na pisarzu z otwartym korzeniem <r> kończy się błędem o podanym kodzie,
// że niczego nie zapisało i że pisarz działa dalej: wyjście jest takie, jakby wywołania nie było.
func expectCode(t *testing.T, opts Options, want string, fn func(w *Writer) error) {
	t.Helper()
	var b bytes.Buffer
	w := New(&b, opts)
	if err := w.Start(n("r")); err != nil {
		t.Fatal(err)
	}
	err := fn(w)
	if got := code(err); got != want {
		t.Errorf("code %q, want %q (%v)", got, want, err)
		return
	}
	if e := err.(*Error); e.Path != "/r" || e.Call != 2 || e.Message == "" {
		t.Errorf("error %+v: want path /r, call 2 and a message", e)
	}
	if err := seq(w.Text("x"), w.End(), w.Close()); err != nil {
		t.Errorf("after %s the Writer does not go on: %v", want, err)
	}
	if got := b.String(); got != "<r>x</r>" {
		t.Errorf("after %s the output is %q, want <r>x</r>", want, got)
	}
}

// expectLast sprawdza kod błędu ciągu wywołań na pisarzu z otwartym korzeniem <r> (błąd daje ostatnie z nich)
// i to, że nieudane wywołanie niczego nie zapisało.
func expectLast(t *testing.T, want string, fn func(w *Writer) error) {
	t.Helper()
	var b bytes.Buffer
	w := New(&b, Options{})
	w.Start(n("r"))
	err := fn(w)
	if got := code(err); got != want {
		t.Errorf("code %q, want %q (%v)", got, want, err)
		return
	}
	w.Flush()
	before := b.String()
	// to samo bez ostatniego wywołania: pisarz, który kończy na tym samym błędzie, zapisał tyle samo
	var b2 bytes.Buffer
	w2 := New(&b2, Options{})
	w2.Start(n("r"))
	fn(w2)
	w2.Flush()
	if b2.String() != before {
		t.Errorf("not repeatable: %q and %q", before, b2.String())
	}
	if e := err.(*Error); e.Call != w.calls-1 || !strings.HasPrefix(e.Path, "/r") {
		t.Errorf("error %+v: want call %d and a path under /r", e, w.calls-1)
	}
}

// TestBufferAndClose: bufor stałego rozmiaru, Flush, Close; wyjście większe niż bufor przechodzi w całości.
// REQ: WR-001
func TestBufferAndClose(t *testing.T) {
	var b bytes.Buffer
	w := New(&b, Options{})
	big := strings.Repeat("zażółć gęślą jaźń <&> ", 10000) // ok. 300 KB, wiele buforów
	if err := seq(w.Start(n("a")), w.Text("x")); err != nil {
		t.Fatal(err)
	}
	if b.Len() != 0 {
		t.Errorf("%d bytes written before Flush, with %d in a buffer of %d", b.Len(), len("<a>x"), bufSize)
	}
	if err := w.Flush(); err != nil || b.String() != "<a>x" {
		t.Errorf("after Flush: %q, %v", b.String(), err)
	}
	if err := seq(w.Text(big), w.End(), w.Close()); err != nil {
		t.Fatal(err)
	}
	want := "<a>x" + strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(big) + "</a>"
	if b.String() != want {
		t.Errorf("output of %d bytes differs from the expected %d", b.Len(), len(want))
	}
	if cap(w.buf) != bufSize {
		t.Errorf("buffer grew to %d", cap(w.buf))
	}
	if got := code(w.Text("x")); got != CodeUsageClosed {
		t.Errorf("call after Close: %q", got)
	}
	if got := code(w.Close()); got != CodeUsageClosed {
		t.Errorf("second Close: %q", got)
	}
	w.Reset(&b, Options{})
	b.Reset()
	if err := seq(w.Start(n("b")), w.EndEmpty(), w.Close()); err != nil || b.String() != "<b/>" {
		t.Errorf("after Reset: %q, %v", b.String(), err)
	}
}

type failingWriter struct {
	after int
	err   error
}

func (f *failingWriter) Write(p []byte) (int, error) {
	if f.after -= len(p); f.after < 0 {
		return 0, f.err
	}
	return len(p), nil
}

// TestWriteError: błąd io.Writer wraca bez zmiany i od tej chwili zwraca go każde wywołanie.
// REQ: WR-001
func TestWriteError(t *testing.T) {
	boom := errors.New("disk full")
	w := New(&failingWriter{after: 10, err: boom}, Options{})
	if err := seq(w.Start(n("a")), w.Text("some text"), w.Flush()); err != boom {
		t.Fatalf("Flush: %v, want the error of the io.Writer", err)
	}
	for i, err := range []error{w.Text("x"), w.Start(n("b")), w.End(), w.Flush(), w.Close(), w.Close()} {
		if err != boom {
			t.Errorf("call %d after the write error: %v", i, err)
		}
	}
	// błąd w środku dużego wywołania wraca z tego wywołania
	w = New(&failingWriter{after: bufSize, err: boom}, Options{})
	if err := seq(w.Start(n("a")), w.Text(strings.Repeat("x", 3*bufSize))); err != boom {
		t.Errorf("Text larger than the buffer: %v", err)
	}
}

// chunkWriter przyjmuje dane po kawałku: sprawdza, że wynik nie zależy od podziału zapisu.
type chunkWriter struct {
	b    bytes.Buffer
	size int
}

func (c *chunkWriter) Write(p []byte) (int, error) {
	for rest := p; len(rest) > 0; {
		k := min(c.size, len(rest))
		c.b.Write(rest[:k])
		rest = rest[k:]
	}
	return len(p), nil
}

// TestFlushIndependent: wynik nie zależy od chwil wywołania Flush (także z otwartym znacznikiem początku) ani od
// tego, jak io.Writer dzieli zapis.
// REQ: DET-004
func TestFlushIndependent(t *testing.T) {
	doc := func(w *Writer, flush func()) error {
		return seq(
			w.Declaration("yes"), w.Start(Name{Space: "urn:a", Local: "a", Prefix: "p"}, bind("p", "urn:a")),
			do(flush), w.Attr(n("x"), "1"), do(flush), w.Attr(n("y"), strings.Repeat("&", 50)), do(flush),
			w.Text("t"), do(flush), w.Text("u"), w.Start(n("b")), do(flush), w.EndEmpty(), do(flush),
			w.Start(n("c")), do(flush), w.End(), w.Comment("c"), w.ProcInst("pi", "d"), do(flush), w.CDATA("x"), w.End(),
		)
	}
	plain, err := write(Options{}, func(w *Writer) error { return doc(w, func() {}) })
	if err != nil {
		t.Fatal(err)
	}
	cw := &chunkWriter{size: 3}
	w := New(cw, Options{})
	if err := seq(doc(w, func() { w.Flush() }), w.Close()); err != nil {
		t.Fatal(err)
	}
	if got := cw.b.String(); got != plain {
		t.Errorf("with Flush after every call and writes of 3 bytes:\n got  %s\n want %s", got, plain)
	}
}

func do(f func()) error { f(); return nil }

// TestDeclaration: deklaracja XML tylko na żądanie, jako pierwsza, wersja 1.0, UTF-8, opcjonalne standalone;
// wyjście jest w UTF-8 bez BOM.
// REQ: WR-002
func TestDeclaration(t *testing.T) {
	for standalone, want := range map[string]string{
		"":    "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<a>ż</a>",
		"yes": "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<a>ż</a>",
		"no":  "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n<a>ż</a>",
	} {
		got, err := write(Options{}, func(w *Writer) error {
			return seq(w.Declaration(standalone), w.Start(n("a")), w.Text("ż"), w.End())
		})
		if err != nil || got != want {
			t.Errorf("standalone %q: %q, %v", standalone, got, err)
		}
		d, err := xmlreader.New(strings.NewReader(got), xmlreader.Options{}).Declaration()
		if err != nil || d.Version != "1.0" || d.Standalone != standalone || d.BOM {
			t.Errorf("standalone %q read back: %+v, %v", standalone, d, err)
		}
	}
	got, err := write(Options{}, func(w *Writer) error { return seq(w.Start(n("a")), w.EndEmpty()) })
	if err != nil || got != "<a/>" {
		t.Errorf("without Declaration: %q, %v", got, err)
	}
	for name, calls := range map[string]func(w *Writer) error{
		"bad standalone": func(w *Writer) error { return w.Declaration("maybe") },
		"twice":          func(w *Writer) error { return seq(w.Declaration(""), w.Declaration("")) },
		"after comment":  func(w *Writer) error { return seq(w.Comment("c"), w.Declaration("")) },
		"after space":    func(w *Writer) error { return seq(w.Text("\n"), w.Declaration("")) },
		"after root":     func(w *Writer) error { return seq(w.Start(n("a")), w.Declaration("")) },
	} {
		if _, err := write(Options{}, calls); code(err) != CodeXMLDecl {
			t.Errorf("%s: %v, want %s", name, err, CodeXMLDecl)
		}
	}
}

// TestDocumentStructure: dokładnie jeden element główny; poza nim tylko komentarze, PI i białe znaki.
// REQ: WR-003
func TestDocumentStructure(t *testing.T) {
	got, err := write(Options{}, func(w *Writer) error {
		return seq(w.Comment("a"), w.Text(" \n\t"), w.ProcInst("p", ""), w.Start(n("r")), w.End(),
			w.Text("\n"), w.Comment("b"), w.ProcInst("q", "x"))
	})
	if want := "<!--a--> \n\t<?p?><r></r>\n<!--b--><?q x?>"; err != nil || got != want {
		t.Errorf("%q, %v; want %q", got, err, want)
	}
	if _, err := streamDump([]byte(got), false); err != nil {
		t.Errorf("the reader rejects it: %v", err)
	}
	for name, c := range map[string]struct {
		code  string
		calls func(w *Writer) error
	}{
		"second root":         {CodeOutsideRoot, func(w *Writer) error { return seq(w.Start(n("a")), w.End(), w.Start(n("b"))) }},
		"text before root":    {CodeOutsideRoot, func(w *Writer) error { return w.Text("x") }},
		"text after root":     {CodeOutsideRoot, func(w *Writer) error { return seq(w.Start(n("a")), w.End(), w.Text(" x")) }},
		"CDATA outside root":  {CodeOutsideRoot, func(w *Writer) error { return w.CDATA("") }},
		"CR outside root":     {CodeLineBreak, func(w *Writer) error { return w.Text("\r\n") }},
		"no root":             {CodeNoRoot, func(w *Writer) error { return w.Comment("only") }},
		"nothing":             {CodeNoRoot, func(w *Writer) error { return nil }},
		"open element":        {CodeEOF, func(w *Writer) error { return seq(w.Start(n("a")), w.Start(n("b")), w.End()) }},
		"end without element": {CodeUsageEnd, func(w *Writer) error { return w.End() }},
	} {
		if _, err := write(Options{}, c.calls); code(err) != c.code {
			t.Errorf("%s: %v, want %s", name, err, c.code)
		}
	}
	// po błędzie Close dokument można dokończyć
	var b bytes.Buffer
	w := New(&b, Options{})
	w.Start(n("a"))
	if code(w.Close()) != CodeEOF {
		t.Error("Close with an open element is not an error")
	}
	if err := seq(w.End(), w.Close()); err != nil || b.String() != "<a></a>" {
		t.Errorf("finishing after a failed Close: %q, %v", b.String(), err)
	}
}

// TestNames: nazwy elementów, atrybutów i celów PI: [5] Name, QName z niepustymi częściami, cel PI inny niż xml.
// REQ: WR-004
func TestNames(t *testing.T) {
	ns := Options{}
	plain := Options{NoNamespaces: true}
	for _, c := range []struct {
		opts Options
		code string
		fn   func(w *Writer) error
	}{
		{ns, CodeName, func(w *Writer) error { return w.Start(n("")) }},
		{ns, CodeName, func(w *Writer) error { return w.Start(n("1a")) }},
		{ns, CodeName, func(w *Writer) error { return w.Start(n("a b")) }},
		{ns, CodeName, func(w *Writer) error { return w.Start(n("a\xff")) }},
		{ns, CodeName, func(w *Writer) error { return w.Start(n("-a")) }},
		{ns, CodeName, func(w *Writer) error { return w.Start(n("a×")) }}, // U+00D7 nie jest znakiem nazwy
		{ns, CodeQName, func(w *Writer) error { return w.Start(n("a:b")) }},
		{ns, CodeQName, func(w *Writer) error { return w.Start(n(":a")) }},
		{ns, CodeName, func(w *Writer) error { return w.Start(Name{Local: "a", Prefix: "1"}) }},
		{ns, CodeQName, func(w *Writer) error { return w.Start(Name{Local: "a", Prefix: "p:q"}) }},
		{ns, CodeName, func(w *Writer) error { return w.Attr(n("a="), "") }},
		{ns, CodeQName, func(w *Writer) error { return w.Attr(n("p:a"), "") }},
		{ns, CodeName, func(w *Writer) error { return w.ProcInst("", "") }},
		{ns, CodeName, func(w *Writer) error { return w.ProcInst("a b", "") }},
		{ns, CodeQName, func(w *Writer) error { return w.ProcInst("a:b", "") }},
		{ns, CodeReservedPITarget, func(w *Writer) error { return w.ProcInst("xml", "") }},
		{ns, CodeReservedPITarget, func(w *Writer) error { return w.ProcInst("XmL", "") }},
		{plain, CodeName, func(w *Writer) error { return w.Start(n("a b")) }},
		{plain, CodeName, func(w *Writer) error { return w.Attr(n(""), "") }},
		{plain, CodeReservedPITarget, func(w *Writer) error { return w.ProcInst("XML", "") }},
		{plain, CodeUsageOptions, func(w *Writer) error { return w.Start(Name{Local: "a", Prefix: "p"}) }},
		{plain, CodeUsageOptions, func(w *Writer) error { return w.Attr(Name{Local: "a", Space: "urn:x"}, "") }},
		{plain, CodeUsageOptions, func(w *Writer) error { return w.Start(n("a"), bind("p", "urn:x")) }},
	} {
		expectCode(t, c.opts, c.code, c.fn)
	}
	// bez przestrzeni nazw dwukropek jest zwykłym znakiem nazwy
	got, err := write(plain, func(w *Writer) error {
		return seq(w.Start(n("a:b:c")), w.Attr(n("xmlns:p"), "u"), w.Attr(n(":x"), "1"), w.ProcInst("p:i", ""), w.End())
	})
	if want := `<a:b:c xmlns:p="u" :x="1"><?p:i?></a:b:c>`; err != nil || got != want {
		t.Errorf("NoNamespaces: %q, %v", got, err)
	}
	// nazwy spoza ASCII
	got, err = write(ns, func(w *Writer) error {
		return seq(w.Start(n("żółw")), w.Attr(n("日本"), "1"), w.Attr(n("_a-b.c·d"), "2"), w.EndEmpty())
	})
	if want := `<żółw 日本="1" _a-b.c·d="2"/>`; err != nil || got != want {
		t.Errorf("non-ASCII names: %q, %v", got, err)
	}
}

// TestCharacters: znak spoza [2] Char i dane, które nie są UTF-8, są błędem w każdym rodzaju danych; pisarz
// niczego nie usuwa ani nie zamienia.
// REQ: WR-005
func TestCharacters(t *testing.T) {
	bad := map[string]string{
		"\x00":             CodeChar,
		"a\x01b":           CodeChar,
		"\x0b":             CodeChar,
		"\x1f":             CodeChar,
		"￾":                CodeChar,
		"x￿":               CodeChar,
		"\xff":             CodeEncodingInvalid,
		"a\xc3":            CodeEncodingInvalid, // ucięty znak
		"\xc0\xaf":         CodeEncodingInvalid, // zapis nadmiarowy
		"\xe0\x80\x80":     CodeEncodingInvalid,
		"\xed\xa0\x80":     CodeEncodingInvalid, // surogat
		"\xf4\x90\x80\x80": CodeEncodingInvalid, // ponad U+10FFFF
		"\x80":             CodeEncodingInvalid,
	}
	for data, want := range bad {
		for kind, fn := range map[string]func(w *Writer) error{
			"text":    func(w *Writer) error { return w.Text(data) },
			"cdata":   func(w *Writer) error { return w.CDATA(data) },
			"comment": func(w *Writer) error { return w.Comment(data) },
			"pi":      func(w *Writer) error { return w.ProcInst("p", "d"+data) },
			"attr":    func(w *Writer) error { return w.Attr(n("a"), data) },
			"namespace": func(w *Writer) error {
				return w.Start(Name{Local: "a", Prefix: "p", Space: "u" + data}, bind("p", "u"+data))
			},
			"token": func(w *Writer) error { return w.WriteToken(Token{Kind: xmlreader.KindText, Data: []byte(data)}) },
		} {
			t.Run(fmt.Sprintf("%s %+q", kind, data), func(t *testing.T) { expectCode(t, Options{}, want, fn) })
		}
	}
	// wszystko, co wolno: granice zakresów [2] Char
	ok := "\t\n !~\x7f\u0080\u0085 ퟿�\U00010000\U0010FFFF"
	got, err := write(Options{}, func(w *Writer) error {
		return seq(w.Start(n("a")), w.Attr(n("v"), ok), w.Text(ok), w.CDATA(ok), w.Comment(ok), w.ProcInst("p", "x"+ok), w.End())
	})
	if err != nil || !utf8.ValidString(got) {
		t.Fatalf("allowed characters: %v", err)
	}
	dump, err := streamDump([]byte(got), false)
	if err != nil {
		t.Fatalf("the reader rejects allowed characters: %v", err)
	}
	want := fmt.Sprintf("declaration false standalone=\"\"\nstart {}\"a\"\n  attr {}\"v\"=%+q\ntext %+q\ntext %+q cdata\ncomment %+q\npi \"p\" %+q\nend {}\"a\"\n",
		ok, ok, ok, ok, "x"+ok)
	if dump != want {
		t.Errorf("read back:\n%s\nwant:\n%s", dump, want)
	}
}

// values to wartości, które muszą wrócić z odczytu bez zmiany.
var values = []string{
	"", " ", "  two  spaces  ", "\t", "\n", "\r", "\r\n", "a\rb\nc\td", "&", "<", ">", "\"", "'", "&amp;", "&#13;", "]]>",
	"a]]>b", "]]", "]>", "<![CDATA[x]]>", "<a b=\"c\">&d;</a>", "zażółć gęślą jaźń", "日本語 \U0001F600", "\u0085 ",
	"--", "?>", "x  y\n\n z", strings.Repeat("ab<", 20000),
}

// TestTextEscaping: tekst wraca z odczytu co do bajta; "&", "<", ">" i CR są zapisywane odwołaniami.
// REQ: WR-006
func TestTextEscaping(t *testing.T) {
	got, _ := write(Options{}, func(w *Writer) error { return seq(w.Start(n("a")), w.Text("a&b<c>d\re\nf\tg\"h'i]]>"), w.End()) })
	if want := "<a>a&amp;b&lt;c&gt;d&#13;e\nf\tg\"h'i]]&gt;</a>"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
	for _, v := range values {
		out, err := write(Options{}, func(w *Writer) error { return seq(w.Start(n("a")), w.Text(v), w.End()) })
		if err != nil {
			t.Errorf("%+q: %v", v, err)
			continue
		}
		r := xmlreader.New(strings.NewReader(out), xmlreader.Options{})
		r.Next()
		tok, err := r.Next()
		if v == "" {
			if tok.Kind != xmlreader.KindEndElement {
				t.Errorf("empty text wrote a token: %q", out)
			}
			continue
		}
		if err != nil || tok.Kind != xmlreader.KindText || string(tok.Data) != v {
			t.Errorf("%+q written as %+q read back as %+q, %v", v, clip(out), clip(string(tok.Data)), err)
		}
	}
	// kilka wywołań Text pod rząd to jeden tekst
	out, _ := write(Options{}, func(w *Writer) error { return seq(w.Start(n("a")), w.Text("]"), w.Text("]"), w.Text(">"), w.End()) })
	if out != "<a>]]&gt;</a>" {
		t.Errorf("split text: %q", out)
	}
}

func clip(s string) string {
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}

// TestAttrEscaping: wartość atrybutu w cudzysłowie, z odwołaniami dla "&", "<", cudzysłowu, TAB, LF i CR, wraca
// z odczytu bez zmiany mimo normalizacji 3.3.3.
// REQ: WR-007
func TestAttrEscaping(t *testing.T) {
	got, _ := write(Options{}, func(w *Writer) error {
		return seq(w.Start(n("a")), w.Attr(n("v"), "a&b<c>d\re\nf\tg\"h'i"), w.EndEmpty())
	})
	if want := `<a v="a&amp;b&lt;c>d&#13;e&#10;f&#9;g&quot;h'i"/>`; got != want {
		t.Errorf("%q, want %q", got, want)
	}
	for _, v := range values {
		out, err := write(Options{}, func(w *Writer) error {
			return seq(w.Start(Name{Local: "a", Prefix: "p", Space: "urn:" + v}, bind("p", "urn:"+v)), w.Attr(n("v"), v), w.EndEmpty())
		})
		if err != nil {
			t.Errorf("%+q: %v", v, err)
			continue
		}
		tok, err := xmlreader.New(strings.NewReader(out), xmlreader.Options{}).Next()
		if err != nil || len(tok.Attrs) != 1 || tok.Attrs[0].Value != v {
			t.Errorf("%+q written as %+q read back as %+v, %v", v, clip(out), tok.Attrs, err)
			continue
		}
		if tok.Name.Space != "urn:"+v {
			t.Errorf("namespace name %+q read back as %+q", clip("urn:"+v), clip(tok.Name.Space))
		}
	}
}

// TestAttributes: kolejność podania; powtórzenie nazwy kwalifikowanej albo rozwiniętej jest błędem; atrybut po
// rozpoczęciu treści jest błędem.
// REQ: WR-008
func TestAttributes(t *testing.T) {
	got, err := write(Options{}, func(w *Writer) error {
		return seq(w.Start(n("a"), bind("p", "urn:p")), w.Attr(n("z"), "1"), w.Attr(n("b"), "2"),
			w.Attr(qn("urn:p", "b", "p"), "3"), w.Attr(qn(nsXML, "lang", "xml"), "pl"), w.EndEmpty())
	})
	if want := `<a xmlns:p="urn:p" z="1" b="2" p:b="3" xml:lang="pl"/>`; err != nil || got != want {
		t.Errorf("%q, %v", got, err)
	}
	two := []Binding{bind("p", "urn:p"), bind("q", "urn:p")}
	for _, c := range []struct {
		code string
		fn   func(w *Writer) error
	}{
		{CodeUniqueAttSpec, func(w *Writer) error { return seq(w.Start(n("a")), w.Attr(n("x"), "1"), w.Attr(n("x"), "2")) }},
		{CodeUniqueAttSpec, func(w *Writer) error {
			return seq(w.Start(n("a"), two...), w.Attr(qn("urn:p", "x", "p"), "1"), w.Attr(qn("urn:p", "x", "p"), "2"))
		}},
		{CodeAttributesUnique, func(w *Writer) error {
			return seq(w.Start(n("a"), two...), w.Attr(qn("urn:p", "x", "p"), "1"), w.Attr(qn("urn:p", "x", "q"), "2"))
		}},
		{CodeUsageAttr, func(w *Writer) error { return seq(w.Start(n("a")), w.Text("t"), w.Attr(n("x"), "1")) }},
		{CodeUsageAttr, func(w *Writer) error { return seq(w.Start(n("a")), w.End(), w.Attr(n("x"), "1")) }},
		{CodeUsageAttr, func(w *Writer) error { return seq(w.Start(n("a")), w.Comment("c"), w.Attr(n("x"), "1")) }},
		{CodeUniqueAttSpec, func(w *Writer) error {
			return w.WriteToken(Token{Kind: xmlreader.KindStartElement, Name: n("a"), Attrs: []Attr{{Name: n("x")}, {Name: n("y")}, {Name: n("x")}}})
		}},
	} {
		expectLast(t, c.code, c.fn)
	}
	// wiele atrybutów: powtórzenie wykrywane także powyżej progu przeglądania listy; czas liniowy
	out, err := write(Options{Limits: Limits{MaxAttributes: -1}}, func(w *Writer) error {
		if err := w.Start(n("a"), bind("p", "urn:p")); err != nil {
			return err
		}
		for i := 0; i < 20000; i++ {
			if err := w.Attr(Name{Local: fmt.Sprintf("a%d", i)}, ""); err != nil {
				return err
			}
			if err := w.Attr(qn("urn:p", fmt.Sprintf("a%d", i), "p"), ""); err != nil {
				return err
			}
		}
		for _, dup := range []Name{n("a0"), n("a19999"), qn("urn:p", "a7", "p"), n("a31"), n("a32")} {
			if got := code(w.Attr(dup, "")); got != CodeUniqueAttSpec {
				return fmt.Errorf("duplicate %v among many attributes: %q", dup, got)
			}
		}
		return w.EndEmpty()
	})
	if err != nil || strings.Count(out, "=") != 40001 {
		t.Errorf("many attributes: %v, %d attributes written", err, strings.Count(out, "=")-1)
	}
}

// TestEndForms: Start i End dają <a></a>, EndEmpty daje <a/>; token SelfClosing daje <a/>, o ile element
// został pusty.
// REQ: WR-009
func TestEndForms(t *testing.T) {
	start := func(name string, self bool) Token {
		return Token{Kind: xmlreader.KindStartElement, Name: n(name), SelfClosing: self}
	}
	end := func(name string) Token { return Token{Kind: xmlreader.KindEndElement, Name: n(name)} }
	got, err := write(Options{}, func(w *Writer) error {
		return seq(w.Start(n("r")),
			w.Start(n("a")), w.End(),
			w.Start(n("b")), w.Attr(n("x"), "1"), w.EndEmpty(),
			w.Start(n("c")), w.Text(""), w.EndEmpty(), // pusty tekst nie jest treścią
			w.WriteToken(start("d", true)), w.WriteToken(end("d")),
			w.WriteToken(start("e", false)), w.WriteToken(end("e")),
			w.WriteToken(start("f", true)), w.Text("t"), w.WriteToken(end("f")), // filtr dopisał treść
			w.WriteToken(start("g", true)), w.End(),
			w.End())
	})
	if want := `<r><a></a><b x="1"/><c/><d/><e></e><f>t</f><g/></r>`; err != nil || got != want {
		t.Errorf("%q, %v\nwant %q", got, err, want)
	}
	expectLast(t, CodeUsageEnd, func(w *Writer) error { return seq(w.Text("t"), w.EndEmpty()) })
	expectCode(t, Options{}, CodeElementTypeMatch, func(w *Writer) error { return w.WriteToken(end("other")) })
	expectCode(t, Options{}, CodeElementTypeMatch, func(w *Writer) error {
		return w.WriteToken(Token{Kind: xmlreader.KindEndElement, Name: Name{Local: "r", Prefix: "p", Space: "urn:p"}})
	})
	if _, err := write(Options{}, func(w *Writer) error { return w.EndEmpty() }); code(err) != CodeUsageEnd {
		t.Errorf("EndEmpty without an element: %v", err)
	}
}

// TestCDATACommentPI: "]]>" w CDATA, "--" w komentarzu i "-" na jego końcu, "?>" w PI są błędami; pisarz nie
// dzieli sekcji ani nie zmienia danych. Powrotu karetki w tych danych żaden czytnik nie odda (2.11).
// REQ: WR-010 WR-018
func TestCDATACommentPI(t *testing.T) {
	got, err := write(Options{}, func(w *Writer) error {
		return seq(w.Start(n("a")), w.CDATA("<&>]] ]>\n"), w.CDATA(""), w.Comment(" a - b "), w.Comment(""), w.ProcInst("p", "a ? > b"),
			w.ProcInst("q", ""), w.End())
	})
	if want := "<a><![CDATA[<&>]] ]>\n]]><![CDATA[]]><!-- a - b --><!----><?p a ? > b?><?q?></a>"; err != nil || got != want {
		t.Errorf("%q, %v", got, err)
	}
	if _, err := streamDump([]byte(got), false); err != nil {
		t.Errorf("the reader rejects it: %v", err)
	}
	for _, c := range []struct {
		code string
		fn   func(w *Writer) error
	}{
		{CodeCDATAEnd, func(w *Writer) error { return w.CDATA("a]]>b") }},
		{CodeCDATAEnd, func(w *Writer) error {
			return w.WriteToken(Token{Kind: xmlreader.KindText, CDATA: true, Data: []byte("]]>")})
		}},
		{CodeComment, func(w *Writer) error { return w.Comment("a--b") }},
		{CodeComment, func(w *Writer) error { return w.Comment("a-") }},
		{CodeComment, func(w *Writer) error { return w.Comment("-") }},
		{CodePIData, func(w *Writer) error { return w.ProcInst("p", "a?>b") }},
		{CodePIData, func(w *Writer) error { return w.ProcInst("p", " a") }},
		{CodePIData, func(w *Writer) error { return w.ProcInst("p", "\na") }},
		{CodeLineBreak, func(w *Writer) error { return w.CDATA("a\rb") }},
		{CodeLineBreak, func(w *Writer) error { return w.Comment("a\r\nb") }},
		{CodeLineBreak, func(w *Writer) error { return w.ProcInst("p", "a\r") }},
	} {
		expectCode(t, Options{}, c.code, c.fn)
	}
}

// TestWriteToken: token DOCTYPE jest pomijany albo, według opcji, jest błędem; nieznany rodzaj tokenu jest
// błędem; dane tokenu nie są zatrzymywane.
// REQ: WR-013
func TestWriteToken(t *testing.T) {
	src := "<!DOCTYPE a [<!ATTLIST a d CDATA 'def' xmlns:p CDATA 'urn:p'><!ENTITY e 'ent'>]>\n<a x='1'>&e;<![CDATA[c]]><!--k--><?p d?><b/></a>"
	out, err := rewrite([]byte(src), false, Options{})
	if want := `<a xmlns:p="urn:p" x="1" d="def">ent<![CDATA[c]]><!--k--><?p d?><b/></a>`; err != nil || string(out) != want {
		t.Errorf("%q, %v\nwant %q", out, err, want)
	}
	if _, err := rewrite([]byte(src), false, Options{RejectDoctype: true}); !strings.Contains(fmt.Sprint(err), CodeDoctype) {
		t.Errorf("RejectDoctype: %v", err)
	}
	expectCode(t, Options{}, CodeUsageToken, func(w *Writer) error { return w.WriteToken(Token{}) })
	expectCode(t, Options{}, CodeUsageToken, func(w *Writer) error { return w.WriteToken(Token{Kind: 99}) })

	// bufory tokenu wolno zmienić zaraz po wywołaniu
	var b bytes.Buffer
	w := New(&b, Options{})
	data := []byte("first")
	attrs := []Attr{{Name: n("x"), Value: "1"}}
	decls := []Binding{bind("p", "urn:p")}
	w.WriteToken(Token{Kind: xmlreader.KindStartElement, Name: n("a"), Attrs: attrs, Namespaces: decls})
	attrs[0], decls[0] = Attr{Name: n("changed")}, bind("q", "urn:q")
	w.WriteToken(Token{Kind: xmlreader.KindText, Data: data})
	copy(data, "XXXXX")
	w.WriteToken(Token{Kind: xmlreader.KindStartElement, Name: qn("urn:p", "b", "p"), SelfClosing: true})
	w.WriteToken(Token{Kind: xmlreader.KindEndElement})
	w.WriteToken(Token{Kind: xmlreader.KindEndElement, Name: n("a")})
	if err := w.Close(); err != nil || b.String() != `<a xmlns:p="urn:p" x="1">first<p:b/></a>` {
		t.Errorf("%q, %v", b.String(), err)
	}
}

// sliceReader to źródło tokenów inne niż czytnik.
type sliceReader struct {
	toks []Token
	i    int
}

func (s *sliceReader) Next() (Token, error) {
	if s.i == len(s.toks) {
		return Token{}, errEOF
	}
	s.i++
	return s.toks[s.i-1], nil
}

var errEOF = errors.New("eof")

// TestOtherTokenSources: pisarz polega wyłącznie na tokenach. Strumień skopiowany z czytnika do wycinka
// i podany z niego daje te same bajty co czytnik.
// REQ: STR-007
func TestOtherTokenSources(t *testing.T) {
	for _, file := range corpusFiles(t) {
		src := readFile(t, file)
		direct, err := rewrite(src, false, Options{})
		if err != nil {
			t.Fatal(err)
		}
		var toks []Token
		r := xmlreader.New(bytes.NewReader(src), xmlreader.Options{})
		d, _ := r.Declaration()
		for {
			tok, err := r.Next()
			if err != nil {
				break
			}
			tok.Data = append([]byte(nil), tok.Data...)
			tok.Attrs = append([]Attr(nil), tok.Attrs...)
			tok.Namespaces = append([]Binding(nil), tok.Namespaces...)
			tok.Line, tok.Column, tok.EndLine, tok.EndColumn = 0, 0, 0, 0
			toks = append(toks, tok)
		}
		var src2 xmlreader.TokenReader = &sliceReader{toks: toks}
		var b bytes.Buffer
		w := New(&b, Options{})
		if d.Version != "" {
			w.Declaration(d.Standalone)
		}
		for {
			tok, err := src2.Next()
			if err != nil {
				break
			}
			if err := w.WriteToken(tok); err != nil {
				t.Fatalf("%s: %v", file, err)
			}
		}
		if err := w.Close(); err != nil || !bytes.Equal(b.Bytes(), direct) {
			t.Errorf("%s: tokens from a slice give other bytes than the reader (%v)", file, err)
		}
	}
}
