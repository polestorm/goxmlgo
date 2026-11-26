// Package xmlchars holds the name character classes of XML 1.0, shared by the XML reader and by other packages
// that need the same rules (in goxsdgo: the regular expression translator and the simple types).
//
// There are two sets of rules for name characters:
//   - XML 1.0 Second Edition, Appendix B (Letter, Digit, CombiningChar, Extender): the edition XSD 1.0 refers to
//     for the types Name, NCName, NMTOKEN and for the classes \i and \c in patterns. This package only names that
//     edition (Edition2); its tables are kept by the packages that use them.
//   - XML 1.0 Fifth Edition, productions [4] NameStartChar and [4a] NameChar: the rules the reader reads
//     documents by. Their tables and functions are here.
//
// The package was moved from goxsdgo (internal/xmlchars) without a change in behaviour; see docs/provenance.md.
package xmlchars

// Edition selects the rules for name characters.
type Edition uint8

const (
	// Edition2 is XML 1.0 Second Edition (what XSD 1.0 Second Edition refers to for name types and patterns).
	Edition2 Edition = iota
	// Edition5 is XML 1.0 Fifth Edition (what the reader reads documents by).
	Edition5
)

// NameStart5 holds the ranges of production [4] NameStartChar of XML 1.0 Fifth Edition, in ascending order.
var NameStart5 = [][2]rune{
	{':', ':'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'},
	{0xC0, 0xD6}, {0xD8, 0xF6}, {0xF8, 0x2FF}, {0x370, 0x37D}, {0x37F, 0x1FFF}, {0x200C, 0x200D},
	{0x2070, 0x218F}, {0x2C00, 0x2FEF}, {0x3001, 0xD7FF}, {0xF900, 0xFDCF}, {0xFDF0, 0xFFFD}, {0x10000, 0xEFFFF},
}

// NameCharExtra5 holds the ranges that production [4a] NameChar adds to NameStartChar, in ascending order.
var NameCharExtra5 = [][2]rune{
	{'-', '.'}, {'0', '9'}, {0xB7, 0xB7}, {0x300, 0x36F}, {0x203F, 0x2040},
}

// IsNameStart5 reports whether r matches production [4] NameStartChar of XML 1.0 Fifth Edition.
func IsNameStart5(r rune) bool {
	if r < 0x80 {
		return r == ':' || r == '_' || ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
	}
	switch {
	case 0xC0 <= r && r <= 0xD6, 0xD8 <= r && r <= 0xF6, 0xF8 <= r && r <= 0x2FF,
		0x370 <= r && r <= 0x37D, 0x37F <= r && r <= 0x1FFF, 0x200C <= r && r <= 0x200D,
		0x2070 <= r && r <= 0x218F, 0x2C00 <= r && r <= 0x2FEF, 0x3001 <= r && r <= 0xD7FF,
		0xF900 <= r && r <= 0xFDCF, 0xFDF0 <= r && r <= 0xFFFD, 0x10000 <= r && r <= 0xEFFFF:
		return true
	}
	return false
}

// IsNameChar5 reports whether r matches production [4a] NameChar of XML 1.0 Fifth Edition.
func IsNameChar5(r rune) bool {
	if r < 0x80 {
		return IsNameStart5(r) || r == '-' || r == '.' || ('0' <= r && r <= '9')
	}
	return IsNameStart5(r) || r == 0xB7 || (0x300 <= r && r <= 0x36F) || (0x203F <= r && r <= 0x2040)
}
