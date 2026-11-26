# Wymagania: czytnik XML

Źródła: Extensible Markup Language (XML) 1.0 Fifth Edition (W3C, 2008-11-26), Namespaces in XML 1.0 Third Edition
(W3C, 2009-12-08). Numery w nawiasach kwadratowych to produkcje gramatyki XML 1.0. Decyzje:
`../docs/adr/0003-reader-transfer.md` (streszczenie ADR 0004 z `goxsdgo`).

## XML-001…018: przeniesione z `goxsdgo`

Status: przyjęte

Tabela przeniesiona z `goxsdgo/requirements/xml.md` z commitu źródłowego (`../docs/provenance.md`) bez zmiany
identyfikatorów i treści; odwołania `REQ:` w przeniesionych testach zostały ważne. Testy: `TestXMLConformance`
(zestaw W3C XML Conformance), `TestCorpus` i testy jednostkowe w `xmlreader/`. „ADR 0004” w treści to ADR
w `goxsdgo`; „od v0.3.0” w XML-018 to wersja `goxsdgo`.

| Id | Wymaganie |
|---|---|
| XML-001 | Dokument ma dokładnie jeden element główny; przed nim i po nim są tylko komentarze, instrukcje przetwarzania i białe znaki, przed nim także deklaracja XML i jedna deklaracja typu dokumentu ([1] document, [22] prolog, [27] Misc). Brak elementu głównego daje `xml-no-root`, inna treść poza nim `xml-content-outside-root`. |
| XML-002 | Każdy znak dokumentu pasuje do [2] Char; inny daje `xml-char`. Bajty niezgodne z kodowaniem dają `xml-encoding-invalid`, w miejscu pierwszego złego bajtu. |
| XML-003 | Kodowanie (4.3.3, dodatek F): UTF-8 z BOM albo bez, UTF-16 LE i BE z BOM, ISO-8859-1 i US-ASCII z deklaracji. Inne kodowania dają `xml-encoding-unsupported`, UTF-32 i EBCDIC też. Deklaracja sprzeczna z wykrytym kodowaniem (BOM UTF-16 albo UTF-8 i inna nazwa) daje `xml-encoding-mismatch`, UTF-16 bez BOM `xml-encoding-invalid`. |
| XML-004 | Końce wierszy są normalizowane przed analizą: CR LF i samo CR stają się LF (2.11). Odwołanie `&#13;` daje znak CR bez zmiany. |
| XML-005 | Nazwy elementów, atrybutów, encji, notacji i celów PI pasują do [5] Name z klasami [4] NameStartChar i [4a] NameChar z piątego wydania; inna daje `xml-name`. |
| XML-006 | Znaczniki: [40] STag, [42] ETag, [44] EmptyElemTag. Znacznik końca musi mieć tę samą nazwę co znacznik początku (`wfc-element-type-match`), nazwa atrybutu nie może się powtórzyć w znaczniku (`wfc-unique-att-spec`), między atrybutami wymagane są białe znaki. Naruszenia gramatyki dają `xml-syntax`, koniec dokumentu w środku konstrukcji `xml-eof`. |
| XML-007 | Wartość atrybutu ([10] AttValue) nie zawiera dosłownego `<`. Jest normalizowana według 3.3.3: białe znaki (dosłowne, także z tekstu encji) stają się spacjami, odwołania są rozwijane; dla atrybutu zadeklarowanego w ATTLIST z typem innym niż CDATA usuwane są spacje na brzegach, a ciągi spacji zastępowane jedną. |
| XML-008 | Odwołania znakowe ([66] CharRef) i do encji predefiniowanych `lt`, `gt`, `amp`, `apos`, `quot` (4.6) dają swój znak jako dane, także gdy podzbiór wewnętrzny deklaruje te encje. Odwołanie do znaku spoza [2] Char daje `wfc-legal-character`. |
| XML-009 | Komentarze ([15]: bez `--` w środku, `xml-comment`), instrukcje przetwarzania ([16], [17]: cel `xml` w dowolnej wielkości liter jest zastrzeżony, `xml-reserved-pi-target`), sekcje CDATA ([18]–[21]) i dane znakowe bez `]]>` ([14], `xml-cdata-end`). Deklaracja XML ([23]–[26], [32], [80], [81]) tylko na samym początku dokumentu, z wersją `1.x` (dokument w wersji innej niż 1.0 jest czytany jak 1.0); błąd daje `xml-decl`. |
| XML-010 | Deklaracja typu dokumentu ([28]) i podzbiór wewnętrzny są sprawdzane składniowo w całości: deklaracje elementów ([45]–[51]), list atrybutów ([52]–[60]), encji ([70]–[76]), notacji ([82]–[83]), PI i komentarze. Odwołanie do encji parametrycznej wewnątrz deklaracji w podzbiorze wewnętrznym daje `wfc-pes-in-internal-subset`; sekcja warunkowa w podzbiorze wewnętrznym `xml-syntax`. |
| XML-011 | Encje wewnętrzne ogólne są rozwijane w treści i w wartościach atrybutów według 4.4 i 4.5 (odwołania znakowe rozwinięte przy deklaracji, odwołania do encji ogólnych przy użyciu). Tekst encji użytej w treści musi być poprawną treścią z zamkniętymi elementami (4.3.2, `xml-entity-structure`). Obowiązują: `wfc-entity-declared` (gdy ograniczenie ma zastosowanie: brak podzbioru zewnętrznego i odwołań do encji parametrycznych albo `standalone="yes"`), `wfc-parsed-entity`, `wfc-no-recursion`, `wfc-no-lt-in-attribute-values`. Encje parametryczne wewnętrzne między deklaracjami są rozwijane. Po odwołaniu do encji parametrycznej, której czytnik nie czyta, dalsze deklaracje encji i list atrybutów nie są przetwarzane, chyba że `standalone="yes"` (5.1). |
| XML-012 | Atrybuty z wartością domyślną z ATTLIST podzbioru wewnętrznego, których znacznik nie podaje, są dodawane do elementu z `Specified=false` (5.1). Obowiązuje pierwsza deklaracja danego atrybutu. |
| XML-013 | Przestrzenie nazw (Namespaces in XML 1.0, rozdziały 3–7): nazwy elementów i atrybutów to QName (`ns-qname`); prefiks musi być zadeklarowany (`nsc-prefix-declared`); prefiksu nie wolno oddeklarować (`nsc-no-prefix-undeclaring`); prefiksy i przestrzenie `xml` i `xmlns` są zastrzeżone (`nsc-reserved-prefixes`); dwa atrybuty elementu nie mogą mieć tej samej nazwy rozwiniętej (`nsc-attributes-unique`); nazwy encji, celów PI i notacji nie zawierają dwukropka (`ns-qname`). Przestrzeń domyślna dotyczy elementów, nie atrybutów; `xmlns=""` ją oddeklarowuje. Deklaracje przestrzeni nazw nie są atrybutami; odczytuje je `LookupNamespace`. |
| XML-014 | Każdy token i każdy błąd ma wiersz i kolumnę liczone od 1; kolumna liczy znaki (punkty kodowe Unicode) po normalizacji końców wierszy. Błąd w tekście zastępczym encji wskazuje miejsce odwołania do niej w dokumencie. Błąd jest krytyczny: po nim `Next` zwraca stale ten sam błąd. |
| XML-015 | Czytnik nigdy nie czyta podzbioru zewnętrznego ani encji zewnętrznych i nie ma dostępu do sieci ani plików. Odwołanie do zewnętrznej encji ogólnej w treści daje `unsupported-external-entity`, w wartości atrybutu `wfc-no-external-entity-references`; odwołanie do niezadeklarowanej encji, gdy WFC Entity Declared nie obowiązuje, daje `unsupported-entity-unresolved`. |
| XML-016 | Limity z bezpiecznymi wartościami domyślnymi (ADR 0004): głębokość elementów (`limit-depth`), długość nazwy (`limit-name-length`), liczba atrybutów (`limit-attributes`), długość tokenu tekstu, komentarza, PI i wartości atrybutu (`limit-text-size`), łączny rozmiar rozwinięć encji (`limit-entity-expansion`, ochrona przed „billion laughs”), zagnieżdżenie encji (`limit-entity-depth`), rozmiar deklaracji typu dokumentu (`limit-dtd-size`). Zero w `Limits` oznacza wartość domyślną, wartość ujemna brak limitu. |
| XML-017 | Czytanie jest strumieniowe: pamięć czytnika zależy od najdłuższego tokenu, głębokości i rozmiaru podzbioru wewnętrznego, nie od rozmiaru dokumentu. |
| XML-018 | Każdy atrybut ma typ z deklaracji ATTLIST podzbioru wewnętrznego (`Attr.Type`: `CDATA`, `ID`, `IDREF`, `IDREFS`, `ENTITY`, `ENTITIES`, `NMTOKEN`, `NMTOKENS`, `NOTATION`, `ENUMERATION`, czyli wartości [attribute type] z XML Infoset), także atrybut dodany z wartości domyślnej; obowiązuje pierwsza deklaracja; atrybut bez przetworzonej deklaracji ma typ pusty. Potrzebne m.in. do funkcji `id()` w XPath i XPointer (od v0.3.0). |

## XML-019…: rozszerzenia (etap 2)

Status: szkic

| Id | Wymaganie |
|---|---|
| XML-019 | `Skip` wywołane po tokenie początku elementu pomija resztę tego elementu: następny `Next` zwraca pierwszy token za jego znacznikiem końca. Dla elementu pustego (`SelfClosing`) pomija jego token końca. Wywołane w innym stanie zwraca błąd ze stałym kodem i nie zmienia stanu czytnika. |
| XML-020 | Treść pomijana przez `Skip` jest sprawdzana tak samo jak czytana: dokument źle uformowany, naruszenie ograniczeń przestrzeni nazw albo limitu w pomijanym poddrzewie daje ten sam błąd (kod, wiersz, kolumna), co przy czytaniu token po tokenie. |
| XML-021 | `Skip` nie alokuje pamięci zależnie od rozmiaru pomijanej treści i nie buduje wartości atrybutów ani tokenów tekstu, których nikt nie odbierze. |
| XML-022 | `Reset(r, opts)` przygotowuje czytnik do nowego dokumentu z zachowaniem buforów. Strumień tokenów i błędów po `Reset` jest identyczny ze strumieniem nowego czytnika (`New`) dla tego samego wejścia i opcji, niezależnie od tego, co czytnik czytał wcześniej i czy skończył błędem. |
| XML-023 | `Limits.MaxElements` ogranicza łączną liczbę elementów dokumentu; przekroczenie daje `limit-elements` w miejscu elementu, który przekroczył limit. Wartość zero oznacza domyślną (bez limitu), ujemna brak limitu. Elementy pomijane przez `Skip` są liczone. |
| XML-024 | Typ `TokenReader` (metoda `Next() (Token, error)`) jest spełniony przez `*Reader`. Kontrakt: `io.EOF` po ostatnim tokenie; po pierwszym błędzie stale ten sam błąd; token ważny do następnego `Next`. |
| XML-025 | Token początku elementu niesie deklaracje przestrzeni nazw tego elementu w kolejności z dokumentu (prefiks, nazwa przestrzeni; pusty prefiks to przestrzeń domyślna), także te z wartości domyślnych ATTLIST. Wycinek jest ważny do następnego `Next` (ADR 0002). |
| XML-026 | Typ śledzący zakres przestrzeni nazw, zasilany tokenami, daje dla każdego tokenu te same odpowiedzi co `Reader.LookupNamespace` w tym samym miejscu dokumentu. |
| XML-027 | Czytnik udostępnia treść deklaracji XML: wersję, kodowanie z deklaracji, kodowanie wykryte i `standalone` (albo informację, że deklaracji nie było). |
| XML-028 | Trwałość danych tokenu jest częścią kontraktu: napisy (`Name.Space`, `Name.Local`, `Name.Prefix`, `Attr.Value`, `Attr.Type`, `Binding.Prefix`, `Binding.URI`) pozostają ważne i niezmienne po następnym `Next`; `Token.Data` oraz wycinki `Token.Attrs` i deklaracji są ważne tylko do następnego `Next`. Test zatrzymuje wartości z dużego dokumentu i porównuje je po zakończeniu czytania. |
| XML-029 | Rozszerzenia nie zmieniają zachowania istniejącego API: strumień tokenów i błędów dla każdego dokumentu zestawu W3C i korpusu jest taki sam jak w v0.1.0 (poza nowymi polami tokenu). |
