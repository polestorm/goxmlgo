# Wymagania: API i zgodność

## Etapy 0 i 1

Status: przyjęte

| Id | Wymaganie |
|---|---|
| API-001 | Publiczne API każdego pakietu jest zapisane we wzorcu `api/<pakiet>.txt` (deklaracje eksportowane, w ustalonej kolejności); `TestAPISnapshot` porównuje wzorzec z kodem. Zmiana wzorca wymaga `-update` i wpisu w ADR albo w `CHANGELOG.md`. |
| API-002 | `goxmlgo/xmlreader` w v0.1.0 jest źródłowo zgodny z `goxsdgo/xmlreader` v0.3.0: wzorzec API różni się tylko ścieżką pakietu, a program testowy napisany dla v0.3.0 kompiluje się po zamianie importu i daje ten sam wynik. |
| API-004 | Kody błędów wszystkich pakietów są stałymi eksportowanymi, opisanymi w `docs/errors.md`; test sprawdza, że każdy kod z kodu źródłowego jest w katalogu i odwrotnie. Kod raz wydany nie zmienia znaczenia i nie znika. |
| API-007 | Moduł główny nie ma sekcji `require` i buduje się z `CGO_ENABLED=0`; najniższa wersja Go z `go.mod` jest w macierzy CI. |

## Kolejne etapy

Status: szkic

| Id | Wymaganie |
|---|---|
| API-003 | W wersjach 0.x po v0.1.0 API czytnika tylko rośnie: usunięcie albo zmiana sygnatury wymaga ADR. Od 1.0.0 obowiązuje zgodność wsteczna w obrębie wersji głównej. |
| API-005 | Komunikaty błędów są po angielsku i nie są częścią kontraktu; kontraktem jest kod, pozycja i parametry. |
| API-006 | Każdy eksportowany identyfikator ma komentarz dokumentacyjny po angielsku; każdy pakiet ma `Example*` dla głównej drogi użycia, sprawdzany przez `go test`. |
