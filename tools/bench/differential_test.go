package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	newreader "github.com/polestorm/goxmlgo/xmlreader"
	oldreader "github.com/polestorm/goxsdgo/xmlreader"
)

// Zrzut strumienia tokenów w polach, które miał już czytnik goxsdgo v0.3.0; dwie kopie tej samej funkcji, bo typy
// obu pakietów są różne.

func dumpOld(data []byte, noNS bool) string {
	var b strings.Builder
	r := oldreader.New(bytes.NewReader(data), oldreader.Options{NoNamespaces: noNS})
	for {
		t, err := r.Next()
		if err == io.EOF {
			return b.String() + "eof"
		}
		if err != nil {
			e := err.(*oldreader.Error)
			return b.String() + fmt.Sprintf("error %s %d:%d %s", e.Code, e.Line, e.Column, e.Message)
		}
		fmt.Fprintf(&b, "%d %+v %q %v %v %v %d:%d %d:%d\n", t.Kind, t.Name, t.Data, t.CDATA, t.Whitespace, t.SelfClosing, t.Line, t.Column, t.EndLine, t.EndColumn)
		for _, a := range t.Attrs {
			fmt.Fprintf(&b, " %+v\n", a)
		}
		if t.Kind == oldreader.KindStartElement {
			fmt.Fprintf(&b, " ns %v\n", r.DeclaredNamespaces())
		}
	}
}

func dumpNew(data []byte, noNS bool) string {
	var b strings.Builder
	r := newreader.New(bytes.NewReader(data), newreader.Options{NoNamespaces: noNS})
	for {
		t, err := r.Next()
		if err == io.EOF {
			return b.String() + "eof"
		}
		if err != nil {
			e := err.(*newreader.Error)
			return b.String() + fmt.Sprintf("error %s %d:%d %s", e.Code, e.Line, e.Column, e.Message)
		}
		fmt.Fprintf(&b, "%d %+v %q %v %v %v %d:%d %d:%d\n", t.Kind, t.Name, t.Data, t.CDATA, t.Whitespace, t.SelfClosing, t.Line, t.Column, t.EndLine, t.EndColumn)
		for _, a := range t.Attrs {
			fmt.Fprintf(&b, " %+v\n", a)
		}
		if t.Kind == newreader.KindStartElement {
			fmt.Fprintf(&b, " ns %v\n", r.DeclaredNamespaces())
		}
	}
}

// fixed to pliki, dla których wynik świadomie różni się od goxsdgo v0.3.0, z wynikiem obu czytników. Jedyny
// przypadek to błąd v0.3.0 w czytaniu deklaracji XML (zapamiętana pozycja w buforze traciła ważność po
// doczytaniu danych; docs/differences.md): plik kończy się tuż za deklaracją, której brakuje poprawnego
// zakończenia, a v0.3.0 zgłaszał koniec dokumentu w złym miejscu.
var fixed = map[string][2]string{
	"ibm03v09.ent": {
		"error xml-eof 1:20 unexpected end of document in XML declaration",
		`error xml-decl 1:21 malformed XML declaration (expected "?>")`,
	},
}

// withoutEmptyText usuwa ze zrzutu czytnika goxsdgo v0.3.0 puste tokeny tekstu (nie CDATA). Do v0.2.0 czytnik
// dawał taki token w miejscu odwołania do encji, która nie daje żadnego znaku przed znacznikiem; od v0.3.0 tego
// tokenu nie ma (docs/differences.md, ADR 0005). To jedyna dopuszczona różnica kształtu strumienia.
func withoutEmptyText(dump string) (string, bool) {
	if !strings.Contains(dump, emptyText) {
		return dump, false
	}
	lines := strings.Split(dump, "\n")
	out := lines[:0]
	for _, l := range lines {
		if !strings.HasPrefix(l, emptyText) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n"), true
}

// emptyText to początek wiersza zrzutu dla pustego tokenu tekstu spoza sekcji CDATA.
const emptyText = `3 {Space: Local: Prefix:} "" false `

// TestSameAsGoxsdgo: dla każdego pliku .xml korpusu i pobranego zestawu W3C XML Conformance (wszystkie pliki,
// także spoza zakresu K1, z przestrzeniami nazw i bez) czytnik goxmlgo daje ten sam strumień tokenów i ten sam
// błąd (kod, miejsce, komunikat) co czytnik goxsdgo v0.3.0, poza plikami z listy fixed i pustymi tokenami tekstu
// (withoutEmptyText). Mierzy wymaganie XML-029
// bezpośrednio, obok sumy w TestTokenStreamStable.
// REQ: XML-029
func TestSameAsGoxsdgo(t *testing.T) {
	files, withEmpty := 0, 0
	for _, dir := range []string{"../../testdata/corpus", "../../testdata/w3c/xmlconf"} {
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !(strings.HasSuffix(p, ".xml") || strings.HasSuffix(p, ".ent") || strings.HasSuffix(p, ".dtd")) {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			files++
			for _, noNS := range []bool{false, true} {
				a, b := dumpOld(data, noNS), dumpNew(data, noNS)
				if strings.Contains(b, emptyText) {
					t.Errorf("%s (NoNamespaces=%v): empty text token", p, noNS)
				}
				var dropped bool
				if a, dropped = withoutEmptyText(a); dropped && !noNS {
					withEmpty++
				}
				if fix, ok := fixed[filepath.Base(p)]; ok {
					if a != fix[0] || b != fix[1] {
						t.Errorf("%s: known difference changed: goxsdgo %q, goxmlgo %q", p, a, b)
					}
					continue
				}
				if a != b {
					t.Errorf("%s (NoNamespaces=%v): streams differ", p, noNS)
				}
			}
			return nil
		})
	}
	if files < 100 {
		t.Skipf("only %d files: the W3C suite is not fetched (go -C tools/fetch run . -root ../..)", files)
	}
	t.Logf("%d files, each read with and without namespaces; in %d of them goxsdgo v0.3.0 gives empty text tokens, which goxmlgo no longer does",
		files, withEmpty)
}
