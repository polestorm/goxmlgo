# 0004 Rozszerzenia czytnika (etap 2)

Status: przyjęta (robocza, do potwierdzenia przez właściciela przy wydaniu v0.2.0) · 2026-11-26

## Kontekst

Etap 2 dodaje do czytnika to, czego walidacja XSD nie potrzebowała, a potrzebują inni odbiorcy (`design.md`,
wymagania XML-019…029): pomijanie poddrzewa, ponowne użycie czytnika, limit liczby elementów, wspólny interfejs
źródła tokenów, deklaracje przestrzeni nazw w tokenie (ADR 0002), treść deklaracji XML i gwarancję trwałości
napisów. Wszystko ma być dodatkiem: program napisany dla v0.1.0 kompiluje się i działa tak samo. Kilka rzeczy
plan zostawił do decyzji w etapie.

## Decyzja

1. **Deklaracje w tokenie: pole `Token.Namespaces []Binding`.** Wycinek należy do czytnika (bufor używany
   ponownie, bez alokacji na token), jest kopią wewnętrznego stosu deklaracji, więc wywołujący nie może nim zepsuć
   rozwiązywania nazw. Element bez deklaracji ma `nil`. Jawna deklaracja `xmlns:xml` nie jest w nim wymieniana,
   tak samo jak w `DeclaredNamespaces` (prefiks `xml` jest związany zawsze).
2. **Zakres poza czytnikiem: typ `NamespaceScope`** w pakiecie `xmlreader` (obok `Token` i `Binding`; osobny pakiet
   na typy wspólne zostaje pytaniem 7). Metody `Update(Token)`, `Push`, `Pop`, `Lookup`, `Depth`, `Reset`. `Lookup`
   ma tę samą sygnaturę i te same odpowiedzi co `Reader.LookupNamespace`. Wyszukiwanie prefiksu dla przestrzeni
   i lista przestrzeni w zakresie dojdą w etapie 3, gdy pisarz pokaże, czego potrzebuje.
3. **`Skip` jako metoda czytnika, dozwolona tylko tuż po tokenie początku elementu.** Sprawdza treść tak samo jak
   czytanie (ta sama ścieżka kodu z flagą), nie tworzy napisów wartości atrybutów. W złym stanie zwraca
   `*Error` z nowym kodem `usage-skip` i niczego nie zmienia; to pierwsza rodzina kodów, które nie mówią
   o dokumencie i nie są krytyczne (`usage-*`). Tekst pomijanych tokenów nadal przechodzi przez bufor tekstu:
   limit `limit-text-size` musi dać ten sam błąd co przy czytaniu, a bufor i tak jest ograniczony najdłuższym
   tokenem.
4. **`Reset(r, opts)`**, także na zerowej wartości `Reader`; `New` jest teraz `Reset` na nowej wartości, więc
   oba mają jedną ścieżkę inicjalizacji. Zachowane są bufory i tablice zapamiętanych nazw (czyszczone, gdy się
   zapełnią). Wycinek notacji nie jest używany ponownie, bo `Notations` oddaje go wywołującemu.
5. **`Limits.MaxElements` domyślnie bez limitu**; zero i wartości ujemne znaczą to samo. To jedyny limit, dla
   którego zero nie włącza ochrony: czytnik strumieniowy nie trzyma niczego na element, a limit służy odbiorcom,
   którzy coś na element budują (drzewo ma własny limit węzłów, etap 4). Kod `limit-elements`, miejsce: znacznik
   początku elementu ponad limit.
6. **Deklaracja XML jako metoda `Declaration() (Declaration, error)`, nie token.** Token zmieniłby strumień
   widziany przez programy pisane dla v0.1.0 (XML-029). Metodę wolno wywołać przed pierwszym `Next`: czyta wtedy
   początek dokumentu, a błąd deklaracji zostaje błędem czytnika. Dzięki temu pisarz może zapisać deklarację przed
   pierwszym tokenem. Łańcuch, który nie ma dostępu do czytnika (drzewo, filtr), deklaracji nie widzi i przekazuje
   ją osobno; to świadome ograniczenie, do oceny w etapie 3.
7. **Trwałość:** napisy w tokenie są trwałe, wycinki nie. Zapisane w dokumentacji `Token` i pakietu, z testem.
8. **Tokeny wewnątrz czytnika przez wskaźnik.** Nowe pole powiększyło `Token`, a funkcje wewnętrzne zwracały go
   przez wartość na czterech poziomach; pierwszy pomiar K7 dał 0,947× na dokumencie JPK (próg 0,95). Funkcje
   wewnętrzne wypełniają teraz token wskazany przez `Next`. API bez zmian; po zmianie czytnik ma 1,22× i 1,19×
   szybkości `goxsdgo` v0.3.0 (`results/reader-vs-goxsdgo.txt`).
9. **Poprawki błędów mimo zasady „bez zmiany zachowania”.** Test czytania porcjami wykrył w kodzie przeniesionym
   z `goxsdgo` panikę i fałszywe błędy przy deklaracji XML oraz kod błędu zależny od granicy bufora
   (`differences.md`). Poprawione tutaj, z testami regresji; strumień dla 1 727 przypadków W3C w zakresie
   i korpusu jest bez zmian (`TestTokenStreamStable`, `TestSameAsGoxsdgo`). Odczyt porcjami wszedł też do
   niezmienników `FuzzReader`, który dotąd podawał wejście jednym odczytem i dlatego tych błędów nie widział.
10. **`tools/bench` powstał teraz** (ADR 0003, pkt 6): osobny moduł z `goxsdgo` v0.3.0 jako zależnością pomiaru.
    Zależność narzędzia nie narusza reguły „zero zależności” modułu biblioteki.
11. **Komentarz `Token.EndLine`** nie wspomina już `goxsdgo` i libxml2 (zapowiedziane w ADR 0003).

## Konsekwencje

+ `godocxgo` ma to, czego potrzebuje jego etap 1 (`Skip`, `Reset`, `Limits.MaxElements`);
+ pisarz i drzewo mogą pracować na samych tokenach;
+ czytnik jest szybszy niż przed rozszerzeniami i odporny na dowolny podział wejścia;
− wydane `goxsdgo` v0.3.0 i v0.4.0 mają błąd deklaracji XML; decyzją właściciela poprawkę przeniesiono do
  zamrożonej kopii czytnika jako wyjątek (`goxsdgo` commit `140b2de`, wydanie v0.4.1 z 2026-11-26);
− `Token` urósł o 24 bajty; wywołujący, którzy trzymają tokeny w tablicach, zużyją więcej pamięci.
