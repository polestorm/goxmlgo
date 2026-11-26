package xmlreader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/polestorm/goxmlgo/internal/testkit"
)

// FuzzReader: dowolne wejście nie powoduje paniki, czytanie się kończy, a pozycje tokenów i błędów leżą
// w granicach wejścia (wiersz nie większy niż liczba wierszy, kolumna nie większa niż długość wiersza + 1).
// Od etapu 2 także: po błędzie Next zwraca stale ten sam błąd; strumień nie zależy od podziału wejścia na
// odczyty; czytnik użyty ponownie po Reset daje ten sam strumień co nowy; czytanie ze Skip kończy się tak samo
// jak czytanie token po tokenie (EOF albo ten sam błąd w tym samym miejscu).
func FuzzReader(f *testing.F) {
	for _, s := range []string{
		"<a/>",
		"<?xml version='1.0' encoding='UTF-8'?><a x='1' xmlns:p='u'><p:b>t&amp;<![CDATA[c]]></p:b><!--c--><?pi d?></a>",
		`<!DOCTYPE a [<!ENTITY e "x<b>&f;</b>"><!ENTITY f "y"><!ATTLIST a d CDATA "v">]><a>&e;</a>`,
		"\xff\xfe<\x00a\x00/\x00>\x00",
		"<?xml version='1.0' encoding='ISO-8859-1'?><a>\xe9</a>",
		"<a>\r\n\r</a>",
	} {
		f.Add([]byte(s))
	}
	// Wejścia startowe: korpus z repozytorium i próbka pobranego zestawu XML Conformance (gdy jest).
	for _, dir := range []struct {
		path string
		max  int
	}{{"../testdata/corpus", 50}, {"../testdata/w3c/xmlconf", 150}} {
		for _, b := range testkit.SampleFiles(dir.path, []string{".xml"}, dir.max, 128<<10) {
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		lines := bytes.Count(data, []byte{'\n'}) + bytes.Count(data, []byte{'\r'}) + 1
		longest := 0
		for _, l := range bytes.FieldsFunc(data, func(r rune) bool { return r == '\n' || r == '\r' }) {
			// kolumna liczy znaki; w kodowaniach jednobajtowych i przy złym UTF-8 znaków nie jest więcej niż bajtów
			longest = max(longest, len(l), utf8.RuneCount(l))
		}
		if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
			// W UTF-16 bajt 0x0A albo 0x0D nie musi być końcem wiersza: granice liczymy w bajtach.
			lines, longest = len(data)+1, len(data)
		}
		check := func(l, c int) {
			if l < 1 || l > lines || c < 1 || c > longest+1 {
				t.Fatalf("position %d:%d outside input (%d lines, longest %d)", l, c, lines, longest)
			}
		}
		r := New(bytes.NewReader(data), Options{})
		for i := 0; ; i++ {
			if i > 2*len(data)+10 {
				t.Fatal("reader does not terminate")
			}
			tok, err := r.Next()
			if err == io.EOF {
				return
			}
			if err != nil {
				var e *Error
				if !errors.As(err, &e) {
					t.Fatalf("error of unexpected type: %v", err)
				}
				check(e.Line, e.Column)
				if _, again := r.Next(); again != err {
					t.Fatalf("after %v Next returns %v", err, again)
				}
				break
			}
			check(tok.Line, tok.Column)
		}

		want := dumpTokens(bytes.NewReader(data))
		end := want[strings.LastIndexByte(want[:len(want)-1], '\n')+1:]
		// podział wejścia na odczyty
		if got := dumpTokens(&chunked{data: data, n: 1 + len(data)%11}); got != want {
			t.Fatalf("stream depends on how the input is split into reads\n%s", firstDiff(got, want))
		}
		// ponowne użycie czytnika
		fuzzReused.Reset(bytes.NewReader(data), Options{})
		if got := dumpReader(fuzzReused); got != want {
			t.Fatalf("stream after Reset differs from a new reader\n%s", firstDiff(got, want))
		}
		// Skip na co k-tym początku elementu
		k := 1 + len(data)%3
		r = New(bytes.NewReader(data), Options{})
		var last error
		for n := 0; ; {
			tok, err := r.Next()
			if err != nil {
				last = err
				break
			}
			if tok.Kind == KindStartElement {
				if n++; n%k == 0 {
					if err := r.Skip(); err != nil {
						last = err
						break
					}
				}
			}
		}
		got := "eof\n"
		if last != io.EOF {
			e := last.(*Error)
			got = fmt.Sprintf("error %s %d:%d\n", e.Code, e.Line, e.Column)
		}
		if got != end {
			t.Fatalf("reading with Skip ends with %q, reading every token with %q", got, end)
		}
	})
}

// fuzzReused to czytnik używany ponownie między wykonaniami fuzzera (Reset po dowolnym poprzednim dokumencie).
var fuzzReused = New(bytes.NewReader(nil), Options{})
