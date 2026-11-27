# Dziennik fuzzingu

Kryterium K5 (ADR 0001): fuzzery bez paniki i bez naruszenia niezmienników przez co najmniej 24 h łącznie na
kandydacie do wydania. Kampanię prowadzi `tools/fuzz` (przeniesione z `goxsdgo` w etapie 0 i dostosowane); każdy
przebieg kampanii dopisuje tu jeden wiersz.

## Gdzie wolno uruchamiać

1. **Synology „StormSyn”** (DS224+, x86_64, 4 rdzenie Celeron J4125): wybór domyślny. Na serwerze nie ma Go:
   binarki testów buduje się na Macu (`GOOS=linux GOARCH=amd64 go test -c`), kopiuje z `testdata/fuzz` i danymi
   startowymi do `~/fuzz/goxmlgo/<commit>/` i zleca przez wspólną kolejkę `~/fuzz/queue.sh` (jedna kampania naraz
   dla wszystkich projektów, `-test.parallel` najwyżej 4).
2. **Mac tylko w nocy (00:00–07:00) i na zasilaniu sieciowym**, z `-fuzztime` dobranym tak, żeby skończyć przed 07:00.
3. W pozostałych przypadkach kampania czeka w kolejce.

Lokalnie zawsze wolno: `go test` z wejściami regresji (bez `-fuzz`) oraz krótki przebieg sprawdzający poprawkę
(≤ 2 min, `-parallel 2`).

## Narzędzie

```sh
go -C tools/fuzz run . -root ../.. -list                       # fuzzery modułu
go -C tools/fuzz run . -root ../.. -pack ../../out/fuzzpack    # paczka dla serwera bez Go: out/fuzzpack/ i .tar.gz
go -C tools/fuzz run . -root ../.. -import journal.tsv         # wiersze dziennika serwera do tabeli „Przebiegi”
go -C tools/fuzz run . -root ../.. -duration 6h                # kampania lokalna (tylko w nocy, na zasilaniu)
```

Paczka (`-pack`, domyślnie linux/amd64) zawiera binarki testów z `go test -c` (`CGO_ENABLED=0`), wejścia regresji
`testdata/fuzz/*` pakietów, korpus `testdata/corpus` i pobrany zestaw XML Conformance (wejścia startowe
`FuzzReader`), w układzie katalogów repozytorium, oraz skrypt `run.sh` (sh, bez rozszerzeń bash). Paczkę buduje
się z czystego commitu kandydata: przebieg z paczki zbudowanej ze zmianami w drzewie nie liczy się do K5. Na serwerze:

```sh
tar czf - -C out/fuzzpack . | ssh <serwer> 'mkdir -p ~/fuzz/goxmlgo/<commit> && tar xzf - -C ~/fuzz/goxmlgo/<commit>'
ssh <serwer> 'nohup setsid ~/fuzz/queue.sh ~/fuzz/goxmlgo/<commit> >/dev/null 2>&1 &'
```

`queue.sh` to wspólna kolejka serwera (jedna kampania naraz dla wszystkich projektów, `nice -n 19`); `run.sh` dzieli
czas równo między fuzzery (domyślnie 1 425 min łącznie, workery: liczba procesorów − 1, najwyżej 4), pisze przebieg
do `run.log`, dziennik do `work/journal.tsv`, pełne wyjście do `work/logs/`. Znalezione wejście, które łamie
niezmiennik, leży w `tree/<pakiet>/testdata/fuzz/<Fuzzer>/` i wraca do repozytorium jako test regresji.

## Jak liczymy godziny do K5

- liczy się czas przebiegów zakończonych wynikiem „ok” na commicie kandydata albo na późniejszych commitach, które
  nie zmieniają kodu biblioteki poza testami (`git diff --stat <kandydat> -- ':!*_test.go' ':!testdata' ':!docs'
  ':!tools' ':!requirements' ':!*.md' ':!.github'` nic nie wypisuje);
- wiersz z dopiskiem „(zmiany)” to przebieg na kodzie z niezatwierdzonymi zmianami: nie liczy się;
- przebieg zakończony błędem zeruje licznik dla kodu, którego dotyczy;
- czas na wolniejszym sprzęcie liczymy jako czas zegarowy, z podaniem hosta.

## Fuzzery i niezmienniki

| Fuzzer | Pakiet | Niezmiennik | Od etapu |
|---|---|---|---|
| `FuzzReader` | `xmlreader` | bez paniki; czytanie się kończy; pozycje tokenów i błędów w granicach wejścia; błąd zawsze typu `*Error`. Od etapu 2: po błędzie stale ten sam błąd; strumień nie zależy od podziału wejścia na odczyty; czytnik po `Reset` daje ten sam strumień co nowy; czytanie ze `Skip` kończy się tak samo jak czytanie token po tokenie | 1, rozszerzenia od 2 |
| `FuzzRoundTrip` | `xmlwriter` | wejście przyjęte przez czytnik: zapis bez błędu, ponowny odczyt daje równy strumień tokenów, drugi zapis te same bajty | 3 |
| `FuzzWriter` | `xmlwriter` | dowolny ciąg wywołań (tryb jawny, automatyczny, bez przestrzeni nazw, z wcięciami, z małymi limitami): każdy błąd to `*Error` z kodem i numerem wywołania; same udane wywołania dają te same bajty; wyjście zamknięte bez błędu czytnik domyślny czyta bez błędu i oddaje te same nazwy rozwinięte, wartości i dane | 3 |
| `FuzzTree` | `xmltree` | wejście przyjęte przez czytnik: `Parse` → `WriteTo` daje bajty przelotu, `NewReader` strumień źródła, kopia głęboka te same bajty; wejście odrzucone przez czytnik odrzuca `Parse` z tym samym błędem. Potem ciąg operacji zmiany (dodanie, wstawienie, usunięcie, zastąpienie, atrybuty, tekst, kopia, deklaracje): wskaźniki na rodzica spójne, błąd operacji zawsze `*Error`; drzewo po zmianach albo daje błąd pisarza ze stałym kodem, albo dokument, który czytnik czyta i z którego odtwarza strumień czytnika drzewa; drugi zapis te same bajty | 4 |

## Przebiegi

| Data | Commit | Host i sprzęt | Fuzzer | Czas zegarowy | Wykonania | Wynik |
|---|---|---|---|---|---|---|
| 2026-11-26 | `96c7949` | Synology DS224+ (Celeron J4125, 4 rdzenie), linux/amd64, kolejka `~/fuzz/queue.sh` | `FuzzReader` | zlecone 23:37, nie rozpoczęte | — | wycofane z kolejki 2026-11-27 przed startem: etap 3 zmienił kod czytnika, więc przebieg nie liczyłby się do K5; zastąpione kampanią `247615e` |
| 2026-11-27 | `247615e` | Synology DS224+ (Celeron J4125, 4 rdzenie), linux/amd64, kolejka `~/fuzz/queue.sh` | `FuzzReader`, `FuzzRoundTrip`, `FuzzWriter` | zlecone 08:21, 3 × 475 min, 3 workery | — | nie rozpoczęte: awaria zasilania 2026-11-27 skasowała kolejkę; nie zlecone ponownie, bo commit nie ma drzewa ani późniejszych zmian czytnika |
| 2026-11-27 09:56 | `2b6c2ae` (zmiany) | Mac (Apple M4, 10 rdzeni), darwin/arm64, lokalnie na prośbę właściciela (Synology bez zasilania), 2 workery | `FuzzReader` | 10m2s | 11265187 | ok; przebieg z drzewa ze zmianami (stan tuż przed `9b1b779`), nie liczy się do K5 |
| 2026-11-27 10:06 | `2b6c2ae` (zmiany) | Mac (Apple M4, 10 rdzeni), darwin/arm64, lokalnie na prośbę właściciela (Synology bez zasilania), 2 workery | `FuzzTree` | 10m2s | 10713648 | ok; przebieg z drzewa ze zmianami (stan tuż przed `9b1b779`), nie liczy się do K5 |
| 2026-11-27 10:16 | `2b6c2ae` (zmiany) | Mac (Apple M4, 10 rdzeni), darwin/arm64, lokalnie na prośbę właściciela (Synology bez zasilania), 2 workery | `FuzzRoundTrip` | 10m3s | 13798644 | ok; przebieg z drzewa ze zmianami (stan tuż przed `9b1b779`), nie liczy się do K5 |
| 2026-11-27 10:26 | `2b6c2ae` (zmiany) | Mac (Apple M4, 10 rdzeni), darwin/arm64, lokalnie na prośbę właściciela (Synology bez zasilania), 2 workery | `FuzzWriter` | 10m3s | 12516702 | ok; przebieg z drzewa ze zmianami (stan tuż przed `9b1b779`), nie liczy się do K5 |
| 2026-11-27 11:03 | `521cbc7` | lokalnie, darwin/arm64, 10 CPU | `FuzzReader` (./xmlreader) | 10m2s | 12416118 | ok |
| 2026-11-27 11:13 | `521cbc7` | lokalnie, darwin/arm64, 10 CPU | `FuzzTree` (./xmltree) | 10m1s | 16665038 | ok |
| 2026-11-27 11:23 | `521cbc7` | lokalnie, darwin/arm64, 10 CPU | `FuzzRoundTrip` (./xmlwriter) | 10m1s | 27130983 | ok |
| 2026-11-27 11:33 | `521cbc7` | lokalnie, darwin/arm64, 10 CPU | `FuzzWriter` (./xmlwriter) | 10m2s | 20194191 | ok |
| 2026-11-27 12:46 | `1c39428` (zmiany) | lokalnie, darwin/arm64, 10 CPU | `FuzzReader` (./xmlreader) | 10m2s | 24142771 | ok |
| 2026-11-27 12:56 | `1c39428` (zmiany) | lokalnie, darwin/arm64, 10 CPU | `FuzzTree` (./xmltree) | 10m3s | 19088258 | ok |
| 2026-11-27 13:07 | `1c39428` (zmiany) | lokalnie, darwin/arm64, 10 CPU | `FuzzRoundTrip` (./xmlwriter) | 10m1s | 27983639 | ok |
| 2026-11-27 13:17 | `1c39428` (zmiany) | lokalnie, darwin/arm64, 10 CPU | `FuzzWriter` (./xmlwriter) | 10m2s | 26511160 | ok |
| 2026-11-27 | `3d2dbc3` | Synology DS224+ (Celeron J4125, 4 rdzenie), linux/amd64, kolejka `~/fuzz/queue.sh` | `FuzzReader`, `FuzzTree`, `FuzzRoundTrip`, `FuzzWriter` | zlecone 13:37, nie rozpoczęte | — | wycofane z kolejki 13:53 przed startem: usunięcie pliku `doc.go` pakietu głównego (sama dokumentacja) sprawiłoby, że czas nie liczyłby się dla późniejszych commitów; zastąpione kampanią `5913b81` |
| 2026-11-27 | `5913b81` | Synology DS224+ (Celeron J4125, 4 rdzenie), linux/amd64, kolejka `~/fuzz/queue.sh` | `FuzzReader`, `FuzzTree`, `FuzzRoundTrip`, `FuzzWriter` | zlecone 13:53, 4 × 356 min, 3 workery | — | w kolejce za kampanią `gozipgo`; wynik dopisze `tools/fuzz -import` |

## Znalezione błędy

| Data | Fuzzer | Błąd | Poprawka | Wejście regresji |
|---|---|---|---|---|
| 2026-11-27 | `FuzzWriter` (przebieg sprawdzający, lokalnie) | tryb automatyczny zapisywał deklarację przestrzeni nazw wziętej z `Name.Space` bez sprawdzenia znaków: `<a xmlns="\x0f">`, którego czytnik nie przyjmuje | nazwa przestrzeni deklarowanej przez pisarza jest sprawdzana jak podana jawnie (WR-012) | `xmlwriter/testdata/fuzz/FuzzWriter/2fc1d81895408ef0`; przypadki w `TestAutoNamespaces` |
| 2026-11-27 | `FuzzTree` (przebieg sprawdzający, lokalnie) | czytnik drzewa dawał token tekstu dla białych znaków na najwyższym poziomie dokumentu, którego czytnik po zapisie nie daje (strumień drzewa różny od strumienia zapisu) | czytnik drzewa pomija tekst na najwyższym poziomie; `WriteTo` nadal go zapisuje (ADR 0006, pkt 5) | `xmltree/testdata/fuzz/FuzzTree/d02a241bbb0f7fa8` |
| 2026-11-27 | `FuzzTree` (przebieg sprawdzający, lokalnie) | czytnik drzewa oddawał deklarację `xmlns:xml` ustawioną na elemencie, której czytnik po zapisie nie oddaje w tokenie | przyczyną był wyjątek w czytniku: od v0.4.0 czytnik oddaje deklarację `xmlns:xml` jak każdą inną, więc wraca ona z zapisu (ADR 0006, pkt 16) | `xmltree/testdata/fuzz/FuzzTree/002bfc93e9510fb2` |
