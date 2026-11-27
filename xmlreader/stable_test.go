package xmlreader

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
)

// Suma strumienia tokenów i błędów w zapisie dumpTokens: rodzaj, nazwy, deklaracje przestrzeni nazw, atrybuty,
// dane, pozycje, kod i miejsce błędu. Zmienia się tylko razem z wpisem w docs/differences.md: rozszerzenia
// czytnika nie mogą po cichu zmienić tego, co widział program napisany dla wcześniejszej wersji.
// Historia: v0.1.0 i v0.2.0 c94e0e4d182466adecdbf989e2c43d0a909e1538275debc59e8ad3d9935fc76d; od v0.3.0 bez
// pustych tokenów tekstu (ADR 0005). Że to jedyna różnica względem goxsdgo v0.3.0, sprawdza plik po pliku
// TestSameAsGoxsdgo w tools/bench. Dla korpusu to samo zapewniają wzorce *.tokens (TestCorpus).
// Od v0.4.0 suma jest inna także dlatego, że czytnik oddaje deklarację xmlns:xml (ADR 0006, pkt 16; poprzednia suma
// 4b0aee66…).
const stableXMLConfSum = "4e290790731fccfafe0c1b302808411088fc5d7ef65ed2d1f5e7f6660309fbd0"

// TestTokenStreamStable: strumień tokenów i błędów każdego przypadku W3C XML Conformance w zakresie (1 727,
// dobrze i źle uformowane) jest taki sam jak w poprzednim wydaniu, poza różnicami wymienionymi
// w docs/differences.md.
// REQ: XML-029
func TestTokenStreamStable(t *testing.T) {
	sum := func(ids []string, dump func(id string) string) string {
		h := sha256.New()
		for _, id := range ids {
			h.Write([]byte(id + "\n" + dump(id) + "\n"))
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	read := func(file string) []byte {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	if _, err := os.Stat(path.Join(xmlconfRoot, "xmlconf.xml")); err != nil {
		t.Skip(testkit.FetchHint)
	}
	cases := map[string]*confTest{}
	var ids []string
	for _, ct := range loadConformance(t) {
		if ct.Excluded == "" {
			id := ct.Group + "/" + ct.ID
			cases[id] = ct
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if got := sum(ids, func(id string) string {
		ct := cases[id]
		return dumpTokensOpts(bytes.NewReader(read(path.Join(xmlconfRoot, ct.Dir, ct.URI))),
			Options{NoNamespaces: !strings.HasPrefix(ct.Recommendation, "NS")})
	}); got != stableXMLConfSum {
		t.Errorf("W3C XML Conformance (%d cases): token streams changed (sum %s); a deliberate change needs an entry in docs/differences.md", len(ids), got)
	}
}
