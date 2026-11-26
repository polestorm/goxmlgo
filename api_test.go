package goxmlgo

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
)

var update = flag.Bool("update", false, "rewrite the API snapshots api/*.txt")

const modulePath = "github.com/polestorm/goxmlgo"

// publicPackages to pakiety biblioteki; każdy ma wzorzec api/<pakiet>.txt. Nowy pakiet dopisuje się tutaj.
var publicPackages = []string{"xmlchars", "xmlreader"}

// TestAPISnapshot mierzy kryterium K4: publiczne API każdego pakietu jest równe wzorcowi api/<pakiet>.txt.
// Zmiana API wymaga `go test . -run TestAPISnapshot -update`, przeglądu różnicy i wpisu w ADR albo CHANGELOG.md.
// REQ: API-001
func TestAPISnapshot(t *testing.T) {
	for _, pkg := range publicPackages {
		got, err := testkit.APISnapshot(pkg, modulePath+"/"+pkg)
		if err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join("api", pkg+".txt")
		if *update {
			if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Errorf("%s: %v (run with -update)", pkg, err)
			continue
		}
		if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
			t.Errorf("public API of %s differs from %s; if the change is intended, run with -update and record it", pkg, golden)
		}
	}
}

// TestReaderAPIMatchesGoxsdgo: API czytnika w v0.1.0 jest równe API goxsdgo/xmlreader v0.3.0 poza ścieżką pakietu.
// Wzorzec api/baseline/xmlreader-goxsdgo-v0.3.0.txt powstał tym samym narzędziem z kodu goxsdgo w commicie
// źródłowym (docs/provenance.md) i się nie zmienia. Od etapu 2 API rośnie: test zostanie wtedy zastąpiony
// sprawdzeniem, że nic z wzorca nie zniknęło (API-003).
// REQ: API-002
func TestReaderAPIMatchesGoxsdgo(t *testing.T) {
	body := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.ReplaceAll(string(b), "\r\n", "\n")
		return s[strings.IndexByte(s, '\n')+1:] // bez wiersza „package …”
	}
	if body("api/xmlreader.txt") != body("api/baseline/xmlreader-goxsdgo-v0.3.0.txt") {
		t.Error("api/xmlreader.txt differs from the API of goxsdgo/xmlreader v0.3.0 (api/baseline)")
	}
}

// TestErrorCatalog: każdy kod błędu z kodu źródłowego jest w docs/errors.md i odwrotnie.
// REQ: API-004
func TestErrorCatalog(t *testing.T) {
	constRe := regexp.MustCompile(`(?m)^\s*Code\w+\s*=\s*"([a-z0-9-]+)"`)
	rowRe := regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\|")
	inCode := map[string]bool{}
	for _, pkg := range publicPackages {
		files, _ := filepath.Glob(filepath.Join(pkg, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range constRe.FindAllSubmatch(b, -1) {
				inCode[string(m[1])] = true
			}
		}
	}
	doc, err := os.ReadFile("docs/errors.md")
	if err != nil {
		t.Fatal(err)
	}
	inDoc := map[string]bool{}
	for _, m := range rowRe.FindAllSubmatch(doc, -1) {
		if inDoc[string(m[1])] {
			t.Errorf("docs/errors.md lists %s twice", m[1])
		}
		inDoc[string(m[1])] = true
	}
	var missing, extra []string
	for c := range inCode {
		if !inDoc[c] {
			missing = append(missing, c)
		}
	}
	for c := range inDoc {
		if !inCode[c] {
			extra = append(extra, c)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("codes missing from docs/errors.md: %s", strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		t.Errorf("codes in docs/errors.md that the code does not define: %s", strings.Join(extra, ", "))
	}
	t.Logf("%d error codes", len(inCode))
}

// TestNoDependencies: moduł główny nie ma sekcji require (CGO_ENABLED=0 i najniższą wersję Go sprawdza CI).
// REQ: API-007
func TestNoDependencies(t *testing.T) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) > 0 && (f[0] == "require" || f[0] == "replace") {
			t.Errorf("go.mod: %q; the library module has no dependencies", line)
		}
	}
}
