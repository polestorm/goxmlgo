# Dane testowe

| Katalog | Zawartość | Źródło | W repozytorium |
|---|---|---|---|
| `w3c/xmlconf/` | W3C XML Conformance Test Suite 20130923 | pobierany przez `tools/fetch` według `sources.json` (adres, suma archiwum, suma drzewa plików, liczba plików) | nie |
| `w3c/*.ratchet` | zapadki: lista zaliczonych przypadków, która nie może się skrócić | generowane przez test z `-update` | tak |
| `corpus/` | dokumenty syntetyczne z wzorcowym zrzutem tokenów (`*.tokens`) i wzorcowym zapisem (`*.out.xml`) | pisane ręcznie i generator | tak |
| `hostile/` | wrogie wejścia (rozwijanie encji, głębokie zagnieżdżenie, długie nazwy) | generowane w teście, nie pliki | — |
| `<pakiet>/testdata/fuzz/` | wejścia regresji fuzzingu | znaleziska kampanii | tak |

## Zestaw W3C

Zestaw jest danymi testowymi, nie kodem, nie jest objęty licencją `goxmlgo` i nie jest w repozytorium: każdy
pobiera go sam z serwera W3C (`https://www.w3.org/XML/Test/xmlts20130923.tar.gz`), a `tools/fetch` sprawdza sumę
archiwum, sumę drzewa plików i liczbę plików (3 386). Sumy z manifestu potwierdzono pobraniem 2026-11-26.

Warunki użycia: archiwum nie ma jednego pliku licencji. W3C publikuje swoje zestawy testów na warunkach opisanych na
stronie zestawu (`https://www.w3.org/XML/Test/`); części zestawu mają własne noty autorów w plikach, m.in.
`xmlconf.xml` (Sun Microsystems 1998, OASIS 1999 i 2001) i `xmltest/readme.html` (James Clark 1998: wolno kopiować
i zmieniać do użytku wewnętrznego, rozpowszechniać wolno tylko niezmienione archiwum `xmltest.zip`). Dlatego
zestawu nie kopiujemy do repozytorium ani do paczek wydań, a z jego plików w repozytorium są tylko identyfikatory
przypadków (zapadka, plik wyników). Paczka fuzzingu (`tools/fuzz -pack`) zawiera zestaw jako wejścia startowe
i służy wyłącznie do użytku wewnętrznego: nie publikujemy jej.

Pobranie i sprawdzenie:

```sh
go -C tools/fetch run . -root ../.. -set w3c-xmlconf
go -C tools/fetch run . -root ../.. -verify
```

Bez Go: `python tools/fetch/fetch.py` (te same sumy).

## Korpus

Stan po etapie 0: 13 dokumentów w `corpus/`, każdy ze wzorcem `*.tokens` (zrzut strumienia tokenów czytnika:
rodzaj, nazwa rozwinięta, deklaracje przestrzeni nazw, atrybuty z typem i pozycją, dane, pozycje; napisy
w zapisie ASCII). Wzorzec zmienia się tylko przez `go test ./xmlreader -run TestCorpus -update` i przegląd
różnicy. Wzorce zapisu `*.out.xml` dojdą z pisarzem (etap 3). Komentarz na początku dokumentu podaje wymagania,
które dokument pokazuje (`REQ:`). Pliki są porównywane co do bajta (CR LF, UTF-16), więc `.gitattributes`
wyłącza dla nich przekształcanie końców wierszy.

| Plik | Rodzina |
|---|---|
| `ns-scopes.xml` | 1: przestrzenie nazw |
| `mixed-whitespace.xml` | 2: treść mieszana, białe znaki, CR LF i samo CR |
| `characters.xml` | 3: odwołania znakowe, encje wbudowane, znaki spoza BMP, polskie litery w nazwach |
| `cdata-comments-pi.xml` | 4: CDATA, komentarze, PI, także poza korzeniem |
| `doctype-internal.xml` | 5: podzbiór wewnętrzny |
| `encoding-utf8-bom.xml`, `encoding-utf16le.xml`, `encoding-utf16be.xml`, `encoding-latin1.xml` | 6: kodowania |
| `style-wordml.xml`, `style-spreadsheetml.xml`, `style-svg.xml`, `style-jpk.xml` | 7: dokumenty „w stylu” (przestrzenie nazw zastąpione wymyślonymi `urn:example:*`) |

Treść wymyślona. Każdy dokument pokazuje jedną rodzinę zjawisk, które zapis powrotny musi utrzymać:

1. przestrzenie nazw: domyślna, zmiana i wyłączenie domyślnej, ten sam prefiks dla różnych przestrzeni na różnych
   poziomach, różne prefiksy tej samej przestrzeni, atrybuty z prefiksem, `xml:space`, `xml:lang`;
2. treść mieszana i białe znaki: tekst między elementami, same białe znaki, CR LF na wejściu, tabulatory;
3. znaki: odwołania znakowe, encje wbudowane, znaki spoza BMP, polskie litery w nazwach elementów i atrybutów,
   znaki TAB, LF, CR w wartościach atrybutów;
4. CDATA, komentarze i instrukcje przetwarzania, także przed korzeniem i po nim;
5. DOCTYPE z podzbiorem wewnętrznym: encje, wartości domyślne atrybutów, typy atrybutów;
6. kodowania: UTF-8 z BOM, UTF-16 LE i BE, ISO-8859-1;
7. dokumenty „w stylu”: fragment WordprocessingML (wiele atrybutów, `mc:Ignorable`), fragment arkusza
   SpreadsheetML, SVG, wiersze JPK. Pisane od zera, bez treści z prawdziwych plików.

Generator dużych dokumentów jest funkcją deterministyczną w repozytorium (`internal/testkit/gen.go`): `GenJPK(n)`
(wiersze w stylu JPK, ok. 430 bajtów na wiersz) i `GenWordML(n)` (akapity w stylu WordprocessingML, ok. 300 bajtów
na akapit). Ten sam rozmiar daje te same bajty; `TestGeneratorsAreDeterministic` trzyma sumy SHA-256, więc zmiana
generatora (czyli wsadu pomiarów) jest widoczna w commicie. Służy pomiarom wydajności i testowi pamięci od etapu 2;
przeniesione benchmarki czytnika używają jeszcze `jpkLike` z `xmlreader/reader_test.go`.

## Dane prywatne: nigdy w repozytorium

Pliki odbiorców nie trafiają do repozytorium ani do porównań. Jeśli kiedyś posłużą do lokalnej próby, do dokumentów
trafiają tylko liczby, oznaczone jako niepowtarzalne z zewnątrz.
