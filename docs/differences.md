# Różnice i wyłączenia

Trzy listy, każda z powodem. Wpis powstaje razem z testem, który różnicę pokazuje.

## Zestaw W3C XML Conformance: wyłączenia i porażki (K1)

Stan z 2026-11-26 (`go test ./xmlreader -run TestXMLConformance -v`, zestaw 20130923, 2 585 przypadków): 1 727
w zakresie, 1 725 zaliczonych; według grup: eduni 476/477, ibm 526/527, oasis 323/323, sun 101/101, xmltest 299/299.
Te same liczby i ta sama lista przypadków co w `goxsdgo` w commicie źródłowym (`testing.md`). Reguły wyłączeń są
w `exclusion` w `xmlreader/conformance_test.go` i czytają atrybuty przypadku z plików zestawu.

Wyłączenia (858 przypadków poza zakresem):

| Grupa | Liczba | Powód |
|---|---|---|
| `RECOMMENDATION` równe `XML1.1` albo `NS1.1` | 273 | XML 1.1 i Namespaces 1.1 są poza zakresem (ADR 0001, pkt 9) |
| `VERSION` bez `1.0` | 1 | jak wyżej: przypadek dotyczy tylko XML 1.1 |
| `EDITION` bez `5` | 310 | przypadek opisuje zachowanie wydań 1–4 XML 1.0; czytnik stosuje piąte wydanie (m.in. inne znaki nazw) |
| `ENTITIES` inne niż `none` | 265 | wynik zależy od encji zewnętrznych albo podzbioru zewnętrznego, których czytnik nigdy nie czyta (XML-015) |
| `TYPE` równe `error` | 9 | błąd opcjonalny: rekomendacja nie wymaga od procesora jego zgłoszenia |

Dwa inne warunki są w kodzie, ale dziś niczego nie wyłączają (0 przypadków): przypadki Namespaces z wyłączonym
przetwarzaniem przestrzeni nazw i dokumenty poprawne w kodowaniu, którego czytnik nie obsługuje.

Porażki w zakresie (2), obie świadome; lista jest w zapadce `testdata/w3c/xmlconf.ratchet`:

| Przypadek | Co się dzieje | Powód |
|---|---|---|
| `ibm/ibm-valid-P29-ibm29v01.xml` | dokument jest przyjęty, ale druga postać kanoniczna nie ma instrukcji przetwarzania `<?sound …?>`, którą zestaw w niej podaje | instrukcja stoi wewnątrz DOCTYPE; czytnik nie podaje instrukcji z podzbioru wewnętrznego jako tokenów (nie należą do treści dokumentu; ADR 0003, pkt 5) |
| `eduni/rmt-e3e-13` | dokument typu „invalid” (dobrze uformowany) jest odrzucany kodem `unsupported-entity-unresolved` | odwołanie do encji niezadeklarowanej w podzbiorze wewnętrznym, gdy WFC Entity Declared nie obowiązuje; czytnik woli błąd niż ciche pominięcie treści (ADR 0003, pkt 6) |

## Zapis powrotny: wyjątki (K2)

Dokumenty, dla których odczyt → zapis → odczyt nie daje równego strumienia tokenów, z powodem (np. dokument, którego
treść zależy od deklaracji w DOCTYPE, którego pisarz nie zapisuje).

| Dokument | Powód |
|---|---|
| — | — |

## Względem `encoding/xml`

Zachowania, w których czytnik albo pisarz świadomie różni się od biblioteki standardowej, z odwołaniem do reguły
rekomendacji.

Czytnik (pomiar na tych samych 1 727 przypadkach: `results/xmlreader-conformance.tsv`, metoda w `methodology.md`):

| Zachowanie | `goxmlgo/xmlreader` | `encoding/xml` (`Decoder`, ustawienia domyślne) | Reguła |
|---|---|---|---|
| powtórzony atrybut `<a x='1' x='2'/>` | błąd `wfc-unique-att-spec` | przyjęty, oba atrybuty zwrócone | WFC: Unique Att Spec |
| drugi element główny `<a/><b/>` | błąd `xml-content-outside-root` | przyjęty | [1] document |
| dokumenty źle uformowane zestawu W3C | odrzucone 951 z 951 | odrzucone 434 z 951 | XML 1.0, 1.2: błąd krytyczny |
| dokumenty dobrze uformowane zestawu W3C | przeczytane 774 z 776 (dwie porażki wyżej) | przeczytane 712 z 776 | — |
| podzbiór wewnętrzny DTD | encje wewnętrzne rozwinięte, atrybuty domyślne dodane, typy atrybutów, notacje | zwracany jako surowy tekst dyrektywy; encje z DTD nieznane | 4.4, 3.3.2, 5.1 |
| ograniczenia przestrzeni nazw | sprawdzane (`nsc-*`, `ns-qname`) | prefiks bez deklaracji przyjęty | Namespaces in XML 1.0, rozdz. 3–7 |
| pozycja | wiersz i kolumna (w znakach) każdego tokenu i błędu | `InputPos` po tokenie; wiersz w tekście błędu | — |
| błędy | stały kod, wiersz, kolumna | tekst komunikatu | — |
| limity | domyślnie włączone | brak wbudowanych | — |
| kodowania wejścia | UTF-8, UTF-16, ISO-8859-1, US-ASCII | UTF-8; inne przez `CharsetReader` | 4.3.3 |
| odwołanie do encji zewnętrznej w treści | błąd `unsupported-external-entity` | błąd (encja nieznana) | 4.4.3, 5.1 |

Pisarz: wiersze dojdą w etapie 3.

## Względem `goxsdgo/xmlreader` v0.3.0 (i `goxmlgo` v0.1.0)

Czytnik przeszedł bez zmiany zachowania (etap 1), a rozszerzenia etapu 2 są dodatkami. Jedyne różnice w zachowaniu
to poprawki błędów znalezionych w etapie 2 testem czytania porcjami (`TestChunkedInput`) i testem różnicowym
(`tools/bench`, `TestSameAsGoxsdgo`). Dla dokumentu czytanego jednym odczytem z bufora większego niż dokument
różnic nie ma poza pierwszym wierszem tabeli: 1 727 przypadków W3C w zakresie daje ten sam strumień
(`TestTokenStreamStable`), a z 3 381 plików zestawu i korpusu różni się jeden.

| Zachowanie | v0.3.0 / v0.1.0 | od v0.2.0 | Reguła |
|---|---|---|---|
| deklaracja XML z białym znakiem przed `?>`, gdy za nią w buforze jest mniej bajtów niż nazwa szukanego pseudoatrybutu (np. cały dokument `<?xml version="1.0" ?><a/>`) | błąd `xml-eof 1:20` | dokument przeczytany | [23] XMLDecl |
| deklaracja XML, gdy wejście przychodzi małymi porcjami (odczyt po jednym bajcie, mały pierwszy pakiet z sieci) | panika (`slice bounds out of range`) albo fałszywy błąd `xml-decl` | dokument przeczytany, strumień taki sam jak przy jednym odczycie | [23]; DET-003 |
| znak niedozwolony ([2] Char) z zakresu ASCII tuż za nazwą | kod `xml-syntax` albo `xml-char` zależnie od tego, gdzie skończył się bufor | zawsze `xml-syntax` (jak dotąd przy jednym odczycie) | DET-003 |
| plik `ibm/xml-1.1/valid/P03/ibm03v09.ent` zestawu W3C (encja zewnętrzna, nie dokument; kończy się tuż za deklaracją ze znakiem U+2028) | `xml-eof 1:20` | `xml-decl 1:21` | skutek pierwszego wiersza |

Przyczyna dwóch pierwszych: czytnik zapamiętywał pozycję w buforze wejścia, a doczytanie danych przesuwa jego
zawartość. Dotyczy to wydanych wersji `goxsdgo` (v0.3.0, v0.4.0). Decyzją właściciela tę jedną poprawkę przeniesiono
także do zamrożonej kopii czytnika w `goxsdgo` (commit `140b2de` z 2026-11-26, jeszcze bez wydania); pozostałe zmiany
etapu 2 `goxsdgo` dostanie przez przejście na `goxmlgo` (`migration.md`).
