package xmlwriter

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlreader"
)

// FuzzRoundTrip: dla wejścia, które czytnik przyjmuje (z przestrzeniami nazw albo bez), pisarz zapisuje strumień
// bez błędu, odczyt zapisu daje równy strumień tokenów (docs/design.md), a drugi zapis te same bajty. Wejście,
// które czytnik odrzuca, nie może dać błędu pisarza przed błędem czytnika.
// REQ: STR-001 STR-002
func FuzzRoundTrip(f *testing.F) {
	for _, s := range []string{
		"<a/>",
		"<?xml version='1.0' standalone='yes'?><a x='1' xmlns:p='u'><p:b p:c='&#10;'>t&amp;&#13;<![CDATA[c]]></p:b><!--c--><?pi d?></a>",
		`<!DOCTYPE a [<!ENTITY e "x<b>&f;</b>"><!ENTITY f ""><!ATTLIST a d CDATA "v" xmlns:q CDATA "urn:q">]><a>&e;&f;</a>`,
		"\xff\xfe<\x00a\x00/\x00>\x00",
		"<a xmlns='urn:d'><b xmlns=''/><c xml:space='preserve'> </c></a>",
	} {
		f.Add([]byte(s), false)
		f.Add([]byte(s), true)
	}
	for _, dir := range []struct {
		path string
		max  int
	}{{"../testdata/corpus", 50}, {"../testdata/w3c/xmlconf", 150}} {
		for _, b := range testkit.SampleFiles(dir.path, []string{".xml"}, dir.max, 128<<10) {
			f.Add(b, false)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, noNS bool) {
		if _, err := streamDump(data, noNS); err != nil {
			if _, werr := rewrite(data, noNS, Options{}); werr == nil || strings.HasPrefix(werr.Error(), "writer:") {
				t.Fatalf("the reader rejects the input (%v), the pass-through says %v", err, werr)
			}
			return
		}
		if _, why := roundTrip(data, noNS); why != "" {
			t.Fatal(why)
		}
	})
}

// Słowniki FuzzWriter: nazwy, przestrzenie nazw i dane, które trafiają w reguły pisarza.
var (
	fuzzNames  = []string{"a", "b", "c", "żółw", "p", "q", "xml", "xmlns", "x:y", "", "1a", "a b", "A-b.c_d", "ns1", "ns2", "lang", "a\xff"}
	fuzzSpaces = []string{"", "", "urn:a", "urn:b", "urn:c", nsXML, nsXMLNS, "urn:<&\">", "urn:\x00"}
	fuzzData   = []string{"", "v", "text", " ", "\n", "\t", "\r", "a\r\nb", "<&>\"'", "]]>", "]]", ">", "--", "-", "?>", " lead", "ż \U0001F600",
		"\x00", "\x1f", "\xff", "\xed\xa0\x80", "￾", "&amp;", "<![CDATA[", strings.Repeat("x&", 40)}
)

// fuzzInput oddaje kolejne bajty wejścia fuzzera; po końcu zera.
type fuzzInput struct {
	data []byte
	i    int
}

func (in *fuzzInput) byte() byte {
	if in.i >= len(in.data) {
		return 0
	}
	in.i++
	return in.data[in.i-1]
}

func (in *fuzzInput) pick(table []string) string {
	b := in.byte()
	if b >= 0xF0 { // surowe bajty wejścia zamiast słownika
		n := int(in.byte() % 16)
		start := min(in.i, len(in.data))
		end := min(start+n, len(in.data))
		in.i = end
		return string(in.data[start:end])
	}
	return table[int(b)%len(table)]
}

func (in *fuzzInput) name() Name {
	return Name{Space: in.pick(fuzzSpaces), Local: in.pick(fuzzNames), Prefix: in.pick([]string{"", "", "p", "q", "xml", "xmlns", "ns1", "1"})}
}

// fuzzOp to jedno wywołanie pisarza i jego skutek w modelu oczekiwanego strumienia.
type fuzzOp struct {
	call  func(w *Writer) error
	model func(m *fuzzModel)
}

// fuzzModel buduje strumień, który czytnik powinien oddać z wyjścia pisarza: te same nazwy rozwinięte, wartości
// i dane, w tej samej kolejności.
type fuzzModel struct {
	lines  []string
	starts []int // indeksy wierszy „start” otwartych elementów
	empty  bool  // ostatni element nie dostał treści
	text   bool  // ostatni wiersz to tekst, który kolejne wywołanie Text przedłuża
	prefix bool  // porównywać prefiksy (tryb jawny)
}

func (m *fuzzModel) name(n Name) string {
	if m.prefix && n.Prefix != "" {
		return fmt.Sprintf("{%s}%s:%s", n.Space, n.Prefix, n.Local)
	}
	return fmt.Sprintf("{%s}%s", n.Space, n.Local)
}

func (m *fuzzModel) add(line string) { m.lines, m.text, m.empty = append(m.lines, line), false, false }

func decodeOps(in *fuzzInput) []fuzzOp {
	var ops []fuzzOp
	for len(ops) < 200 && in.i < len(in.data) {
		switch in.byte() % 12 {
		case 0, 1:
			name := in.name()
			var decls []Binding
			for k := int(in.byte() % 4); k > 1; k-- {
				decls = append(decls, Binding{Prefix: in.pick([]string{"", "p", "q", "xml", "xmlns", "ns1", "ns2", "1"}), URI: in.pick(fuzzSpaces)})
			}
			ops = append(ops, fuzzOp{
				func(w *Writer) error { return w.Start(name, decls...) },
				func(m *fuzzModel) {
					m.starts = append(m.starts, len(m.lines))
					m.add("start " + m.name(name))
					m.empty = true
				}})
		case 2, 3:
			name, value := in.name(), in.pick(fuzzData)
			ops = append(ops, fuzzOp{
				func(w *Writer) error { return w.Attr(name, value) },
				func(m *fuzzModel) {
					m.lines = append(m.lines, fmt.Sprintf("  attr %s=%q", m.name(name), value))
				}})
		case 4:
			data := in.pick(fuzzData)
			ops = append(ops, fuzzOp{
				func(w *Writer) error { return w.Text(data) },
				func(m *fuzzModel) {
					switch {
					case len(m.starts) == 0 || data == "": // białe znaki poza korzeniem nie są tokenem
					case m.text:
						m.lines[len(m.lines)-1] += data
					default:
						m.add("text " + data)
						m.text = true
					}
				}})
		case 5:
			data := in.pick(fuzzData)
			ops = append(ops, fuzzOp{func(w *Writer) error { return w.CDATA(data) }, func(m *fuzzModel) { m.add("cdata " + data) }})
		case 6:
			data := in.pick(fuzzData)
			ops = append(ops, fuzzOp{func(w *Writer) error { return w.Comment(data) }, func(m *fuzzModel) { m.add("comment " + data) }})
		case 7:
			target, data := in.pick(fuzzNames), in.pick(fuzzData)
			ops = append(ops, fuzzOp{func(w *Writer) error { return w.ProcInst(target, data) }, func(m *fuzzModel) { m.add("pi " + target + " " + data) }})
		case 8, 9:
			ops = append(ops, fuzzOp{func(w *Writer) error { return w.End() }, (*fuzzModel).end})
		case 10:
			ops = append(ops, fuzzOp{func(w *Writer) error { return w.EndEmpty() }, func(m *fuzzModel) {
				m.lines[m.starts[len(m.starts)-1]] += " selfclosing"
				m.end()
			}})
		case 11:
			ops = append(ops, fuzzOp{func(w *Writer) error { return w.Flush() }, func(m *fuzzModel) {}})
		}
	}
	return ops
}

func (m *fuzzModel) end() {
	m.starts = m.starts[:len(m.starts)-1]
	m.add("end")
}

// fuzzDump czyta wyjście pisarza i zapisuje strumień tak jak fuzzModel.
func fuzzDump(out []byte, noNS, prefix, dropSpace bool) (string, error) {
	m := fuzzModel{prefix: prefix}
	var lines []string
	r := xmlreader.New(bytes.NewReader(out), xmlreader.Options{NoNamespaces: noNS})
	for {
		t, err := r.Next()
		if err == io.EOF {
			return strings.Join(lines, "\n"), nil
		}
		if err != nil {
			return "", err
		}
		switch t.Kind {
		case xmlreader.KindStartElement:
			l := "start " + m.name(t.Name)
			if t.SelfClosing {
				l += " selfclosing"
			}
			lines = append(lines, l)
			for _, a := range t.Attrs {
				lines = append(lines, fmt.Sprintf("  attr %s=%q", m.name(a.Name), a.Value))
			}
		case xmlreader.KindEndElement:
			lines = append(lines, "end")
		case xmlreader.KindText:
			switch {
			case t.CDATA:
				lines = append(lines, "cdata "+string(t.Data))
			case !(dropSpace && t.Whitespace):
				lines = append(lines, "text "+string(t.Data))
			}
		case xmlreader.KindComment:
			lines = append(lines, "comment "+string(t.Data))
		case xmlreader.KindProcInst:
			lines = append(lines, "pi "+t.Name.Local+" "+string(t.Data))
		}
	}
}

// FuzzWriter: dowolny ciąg wywołań z dowolnymi nazwami i danymi. Każde wywołanie kończy się powodzeniem albo
// błędem *Error ze stałym kodem; wywołanie nieudane niczego nie zmienia (te same bajty daje ciąg złożony z samych
// udanych wywołań); to, co pisarz zapisał i zamknął bez błędu, czytnik z ustawieniami domyślnymi czyta bez błędu
// i oddaje te same nazwy rozwinięte, wartości atrybutów i dane w tej samej kolejności.
// REQ: WR-014 SAFE-005
func FuzzWriter(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0, 2, 0, 0, 0, 1, 4, 2, 8})
	f.Add([]byte{2, 0, 2, 0, 2, 0, 2, 0, 3, 2, 3, 0, 1, 4, 8, 5, 3, 6, 4, 7, 0, 1, 10, 8})
	f.Add([]byte{1, 0, 2, 2, 3, 4, 5, 1, 2, 2, 3, 0, 10, 0xF5, 3, 'a', 'b', 'c', 8})
	f.Add([]byte{6, 0, 4, 1, 4, 3, 0, 4, 0, 0, 0, 4, 8, 9, 4, 10, 6, 12, 7, 0, 13})
	for i := 0; i < 64; i++ {
		seed := make([]byte, 48)
		for j := range seed {
			seed[j] = byte((i*31 + j*17 + i*j) % 251)
		}
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		in := &fuzzInput{data: data}
		mode := in.byte()
		opts := Options{NoNamespaces: mode&3 == 1, AutoNamespaces: mode&3 == 2}
		if mode&4 != 0 {
			opts.Indent = " "
		}
		if mode&8 != 0 {
			opts.Limits = Limits{MaxDepth: 3, MaxNameLength: 6, MaxAttributes: 3, MaxTextSize: 12}
		}
		ops := decodeOps(in)

		var out bytes.Buffer
		w := New(&out, opts)
		model := fuzzModel{prefix: !opts.AutoNamespaces}
		ok := make([]bool, len(ops))
		for i, op := range ops {
			err := op.call(w)
			if err == nil {
				ok[i] = true
				op.model(&model)
				continue
			}
			if e, isErr := err.(*Error); !isErr || e.Code == "" || e.Call != i+1 {
				t.Fatalf("call %d: error %#v, want an *Error with a code and call %d", i+1, err, i+1)
			}
		}
		for len(model.starts) > 0 {
			if err := w.End(); err != nil {
				t.Fatalf("End of an open element: %v", err)
			}
			model.end()
		}
		closeErr := w.Close()
		rootless := true
		for _, l := range model.lines {
			rootless = rootless && !strings.HasPrefix(l, "start ")
		}
		if rootless {
			if code(closeErr) != CodeNoRoot {
				t.Fatalf("Close without a root element: %v", closeErr)
			}
			return
		}
		if closeErr != nil {
			t.Fatalf("Close: %v", closeErr)
		}

		// same udane wywołania dają te same bajty
		var out2 bytes.Buffer
		w2 := New(&out2, opts)
		for i, op := range ops {
			if ok[i] {
				if err := op.call(w2); err != nil {
					t.Fatalf("call %d succeeded among failing calls, fails without them: %v", i+1, err)
				}
			}
		}
		for len(w2.stack) > 0 {
			w2.End()
		}
		if err := w2.Close(); err != nil || !bytes.Equal(out.Bytes(), out2.Bytes()) {
			t.Fatalf("failed calls changed the output (%v):\n with    %q\n without %q", err, out.Bytes(), out2.Bytes())
		}

		// czytnik odtwarza to, co podano
		indent := opts.Indent != ""
		got, err := fuzzDump(out.Bytes(), opts.NoNamespaces, model.prefix, indent)
		if err != nil {
			t.Fatalf("the reader rejects what the writer wrote: %v\n%q", err, out.Bytes())
		}
		want := model.lines
		if indent { // wcięcia dodają tokeny białych znaków: porównujemy bez nich po obu stronach
			want = nil
			for _, l := range model.lines {
				if !strings.HasPrefix(l, "text ") || strings.Trim(l[5:], " \t\n\r") != "" {
					want = append(want, l)
				}
			}
		}
		if got != strings.Join(want, "\n") {
			t.Fatalf("the reader gives another stream than the calls:\n%s\noutput: %q", firstDiff(got, strings.Join(want, "\n")), out.Bytes())
		}
	})
}
