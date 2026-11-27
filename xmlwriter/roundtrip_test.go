package xmlwriter

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
	"unicode/utf8"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlreader"
)

var update = flag.Bool("update", false, "rewrite the golden files testdata/corpus/*.out.xml and the ratchet testdata/w3c/roundtrip.ratchet")

const (
	corpusDir        = "../testdata/corpus"
	xmlconfRoot      = "../testdata/w3c/xmlconf"
	roundTripRatchet = "../testdata/w3c/roundtrip.ratchet"
)

// streamDump zapisuje strumień tokenów dokumentu w postaci, w której dwa strumienie równe według definicji
// z docs/design.md („Co wraca z zapisu”) dają ten sam tekst: rodzaj, nazwy z prefiksami, deklaracje przestrzeni
// nazw i atrybuty w kolejności, dane, CDATA, Whitespace, SelfClosing; bez pozycji, bez Specified i Type, bez
// tokenu DOCTYPE. Pierwszy wiersz to standalone z deklaracji XML. Błąd czytnika kończy zrzut i jest zwracany.
func streamDump(src []byte, noNS bool) (string, error) {
	var b strings.Builder
	name := func(n xmlreader.Name) string {
		s := n.Local
		if n.Prefix != "" {
			s = n.Prefix + ":" + s
		}
		return fmt.Sprintf("{%s}%+q", n.Space, s)
	}
	r := xmlreader.New(bytes.NewReader(src), xmlreader.Options{NoNamespaces: noNS})
	d, err := r.Declaration()
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "declaration %v standalone=%q\n", d.Version != "", d.Standalone)
	for {
		t, err := r.Next()
		if err == io.EOF {
			return b.String(), nil
		}
		if err != nil {
			return b.String(), err
		}
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
			fmt.Fprintf(&b, "text %+q", t.Data)
			if t.CDATA {
				b.WriteString(" cdata")
			}
			if t.Whitespace {
				b.WriteString(" whitespace")
			}
			b.WriteString("\n")
		case xmlreader.KindComment:
			fmt.Fprintf(&b, "comment %+q\n", t.Data)
		case xmlreader.KindProcInst:
			fmt.Fprintf(&b, "pi %+q %+q\n", t.Name.Local, t.Data)
		}
	}
}

// rewrite to przelot: czytnik → pisarz, z deklaracją XML, gdy miał ją dokument źródłowy. Zwraca błąd czytnika
// albo pisarza (pierwszy, który wystąpi).
func rewrite(src []byte, noNS bool, opts Options) ([]byte, error) {
	var out bytes.Buffer
	r := xmlreader.New(bytes.NewReader(src), xmlreader.Options{NoNamespaces: noNS})
	opts.NoNamespaces = noNS
	w := New(&out, opts)
	d, err := r.Declaration()
	if err != nil {
		return nil, err
	}
	if d.Version != "" {
		if err := w.Declaration(d.Standalone); err != nil {
			return nil, err
		}
	}
	for {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
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

// roundTrip sprawdza obie własności K2 dla dokumentu przyjętego przez czytnik: strumień po zapisie jest równy
// strumieniowi wejścia, a drugi zapis daje te same bajty. Zwraca pierwszy zapis i opis porażki albo "".
func roundTrip(src []byte, noNS bool) ([]byte, string) {
	want, err := streamDump(src, noNS)
	if err != nil {
		return nil, "reader: " + err.Error()
	}
	out1, err := rewrite(src, noNS, Options{})
	if err != nil {
		return nil, err.Error()
	}
	got, err := streamDump(out1, noNS)
	if err != nil {
		return out1, "the reader rejects what the writer wrote: " + err.Error()
	}
	if got != want {
		return out1, "token stream differs after the round trip\n" + firstDiff(got, want)
	}
	out2, err := rewrite(out1, noNS, Options{})
	if err != nil {
		return out1, "second write: " + err.Error()
	}
	if !bytes.Equal(out1, out2) {
		return out1, "the second write gives other bytes than the first"
	}
	return out1, ""
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

// TestRoundTrip mierzy kryterium K2 na korpusie i na zestawie W3C XML Conformance: dla każdego dokumentu, który
// czytnik przyjmuje, odczyt → zapis → odczyt daje równy strumień tokenów (docs/design.md), a zapis jest
// idempotentny co do bajta. Dokumenty korpusu mają też wzorzec zapisu *.out.xml, wspólny dla wszystkich systemów.
// Przypadki W3C są czytane i pisane jak w TestXMLConformance: XML 1.0 bez przestrzeni nazw, Namespaces 1.0
// z przestrzeniami nazw. Liczba zaliczonych nie spada (zapadka testdata/w3c/roundtrip.ratchet).
// REQ: STR-001 STR-002 STR-006 WR-013 DET-002
func TestRoundTrip(t *testing.T) {
	for _, file := range corpusFiles(t) {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		out, why := roundTrip(src, false)
		if why != "" {
			t.Errorf("%s: %s", filepath.Base(file), why)
			continue
		}
		// wyjście jest w UTF-8 bez BOM także dla wejścia w UTF-16 i ISO-8859-1, a deklaracja nie podaje kodowania wejścia
		if !utf8.Valid(out) || bytes.HasPrefix(out, []byte("\xef\xbb\xbf")) ||
			(bytes.HasPrefix(out, []byte("<?xml")) && !bytes.HasPrefix(out, []byte(`<?xml version="1.0" encoding="UTF-8"`))) {
			t.Errorf("%s: output is not UTF-8 without a BOM with a UTF-8 declaration: %q", filepath.Base(file), out[:min(len(out), 60)])
		}
		golden := strings.TrimSuffix(file, ".xml") + ".out.xml"
		if *update {
			if err := os.WriteFile(golden, out, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Errorf("%s: no golden file (run with -update): %v", filepath.Base(file), err)
			continue
		}
		if !bytes.Equal(out, want) {
			t.Errorf("%s: output differs from %s", filepath.Base(file), filepath.Base(golden))
		}
	}

	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err != nil {
		t.Skip(testkit.FetchHint)
	}
	inScope, passed := map[string]bool{}, map[string]bool{}
	rejected := 0
	var failures []string
	for _, ct := range testkit.LoadXMLConf(t, xmlconfRoot) {
		if ct.Excluded != "" || ct.Type == "not-wf" {
			continue
		}
		src, err := os.ReadFile(path.Join(xmlconfRoot, ct.Dir, ct.URI))
		if err != nil {
			t.Fatal(err)
		}
		noNS := !strings.HasPrefix(ct.Recommendation, "NS")
		if _, err := streamDump(src, noNS); err != nil {
			rejected++ // porażki K1 (docs/differences.md): czytnik nie daje strumienia, więc nie ma czego zapisać
			continue
		}
		id := ct.Group + "/" + ct.ID
		inScope[id] = true
		if _, why := roundTrip(src, noNS); why != "" {
			failures = append(failures, id+" ("+ct.URI+"): "+why)
			continue
		}
		passed[id] = true
	}
	t.Logf("round trip on the W3C XML Conformance suite: %d of %d documents the reader accepts (%d well-formed cases in scope are rejected by the reader)",
		len(passed), len(inScope), rejected)
	for i, f := range failures {
		if i < 20 || testing.Verbose() {
			t.Log("fail: " + f)
		}
	}
	testkit.Ratchet(t, roundTripRatchet, "Round trip on W3C XML Conformance (TestRoundTrip, go test ./xmlwriter -update).",
		inScope, passed, *update)
	if len(passed) != len(inScope) {
		t.Errorf("K2 needs every document: %d of %d", len(passed), len(inScope))
	}
}

func readFile(t testing.TB, file string) []byte {
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
