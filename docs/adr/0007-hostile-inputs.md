# ADR 0007: wrogie dane i limit deklaracji przestrzeni nazw w zakresie

Data: 2026-11-27. Status: przyjęty (limit z pkt 1 potwierdził właściciel, z wartością domyślną 256).

## Kontekst

Testy wrogich danych (kryterium K6, wymagania SAFE) były planowane na etap 5. Przygotowane wcześniej, na prośbę
właściciela, od razu znalazły wejście, które mieści się we wszystkich limitach domyślnych, a spowalnia czytnik
ok. 150 razy: 500 zagnieżdżonych elementów po 4000 deklaracji przestrzeni nazw daje 2 mln deklaracji w zakresie,
a każde rozwiązanie prefiksu przegląda je od końca. 20 tys. elementów `<p0:a/>` czytało się 24 s (1,6 MB/s zamiast
ok. 250 MB/s), a koszt rośnie z liczbą takich elementów bez ograniczenia. Zachowanie pochodzi z czytnika
przeniesionego z `goxsdgo` (ten sam kod w v0.1.0…v0.3.0).

## Decyzja

1. **Nowy limit `Limits.MaxNamespaces`**: liczba deklaracji przestrzeni nazw w zakresie naraz, łącznie we
   wszystkich otwartych elementach; domyślnie 256, zero to wartość domyślna, wartość ujemna brak limitu; kod
   `limit-namespaces` (XML-031). Ten sam limit ma pisarz (WR-020). Rozważona alternatywa: indeks prefiks → stos
   deklaracji, czyli wyszukiwanie w stałym czasie bez limitu. Odrzucona na teraz: zmienia gorącą ścieżkę czytnika
   (każdy znacznik początku) i wymaga tego samego w pisarzu, typie `NamespaceScope` i drzewie, a limit to jeden
   licznik, który łatwo sprawdzić. Wartość 256: dokumenty spotykane w praktyce mają w zakresie zwykle do
   kilkunastu deklaracji, a element główny WordprocessingML ok. trzydziestu; limit liczy deklaracje wszystkich
   otwartych elementów, więc zostaje zapas na generatory, które powtarzają deklaracje na kolejnych poziomach.
   Pierwsza wersja miała 1024; właściciel uznał to za zbyt wiele.
2. **Skutek dla zgodności**: dokument z więcej niż 256 deklaracjami w zakresie, przyjmowany do v0.3.0, jest
   teraz odrzucany przy ustawieniach domyślnych. Żaden dokument zestawu W3C ani korpusu nie przekracza limitu
   (strumień 1 727 przypadków bez zmian, `TestTokenStreamStable`). Wpis w `../differences.md` i w CHANGELOG.
3. **Miara pamięci w SAFE-003 to szczyt sterty** ponad stan sprzed wywołania, nie suma przydziałów: bufor tekstu
   rośnie przez podwajanie, więc suma dla tokenu na granicy limitu 16 MiB to ok. 95 MB, a szczyt 48 MB.
4. **Wymagania SAFE-001…006 są przyjęte** i mają testy w pakiecie głównym (`safety_test.go`): `TestNoIOImports`,
   `TestSafeByDefault`, `TestHostileInputs`, `TestPathological`.

## Pomiary (2026-11-27, drzewo robocze na `2b6c2ae`; Apple M4, w tle biegł fuzzing na dwóch rdzeniach)

- `TestHostileInputs`: 18 rodzin wejść przez czytnik i przez `xmltree.Parse`, 2 przez limit węzłów drzewa, 6 ciągów
  wywołań pisarza. Każde odrzucone właściwym kodem; najdłuższe 0,18 s (wartość atrybutu 17 MB), największy szczyt
  pamięci 48 MB (tokeny na granicy limitu tekstu 16 MiB).
- `TestPathological`: 15 rodzin (czytnik 9, pisarz 3, drzewo 3), stosunek czasu przy podwojeniu rozmiaru od 1,83
  do 2,37 (próg 2,5).
- Wejście z kontekstu po zmianie: odrzucone kodem `limit-namespaces` w 23 ms.

## Co zostaje do etapu 5

- **Pamięć drzewa przy limicie domyślnym.** Limit węzłów (2 mln) nie ogranicza pamięci do 64 MB: element `<a/>`
  to 4 bajty wejścia i 208 bajtów drzewa, więc 8 MB wejścia daje ok. 420 MB. To cecha drzewa (trzyma dokument),
  ale wartość domyślna limitu albo dodatkowy limit bajtów wymaga decyzji.
- **Prefiksy automatyczne pisarza.** Wybór wolnego prefiksu `nsN` przegląda deklaracje w zakresie dla każdego
  kandydata; przy 100 prefiksach automatycznych na element to ok. 4 µs na atrybut, a koszt rośnie z kwadratem
  ich liczby w znaczniku. Ogranicza go `MaxNamespaces` (256). Dotyczy tylko trybu `AutoNamespaces` i nazw bez prefiksu podanego przez wywołującego.
- Plik 1 GB w stałej pamięci (K7), kampania fuzzingu 24 h (K5), pakiet dowodów (K9).

## Konsekwencje

+ wejście mieszczące się w limitach nie może już spowolnić czytnika i pisarza więcej niż o stały czynnik;
+ SAFE-001…006 mają testy, które biegną w zwykłym `go test ./...`;
− zmiana zachowania czytnika wobec v0.3.0 (pkt 2);
− `TestPathological` mierzy czas, więc na bardzo obciążonej maszynie może dać fałszywy alarm (najmniejszy z siedmiu
  przebiegów, próg 2,5; `go test -short` go pomija).
