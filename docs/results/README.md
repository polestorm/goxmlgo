# Surowe wyniki pomiarów

Każdy plik to niezmieniony wynik przebiegu narzędzia z repozytorium, z datą, commitem, wersją Go, systemem
i maszyną w nagłówku albo w pliku `environment-*.json` obok. Liczby w README i w `../testing.md` dają się z tych
plików wyprowadzić; sposób jest w `../methodology.md`. Pliki zastępuje nowy przebieg tym samym narzędziem, nigdy
ręczna poprawka.

| Plik | Co zawiera | Polecenie |
|---|---|---|
| `xmlreader-conformance.tsv` | wynik każdego z 1 727 przypadków W3C XML Conformance w zakresie dla `xmlreader` i `encoding/xml` | `go test ./xmlreader -run TestStdlibConformance -v -stdlib-out ../docs/results/xmlreader-conformance.tsv` |
| `xmlreader-bench.txt` | szybkość i alokacje `xmlreader` i `encoding/xml` na dokumencie z `jpkLike`, 5 przebiegów | `go test ./xmlreader -run '^$' -bench 'BenchmarkJPK' -benchmem -count 5` (nagłówek z datą, commitem, wersją Go i systemem dopisany przy zapisie) |
| `reader-vs-goxsdgo.txt` | kryterium K7: szybkość i alokacje czytnika wobec `goxsdgo/xmlreader` v0.3.0 na dwóch dokumentach z generatorów, na przemian w jednym procesie; od etapu 3 także przelot czytnik → pisarz i przelot `encoding/xml` | `go -C tools/bench run . -root ../.. -out ../../docs/results/reader-vs-goxsdgo.txt` |
| `streaming-memory.txt` | kryterium K7: przelot 1 GB przez czytnik → pisarz, szczyt sterty i pamięć procesu | `GOXMLGO_STREAM_MB=1024 go test . -run '^TestStreamingMemory$' -count=1 -v` (nagłówek dopisany przy zapisie) |
| `tree-vs-others.txt` | drzewo `xmltree` obok `beevik/etree` i `antchfx/xmlquery`: budowa, zapis, alokacje, pamięć zajęta przez drzewo | `go -C tools/compare run . -root ../.. -out ../../docs/results/tree-vs-others.txt` |
| `tree-conformance.tsv` | ci sami kandydaci na 1 727 przypadkach W3C w zakresie: ile dobrze uformowanych przyjmują, ile źle uformowanych odrzucają | `go -C tools/compare test . -run TestCandidatesConformance -conformance-out ../../docs/results/tree-conformance.tsv` |
| `transfer-goxsdgo-tests.txt` | dowód 4 przeniesienia czytnika: testy `goxsdgo` przed zamianą importu na `goxmlgo` i po niej | opis w nagłówku pliku |

`reader-vs-goxsdgo.txt` z 2026-11-27 pochodzi z commitu etapu 3 (`247615e`). Od `e415926` narzędzie dopisuje też werdykt
progu przelotu (ADR 0005, pkt 13); kod biblioteki od `247615e` się nie zmienił, a przebieg na `e415926` odbył się na
obciążonej maszynie (liczby bezwzględne o ok. 20% niższe, stosunki 0,64 i 0,67 czytnika, 2,01× i 1,22× `encoding/xml`,
oba progi spełnione), więc zapisany zostaje przebieg z `247615e`. `xmlreader-bench.txt` pochodzi z drzewa po
etapie 2 sprzed pierwszego commitu.

`streaming-memory.txt`, `tree-vs-others.txt` i `tree-conformance.tsv` z 2026-11-27 powstały z drzewa roboczego
etapu 5 (kod biblioteki jak w v0.4.0) na maszynie obciążonej innymi zadaniami; po commicie etapu pomiary
powtarzamy tymi samymi poleceniami.
