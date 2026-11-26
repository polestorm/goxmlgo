package xmlreader

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
)

const corpusDir = "../testdata/corpus"

// dumpTokens zapisuje strumień tokenów dokumentu jako tekst: jeden wiersz na token, pod tokenem początku elementu
// jego deklaracje przestrzeni nazw i atrybuty. Napisy są w cudzysłowie z samym ASCII (%+q), więc wzorzec nie
// zależy od kodowania pliku ani od systemu. Ostatni wiersz to „eof” albo błąd z kodem i pozycją.
func dumpTokens(src io.Reader) string {
	var b strings.Builder
	name := func(n Name) string {
		s := n.Local
		if n.Prefix != "" {
			s = n.Prefix + ":" + s
		}
		return fmt.Sprintf("{%s}%+q", n.Space, s)
	}
	r := New(src, Options{})
	for {
		t, err := r.Next()
		if err == io.EOF {
			b.WriteString("eof\n")
			return b.String()
		}
		if err != nil {
			e := err.(*Error)
			fmt.Fprintf(&b, "error %s %d:%d\n", e.Code, e.Line, e.Column)
			return b.String()
		}
		fmt.Fprintf(&b, "%d:%d ", t.Line, t.Column)
		switch t.Kind {
		case KindStartElement:
			fmt.Fprintf(&b, "start %s end=%d:%d", name(t.Name), t.EndLine, t.EndColumn)
			if t.SelfClosing {
				b.WriteString(" selfclosing")
			}
			b.WriteString("\n")
			for _, ns := range r.DeclaredNamespaces() {
				fmt.Fprintf(&b, "  ns %+q=%+q\n", ns.Prefix, ns.URI)
			}
			for _, a := range t.Attrs {
				fmt.Fprintf(&b, "  attr %s=%+q", name(a.Name), a.Value)
				if a.Type != "" {
					b.WriteString(" type=" + a.Type)
				}
				if !a.Specified {
					b.WriteString(" default")
				}
				fmt.Fprintf(&b, " %d:%d\n", a.Line, a.Column)
			}
		case KindEndElement:
			fmt.Fprintf(&b, "end %s\n", name(t.Name))
		case KindText:
			fmt.Fprintf(&b, "text %+q", t.Data)
			if t.CDATA {
				b.WriteString(" cdata")
			}
			if t.Whitespace {
				b.WriteString(" whitespace")
			}
			b.WriteString("\n")
		case KindComment:
			fmt.Fprintf(&b, "comment %+q\n", t.Data)
		case KindProcInst:
			fmt.Fprintf(&b, "pi %+q %+q\n", t.Name.Local, t.Data)
		case KindDoctype:
			fmt.Fprintf(&b, "doctype %+q\n", t.Name.Local)
			for _, n := range r.Notations() {
				fmt.Fprintf(&b, "  notation %+q public=%+q system=%+q\n", n.Name, n.PublicID, n.SystemID)
			}
		}
	}
}

// TestCorpus: każdy dokument korpusu (testdata/corpus/*.xml, treść wymyślona) daje strumień tokenów równy
// wzorcowi *.tokens co do bajta. Wzorzec zmienia się tylko przez `go test ./xmlreader -run TestCorpus -update`
// i przegląd różnicy. Od etapu 3 te same dokumenty mają też wzorcowy zapis (*.out.xml).
// REQ: XML-014
func TestCorpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(corpusDir, "*.xml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus documents in %s (%v)", corpusDir, err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, ".out.xml") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		got := dumpTokens(bytes.NewReader(data))
		if strings.HasPrefix(got[strings.LastIndex(strings.TrimSuffix(got, "\n"), "\n")+1:], "error") {
			t.Errorf("%s: corpus documents are well-formed, got %s", filepath.Base(file), got[strings.LastIndex(strings.TrimSuffix(got, "\n"), "\n")+1:])
		}
		golden := strings.TrimSuffix(file, ".xml") + ".tokens"
		if *update {
			if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Errorf("%s: no golden file (run with -update): %v", filepath.Base(file), err)
			continue
		}
		if got != string(want) {
			t.Errorf("%s: token stream differs from %s\n%s", filepath.Base(file), filepath.Base(golden), firstDiff(got, string(want)))
		}
	}
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

// TestGeneratedDocuments: dokumenty z generatorów (internal/testkit, wsad pomiarów wydajności i pamięci) są dobrze
// uformowane i mają tyle elementów rekordów, ile zamówiono.
func TestGeneratedDocuments(t *testing.T) {
	for _, c := range []struct {
		name, record string
		src          io.Reader
	}{
		{"GenJPK", "SprzedazWiersz", testkit.GenJPK(2000)},
		{"GenWordML", "p", testkit.GenWordML(2000)},
	} {
		r := New(c.src, Options{})
		records := 0
		for {
			tok, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if tok.Kind == KindStartElement && tok.Name.Local == c.record {
				records++
			}
		}
		if records != 2000 {
			t.Errorf("%s: %d records, want 2000", c.name, records)
		}
	}
}
