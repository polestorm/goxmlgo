# Testy i porównania

Nacisk projektu: każda własność, którą obiecujemy, ma test, który da się uruchomić z czystego klonu, i liczbę, którą
da się odtworzyć. Dane cudze są pobierane według manifestu z sumami, dane własne generowane deterministycznie.

## Warstwy

| Warstwa | Gdzie | Co sprawdza |
|---|---|---|
| zgodność czytnika | `TestXMLConformance`, `testdata/w3c/xmlconf` (pobierany), zapadka `testdata/w3c/xmlconf.ratchet` | werdykt dobrej formy i druga postać kanoniczna dla przypadków W3C w zakresie; liczba nie spada (K1) |
| wymagania | `requirements/*.md`, `TestRequirementsCovered` | każde przyjęte wymaganie ma test; każde `REQ:` wskazuje istniejące wymaganie (K4) |
| wzorzec API | `api/<pakiet>.txt`, `TestAPISnapshot`, `TestReaderAPIMatchesGoxsdgo`, `TestErrorCatalog` | publiczne API każdego pakietu równe wzorcowi; zmiana tylko przez `go test . -run TestAPISnapshot -update` i wpis w ADR (K4); API czytnika równe `goxsdgo` v0.3.0; kody błędów równe katalogowi `errors.md` |
| korpus | `testdata/corpus/*.xml` + `*.tokens` (od etapu 1) + `*.out.xml` (od etapu 3), `TestCorpus` | wzorcowy zrzut tokenów i wzorcowy zapis co do bajta; zmiana wzorca przez `go test ./xmlreader -run TestCorpus -update` i przegląd różnicy |
| zapis powrotny | `TestRoundTrip` | odczyt → zapis → odczyt: równość strumieni tokenów; zapis idempotentny co do bajta; na zestawie W3C i korpusie (K2) |
| własności pisarza | `TestWriterWellFormed`, `FuzzWriter` | dowolny ciąg wywołań: albo błąd ze stałym kodem, albo wyjście, które czytnik przyjmuje i z którego odtwarza te same wartości |
| drzewo | `TestTreeRoundTrip`, `TestSubtreeInStream`, testy operacji | drzewo → zapis równe strumień → zapis; wyspa w strumieniu; każda operacja zmiany (K3) |
| różnicowe | `TestStdlibConformance`, `TestStdlibWriter` | `encoding/xml` jako czarna skrzynka: te same wejścia przez oba czytniki; wyjście pisarza czytane przez `encoding/xml` daje te same nazwy i wartości |
| bezpieczeństwo | `TestSafeByDefault`, `TestHostileInputs`, `TestNoIOImports` | limity domyślne, encje zewnętrzne, wrogie wejścia w czasie i pamięci; pakiety biblioteki nie importują `os`, `net`, `net/http` (K6) |
| złożoność | `TestPathological` | czas liniowy na wejściach patologicznych (powtórzone atrybuty, głębokie zagnieżdżenie, długie nazwy, wiele deklaracji przestrzeni nazw, łańcuchy encji) (K6) |
| pamięć strumienia | `TestStreamingMemory` (duży plik przez zmienną środowiskową) | czytnik → pisarz w stałej pamięci; 1 GB w < 64 MB (K7) |
| powtarzalność | `TestDeterministic`, macierz CI | 10 przebiegów, te same bajty; wzorce korpusu zgodne na Linux, macOS i Windows; `go test -race` (K8) |
| odporność | `FuzzReader`, `FuzzRoundTrip`, `FuzzWriter`, `FuzzTree`, `tools/fuzz` | bez paniki, niezmienniki z ADR 0001 (K5); dziennik w `fuzzing.md` |
| wydajność | `Benchmark*`, `tools/bench` | szybkość i alokacje czytnika, pisarza, przelotu i drzewa; porównanie z czytnikiem `goxsdgo` v0.3.0 i z `encoding/xml` (K7); metoda w `methodology.md` |
| przykłady | `Example*` | przykłady z dokumentacji kompilują się i dają zapisany wynik |
| dowody | `tools/evidence` | pakiet K9: przebiegi testów, macierz śledzenia, tabela K1–K9 |

## Zasady

- test odwołuje się do wymagania komentarzem `REQ: PREFIKS-nnn`;
- wzorce (`*.tokens`, `*.out.xml`, `api/*.txt`) zmieniają się tylko przez `-update` i przegląd różnicy w commicie;
- błąd znaleziony przez fuzzing trafia do `testdata/fuzz/` pakietu jako wejście regresji i dostaje test nazwany;
- liczba przypadków W3C w podziale na grupy jest datowaną miarą; spadek wymaga wpisu w `differences.md`;
- porównania z innymi implementacjami: wsad publiczny albo generowany, narzędzie w repozytorium, surowy wynik
  w `results/`, metoda w `methodology.md`. Liczby w README mają odsyłacz do obu;
- testy nie sięgają do sieci; dane pobiera wyłącznie `tools/fetch`, a test bez danych jest pomijany z komunikatem,
  którego pakiet dowodów nie liczy jako zaliczenia.

## Pomiary

| Data | Commit | K1: W3C (zaliczone / w zakresie) | K2: zapis powrotny | K3: drzewo | Uwagi |
|---|---|---|---|---|---|
| 2026-11-26 | etapy 0–1, drzewo robocze przed pierwszym commitem (źródło: `goxsdgo` `698affa`) | 1 725 / 1 727 (eduni 476/477, ibm 526/527, oasis 323/323, sun 101/101, xmltest 299/299) | — (etap 3) | — (etap 4) | `go test ./xmlreader -run TestXMLConformance -v`, Go 1.26.6, macOS arm64; ta sama liczba i lista co w `goxsdgo`; korpus: 13 dokumentów, `TestCorpus` zgodny ze wzorcami; wymagania: 25 przyjętych, wszystkie z testem |

## Dowody przeniesienia czytnika (etap 1, 2026-11-26)

Cztery sprawdzenia z `migration.md`. Commit źródłowy `goxsdgo`: `698affa` (czysty, `git archive`); pliki czytnika są
w nim identyczne jak w v0.3.0 (`git diff v0.3.0 698affa -- xmlreader internal/xmlchars` nic nie wypisuje).

| # | Sprawdzenie | Wynik | Jak powtórzyć |
|---|---|---|---|
| 1 | różnica plików kodu czytnika ogranicza się do ścieżki importu `xmlchars` i komentarzy | `chars.go`: import i komentarz pod nim; `reader.go`: ostatnie zdanie komentarza pakietu; `dtd.go`, `errors.go`, `input.go` bez różnic; `xmlchars.go`: tylko wiersze komentarzy | polecenia `diff` w `provenance.md` |
| 2 | `TestXMLConformance` daje tę samą liczbę i tę samą listę przypadków | 1 725 / 1 727; zapadka `testdata/w3c/xmlconf.ratchet` identyczna co do bajta z plikiem z `goxsdgo`; plik wyników 1 727 przypadków (`results/xmlreader-conformance.tsv`, oba czytniki) identyczny co do bajta z `docs/results/xmlreader-conformance.tsv` z `goxsdgo` | `diff testdata/w3c/xmlconf.ratchet <(git -C ../goxsdgo show 698affa:testdata/w3c/xmlconf.ratchet)` i to samo dla pliku TSV |
| 3 | publiczne API czytnika równe API `goxsdgo/xmlreader` v0.3.0 poza ścieżką pakietu | `api/xmlreader.txt` i `api/baseline/xmlreader-goxsdgo-v0.3.0.txt` różnią się tylko pierwszym wierszem (`package …`) | `TestReaderAPIMatchesGoxsdgo`; wzorzec źródła: `go run ./internal/apisnap <katalog xmlreader z commitu źródłowego> github.com/polestorm/goxsdgo/xmlreader` |
| 4 | `goxsdgo` z importem zamienionym na `goxmlgo` przechodzi wszystkie swoje testy bez innych zmian | przed zamianą i po niej: 9 pakietów `ok`, 1 355 testów zaliczonych, 0 porażek, 1 pominięty (ten sam, `TestDumpErrors`, wymaga zmiennej środowiskowej); listy wyników testów i wszystkie wiersze z liczbami (W3C XSD 39 369 / 39 439, typy NIST i Microsoft, XML Conformance) identyczne | opis i wynik: `results/transfer-goxsdgo-tests.txt`; przebieg w kopii z `git archive`, w `../goxsdgo` nic nie zmieniono |

Dowód 4 objął moduł główny `goxsdgo`. Jego narzędzia w osobnych modułach (`tools/compare` z cgo i libxml2,
`tools/bench`, `tools/evidence`) nie były budowane: to zakres etapu 6a w sesji `goxsdgo`.

Nie sprawdzono lokalnie: Windows (kompilacja skrośna i `go vet` przechodzą, testy uruchomi dopiero CI; `goxsdgo`
nie miało Windows w macierzy, więc dla przeniesionych testów to pierwszy przebieg na tym systemie).
