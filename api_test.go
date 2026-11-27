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
var publicPackages = []string{"xmlchars", "xmlreader", "xmltree", "xmlwriter"}

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

// TestReaderAPIKeepsGoxsdgo: API czytnika tylko rośnie. Każdy wiersz wzorca API goxsdgo/xmlreader v0.3.0
// (api/baseline/xmlreader-goxsdgo-v0.3.0.txt, zrobionego tym samym narzędziem z kodu goxsdgo w commicie
// źródłowym; równego API goxmlgo v0.1.0) jest nadal wierszem api/xmlreader.txt: żadna stała, pole, typ, funkcja
// ani metoda nie zniknęły i nie zmieniły zapisu. Odstępy są pomijane, bo nowe pola zmieniają wyrównanie.
// REQ: API-002 API-003
func TestReaderAPIKeepsGoxsdgo(t *testing.T) {
	lines := func(path string) []string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for i, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
			if l = strings.Join(strings.Fields(l), " "); l != "" && i > 0 { // bez wiersza „package …”
				out = append(out, l)
			}
		}
		return out
	}
	have := map[string]int{}
	for _, l := range lines("api/xmlreader.txt") {
		have[l]++
	}
	for _, l := range lines("api/baseline/xmlreader-goxsdgo-v0.3.0.txt") {
		if have[l] == 0 {
			t.Errorf("the API of goxsdgo/xmlreader v0.3.0 has %q, api/xmlreader.txt does not", l)
			continue
		}
		have[l]--
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
