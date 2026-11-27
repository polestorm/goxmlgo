# 0005 Pisarz strumieniowy

Status: przyjęta · 2026-11-27 (etap 3; rekomendacje do pytań 6 i 7 przyjęte roboczo, do potwierdzenia przez
właściciela przed wydaniem v0.3.0)

## Kontekst

Etap 3 dodaje pakiet `xmlwriter`: zapis dokumentu wywołaniami albo tokenami czytnika, z kontrolą dobrej formy
wyjścia (ADR 0001, pkt 5 i 6; ADR 0002, pkt 3). Szkice wymagań WR-001…017 zostały przejrzane, doprecyzowane
i przyjęte (`../../requirements/writer.md`), razem z STR-001, 002, 003, 006, 007 i DET-002, 004, 007.

## Decyzja

1. **Kształt API.** `New`, `Reset`, `Declaration(standalone)`, `Start(name, deklaracje...)`, `Attr`, `Text`,
   `CDATA`, `Comment`, `ProcInst`, `End`, `EndEmpty`, `WriteToken`, `Flush`, `Close`. Deklaracje przestrzeni nazw
   elementu podaje się w `Start`, a nie osobnym wywołaniem po nim: dzięki temu nazwa elementu i każdy atrybut są
   sprawdzane i zapisywane od razu, bez odkładania znacznika początku do bufora i bez błędów zgłaszanych
   o wywołanie za późno. Dane przyjmowane są jako napisy; `WriteToken` przekazuje `Token.Data` bez kopiowania
   (funkcje danych są generyczne po napisie i wycinku bajtów).
2. **Typy wspólne zostają w `xmlreader`** (pytanie 7): `xmlwriter` ma aliasy `Name`, `Attr`, `Binding`, `Token`,
   więc kod, który tylko pisze XML, nie musi importować czytnika z nazwy, a typy są tożsame.
3. **Kod błędu nazywa regułę, nie pakiet.** Gdzie pisarz odmawia tego, co czytnik by odrzucił, zwraca kod czytnika
   (`xml-char`, `xml-name`, `nsc-prefix-declared`, `limit-depth`, …; stałe `xmlwriter.Code*` równe stałym
   czytnika). Własne kody pisarza: `xml-pi-data`, `ns-name-mismatch`, `unsupported-carriage-return`,
   `unsupported-doctype` i pięć `usage-*`. Katalog: `../errors.md`.
4. **Błąd wywołania niczego nie zmienia.** Każde wywołanie najpierw sprawdza wszystko, potem pisze. Po błędzie
   pisarz jest w stanie sprzed wywołania, także po `WriteToken` z błędem w ostatnim atrybucie (atrybuty nowego
   znacznika są sprawdzane w osobnym zbiorze, który zastępuje zbiór otwartego znacznika dopiero po powodzeniu).
   Test: ciąg wywołań z wplecionymi wywołaniami błędnymi daje te same bajty co bez nich (`TestErrorState`,
   `FuzzWriter`). Błąd `io.Writer` nie jest `*Error`: wraca bez zmiany i jest trwały.
5. **Tryb jawny jest ścisły.** `Name.Space` musi być przestrzenią, z którą w zakresie związany jest
   `Name.Prefix`. Pisarz nie uzupełnia pustego `Space` z prefiksu: element bez przestrzeni nazw wpisany pod
   przestrzeń domyślną zmieniłby znaczenie po cichu. Tokeny czytnika spełniają ten warunek zawsze.
6. **Tryb automatyczny** (`Options.AutoNamespaces`) służy generowaniu dokumentów i zapisowi węzłów drzewa
   przeniesionych w inne miejsce (etap 4). Reguły w WR-012. Dwie decyzje warte zapisania: prefiks pusty w nazwie
   elementu jest brany dosłownie (deklaracja przestrzeni domyślnej, a nie szukanie innego prefiksu), a atrybut
   nie może przesłonić prefiksu, który jest w zakresie, bo zmieniłby znaczenie nazwy elementu albo wcześniejszych
   atrybutów tego samego znacznika: dostaje wtedy prefiks już związany z jego przestrzenią albo kolejny
   z `ns1`, `ns2`, …
7. **Deklaracja XML**: zawsze `version="1.0" encoding="UTF-8"`, opcjonalne `standalone`, po niej znak końca
   wiersza. Czytnik nie oddaje białych znaków poza korzeniem, więc ten znak nie narasta przy kolejnych zapisach.
   Wersja z deklaracji wejścia nie jest przenoszona (zakres to XML 1.0).
8. **Czego nie da się oddać, jest błędem, nie stratą.** CR w komentarzu, PI, CDATA i w białych znakach poza
   korzeniem (`unsupported-carriage-return`: nie ma tam odwołań znakowych, a czytnik zamienia CR na LF, 2.11);
   biały znak na początku danych PI (`xml-pi-data`: czytnik oddaje dane bez niego). Wymagania WR-010, WR-018.
9. **`<a/>` i `<a></a>`.** `End` po `Start` daje `<a></a>`, `EndEmpty` daje `<a/>`. `SelfClosing` w tokenie jest
   wskazówką: element zostaje zapisany jako `<a/>`, o ile między tokenem początku i końca nic nie zapisano; filtr,
   który dopisał treść, dostaje zwykłą parę znaczników.
10. **Wcięcia** (pytanie 6): `Options.Indent`, domyślnie wyłączone. Pisarz strumieniowy nie zna przyszłej treści
    elementu, więc reguła „tylko w elementach bez tekstu” ze szkicu jest niewykonalna dosłownie. Przyjęta reguła:
    wcięcie przed znacznikiem, komentarzem i PI, nigdy w elemencie, w którym wcześniej zapisano tekst albo CDATA,
    ani w jego potomkach. Tekst nie zmienia się nigdy; w elemencie, który dostaje tekst po elementach podrzędnych,
    białe znaki zapisane przed nimi zostają. Dokument już wcięty przechodzi bez zmian (WR-016).
11. **Limity** pisarza obejmują także liczbę atrybutów i rozmiar tekstu (szkic wymieniał tylko głębokość i długość
    nazwy): inaczej pisarz z ustawieniami domyślnymi mógłby zapisać dokument, którego czytnik z ustawieniami
    domyślnymi nie przeczyta (WR-017).
12. **Czytnik nie daje już pustego tokenu tekstu.** Do v0.2.0 odwołanie do encji, która nie daje żadnego znaku
    przed znacznikiem (pusta albo zaczynająca się od znacznika), tworzyło token tekstu o pustych danych. Takiego
    tokenu nie da się zapisać tak, żeby wrócił, więc 12 z 775 dokumentów W3C nie przechodziło zapisu powrotnego.
    Poprawiono czytnik, a nie definicję równości: pusty tekst nie jest treścią dokumentu (XML Information Set nie
    zna pustej informacji znakowej), a każdy konsument musiałby go odfiltrować. To zmiana zachowania względem
    v0.2.0, zapisana w `../differences.md`; `TestSameAsGoxsdgo` sprawdza, że jest jedyną różnicą kształtu
    strumienia wobec `goxsdgo` v0.3.0 (22 z 3 394 plików), a testy `goxsdgo` v0.4.1 na czytniku po zmianie dają
    ten sam wynik co na v0.2.0 (1 127 zaliczonych, te same dwa testy księgowe).

## Pomiary (2026-11-27, drzewo robocze etapu 3)

- K2: korpus 13 z 13 dokumentów (wzorce `*.out.xml`), zestaw W3C 775 z 775 dokumentów, które czytnik przyjmuje
  (`TestRoundTrip`, zapadka `testdata/w3c/roundtrip.ratchet`); jeden dobrze uformowany przypadek w zakresie czytnik
  odrzuca (porażka K1, `../differences.md`).
- `encoding/xml` jako czarna skrzynka na wyjściu pisarza: 767 z 789 dokumentów daje te same nazwy rozwinięte,
  wartości i dane; 13 `encoding/xml` odrzuca (nazwy z piątej edycji XML 1.0), 9 nie jest czytanych z przestrzeniami
  nazw (`TestStdlibWriter`).
- Szybkość przelotu czytnik → pisarz (`tools/bench`, `../results/reader-vs-goxsdgo.txt`): 159 MB/s na dokumencie
  w stylu JPK i 82 MB/s w stylu WordprocessingML, czyli 0,62 i 0,67 szybkości samego czytnika oraz 1,85× i 1,22×
  przelotu `Decoder` → `Encoder` z `encoding/xml`. Pisarz nie dodaje alokacji do alokacji czytnika; wywołania nie
  alokują po rozgrzaniu (`TestWriterAllocations`).
- Cel roboczy K7 „przelot ≥ 100 MB/s na dokumencie wzorcowym” jest spełniony na jednym z dwóch dokumentów
  wzorcowych. Na drugim sam czytnik daje 123 MB/s, więc cel bezwzględny jest tam blisko granicy tego, co możliwe.
  Pytanie 13 w `open-questions.md` proponuje, jak go sformułować.

## Konsekwencje

+ zapis powrotny działa na całym zestawie W3C i korpusie; pisarz jest gotowy pod drzewo (etap 4);
+ `FuzzWriter` sprawdza na dowolnym ciągu wywołań i to, że wyjście jest dobrze uformowane, i to, że wraca;
− czytnik zmienia zachowanie w jednym punkcie (pkt 12); konsument, który liczył tokeny tekstu, zobaczy różnicę;
− wcięcia są kompromisem (pkt 10) i zostają opisane jako zmiana treści;
− przelot na dokumencie z wieloma atrybutami jest poniżej celu roboczego (pytanie 13).
