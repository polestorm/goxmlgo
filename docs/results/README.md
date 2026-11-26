# Surowe wyniki pomiarów

Każdy plik to niezmieniony wynik przebiegu narzędzia z repozytorium, z datą, commitem, wersją Go, systemem
i maszyną w nagłówku albo w pliku `environment-*.json` obok. Liczby w README i w `../testing.md` dają się z tych
plików wyprowadzić; sposób jest w `../methodology.md`. Pliki zastępuje nowy przebieg tym samym narzędziem, nigdy
ręczna poprawka.

| Plik | Co zawiera | Polecenie |
|---|---|---|
| `xmlreader-conformance.tsv` | wynik każdego z 1 727 przypadków W3C XML Conformance w zakresie dla `xmlreader` i `encoding/xml` | `go test ./xmlreader -run TestStdlibConformance -v -stdlib-out ../docs/results/xmlreader-conformance.tsv` |
| `xmlreader-bench.txt` | szybkość i alokacje `xmlreader` i `encoding/xml` na dokumencie z `jpkLike`, 5 przebiegów | `go test ./xmlreader -run '^$' -bench 'BenchmarkJPK' -benchmem -count 5` (nagłówek z datą, commitem, wersją Go i systemem dopisany przy zapisie) |
| `transfer-goxsdgo-tests.txt` | dowód 4 przeniesienia czytnika: testy `goxsdgo` przed zamianą importu na `goxmlgo` i po niej | opis w nagłówku pliku |

Pliki z 2026-11-26 powstały z drzewa roboczego przed pierwszym commitem repozytorium, więc w nagłówku zamiast
commitu `goxmlgo` mają commit źródłowy czytnika. Przy wydaniu v0.1.0 pomiar szybkości powtarzamy z commitu wydania.
