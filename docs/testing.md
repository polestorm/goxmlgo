# Testy i porównania

Nacisk projektu: każda własność, którą obiecujemy, ma test, który da się uruchomić z czystego klonu, i liczbę, którą
da się odtworzyć. Dane cudze są pobierane według manifestu z sumami, dane własne generowane deterministycznie.

## Warstwy

| Warstwa | Gdzie | Co sprawdza |
|---|---|---|
| zgodność czytnika | `TestXMLConformance`, `testdata/w3c/xmlconf` (pobierany), zapadka `testdata/w3c/xmlconf.ratchet` | werdykt dobrej formy i druga postać kanoniczna dla przypadków W3C w zakresie; liczba nie spada (K1) |
| wymagania | `requirements/*.md`, `TestRequirementsCovered` | każde przyjęte wymaganie ma test; każde `REQ:` wskazuje istniejące wymaganie (K4) |
| wzorzec API | `api/<pakiet>.txt`, `TestAPISnapshot`, `TestReaderAPIKeepsGoxsdgo`, `TestErrorCatalog` | publiczne API każdego pakietu równe wzorcowi; zmiana tylko przez `go test . -run TestAPISnapshot -update` i wpis w ADR (K4); API czytnika równe `goxsdgo` v0.3.0; kody błędów równe katalogowi `errors.md` |
| korpus | `testdata/corpus/*.xml` + `*.tokens` (od etapu 1) + `*.out.xml` (od etapu 3), `TestCorpus` | wzorcowy zrzut tokenów i wzorcowy zapis co do bajta; zmiana wzorca przez `go test ./xmlreader -run TestCorpus -update` i przegląd różnicy |
| zapis powrotny | `TestRoundTrip` | odczyt → zapis → odczyt: równość strumieni tokenów; zapis idempotentny co do bajta; na zestawie W3C i korpusie (K2) |
| własności pisarza | testy wymagań WR w `xmlwriter` (`TestTextEscaping`, `TestExplicitNamespaces`, `TestAutoNamespaces`, `TestErrorState`, `TestLimitsMatchReader`, …), `FuzzWriter` | dowolny ciąg wywołań: albo błąd ze stałym kodem i stan bez zmian, albo wyjście, które czytnik przyjmuje i z którego odtwarza te same wartości |
| drzewo | `TestTreeRoundTrip`, `TestSubtreeInStream`, testy operacji | drzewo → zapis równe strumień → zapis; wyspa w strumieniu; każda operacja zmiany (K3) |
| różnicowe | `TestStdlibConformance`, `TestStdlibWriter` | `encoding/xml` jako czarna skrzynka: te same wejścia przez oba czytniki; wyjście pisarza czytane przez `encoding/xml` daje te same nazwy i wartości |
| bezpieczeństwo | `TestSafeByDefault`, `TestHostileInputs`, `TestNoIOImports` | limity domyślne, encje zewnętrzne, wrogie wejścia w czasie i pamięci; pakiety biblioteki nie importują `os`, `net`, `net/http` (K6) |
| złożoność | `TestPathological` | czas liniowy na wejściach patologicznych (powtórzone atrybuty, głębokie zagnieżdżenie, długie nazwy, wiele deklaracji przestrzeni nazw, łańcuchy encji) (K6) |
| pamięć strumienia | `TestStreamingMemory` (duży plik przez zmienną środowiskową) | czytnik → pisarz w stałej pamięci; 1 GB w < 64 MB (K7) |
| powtarzalność | `TestDeterministic`, macierz CI | 10 przebiegów, te same bajty; wzorce korpusu zgodne na Linux, macOS i Windows; `go test -race` (K8) |
| podział wejścia | `TestChunkedInput`, `TestXMLDeclSmallDocument` | ten sam strumień przy jednym odczycie, po jednym bajcie i porcjami po 7 bajtów, na korpusie i 1 727 przypadkach W3C |
| zgodność z poprzednikiem | `TestTokenStreamStable`, `tools/bench`: `TestSameAsGoxsdgo` | strumień tokenów i błędów taki sam jak w v0.1.0 i w `goxsdgo` v0.3.0; różnice tylko wymienione w `differences.md` |
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

## Pakiet dowodów (K9)

```sh
go -C tools/evidence run . -root ../..                    # pakiet w out/evidence/<data>-<commit>/
go -C tools/evidence run . -root ../.. -stream-mb 1024    # dodatkowo pomiar 1 GB (K7) w osobnym procesie
```

Pakiet zawiera `report.md` (tabela K1–K9 ze źródłem każdej liczby, macierz wymaganie → testy → wynik),
`evidence.json` (to samo w postaci maszynowej) i surowe wyjścia: `go test -json` modułu głównego i każdego modułu
narzędzi, testy Pythona, `tools/fetch -verify`. Stan „oczekuje” oznacza, że liczby nie da się dziś uzyskać albo
pomiar trzeba powtórzyć na commicie kandydata; nigdy nie jest liczony jako spełnienie. Narzędzie kończy się
statusem 1, gdy K4 albo K9 nie są spełnione. Zasady: `adr/0008-hardening.md`, pkt 6.

## Pomiary

| Data | Commit | K1: W3C (zaliczone / w zakresie) | K2: zapis powrotny | K3: drzewo | Uwagi |
|---|---|---|---|---|---|
| 2026-11-26 | etapy 0–1, drzewo robocze przed pierwszym commitem (źródło: `goxsdgo` `698affa`) | 1 725 / 1 727 (eduni 476/477, ibm 526/527, oasis 323/323, sun 101/101, xmltest 299/299) | — (etap 3) | — (etap 4) | `go test ./xmlreader -run TestXMLConformance -v`, Go 1.26.6, macOS arm64; ta sama liczba i lista co w `goxsdgo`; korpus: 13 dokumentów, `TestCorpus` zgodny ze wzorcami; wymagania: 25 przyjętych, wszystkie z testem |
| 2026-11-27 | etap 5 (w toku), `3d2dbc3`: Go 1.26, mniejsze drzewo (ADR 0006, pkt 15) | 1 725 / 1 727 (bez zmian) | bez zmian | bez zmian | K7: czytnik 1,23× i 1,20× czytnika `goxsdgo` v0.3.0, przelot 0,62 i 0,67 czytnika oraz 1,82× i 1,21× `encoding/xml`; 1 GB przelotu w 14 MB pamięci procesu; drzewo wobec `etree` i `xmlquery`: budowa 187 / 84 / 56 MB/s (JPK) i 94 / 73 / 45 MB/s (WordML), pamięć 4,8× / 4,3× / 6,7× i 7,2× / 6,5× / 8,4× dokumentu (`results/`); `FuzzTree` i pozostałe fuzzery: 40 min lokalnie na drzewie roboczym, 98 mln wykonań, bez błędu; kampania 24 h zlecona na Synology |
| 2026-11-27 | etap 5 (w toku), `445e58e`; kod biblioteki jak w v0.4.0 | 1 725 / 1 727 (bez zmian) | bez zmian | bez zmian | K7: czytnik 1,24× i 1,19× czytnika `goxsdgo` v0.3.0, przelot 0,62 i 0,67 czytnika oraz 1,84× i 1,21× `encoding/xml` (`results/reader-vs-goxsdgo.txt`); 1 GB przelotu w 14 MB pamięci procesu, szczyt sterty 1,2 MB (`results/streaming-memory.txt`); wyspa na wiersz: szczyt 1,2 MB niezależnie od rozmiaru; `TestDeterministic` (10 przebiegów, 4 podziały wejścia), `TestParallelUse`; drzewo wobec `etree` i `xmlquery`: budowa 189 / 84 / 57 MB/s (JPK) i 100 / 74 / 45 MB/s (WordML), pamięć 6,8× / 4,3× / 6,7× i 11,5× / 6,5× / 8,4× dokumentu, źle uformowane odrzucone 951 / 410 / 432 z 951 (`results/tree-vs-others.txt`, `results/tree-conformance.tsv`); fuzzing 40 min na czystym v0.4.0 bez błędu; pakiet dowodów: K1, K2, K4, K6, K9 spełnione, K3, K5, K7, K8 oczekują |
| 2026-11-27 | etap 4, `9b1b779` | 1 725 / 1 727 (bez zmian) | korpus 13 / 13, W3C 775 / 775 (bez zmian) | korpus 13 / 13, W3C 775 / 775 dokumentów przyjmowanych przez czytnik: `Parse` → `WriteTo` równe przelotowi co do bajta, `NewReader` równe strumieniowi źródła; zapadka `testdata/w3c/tree.ratchet`; 1 156 elementów jako poddrzewo w środku przelotu bez zmiany wyniku | `go test ./xmltree -run 'TestTreeRoundTrip|TestSubtreeInStream|TestTreeLinear' -v`; węzeł tylko przez metody (ADR 0006); pamięć drzewa: węzeł 72 B, element dodatkowo 136 B; 142 B na węzeł (styl JPK) i 250 B (styl WordML), 5,9× i 10,5× rozmiaru dokumentu; budowa 0,01 i 0,47 alokacji na węzeł, zapis 1 alokacja na dokument; budowa ok. 191 MB/s i zapis ok. 573 MB/s na `GenJPK(20000)` (benchmarki pakietu, maszyna obciążona: orientacyjnie); wymagania: 80 przyjętych, wszystkie z testem; `FuzzTree`: przebiegi sprawdzające lokalnie (do 90 s, 1,9 mln wykonań), dwa błędy znalezione i poprawione przed pomiarem (nie liczy się do K5); K6: `go test . -run 'TestHostileInputs|TestPathological|TestSafeByDefault|TestNoIOImports' -v`: 26 rodzin wrogich wejść i wywołań odrzuconych właściwym kodem, najdłuższe 0,18 s, szczyt pamięci do 48 MB; 15 rodzin patologicznych, czas przy podwojeniu rozmiaru 1,6–2,4×; nowy limit `MaxNamespaces` 256 (ADR 0007) |
| 2026-11-27 | etap 3, `247615e` | 1 725 / 1 727 (bez zmian) | korpus 13 / 13, W3C 775 / 775 dokumentów przyjmowanych przez czytnik; zapis idempotentny co do bajta; zapadka `testdata/w3c/roundtrip.ratchet` | — (etap 4) | `go test ./xmlwriter -run TestRoundTrip -v`; `encoding/xml` czyta wyjście pisarza tak samo w 767 z 789 dokumentów (13 odrzuca, 9 poza trybem przestrzeni nazw); przelot czytnik → pisarz 164 MB/s (JPK) i 83 MB/s (WordML), 1,83× i 1,20× przelotu `encoding/xml`, K7 czytnika 1,23× i 1,20× (`results/reader-vs-goxsdgo.txt`); czytnik bez pustych tokenów tekstu (`differences.md`); wymagania: 63 przyjęte, wszystkie z testem; `FuzzWriter` i `FuzzRoundTrip`: przebiegi sprawdzające po 40 s lokalnie (1,74 mln i 1,30 mln wykonań), jeden błąd znaleziony i poprawiony przed pomiarem (nie liczy się do K5) |
| 2026-11-26 | etap 2, `96c7949` | 1 725 / 1 727 (bez zmian; strumień 1 727 przypadków identyczny z v0.1.0, `TestTokenStreamStable`) | — (etap 3) | — (etap 4) | K7: 1,22× (JPK) i 1,19× (WordML) szybkości `goxsdgo` v0.3.0, `results/reader-vs-goxsdgo.txt`; test różnicowy z `goxsdgo` v0.3.0 na 3 381 plikach: 1 świadoma różnica (`differences.md`); wymagania: 37 przyjętych, wszystkie z testem; `FuzzReader` z nowymi niezmiennikami: przebieg sprawdzający 90 s lokalnie, 2,09 mln wykonań, bez błędu (nie liczy się do K5) |

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
