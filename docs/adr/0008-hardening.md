# 0008 Utwardzanie (etap 5)

Status: przyjęta (robocza: etap 5 nie jest zamknięty, brakuje kampanii fuzzingu 24 h i decyzji z pytań 18 i 19) · 2026-11-27

## Kontekst

Etap 5 planu: wzorce strumieniowe z przykładami, plik 1 GB w stałej pamięci (K7), wrogie wejścia (K6, zrobione
wcześniej: ADR 0007), fuzzing 24 h (K5), publikacja pomiarów i pakiet dowodów (K9). Kod biblioteki od v0.4.0 się
nie zmienia; etap dodaje testy, narzędzia i pomiary.

## Decyzja

1. **Wzorce strumieniowe mają testy i przykłady w pakiecie głównym** (`stream_test.go`): filtr (`Skip`, zmiana
   atrybutu i tekstu, dopisanie elementu) i wyspa (`ReadElement` → zmiana → `WriteTo` w środku przelotu).
   Wymagania STR-004 i STR-005 przyjęte. Przykłady `Example_filter` i `Example_island` pokazuje pkg.go.dev
   w dokumentacji modułu.
2. **Pamięć strumienia mierzymy dwiema liczbami**: szczytem sterty ponad stan sprzed przebiegu (próbkowanie co
   200 µs) i pamięcią pobraną od systemu przez cały proces testu. Dokument powstaje w generatorze, więc nie
   zajmuje pamięci ani dysku. W każdym `go test` test biegnie na 100 MB z granicą 8 MB sterty; pomiar K7 (1 GB,
   granica 64 MB dla procesu) uruchamia zmienna `GOXMLGO_STREAM_MB=1024`, a surowy wynik trafia do
   `docs/results/streaming-memory.txt`. DET-006 przyjęte.
3. **Powtarzalność (DET-001, DET-003, DET-005)**: 10 przebiegów każdego dokumentu korpusu i wejść z błędem daje
   ten sam strumień, zapis i drzewo; wynik nie zależy od podziału wejścia (po bajcie, po 7 bajtów, połówkami);
   osiem gorutyn z osobnymi egzemplarzami daje ten sam wynik pod detektorem wyścigów. Brak zależności od czasu,
   losowości i środowiska wynika z listy importów: `TestNoIOImports` zakazuje także `time`, `math/rand`
   i `crypto/rand`.
4. **DET-008 (czytnik ≥ 0,95× czytnika `goxsdgo` v0.3.0)** mierzy narzędzie `tools/bench`; test
   `TestRecordedResult` pilnuje, żeby zapisany wynik spełniał progi i pochodził z commitu, a nie z drzewa ze
   zmianami. Pomiaru czasu nie robimy w `go test`: na wspólnych maszynach CI dawałby fałszywe alarmy.
5. **Porównanie drzewa z innymi bibliotekami Go jest osobnym modułem `tools/compare`** (pytanie 16): `beevik/etree`
   i `antchfx/xmlquery` jako czarne skrzynki, na dokumentach z generatorów i na zestawie W3C. Osobny moduł, bo
   zależności kandydatów wymagają Go 1.23, a `tools/bench` ma działać na Go 1.22 z macierzy CI. Obok szybkości
   publikujemy to, co kandydaci sprawdzają, i to, w czym przegrywamy (pamięć).
6. **Pakiet dowodów (`tools/evidence`, K9)** przeniesiony z `goxsdgo` i dostosowany; wymagania EVD-001…005
   przyjęte. Zasady, które narzędzie stosuje:
   - kryterium, którego liczby nie da się uzyskać, ma stan „oczekuje”, nigdy „spełnione”. Dziś dotyczy to K3
     (zastąpienie drzew konsumentów mierzy etap 6), K5 (40 minut z 24 godzin), K7 (pomiary do powtórzenia na
     commicie kandydata) i K8 (macierz CI i konsumenci nie mają źródła w repozytorium);
   - wyniki spoza `go test` narzędzie czyta z `docs/results/` (nagłówki `# date:` i `# goxmlgo:`, wiersze
     `# ratio` i `# pass-through`, wiersz „pass-through of N MB”), więc te formaty są umową; pomiary etapu 6
     (K3 u konsumentów, K8) trafią do opcjonalnego `docs/measurements.json`;
   - przebiegi testów dostają `GOPROXY=off` i `GOTOOLCHAIN=local` (bez sieci), więc narzędzie trzeba uruchamiać
     wersją Go, która buduje wszystkie moduły narzędzi (`tools/compare` wymaga Go 1.23);
   - pliki nieśledzone liczą się jako drzewo ze zmianami; nieudany przebieg fuzzingu na kodzie kandydata zeruje
     godziny K5;
   - pomiar 1 GB (`-stream-mb 1024`) biegnie w osobnym procesie testu, bo pamięć procesu w pełnym zestawie
     zawyżają inne testy.
7. **Limit węzłów drzewa zostaje na 2 000 000** (pytanie 18, do potwierdzenia): to wartość z `goxadesgo`,
   a dokumentacja `Options.MaxNodes` mówi teraz wprost, że limit ogranicza pamięć tylko przez liczbę węzłów
   i że dla danych niezaufanych trzeba go obniżyć. Koszt prefiksów automatycznych pisarza ogranicza
   `MaxNamespaces` (256); bez zmian.

8. **Najniższa wersja Go to 1.26** (pytanie 11, decyzja właściciela z 2026-11-27, od v0.5.0). Wszystkie moduły
   repozytorium deklarują `go 1.26`, a CI ma jedną wersję. Powód podany w pkt 5 dla osobnego modułu
   `tools/compare` przestaje obowiązywać; moduł zostaje osobny, bo ma zależności, których `tools/bench` nie
   potrzebuje. Moduł, który zależy od `goxmlgo`, musi deklarować co najmniej tę samą wersję.

## Pomiary (2026-11-27, commit `445e58e`; Apple M4, Go 1.26.6)

- K7, przelot 1 GB: 1030 MB wejścia, szczyt sterty 1,2 MB, pamięć procesu 14 MB
  (`../results/streaming-memory.txt`).
- Wyspa na każdym wierszu dokumentu: szczyt sterty 1,2 MB dla 20 tys. i dla 80 tys. wierszy (`TestIsland`).
- Drzewo wobec innych bibliotek (`../results/tree-vs-others.txt`, `../results/tree-conformance.tsv`):

  | | `xmltree` | `etree` | `xmlquery` |
  |---|---|---|---|
  | budowa, styl JPK | 189 MB/s | 84 MB/s | 57 MB/s |
  | budowa, styl WordML | 100 MB/s | 74 MB/s | 45 MB/s |
  | zapis, styl JPK / WordML | 565 / 327 MB/s | 570 / 297 MB/s | 219 / 115 MB/s |
  | alokacje budowy, styl JPK | 1 557 | 1 660 102 | 2 980 122 |
  | pamięć drzewa, styl JPK / WordML | 6,8× / 11,5× dokumentu | 4,3× / 6,5× | 6,7× / 8,4× |
  | dobrze uformowane przyjęte (z 776) | 775 | 712 | 711 |
  | źle uformowane odrzucone (z 951) | 951 | 410 | 432 |

  Pierwszy przebieg, z drzewa roboczego na obciążonej maszynie, dał liczby bezwzględne o ok. 40% niższe przy tych
  samych stosunkach między kandydatami.
- K7 czytnika i przelotu na tym samym commicie (`../results/reader-vs-goxsdgo.txt`): czytnik 1,24× i 1,19×
  czytnika `goxsdgo` v0.3.0; przelot 158 i 82 MB/s, czyli 0,62 i 0,67 czytnika oraz 1,84× i 1,21× `encoding/xml`.
- Fuzzing: 40 minut na czystym commicie v0.4.0, 76 mln wykonań, bez błędu (`../fuzzing.md`); do K5 brakuje
  ok. 23 godzin.

## Konsekwencje

+ K6 i K7 mają testy i zapisane pomiary; wzorce strumieniowe mają przykłady;
+ porównanie z innymi jest powtarzalne i pokazuje także to, w czym przegrywamy;
− drzewo zajmuje więcej pamięci niż `etree` (pytanie 19: co z tym zrobić);
− K5 (24 h fuzzingu) i K8 (konsumenci) zostają otwarte.
