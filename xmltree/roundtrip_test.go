package xmltree

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

var update = flag.Bool("update", false, "rewrite the ratchet testdata/w3c/tree.ratchet")

const (
	corpusDir   = "../testdata/corpus"
	xmlconfRoot = "../testdata/w3c/xmlconf"
	treeRatchet = "../testdata/w3c/tree.ratchet"
)

// dump zapisuje strumień tokenów w postaci, w której strumienie równe według docs/design.md („Co wraca z zapisu”)
// dają ten sam tekst: bez pozycji, bez Specified i Type, bez tokenu DOCTYPE.
func dump(src xmlreader.TokenReader) (string, error) { return dumpTokens(src, false) }

// dumpTokens z merge łączy sąsiednie tokeny zwykłego tekstu w jeden, tak jak połączy je czytnik po zapisie.
func dumpTokens(src xmlreader.TokenReader, merge bool) (string, error) {
	var b strings.Builder
	name := func(n xmlreader.Name) string {
		s := n.Local
		if n.Prefix != "" {
			s = n.Prefix + ":" + s
		}
		return fmt.Sprintf("{%s}%+q", n.Space, s)
	}
	var text []byte
	pending := false
	flush := func() {
		if pending {
			fmt.Fprintf(&b, "text %+q cdata=false whitespace=%v\n", text, isSpace(text))
			text, pending = text[:0], false
		}
	}
	for {
		t, err := src.Next()
		if err != nil {
			flush()
			if err == io.EOF {
				return b.String(), nil
			}
			return b.String(), err
		}
		if merge && t.Kind == xmlreader.KindText && !t.CDATA {
			text, pending = append(text, t.Data...), true
			continue
		}
		flush()
		switch t.Kind {
		case xmlreader.KindStartElement:
			fmt.Fprintf(&b, "start %s", name(t.Name))
			if t.SelfClosing {
				b.WriteString(" selfclosing")
			}
			b.WriteString("\n")
			for _, ns := range t.Namespaces {
				fmt.Fprintf(&b, "  ns %+q=%+q\n", ns.Prefix, ns.URI)
			}
			for _, a := range t.Attrs {
				fmt.Fprintf(&b, "  attr %s=%+q\n", name(a.Name), a.Value)
			}
		case xmlreader.KindEndElement:
			fmt.Fprintf(&b, "end %s\n", name(t.Name))
		case xmlreader.KindText:
			fmt.Fprintf(&b, "text %+q cdata=%v whitespace=%v\n", t.Data, t.CDATA, t.Whitespace)
		case xmlreader.KindComment:
			fmt.Fprintf(&b, "comment %+q\n", t.Data)
		case xmlreader.KindProcInst:
			fmt.Fprintf(&b, "pi %+q %+q\n", t.Name.Local, t.Data)
		}
	}
}

func dumpBytes(src []byte, noNS bool) (string, error) {
	return dump(xmlreader.New(bytes.NewReader(src), xmlreader.Options{NoNamespaces: noNS}))
}

// passThrough to przelot czytnik → pisarz, z deklaracją XML, gdy miał ją dokument. island, jeśli nie jest nil,
// dostaje każdy token początku elementu i mówi, czy ten element ma przejść przez drzewo (ReadElement → WriteTo).
func passThrough(src []byte, noNS bool, island func(n int, t xmlreader.Token) bool, change func(*Node)) ([]byte, error) {
	var out bytes.Buffer
	r := xmlreader.New(bytes.NewReader(src), xmlreader.Options{NoNamespaces: noNS})
	w := xmlwriter.New(&out, xmlwriter.Options{NoNamespaces: noNS})
	d, err := r.Declaration()
	if err != nil {
		return nil, err
	}
	if d.Version != "" {
		if err := w.Declaration(d.Standalone); err != nil {
			return nil, err
		}
	}
	for n := 0; ; {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if t.Kind == xmlreader.KindStartElement {
			n++
			if island != nil && island(n, t) {
				el, err := ReadElement(r, t, Options{})
				if err != nil {
					return nil, fmt.Errorf("ReadElement: %w", err)
				}
				if change != nil {
					change(el)
				}
				if err := el.WriteTo(w); err != nil {
					return nil, fmt.Errorf("WriteTo: %w", err)
				}
				continue
			}
		}
		if err := w.WriteToken(t); err != nil {
			return nil, fmt.Errorf("writer: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("writer: %w", err)
	}
	return out.Bytes(), nil
}

func writeDoc(d *Document, opts xmlwriter.Options) ([]byte, error) {
	var out bytes.Buffer
	w := xmlwriter.New(&out, opts)
	if err := d.WriteTo(w); err != nil {
		return out.Bytes(), err
	}
	if err := w.Close(); err != nil {
		return out.Bytes(), err
	}
	return out.Bytes(), nil
}

// treeRoundTrip sprawdza K3 dla jednego dokumentu przyjętego przez czytnik: Parse → WriteTo daje bajty przelotu,
// NewReader daje strumień źródła, a strumień drzewa przez pisarza daje te same bajty. Zwraca opis porażki albo "".
func treeRoundTrip(src []byte, noNS bool) string {
	want, err := passThrough(src, noNS, nil, nil)
	if err != nil {
		return "pass-through: " + err.Error()
	}
	doc, err := Parse(bytes.NewReader(src), Options{Reader: xmlreader.Options{NoNamespaces: noNS}})
	if err != nil {
		return "Parse: " + err.Error()
	}
	got, err := writeDoc(doc, xmlwriter.Options{NoNamespaces: noNS})
	if err != nil {
		return "WriteTo: " + err.Error()
	}
	if !bytes.Equal(got, want) {
		return "Parse → WriteTo gives other bytes than reader → writer\n" + firstDiff(string(got), string(want))
	}
	wantStream, err := dumpBytes(src, noNS)
	if err != nil {
		return "reader: " + err.Error()
	}
	gotStream, err := dump(NewReader(doc.Node()))
	if err != nil {
		return "NewReader: " + err.Error()
	}
	if gotStream != wantStream {
		return "NewReader gives another token stream than the source\n" + firstDiff(gotStream, wantStream)
	}
	// drzewo jako źródło tokenów pisarza (STR-007): te same bajty
	var out bytes.Buffer
	w := xmlwriter.New(&out, xmlwriter.Options{NoNamespaces: noNS})
	if doc.XMLDeclaration {
		w.Declaration(doc.Standalone)
	}
	for r := NewReader(doc.Node()); ; {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err == nil {
			err = w.WriteToken(t)
		}
		if err != nil {
			return "NewReader → WriteToken: " + err.Error()
		}
	}
	if err := w.Close(); err != nil || !bytes.Equal(out.Bytes(), want) {
		return fmt.Sprintf("NewReader → WriteToken gives other bytes than reader → writer (%v)", err)
	}
	// kopia głęboka zapisuje się tak samo
	clone := NewDocument()
	clone.XMLDeclaration, clone.Standalone = doc.XMLDeclaration, doc.Standalone
	for c := doc.Node().FirstChild(); c != nil; c = c.NextSibling() {
		if err := clone.Node().Append(c.Clone()); err != nil {
			return "Clone: " + err.Error()
		}
	}
	if got, err := writeDoc(clone, xmlwriter.Options{NoNamespaces: noNS}); err != nil || !bytes.Equal(got, want) {
		return fmt.Sprintf("a deep copy is written with other bytes (%v)\n%s", err, firstDiff(string(got), string(want)))
	}
	return ""
}

func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		var a, b string
		if i < len(g) {
			a = g[i]
		}
		if i < len(w) {
			b = w[i]
		}
		if a != b {
			return fmt.Sprintf("line %d:\n got  %s\n want %s", i+1, a, b)
		}
	}
	return ""
}

func corpusFiles(t testing.TB) []string {
	all, err := filepath.Glob(filepath.Join(corpusDir, "*.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, f := range all {
		if !strings.HasSuffix(f, ".out.xml") {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		t.Fatalf("no corpus documents in %s", corpusDir)
	}
	return files
}

// w3cDocuments zwraca dokumenty zestawu W3C, które czytnik przyjmuje (zakres K2 i K3), z trybem przestrzeni nazw.
func w3cDocuments(t testing.TB) (ids []string, srcs [][]byte, noNS []bool) {
	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err != nil {
		t.Skip(testkit.FetchHint)
	}
	for _, ct := range testkit.LoadXMLConf(t, xmlconfRoot) {
		if ct.Excluded != "" || ct.Type == "not-wf" {
			continue
		}
		src, err := os.ReadFile(path.Join(xmlconfRoot, ct.Dir, ct.URI))
		if err != nil {
			t.Fatal(err)
		}
		no := !strings.HasPrefix(ct.Recommendation, "NS")
		if _, err := dumpBytes(src, no); err != nil {
			continue // porażka K1: czytnik nie daje strumienia
		}
		ids, srcs, noNS = append(ids, ct.Group+"/"+ct.ID), append(srcs, src), append(noNS, no)
	}
	return
}

// TestTreeRoundTrip mierzy kryterium K3 na korpusie i na zestawie W3C XML Conformance: dla każdego dokumentu,
// który czytnik przyjmuje, Parse → WriteTo daje te same bajty co czytnik → pisarz, NewReader daje strumień tokenów
// źródła, a kopia głęboka zapisuje się tak samo. Liczba zaliczonych nie spada (zapadka testdata/w3c/tree.ratchet).
// REQ: TR-001 TR-003 TR-010 TR-012 STR-007 DET-002
func TestTreeRoundTrip(t *testing.T) {
	for _, file := range corpusFiles(t) {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if why := treeRoundTrip(src, false); why != "" {
			t.Errorf("%s: %s", filepath.Base(file), why)
		}
	}
	ids, srcs, noNS := w3cDocuments(t)
	inScope, passed := map[string]bool{}, map[string]bool{}
	var failures []string
	for i, id := range ids {
		inScope[id] = true
		if why := treeRoundTrip(srcs[i], noNS[i]); why != "" {
			failures = append(failures, id+": "+why)
			continue
		}
		passed[id] = true
	}
	t.Logf("tree round trip on the W3C XML Conformance suite: %d of %d documents the reader accepts", len(passed), len(inScope))
	for i, f := range failures {
		if i < 20 || testing.Verbose() {
			t.Log("fail: " + f)
		}
	}
	testkit.Ratchet(t, treeRatchet, "Tree round trip on W3C XML Conformance (TestTreeRoundTrip, go test ./xmltree -update).",
		inScope, passed, *update)
	if len(passed) != len(inScope) {
		t.Errorf("K3 needs every document: %d of %d", len(passed), len(inScope))
	}
}

// TestSubtreeInStream: element zmaterializowany w środku przelotu (ReadElement → WriteTo) nie zmienia wyniku
// całości, którykolwiek element dokumentu to jest: źródło zostaje tuż za jego znacznikiem końca, a zapis poddrzewa
// daje te same bajty co zapis jego tokenów. Sprawdzane dla każdego elementu każdego dokumentu korpusu i dla
// pierwszych elementów każdego dokumentu W3C.
// REQ: TR-002 TR-005 TR-010
func TestSubtreeInStream(t *testing.T) {
	check := func(name string, src []byte, noNS bool, most int) int {
		want, err := passThrough(src, noNS, nil, nil)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return 0
		}
		elements := 0
		for k := 1; k <= most; k++ {
			found := false
			got, err := passThrough(src, noNS, func(n int, _ xmlreader.Token) bool {
				if n == k {
					found = true
				}
				return n == k
			}, nil)
			if !found {
				break
			}
			elements++
			if err != nil || !bytes.Equal(got, want) {
				t.Errorf("%s: element %d through a tree changes the output (%v)\n%s", name, k, err, firstDiff(string(got), string(want)))
				break
			}
		}
		return elements
	}
	total := 0
	for _, file := range corpusFiles(t) {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		most := 1 << 30
		if len(src) > 20000 {
			most = 40
		}
		total += check(filepath.Base(file), src, false, most)
	}
	// kilka poddrzew naraz: co trzeci element najwyższych poziomów
	for _, file := range corpusFiles(t) {
		src, _ := os.ReadFile(file)
		want, _ := passThrough(src, false, nil, nil)
		got, err := passThrough(src, false, func(n int, _ xmlreader.Token) bool { return n > 1 && n%3 == 0 }, nil)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: every third element through a tree changes the output (%v)", filepath.Base(file), err)
		}
	}
	ids, srcs, noNS := w3cDocuments(t)
	for i, id := range ids {
		total += check(id, srcs[i], noNS[i], 6)
	}
	t.Logf("%d elements read as a subtree in the middle of a stream", total)
}
