package testkit

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
	"testing/iotest"
)

func sum(t *testing.T, r io.Reader) (string, int64) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil)), n
}

// TestGeneratorsAreDeterministic: ten sam rozmiar daje te same bajty, niezależnie od przebiegu, systemu
// i wielkości odczytów; sumy są zapisane, więc zmiana generatora jest widoczna (zmienia wsad pomiarów).
func TestGeneratorsAreDeterministic(t *testing.T) {
	for _, c := range []struct {
		name string
		gen  func(int) io.Reader
		n    int
		want string
		size int64
	}{
		{"GenJPK", GenJPK, 1000, "913bc222fd80ed225e9fc46b96f58c91bdf2f3273e4576023d3d35d1f862dc7e", 435893},
		{"GenWordML", GenWordML, 1000, "29912597f30dfc3d1905830c47170f6579170e1f0d92199315c891ac0dfcf971", 307047},
	} {
		got, size := sum(t, c.gen(c.n))
		if got != c.want || size != c.size {
			t.Errorf("%s(%d): sha256 %s, %d bytes; want %s, %d bytes", c.name, c.n, got, size, c.want, c.size)
		}
		if again, _ := sum(t, iotest.OneByteReader(c.gen(c.n))); again != got {
			t.Errorf("%s: output depends on the size of the reads", c.name)
		}
	}
}
