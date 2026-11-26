# 0003 Przeniesienie czytnika z goxsdgo

Status: przyjęta · 2026-11-26 (wykonanie etapu 1; decyzje o zakresie czytnika pochodzą z ADR 0004 w `goxsdgo`)

## Kontekst

ADR 0001 (pkt 3) postanawia, że czytnik przechodzi z `goxsdgo` bez zmiany zachowania. Decyzje o tym, co czytnik robi
i czego nie robi, zapadły w `goxsdgo` (ADR 0004 tamże, 2026-11-25, z uzupełnieniem z 2026-11-26 o pakiecie
publicznym). Ten plik je streszcza, żeby `goxmlgo` nie zależało od dokumentów innego repozytorium, i zapisuje
decyzje podjęte przy samym przeniesieniu. ADR źródłowy zostaje w `goxsdgo` jako historia.

## Decyzje przejęte (streszczenie ADR 0004 z `goxsdgo`)

1. **Model.** Czytnik strumieniowy typu pull: `New(r, Options)`, `Next()` zwraca tokeny (początek i koniec elementu
   z nazwą rozwiniętą i atrybutami, tekst z oznaczeniem CDATA i „same białe znaki”, PI, komentarz, DOCTYPE).
   `LookupNamespace` i `DeclaredNamespaces` podają przestrzenie nazw, `UnparsedEntity` i `Notations` deklaracje z DTD.
2. **Pełna dobra forma i przestrzenie nazw.** Każde naruszenie jest błędem krytycznym (XML 1.0, 1.2): `Next` zwraca
   `*Error` ze stałym kodem (`wfc-*`, `nsc-*` według nazw ograniczeń rekomendacji, `xml-*`, `ns-*`, `limit-*`,
   `unsupported-*`), wierszem i kolumną, a potem stale ten sam błąd. `Options.NoNamespaces` służy testom zgodności
   czystego XML 1.0.
3. **Pozycja.** Wiersz i kolumna od 1, po normalizacji końców wierszy; kolumna liczy znaki (punkty kodowe), nie
   bajty, więc ten sam dokument w UTF-8 i UTF-16 ma te same pozycje. Błąd w tekście zastępczym encji wskazuje
   miejsce odwołania do niej w dokumencie.
4. **Kodowania:** UTF-8 (z BOM i bez), UTF-16 LE i BE z BOM, ISO-8859-1 i US-ASCII według deklaracji. Inne dają
   `xml-encoding-unsupported`; UTF-16 bez BOM i deklaracja sprzeczna z BOM są błędami (4.3.3). Dalsze kodowania:
   pytanie 5 w `open-questions.md`.
5. **DTD.** Podzbiór wewnętrzny jest sprawdzany składniowo w całości. Czytnik robi to, czego rekomendacja wymaga
   od procesora bez walidacji (5.1): rozwija encje wewnętrzne, dodaje atrybuty z wartości domyślnych
   (`Specified=false`), normalizuje atrybuty według typów z ATTLIST, zapamiętuje notacje. Domyślne deklaracje
   przestrzeni nazw z ATTLIST działają jak podane w znaczniku. Po odwołaniu do encji parametrycznej, której nie
   czyta, nie przetwarza dalszych deklaracji encji i list atrybutów, chyba że `standalone="yes"`. Sekcje warunkowe
   w tekście encji parametrycznej: `unsupported-conditional-section`. Instrukcji przetwarzania
   z wnętrza DOCTYPE nie podaje jako tokenów (stąd porażka `ibm-valid-P29-ibm29v01.xml`).
6. **Encje zewnętrzne nigdy nie są czytane**, tak samo podzbiór zewnętrzny; czytnik nie ma dostępu do plików ani
   sieci. Odwołanie do zewnętrznej encji ogólnej w treści daje `unsupported-external-entity` zamiast pominięcia
   treści, które rekomendacja dopuszcza: pominięta treść zmieniłaby wynik odbiorcy bez śladu. W wartości atrybutu
   to naruszenie WFC No External Entity References. Odwołanie do niezadeklarowanej encji, gdy WFC Entity Declared
   nie obowiązuje, daje `unsupported-entity-unresolved` (stąd porażka `rmt-e3e-13`).
7. **Limity z bezpiecznymi wartościami domyślnymi** (`DefaultLimits`; zero to wartość domyślna, wartość ujemna
   brak limitu): głębokość 512, nazwa 16 KiB, 4 096 atrybutów w elemencie, token tekstu, komentarza, PI albo
   wartość atrybutu 16 MiB, łączne rozwinięcia encji 4 MiB, zagnieżdżenie encji 32, DOCTYPE 1 MiB.
8. **Strumieniowo.** Bufor wejścia 64 KiB; pamięć zależy od najdłuższego tokenu, głębokości i podzbioru
   wewnętrznego, nie od rozmiaru dokumentu. Kolejne dane znakowe (także z encji) łączą się w jeden token tekstu,
   sekcja CDATA jest osobnym tokenem.

## Decyzje przy przeniesieniu

1. **Commit źródłowy: `698affa`** (HEAD `goxsdgo` w dniu przeniesienia, drzewo czyste). Plan podawał `cfb54cf`
   (v0.3.0); pliki czytnika i `xmlchars` są w obu commitach identyczne co do bajta, więc zgodność API i zachowania
   odnosi się do v0.3.0, a `provenance.md` podaje commit, z którego faktycznie wzięto pliki.
2. **Zmiany w kodzie czytnika:** ścieżka importu `xmlchars`, komentarz przy tym imporcie i ostatnie zdanie
   komentarza pakietu. Komentarz pola `Token.EndLine`, który wspomina `goxsdgo` i libxml2, został bez zmian, bo
   plan dopuszcza tylko zmianę komentarza pakietu; do poprawienia w etapie 2 razem z innymi zmianami dokumentacji API.
3. **`xmlchars` jest pakietem publicznym** z tym samym API co `goxsdgo/internal/xmlchars` (`Edition`, `Edition2`,
   `Edition5`, `NameStart5`, `NameCharExtra5`, `IsNameStart5`, `IsNameChar5`); komentarze przetłumaczone na
   angielski. Funkcje dla [2] Char i [3] S oraz sprawdzanie nazw na napisach (szkic CH-001, CH-002) nie weszły:
   to dodatki do API, a etap 1 niczego nie dodaje. Zostają jako szkice CH-002 i CH-005 na etap 2 albo 3 (pisarz
   ich potrzebuje).
4. **Testy:** przeniesione bez zmian poza ścieżkami importu, katalogami wejść startowych fuzzera (tu nie ma danych
   MF) i dwoma odwołaniami `REQ:` do wymagań, które w `goxmlgo` mają inne identyfikatory (`DT-019` → `CH-001`,
   `DET-001` → `SAFE-004`). Nowe są `TestCorpus` i `TestGeneratedDocuments` (korpus etapu 0).
5. **Status wymagań jest na tabelę, nie na plik:** `xml.md` ma część przyjętą (001…018) i szkic (019…), więc wiersz
   `Status:` dotyczy tabel pod nim (`requirements/README.md`). Przyjęte w etapach 0–1: XML-001…018, CH-001, CH-003,
   CH-004, API-001, API-002, API-004, API-007. Wymagania SAFE i DET zostają szkicami: ich testy powstają w etapach
   3 i 5; własności czytnika, które już są testowane, opisują XML-015, XML-016 i XML-017.
6. **Porównanie z czytnikiem `goxsdgo` v0.3.0 (K7)** zaczyna się w etapie 2: przy identycznym kodzie zmierzyłoby
   tylko szum maszyny (`methodology.md`). `tools/bench` powstaje wtedy, a nie w etapie 5.
7. **Wzorzec API** (`api/*.txt`) jest tekstem deklaracji eksportowanych, robionym przez `internal/testkit`
   (`go/ast`, bez sprawdzania typów). Wzorzec API czytnika `goxsdgo` z commitu źródłowego leży w
   `api/baseline/` i służy testowi zgodności (API-002); od etapu 2 test sprawdza, że nic z niego nie zniknęło.

## Konsekwencje

+ `goxmlgo` v0.1.0 da się podstawić za `goxsdgo/xmlreader` v0.3.0 samą zmianą importu (dowody w `testing.md`);
+ od przeniesienia czytnik w `goxsdgo` jest zamrożony: poprawki wchodzą tutaj;
− do zakończenia etapu 6 istnieją dwie kopie czytnika;
− komentarz `Token.EndLine` i nazwy z przyrostkiem `5` w `xmlchars` zostają do uporządkowania w etapie 2.
