package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmltree"
)

var conformanceOut = flag.String("conformance-out", "", "write the result of TestCandidatesConformance to this file")

// TestCandidatesConformance: ile dokumentów zestawu W3C XML Conformance w zakresie goxmlgo (XML 1.0 piąte wydanie,
// Namespaces 1.0, bez encji zewnętrznych: te same 1 727 przypadków co w K1) każdy kandydat buduje bez błędu.
// Dokument dobrze uformowany powinien dać drzewo, źle uformowany błąd. Test niczego nie wymaga od cudzych
// bibliotek: zapisuje liczby (flaga -conformance-out) i pilnuje tylko wyniku goxmlgo.
func TestCandidatesConformance(t *testing.T) {
	const root = "../../testdata/w3c/xmlconf"
	if _, err := os.Stat(path.Join(root, "xmlconf.xml")); err != nil {
		t.Skip(testkit.FetchHint)
	}
	type count struct{ wfAccepted, notWFRejected, notWFAccepted, panics int }
	counts := make([]count, len(candidates))
	wf, notWF := 0, 0
	for _, ct := range testkit.LoadXMLConf(t, root) {
		if ct.Excluded != "" {
			continue
		}
		data, err := os.ReadFile(path.Join(root, ct.Dir, ct.URI))
		if err != nil {
			t.Fatal(err)
		}
		wellFormed := ct.Type != "not-wf"
		if wellFormed {
			wf++
		} else {
			notWF++
		}
		for i, c := range candidates {
			err, panicked := func() (err error, panicked bool) {
				defer func() {
					if recover() != nil {
						panicked = true
					}
				}()
				if i == 0 && !strings.HasPrefix(ct.Recommendation, "NS") {
					// goxmlgo jak w K1: przypadki rekomendacji XML 1.0 bez przetwarzania przestrzeni nazw (dwukropek
					// w nazwie jest tam dozwolony), przypadki Namespaces 1.0 z nim. Pozostali kandydaci nie mają
					// takiego ustawienia.
					_, err = xmltree.Parse(bytes.NewReader(data), xmltree.Options{MaxNodes: -1, Reader: xmlreader.Options{NoNamespaces: true}})
					return err, false
				}
				_, err = c.build(data)
				return err, false
			}()
			switch {
			case panicked:
				counts[i].panics++
			case wellFormed && err == nil:
				counts[i].wfAccepted++
			case !wellFormed && err != nil:
				counts[i].notWFRejected++
			case !wellFormed:
				counts[i].notWFAccepted++
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# go -C tools/compare test . -run TestCandidatesConformance -conformance-out ../../docs/results/tree-conformance.tsv\n")
	fmt.Fprintf(&b, "# goxmlgo: %s; candidates: %s\n", commit("../.."), versions())
	fmt.Fprintf(&b, "# W3C XML Conformance Test Suite, cases in scope of goxmlgo: %d well-formed, %d not well-formed; each document is given to the function that builds a tree, with default settings; goxmlgo reads the cases of the XML 1.0 recommendation without namespace processing, as in its own conformance test\n", wf, notWF)
	fmt.Fprintf(&b, "candidate\twell-formed_accepted\twell-formed_total\tnot-well-formed_rejected\tnot-well-formed_accepted\tnot-well-formed_total\tpanics\n")
	for i, c := range candidates {
		n := counts[i]
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\t%d\t%d\t%d\n", c.name, n.wfAccepted, wf, n.notWFRejected, n.notWFAccepted, notWF, n.panics)
	}
	t.Log("\n" + b.String())
	if *conformanceOut != "" {
		if err := os.WriteFile(*conformanceOut, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// goxmlgo: liczby K1 (dwie porażki wśród dobrze uformowanych są opisane w docs/differences.md)
	if g := counts[0]; g.notWFAccepted != 0 || g.panics != 0 || g.wfAccepted < wf-2 {
		t.Errorf("goxmlgo: %+v", g)
	}
}
