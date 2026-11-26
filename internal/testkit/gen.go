package testkit

import (
	"fmt"
	"io"
)

// Generatory dużych dokumentów (testdata/README.md): ten sam rozmiar daje te same bajty, na każdym systemie
// i w każdym przebiegu. Treść jest wymyślona; wartości zależą tylko od numeru rekordu. Dokument powstaje
// strumieniem, więc 1 GB nie zajmuje pamięci. Służą pomiarom wydajności i testom pamięci (K7).

// chunks to io.Reader oddający nagłówek, n rekordów i zakończenie; rekord i tworzy funkcja rec.
type chunks struct {
	head, tail string
	rec        func(buf []byte, i int) []byte
	n, i       int
	buf        []byte
	state      int // 0 nagłówek, 1 rekordy, 2 zakończenie, 3 koniec
}

func (c *chunks) Read(p []byte) (int, error) {
	for len(c.buf) == 0 {
		switch c.state {
		case 0:
			c.buf, c.state = append(c.buf[:0], c.head...), 1
		case 1:
			if c.i >= c.n {
				c.state = 2
				continue
			}
			c.buf = c.rec(c.buf[:0], c.i)
			c.i++
		case 2:
			c.buf, c.state = append(c.buf[:0], c.tail...), 3
		default:
			return 0, io.EOF
		}
	}
	n := copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}

// GenJPK zwraca dokument w stylu JPK (ewidencja sprzedaży) z rows wierszami: dwie przestrzenie nazw z prefiksami,
// elementy z tekstem, polskie litery, jeden wiersz na rekord (ok. 420 bajtów).
func GenJPK(rows int) io.Reader {
	return &chunks{
		head: `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
			`<tns:JPK xmlns:tns="urn:example:jpk" xmlns:etd="urn:example:types">` + "\n" +
			`<tns:Naglowek><tns:KodFormularza kodSystemowy="JPK_EXAMPLE (1)" wersjaSchemy="1-0">JPK_EXAMPLE</tns:KodFormularza>` +
			`<etd:Uwagi>Dane wymyślone, generator testkit.GenJPK</etd:Uwagi></tns:Naglowek>` + "\n<tns:Ewidencja>\n",
		tail: "</tns:Ewidencja>\n</tns:JPK>\n",
		n:    rows,
		rec: func(b []byte, i int) []byte {
			net := int64(i%9973)*137 + 1000 // grosze
			vat := net * 23 / 100
			return fmt.Appendf(b, `<tns:SprzedazWiersz><tns:LpSprzedazy>%d</tns:LpSprzedazy>`+
				`<tns:KodKrajuNadaniaTIN>PL</tns:KodKrajuNadaniaTIN><tns:NrKontrahenta>%010d</tns:NrKontrahenta>`+
				`<tns:NazwaKontrahenta>Przedsiębiorstwo „Żółw %d” sp. z o.o.</tns:NazwaKontrahenta>`+
				`<tns:DowodSprzedazy>FV/2026/%02d/%06d</tns:DowodSprzedazy>`+
				`<tns:DataWystawienia>2026-%02d-%02d</tns:DataWystawienia>`+
				`<tns:K_19>%d.%02d</tns:K_19><tns:K_20>%d.%02d</tns:K_20></tns:SprzedazWiersz>`+"\n",
				i+1, 1000000000+int64(i%89989)*7, i%500, i%12+1, i+1, i%12+1, i%28+1, net/100, net%100, vat/100, vat%100)
		},
	}
}

// GenWordML zwraca dokument w stylu WordprocessingML z paragraphs akapitami: wiele atrybutów z prefiksem,
// elementy puste, xml:space, treść mieszana z odwołaniami znakowymi (ok. 330 bajtów na akapit).
func GenWordML(paragraphs int) io.Reader {
	return &chunks{
		head: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
			`<w:document xmlns:w="urn:example:wordml" xmlns:mc="urn:example:compat" xmlns:w14="urn:example:wordml14" mc:Ignorable="w14">` +
			`<w:body>`,
		tail: `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr></w:body></w:document>` + "\n",
		n:    paragraphs,
		rec: func(b []byte, i int) []byte {
			return fmt.Appendf(b, `<w:p w14:paraId="%08X" w:rsidR="%08X"><w:pPr><w:pStyle w:val="Styl%d"/><w:jc w:val="both"/></w:pPr>`+
				`<w:r><w:rPr><w:b/><w:sz w:val="%d"/></w:rPr><w:t xml:space="preserve">Akapit %d: zażółć gęślą jaźń &amp; &#x201E;cytat&#x201D; </w:t></w:r>`+
				`<w:r><w:t>ciąg dalszy &lt;%d&gt;</w:t></w:r></w:p>`,
				uint32(i)*2654435761, uint32(i+7)*40503, i%9+1, 20+i%5*2, i+1, i)
		},
	}
}
