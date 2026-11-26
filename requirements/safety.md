# Wymagania: bezpieczeństwo

Status: szkic

Limity czytnika i encje zewnętrzne opisują XML-015 i XML-016 (przenoszone); tu są wymagania dla całego modułu.

| Id | Wymaganie |
|---|---|
| SAFE-001 | Pakiety biblioteki nie importują `os`, `net`, `net/http`, `os/exec`, `syscall`, `unsafe` ani `plugin`; test sprawdza listę importów każdego pakietu (bez testów). Jedyne wejście i wyjście to `io.Reader` i `io.Writer` podane przez wywołującego. |
| SAFE-002 | Ustawienia domyślne są bezpieczne: zerowe `Options` każdego pakietu włączają wszystkie limity o wartościach domyślnych z dokumentacji; wyłączenie limitu wymaga jawnej wartości ujemnej. |
| SAFE-003 | Wrogie wejścia są odrzucane błędem ze stałym kodem `limit-*` albo `wfc-*`, każde w < 1 s i < 64 MB: rozwijanie encji (wykładnicze i kwadratowe), zagnieżdżenie 10⁶ elementów, nazwa i wartość atrybutu ponad limit, 10⁶ atrybutów w elemencie, 10⁶ deklaracji przestrzeni nazw, token tekstu ponad limit, DOCTYPE ponad limit. Wejścia generuje test. |
| SAFE-004 | Czas liniowy: dla każdej rodziny wejść patologicznych podwojenie rozmiaru zwiększa czas najwyżej 2,5× (czytnik, `Skip`, pisarz, budowa i zapis drzewa). |
| SAFE-005 | Żadna funkcja publiczna nie panikuje dla żadnego wejścia ani ciągu wywołań, także po błędzie i po `Close`; błędne użycie API jest błędem ze stałym kodem. |
| SAFE-006 | Drzewo nie przekracza limitu węzłów także przy budowie z poddrzewa strumienia; zmiany w pamięci podlegają limitowi przy zapisie przez limity pisarza (WR-017). |
| SAFE-007 | Dane prywatne nie trafiają do repozytorium ani do pakietu dowodów; narzędzia nie czytają katalogów spoza repozytorium bez jawnego parametru. |
