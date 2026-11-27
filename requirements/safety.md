# Wymagania: bezpieczeństwo

Status: przyjęte

Limity czytnika i encje zewnętrzne opisują XML-015 i XML-016 (przenoszone); tu są wymagania dla całego modułu.
SAFE-001…006 przyjęte 2026-11-27 (testy wrogich danych przygotowane na prośbę właściciela, przed etapem 5;
`../docs/adr/0007-hostile-inputs.md`). Zmiany wobec szkicu: SAFE-003 mierzy szczyt pamięci, nie sumę przydziałów,
i obejmuje pisarza oraz drzewo; doszła rodzina „deklaracje przestrzeni nazw w zakresie” z nowym limitem.

| Id | Wymaganie |
|---|---|
| SAFE-001 | Pakiety biblioteki nie importują `os`, `net`, `net/http`, `os/exec`, `syscall`, `unsafe` ani `plugin`, ani niczego spoza biblioteki standardowej i tego modułu; test sprawdza listę importów każdego pakietu (bez testów). Jedyne wejście i wyjście to `io.Reader` i `io.Writer` podane przez wywołującego. |
| SAFE-002 | Ustawienia domyślne są bezpieczne: zerowe `Options` każdego pakietu włączają wszystkie limity o wartościach domyślnych z dokumentacji (wyjątek: `MaxElements`, ADR 0004); wyłączenie limitu wymaga jawnej wartości ujemnej. Limity pisarza są równe limitom czytnika. |
| SAFE-003 | Wrogie wejścia są odrzucane błędem ze stałym kodem z rodzin `limit-*`, `wfc-*` albo `unsupported-*`, każde w < 1 s i przy szczycie pamięci < 64 MB ponad stan sprzed wywołania, przy ustawieniach domyślnych, przez czytnik i przez budowę drzewa: rozwijanie encji (wykładnicze i kwadratowe), encja odwołująca się do siebie, łańcuch zagnieżdżonych encji, encje zewnętrzne (nigdy nie czytane), zagnieżdżenie 10⁶ elementów, nazwa elementu i atrybutu 1 MB, wartość atrybutu, tekst, CDATA, komentarz i PI ponad limit, 10⁶ atrybutów i 10⁶ deklaracji przestrzeni nazw w elemencie, deklaracje przestrzeni nazw spiętrzone w zagnieżdżonych elementach, DOCTYPE ponad limit. Dla pisarza to samo dotyczy ciągów wywołań (zagnieżdżenie, atrybuty, nazwa, tekst w wielu wywołaniach, deklaracje). Wejścia generuje test. |
| SAFE-004 | Czas liniowy: dla każdej rodziny wejść patologicznych podwojenie rozmiaru zwiększa czas najwyżej 2,5× (czytnik, `Skip`, pisarz, budowa, kopia i zapis drzewa, usuwanie dzieci). Koszt zależny od liczby deklaracji przestrzeni nazw w zakresie jest ograniczony limitem `MaxNamespaces`. |
| SAFE-005 | Żadna funkcja publiczna nie panikuje dla żadnego wejścia ani ciągu wywołań, także po błędzie i po `Close`; błędne użycie API jest błędem ze stałym kodem. |
| SAFE-006 | Drzewo nie przekracza limitu węzłów także przy budowie z poddrzewa strumienia; zmiany w pamięci podlegają limitowi przy zapisie przez limity pisarza (WR-017). |

Status: szkic

| Id | Wymaganie |
|---|---|
| SAFE-007 | Dane prywatne nie trafiają do repozytorium ani do pakietu dowodów; narzędzia nie czytają katalogów spoza repozytorium bez jawnego parametru. |
