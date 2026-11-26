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
  ':!tools'` nic nie wypisuje);
- wiersz z dopiskiem „(zmiany)” to przebieg na kodzie z niezatwierdzonymi zmianami: nie liczy się;
- przebieg zakończony błędem zeruje licznik dla kodu, którego dotyczy;
- czas na wolniejszym sprzęcie liczymy jako czas zegarowy, z podaniem hosta.

## Fuzzery i niezmienniki

| Fuzzer | Pakiet | Niezmiennik | Od etapu |
|---|---|---|---|
| `FuzzReader` | `xmlreader` | bez paniki; czytanie się kończy; pozycje tokenów i błędów w granicach wejścia; błąd zawsze typu `*Error`. Od etapu 2: po błędzie stale ten sam błąd; strumień nie zależy od podziału wejścia na odczyty; czytnik po `Reset` daje ten sam strumień co nowy; czytanie ze `Skip` kończy się tak samo jak czytanie token po tokenie | 1, rozszerzenia od 2 |
| `FuzzRoundTrip` | `xmlwriter` | wejście przyjęte przez czytnik: zapis bez błędu, ponowny odczyt daje równy strumień tokenów, drugi zapis te same bajty | 3 |
| `FuzzWriter` | `xmlwriter` | dowolny ciąg wywołań: błąd ze stałym kodem albo wyjście przyjęte przez czytnik z tymi samymi wartościami | 3 |
| `FuzzTree` | `xmltree` | drzewo → zapis równe strumień → zapis; ciąg operacji zmiany nie psuje wskaźników na rodzica ani dobrej formy zapisu | 4 |

## Przebiegi

| Data | Commit | Host i sprzęt | Fuzzer | Czas zegarowy | Wykonania | Wynik |
|---|---|---|---|---|---|---|
| — | — | — | — | — | — | kampanii jeszcze nie było: przed pierwszym commitem nie ma kandydata. Przebiegi `FuzzReader` w `goxsdgo` (jego `docs/fuzzing.md`) dotyczą tego samego kodu, ale do K5 tutaj się nie liczą |
