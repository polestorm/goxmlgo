package testkit

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

// Zestaw W3C XML Conformance (testdata/w3c/xmlconf, pobierany przez tools/fetch): lista przypadków i zakres.
// Wspólne dla testu zgodności czytnika (K1) i testu zapisu powrotnego (K2).

// XMLConfCase to jeden przypadek zestawu W3C XML Conformance (element TEST, testcases.dtd).
type XMLConfCase struct {
	Group          string
	ID             string `xml:"ID,attr"`
	Type           string `xml:"TYPE,attr"`
	Entities       string `xml:"ENTITIES,attr"`
	URI            string `xml:"URI,attr"`
	Output         string `xml:"OUTPUT,attr"`
	Recommendation string `xml:"RECOMMENDATION,attr"`
	Version        string `xml:"VERSION,attr"`
	Edition        string `xml:"EDITION,attr"`
	Namespace      string `xml:"NAMESPACE,attr"`
	Sections       string `xml:"SECTIONS,attr"`
	// Dir to katalog pliku względem katalogu zestawu; Excluded to powód wyłączenia z zakresu albo "".
	Dir      string
	Excluded string
}

var entityDecl = regexp.MustCompile(`<!ENTITY\s+(\S+)\s+SYSTEM\s+"([^"]+)"\s*>`)

// LoadXMLConf czyta xmlconf.xml i pliki przypadków wskazane w nim przez encje zewnętrzne. Do metadanych
// zestawu (dane testowe, zaufane) wystarcza encoding/xml.
func LoadXMLConf(t testing.TB, root string) []*XMLConfCase {
	t.Helper()
	b, err := os.ReadFile(path.Join(root, "xmlconf.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var tests []*XMLConfCase
	seen := map[string]bool{}
	for _, m := range entityDecl.FindAllSubmatch(b, -1) {
		file := string(m[2])
		data, err := os.ReadFile(path.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		dec.Strict = false
		dec.Entity = xml.HTMLEntity
		bases := []string{path.Dir(file)}
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			switch e := tok.(type) {
			case xml.StartElement:
				base := bases[len(bases)-1]
				for _, a := range e.Attr {
					if a.Name.Local == "base" {
						base = path.Join(base, a.Value)
					}
				}
				bases = append(bases, base)
				if e.Name.Local == "TEST" {
					ct := &XMLConfCase{Group: strings.SplitN(file, "/", 2)[0], Entities: "none", Recommendation: "XML1.0", Namespace: "yes"}
					for _, a := range e.Attr {
						switch a.Name.Local {
						case "ID":
							ct.ID = a.Value
						case "TYPE":
							ct.Type = a.Value
						case "ENTITIES":
							ct.Entities = a.Value
						case "URI":
							ct.URI = a.Value
						case "OUTPUT":
							ct.Output = a.Value
						case "RECOMMENDATION":
							ct.Recommendation = a.Value
						case "VERSION":
							ct.Version = a.Value
						case "EDITION":
							ct.Edition = a.Value
						case "NAMESPACE":
							ct.Namespace = a.Value
						case "SECTIONS":
							ct.Sections = a.Value
						}
					}
					ct.Dir = base
					key := ct.Group + "/" + ct.ID
					if seen[key] {
						ct.ID = ct.ID + "@" + ct.URI
					}
					seen[ct.Group+"/"+ct.ID] = true
					ct.Excluded = exclusion(ct, root)
					tests = append(tests, ct)
				}
			case xml.EndElement:
				bases = bases[:len(bases)-1]
			}
		}
	}
	return tests
}

var declaredEncoding = regexp.MustCompile(`^(?:\xef\xbb\xbf)?<\?xml[^>]*encoding\s*=\s*["']([A-Za-z][A-Za-z0-9._-]*)["']`)

// exclusion podaje powód wyłączenia przypadku z zakresu (ADR 0004) albo "".
func exclusion(ct *XMLConfCase, root string) string {
	fields := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, f := range strings.Fields(s) {
			m[f] = true
		}
		return m
	}
	switch {
	case ct.Recommendation == "XML1.1" || ct.Recommendation == "NS1.1":
		return "XML 1.1 or Namespaces 1.1"
	case ct.Version != "" && !fields(ct.Version)["1.0"]:
		return "XML 1.1"
	case ct.Edition != "" && !fields(ct.Edition)["5"]:
		return "applies only to XML 1.0 editions 1-4"
	case ct.Entities != "none":
		return "needs external entities, which are never read"
	case ct.Type == "error":
		return "optional error (processors need not report it)"
	case ct.Namespace == "no" && strings.HasPrefix(ct.Recommendation, "NS"):
		return "namespace processing off"
	}
	if ct.Type == "valid" || ct.Type == "invalid" {
		if data, err := os.ReadFile(path.Join(root, ct.Dir, ct.URI)); err == nil {
			if m := declaredEncoding.FindSubmatch(data); m != nil && !supportedEncoding(string(m[1])) {
				return "encoding " + strings.ToLower(string(m[1])) + " not supported"
			}
		}
	}
	return ""
}

func supportedEncoding(name string) bool {
	switch strings.ToLower(name) {
	case "utf-8", "utf-16", "utf-16le", "utf-16be", "iso-8859-1", "iso_8859-1", "latin1", "l1", "iso-ir-100", "cp819",
		"ibm819", "csisolatin1", "us-ascii", "ascii", "iso646-us", "ansi_x3.4-1968", "csascii":
		return true
	}
	return false
}
