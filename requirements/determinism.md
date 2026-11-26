# Wymagania: powtarzalność i zasoby

Status: szkic

| Id | Wymaganie |
|---|---|
| DET-001 | Ten sam dokument i te same opcje dają ten sam strumień tokenów i błędów w każdym przebiegu; 10 przebiegów na korpusie daje identyczne zrzuty. |
| DET-002 | Ten sam ciąg wywołań pisarza i to samo drzewo dają te same bajty w każdym przebiegu, na każdym systemie i architekturze z macierzy CI; wzorce `*.out.xml` są wspólne dla wszystkich systemów. |
| DET-003 | Wynik nie zależy od kolejności iteracji po mapach, od czasu, od zmiennych środowiskowych, od ustawień regionalnych ani od sposobu podziału wejścia na odczyty: czytnik karmiony po jednym bajcie daje ten sam strumień co karmiony całością. |
| DET-004 | Wynik pisarza nie zależy od podziału zapisu na wywołania `Write` docelowego `io.Writer` ani od chwil wywołania `Flush`. |
| DET-005 | Pakiety są bezpieczne przy równoległym użyciu różnych egzemplarzy (brak stanu globalnego zmienianego po inicjalizacji); `go test -race` przechodzi. Jeden egzemplarz nie jest przeznaczony do użycia z wielu gorutyn i dokumentacja to mówi. |
| DET-006 | Pamięć czytnika i pisarza zależy od głębokości, najdłuższego tokenu i podzbioru wewnętrznego, nie od rozmiaru dokumentu; plik 1 GB przechodzi przez czytnik → pisarz w < 64 MB pamięci procesu. |
| DET-007 | Liczba alokacji czytnika, `Skip` i pisarza na dokumencie wzorcowym nie rośnie z jego rozmiarem (pomiar na dwóch rozmiarach różniących się 10×). |
| DET-008 | Szybkość czytnika nie spada poniżej 0,95× czytnika `goxsdgo` v0.3.0 na dokumencie wzorcowym, mierzona w tym samym przebiegu narzędzia `tools/bench`. |
