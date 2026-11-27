package xmlreader

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
)

var update = flag.Bool("update", false, "rewrite the ratchet file testdata/w3c/xmlconf.ratchet")

const (
	xmlconfRoot    = "../testdata/w3c/xmlconf"
	xmlconfRatchet = "../testdata/w3c/xmlconf.ratchet"
)

type confTest = testkit.XMLConfCase

func loadConformance(t *testing.T) []*confTest { return testkit.LoadXMLConf(t, xmlconfRoot) }

// TestXMLConformance mierzy czytnik na zestawie W3C XML Conformance: dokumenty "valid" i "invalid" (walidacja DTD
// nie jest w zakresie) muszą zostać przeczytane bez błędu, a gdy jest OUTPUT, dać tę samą drugą postać
// kanoniczną; dokumenty "not-wf" muszą zostać odrzucone. Przypadki XML 1.0 są czytane bez przestrzeni nazw,
// przypadki Namespaces 1.0 z przestrzeniami nazw.
// REQ: XML-001 XML-002 XML-003 XML-004 XML-005 XML-006 XML-007 XML-008 XML-009 XML-010 XML-011 XML-012 XML-013
func TestXMLConformance(t *testing.T) {
	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err != nil {
		t.Skip(testkit.FetchHint)
	}
	tests := loadConformance(t)
	passed := map[string]bool{}
	inScope := map[string]bool{}
	type row struct{ in, ok int }
	rows := map[string]*row{}
	excluded := map[string]int{}
	var failures []string
	for _, ct := range tests {
		if ct.Excluded != "" {
			excluded[ct.Excluded]++
			continue
		}
		rw := rows[ct.Group]
		if rw == nil {
			rw = &row{}
			rows[ct.Group] = rw
		}
		rw.in++
		inScope[ct.Group+"/"+ct.ID] = true
		ok, why := runConformance(ct)
		if ok {
			rw.ok++
			passed[ct.Group+"/"+ct.ID] = true
		} else {
			failures = append(failures, fmt.Sprintf("%s/%s (%s %s, %s): %s", ct.Group, ct.ID, ct.Type, ct.URI, ct.Sections, why))
		}
	}
	var b strings.Builder
	groups := make([]string, 0, len(rows))
	for g := range rows {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	total := row{}
	fmt.Fprintf(&b, "W3C XML Conformance: passed/in scope\n")
	for _, g := range groups {
		fmt.Fprintf(&b, "%-10s %5d/%-5d\n", g, rows[g].ok, rows[g].in)
		total.in += rows[g].in
		total.ok += rows[g].ok
	}
	fmt.Fprintf(&b, "%-10s %5d/%-5d\n", "all", total.ok, total.in)
	reasons := make([]string, 0, len(excluded))
	for r := range excluded {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		fmt.Fprintf(&b, "excluded %5d: %s\n", excluded[r], r)
	}
	t.Log("\n" + b.String())
	if testing.Verbose() {
		for _, f := range failures {
			t.Log("fail: " + f)
		}
	}

	testkit.Ratchet(t, xmlconfRatchet, "W3C XML Conformance (TestXMLConformance, go test ./xmlreader -update).",
		inScope, passed, *update)
}

func runConformance(ct *confTest) (bool, string) {
	file := path.Join(xmlconfRoot, ct.Dir, ct.URI)
	f, err := os.Open(file)
	if err != nil {
		return false, err.Error()
	}
	defer f.Close()
	opts := Options{NoNamespaces: !strings.HasPrefix(ct.Recommendation, "NS")}
	canon, err := canonical(New(f, opts))
	if ct.Type == "not-wf" {
		if err == nil {
			return false, "accepted"
		}
		return true, ""
	}
	if err != nil {
		return false, err.Error()
	}
	if ct.Output != "" {
		want, err := os.ReadFile(path.Join(xmlconfRoot, ct.Dir, ct.Output))
		if err != nil {
			return false, err.Error()
		}
		if !bytes.Equal(canon, want) {
			return false, fmt.Sprintf("canonical form differs:\n got  %q\n want %q", canon, want)
		}
	}
	return true, ""
}

// canonical czyta cały dokument i zwraca jego drugą postać kanoniczną (James Clark, z deklaracjami notacji;
// opis w testcases.dtd zestawu).
func canonical(r *Reader) ([]byte, error) {
	var out bytes.Buffer
	var root string
	escape := func(s []byte) {
		for _, c := range string(s) {
			switch c {
			case '&':
				out.WriteString("&amp;")
			case '<':
				out.WriteString("&lt;")
			case '>':
				out.WriteString("&gt;")
			case '"':
				out.WriteString("&quot;")
			case '\t':
				out.WriteString("&#9;")
			case '\n':
				out.WriteString("&#10;")
			case '\r':
				out.WriteString("&#13;")
			default:
				out.WriteRune(c)
			}
		}
	}
	name := func(n Name) string {
		if n.Prefix != "" {
			return n.Prefix + ":" + n.Local
		}
		return n.Local
	}
	wroteDoctype := false
	for {
		t, err := r.Next()
		if err == io.EOF {
			return out.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
		switch t.Kind {
		case KindDoctype:
			root = t.Name.Local
			if !wroteDoctype {
				wroteDoctype = true
				if n := r.Notations(); len(n) > 0 {
					sorted := append([]Notation(nil), n...)
					sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
					out.WriteString("<!DOCTYPE " + root + " [\n")
					for _, x := range sorted {
						out.WriteString("<!NOTATION " + x.Name)
						switch {
						case x.HasPublic && x.HasSystem:
							out.WriteString(" PUBLIC '" + x.PublicID + "' '" + x.SystemID + "'")
						case x.HasPublic:
							out.WriteString(" PUBLIC '" + x.PublicID + "'")
						default:
							out.WriteString(" SYSTEM '" + x.SystemID + "'")
						}
						out.WriteString(">\n")
					}
					out.WriteString("]>\n")
				}
			}
		case KindStartElement:
			out.WriteString("<" + name(t.Name))
			attrs := append([]Attr(nil), t.Attrs...)
			sort.Slice(attrs, func(i, j int) bool { return name(attrs[i].Name) < name(attrs[j].Name) })
			for _, a := range attrs {
				out.WriteString(" " + name(a.Name) + `="`)
				escape([]byte(a.Value))
				out.WriteString(`"`)
			}
			out.WriteString(">")
		case KindEndElement:
			out.WriteString("</" + name(t.Name) + ">")
		case KindText:
			escape(t.Data)
		case KindProcInst:
			out.WriteString("<?" + t.Name.Local + " ")
			out.Write(t.Data)
			out.WriteString("?>")
		}
	}
}
