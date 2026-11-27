package xmlwriter

import (
	"unicode/utf8"

	"github.com/polestorm/goxmlgo/xmlchars"
)

// bytestring to dane wywołującego: napis (wywołania) albo wycinek bajtów (Token.Data). Funkcje danych są
// generyczne, żeby żadna z dróg nie kopiowała danych.
type bytestring interface{ ~string | ~[]byte }

// Klasy bajtów. cBad: znak sterujący spoza [2] Char. cHigh: początek albo dalszy bajt znaku spoza ASCII
// (sprawdzany osobno). cText i cAttr: znak zapisywany odwołaniem w tekście i w wartości atrybutu. cCR: powrót
// karetki. cSpace: [3] S.
const (
	cBad = 1 << iota
	cHigh
	cText
	cAttr
	cCR
	cSpace
)

var class = func() (t [256]uint8) {
	for b := 0; b < 0x20; b++ {
		t[b] = cBad
	}
	for b := 0x80; b < 0x100; b++ {
		t[b] = cHigh
	}
	t['\t'], t['\n'], t['\r'], t[' '] = cSpace|cAttr, cSpace|cAttr, cSpace|cAttr|cText|cCR, cSpace
	t['&'] |= cText | cAttr
	t['<'] |= cText | cAttr
	t['>'] |= cText
	t['"'] |= cAttr
	return t
}()

// decode czyta jeden znak UTF-8 zaczynający się w s[i] (bajt ≥ 0x80). Rozmiar 0 oznacza ciąg, który nie jest
// poprawnym UTF-8: zły bajt początkowy, brak bajtów dalszych, zapis nadmiarowy, surogat albo wartość ponad
// U+10FFFF (Unicode, tabela 3-7).
func decode[T bytestring](s T, i int) (rune, int) {
	b0 := s[i]
	cont := func(k int) (byte, bool) {
		if i+k >= len(s) {
			return 0, false
		}
		b := s[i+k]
		return b & 0x3F, b&0xC0 == 0x80
	}
	switch {
	case b0 >= 0xC2 && b0 <= 0xDF:
		b1, ok := cont(1)
		if !ok {
			return 0, 0
		}
		return rune(b0&0x1F)<<6 | rune(b1), 2
	case b0 >= 0xE0 && b0 <= 0xEF:
		b1, ok1 := cont(1)
		b2, ok2 := cont(2)
		if !ok1 || !ok2 {
			return 0, 0
		}
		r := rune(b0&0x0F)<<12 | rune(b1)<<6 | rune(b2)
		if r < 0x800 || (r >= 0xD800 && r <= 0xDFFF) {
			return 0, 0
		}
		return r, 3
	case b0 >= 0xF0 && b0 <= 0xF4:
		b1, ok1 := cont(1)
		b2, ok2 := cont(2)
		b3, ok3 := cont(3)
		if !ok1 || !ok2 || !ok3 {
			return 0, 0
		}
		r := rune(b0&0x07)<<18 | rune(b1)<<12 | rune(b2)<<6 | rune(b3)
		if r < 0x10000 || r > 0x10FFFF {
			return 0, 0
		}
		return r, 4
	}
	return 0, 0
}

// Wynik check poza indeksem: kod błędu albo "".
//
// check sprawdza dane: każdy znak musi pasować do [2] Char, a bajty być poprawnym UTF-8. Zwraca indeks pierwszego
// bajtu z klasy esc (albo len(s), gdy takiego nie ma) i kod błędu. Bajt z klasy stop kończy sprawdzanie kodem
// CodeLineBreak.
func check[T bytestring](s T, esc, stop uint8) (first int, code string) {
	first = -1
	mask := esc | stop | cBad | cHigh
	for i := 0; i < len(s); {
		c := class[s[i]]
		if c&mask == 0 {
			i++
			continue
		}
		switch {
		case c&cBad != 0:
			return i, CodeChar
		case c&cHigh != 0:
			r, n := decode(s, i)
			if n == 0 {
				return i, CodeEncodingInvalid
			}
			if r == 0xFFFE || r == 0xFFFF {
				return i, CodeChar
			}
			i += n
			continue
		case c&stop != 0:
			return i, CodeLineBreak
		}
		if first < 0 {
			first = i
		}
		i++
	}
	if first < 0 {
		first = len(s)
	}
	return first, ""
}

// contains mówi, czy dane zawierają ciąg pat (krótki, ASCII).
func contains[T bytestring](s T, pat string) bool {
	for i := 0; i+len(pat) <= len(s); i++ {
		if s[i] != pat[0] {
			continue
		}
		j := 1
		for j < len(pat) && s[i+j] == pat[j] {
			j++
		}
		if j == len(pat) {
			return true
		}
	}
	return false
}

// asciiName: bit 1 to NameStartChar, bit 2 to NameChar, dla znaków ASCII bez dwukropka (dwukropek sprawdza
// wywołujący, bo jego znaczenie zależy od trybu przestrzeni nazw).
var asciiName = func() (t [128]uint8) {
	for c := 0; c < 128; c++ {
		if c == '_' || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z') {
			t[c] = 1 | 2
		}
		if c == '-' || c == '.' || ('0' <= c && c <= '9') {
			t[c] = 2
		}
	}
	return t
}()

// Wynik nameKind.
const (
	nameBad    = iota // nie pasuje do [5] Name
	nameColon         // pasuje do [5] Name, ale zawiera dwukropek
	nameNCName        // pasuje do NCName (Namespaces in XML 1.0, [4])
)

// nameKind klasyfikuje nazwę według [4] NameStartChar, [4a] NameChar i [5] Name.
func nameKind(s string) int {
	if s == "" {
		return nameBad
	}
	kind := nameNCName
	for i := 0; i < len(s); {
		b := s[i]
		if b < 0x80 {
			switch {
			case b == ':':
				kind = nameColon
			case asciiName[b] == 0, i == 0 && asciiName[b]&1 == 0:
				return nameBad
			}
			i++
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 {
			return nameBad
		}
		if !xmlchars.IsNameChar5(r) || (i == 0 && !xmlchars.IsNameStart5(r)) {
			return nameBad
		}
		i += n
	}
	return kind
}
