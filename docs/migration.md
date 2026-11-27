# Przeniesienie czytnika i przełączenie konsumentów

Co skąd przychodzi, w jakiej kolejności i jak sprawdzamy, że nic się nie zmieniło. Prace w repozytoriach
konsumentów prowadzą ich sesje; ten plik jest dla nich wspólną listą.

## Etap 1: przeniesienie z `goxsdgo`

Źródło: `../goxsdgo`, commit `698affa` (przeniesienie wykonane 2026-11-26 z czystego commitu przez `git archive`;
pliki czytnika są w nim identyczne jak w `cfb54cf`, wydanie v0.3.0). Commit i polecenia `diff` są w `provenance.md`,
wyniki czterech dowodów w `testing.md`, decyzje podjęte przy przeniesieniu w `adr/0003-reader-transfer.md`.

| Skąd (`goxsdgo`) | Dokąd (`goxmlgo`) | Uwagi |
|---|---|---|
| `xmlreader/*.go` (kod, ok. 2,7 tys. linii) | `xmlreader/` | bez zmian poza ścieżką importu `xmlchars` i komentarzem pakietu |
| `xmlreader/*_test.go`, `xmlreader/testdata/fuzz/` | `xmlreader/` | testy zgodności czytają zestaw W3C z `testdata/w3c/xmlconf` |
| `internal/xmlchars/` | `xmlchars/` (pakiet publiczny) | potrzebny też wyrażeniom regularnym i typom w `goxsdgo`; komentarze API po angielsku |
| `requirements/xml.md` (XML-001…018) | `requirements/xml.md` | te same identyfikatory i treść; odwołania `REQ:` w testach zostają ważne |
| `docs/adr/0004-xml-reader.md` | streszczenie w `adr/0003-reader-transfer.md` | ADR źródłowy zostaje w `goxsdgo` jako historia |
| `xmlreader/README.md` | `xmlreader/README.md` | liczby porównania z `encoding/xml` mierzone ponownie tutaj, nie kopiowane |
| część „XML reader” z `docs/methodology.md`, `docs/results/xmlreader-*` | `methodology.md`, `results/` | z nowego przebiegu |
| wpis `w3c-xmlconf` z `testdata/sources.json`, `tools/fetch` | jak w źródle | już przeniesiony do manifestu; sumy potwierdza pierwsze pobranie |
| pliki zapadek i wyłączeń zestawu W3C dla czytnika | `testdata/w3c/*.ratchet` | liczba 1 725 / 1 727 jest stanem wyjściowym K1 |

Dowód, że przeniesienie niczego nie zmieniło (zapisany w `testing.md`):

1. różnica plików kodu czytnika między commitem źródłowym a v0.1.0 ogranicza się do ścieżki importu `xmlchars`
   i komentarzy (polecenie `diff` w `provenance.md`);
2. `TestXMLConformance` daje tę samą liczbę i tę samą listę przypadków co w `goxsdgo` (porównanie plików wyników
   przypadek po przypadku);
3. `api/xmlreader.txt` jest równy publicznemu API `goxsdgo/xmlreader` v0.3.0 poza ścieżką pakietu;
4. `goxsdgo` z importem zamienionym na `goxmlgo` (dyrektywa `replace` w kopii roboczej) przechodzi wszystkie swoje
   testy bez innych zmian.

Od tej chwili czytnik w `goxsdgo` jest zamrożony: poprawka czytnika wchodzi do `goxmlgo`, a `goxsdgo` dostaje ją
przez podniesienie wersji.

**Uwaga dla sesji `goxsdgo` (2026-11-26).** Etap 2 znalazł w czytniku wydanym w `goxsdgo` v0.3.0 i v0.4.0 błąd:
deklaracja XML czytana małymi porcjami (np. `iotest.OneByteReader`, mały pierwszy pakiet z sieci) kończy się paniką
albo fałszywym błędem, a bardzo mały dokument z białym znakiem przed `?>` (`<?xml version="1.0" ?><a/>`) jest
odrzucany jako `xml-eof`. Opis i poprawka: `differences.md`, ADR 0004 pkt 9. Decyzją właściciela poprawkę
przeniesiono do zamrożonej kopii jako wyjątek (`goxsdgo` commit `140b2de`, wydanie v0.4.1 z 2026-11-26);
to jedyna zmiana czytnika w `goxsdgo` po przeniesieniu.

## Etap 6a: `goxsdgo` (na v0.2.0)

Przełączenie od razu na v0.2.0, nie na v0.1.0: v0.1.0 nie ma poprawki deklaracji XML, którą `goxsdgo` ma od
v0.4.1, więc byłoby cofnięciem. Sprawdzone 2026-11-27 na kopii `goxsdgo` v0.4.1 z publicznym modułem v0.2.0: po
zamianie importów (7 plików `xmlreader`, 9 plików `xmlchars`) kod buduje się bez innych zmian i 1 127 testów
przechodzi; nie przechodzą dwa testy księgowe, które czytają źródła usuniętego czytnika (`TestRequirementsCovered`:
XML-001…018 i DT-019; `TestCatalogMatchesSource`: kody czytnika). Oba porządkuje sesja `goxsdgo`.

- `go.mod`: `require github.com/polestorm/goxmlgo`; reguła „zero zależności” w `CLAUDE.md`, README i CI zmienia się
  na „jedyna zależność to `goxmlgo`”; nowy ADR w `goxsdgo`;
- importy `goxsdgo/xmlreader` i `goxsdgo/internal/xmlchars` w kodzie i testach zamienione na pakiety `goxmlgo`;
- `goxsdgo/xmlreader` zostaje jako pakiet aliasów (aliasy typów, stałe, funkcje przekazujące), oznaczony jako
  przestarzały; jego test sprawdza, że program napisany dla v0.3.0 kompiluje się bez zmian;
- wymagania XML-* w `goxsdgo` zastąpione odsyłaczem do `goxmlgo`; pakiet dowodów `goxsdgo` wskazuje wersję
  i pakiet dowodów `goxmlgo`;
- miara: wszystkie testy `goxsdgo` przechodzą, liczby K1–K3 `goxsdgo` bez zmian, szybkość walidacji bez regresji
  (ten sam pomiar co w jego `docs/methodology.md`).

Do rozważenia przy okazji, jako osobne decyzje `goxsdgo`: walidator przyjmujący `xmlreader.TokenReader` (walidacja
drzewa albo wyniku filtra) i walidator jako źródło tokenów (odczyt z walidacją w jednym przejściu); `Skip` dla
treści z symbolu wieloznacznego `skip`.

## Etap 6a: `goxadesgo` (na v0.2.0)

- `go.mod`: `goxmlgo` zamiast `goxsdgo` (o ile nie używa walidacji XSD); import czytnika zamieniony;
- miara: wszystkie testy `goxadesgo` przechodzą.

## Etap 6b: drzewa (po v0.4.0)

- `goxadesgo/internal/xmltree` zastąpione przez `goxmlgo/xmltree`; `c14n` pisze z węzłów `xmltree`;
- `goxsdgo/internal/schema/dom.go` zastąpione przez `goxmlgo/xmltree`;
- miara: testy obu projektów przechodzą bez zmiany ich liczb; jeśli `xmltree` czegoś nie pokrywa, brak wraca tu
  jako wymaganie, a nie jest obchodzony u konsumenta.

## `godocxgo` (po v0.2.0)

ADR 0002 w `godocxgo` wskazuje dziś `goxsdgo/xmlreader`; do poprawienia na `goxmlgo/xmlreader`, razem z regułą
zależności i krokiem CI. Etap 1 `godocxgo` używa `Skip`, `Reset` i `Limits.MaxElements`.

## Kandydaci bez terminu

- `gopdfgo`: SVG (`internal/webrender/svg/xmlparse.go`) i odczyt XML Factur-X (`htmlpdf/facturx.go`) korzystają
  z `encoding/xml`; SVG bywa wejściem niezaufanym;
- przyszły czytnik XLSX: arkusze bywają duże, więc wzorzec „wyspa” (wiersz jako poddrzewo) jest dla niego.
