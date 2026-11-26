package xmlchars

import "testing"

// TestTablesMatchFunctions: przedziały NameStart5 i NameCharExtra5 opisują te same zbiory co IsNameStart5
// i IsNameChar5 (jedno źródło dla czytnika i innych odbiorców). W goxsdgo test odwoływał się do DT-019.
// REQ: CH-001 CH-004
func TestTablesMatchFunctions(t *testing.T) {
	in := func(rs [][2]rune, r rune) bool {
		for _, p := range rs {
			if p[0] <= r && r <= p[1] {
				return true
			}
		}
		return false
	}
	for r := rune(0); r <= 0x10FFFF; r++ {
		start := in(NameStart5, r)
		if start != IsNameStart5(r) {
			t.Fatalf("U+%04X: table %v, IsNameStart5 %v", r, start, IsNameStart5(r))
		}
		if c := start || in(NameCharExtra5, r); c != IsNameChar5(r) {
			t.Fatalf("U+%04X: table %v, IsNameChar5 %v", r, c, IsNameChar5(r))
		}
	}
}

// TestNoAllocations: funkcje klas znaków nie alokują pamięci. REQ: CH-003
func TestNoAllocations(t *testing.T) {
	sink := false
	if n := testing.AllocsPerRun(100, func() {
		for _, r := range []rune{'a', ':', '-', 0xB7, 0x142, 0x3001, 0x10000, 0x10FFFF, -1} {
			sink = sink != (IsNameStart5(r) || IsNameChar5(r))
		}
	}); n != 0 {
		t.Fatalf("%v allocations", n)
	}
}
