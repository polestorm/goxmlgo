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

// Sumy strumienia tokenów i błędów z kodu v0.1.0 (czytnik przeniesiony z goxsdgo, przed rozszerzeniami etapu 2),
// w zapisie dumpTokens: rodzaj, nazwy, deklaracje przestrzeni nazw, atrybuty, dane, pozycje, kod i miejsce błędu.
// Nie zmieniają się: rozszerzenia czytnika nie mogą zmienić tego, co widział program napisany dla v0.1.0.
// Dla korpusu to samo zapewniają wzorce *.tokens (TestCorpus), które powstały z kodu v0.1.0.
const stableXMLConfSum = "c94e0e4d182466adecdbf989e2c43d0a909e1538275debc59e8ad3d9935fc76d"

// TestTokenStreamStable: strumień tokenów i błędów każdego przypadku W3C XML Conformance w zakresie (1 727,
// dobrze i źle uformowane) jest taki sam jak w v0.1.0.
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
		if ct.excluded == "" {
			id := ct.Group + "/" + ct.ID
			cases[id] = ct
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if got := sum(ids, func(id string) string {
		ct := cases[id]
		return dumpTokensOpts(bytes.NewReader(read(path.Join(xmlconfRoot, ct.dir, ct.URI))),
			Options{NoNamespaces: !strings.HasPrefix(ct.Recommendation, "NS")})
	}); got != stableXMLConfSum {
		t.Errorf("W3C XML Conformance (%d cases): token streams changed since v0.1.0 (sum %s)", len(ids), got)
	}
}
