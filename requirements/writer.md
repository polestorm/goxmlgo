# Wymagania: pisarz

Status: szkic

Źródła: XML 1.0 Fifth Edition (produkcje wskazane przy wymaganiach), Namespaces in XML 1.0 Third Edition.
Zasada nadrzędna: to, co pisarz zapisał bez błędu, jest dokumentem dobrze uformowanym, a jego odczyt daje wartości,
które podał wywołujący.

| Id | Wymaganie |
|---|---|
| WR-001 | Pisarz zapisuje do `io.Writer` przez bufor o stałym rozmiarze; `Flush` opróżnia bufor, `Close` sprawdza kompletność dokumentu i opróżnia bufor. Błąd zapisu z `io.Writer` jest zwracany bez zmiany i od tej chwili każde wywołanie zwraca ten sam błąd. |
| WR-002 | Wyjście jest w UTF-8 bez BOM. Deklaracja XML jest zapisywana tylko na żądanie, jako pierwsza rzecz w dokumencie, z wersją `1.0`, bez `encoding` albo z `encoding="UTF-8"`, i opcjonalnym `standalone` ([23]–[32]). |
| WR-003 | Dokument ma dokładnie jeden element główny ([1]): drugi element na poziomie dokumentu, tekst inny niż białe znaki poza korzeniem i `Close` bez korzenia albo z otwartymi elementami są błędami. Komentarze, PI i białe znaki przed korzeniem i po nim są dozwolone. |
| WR-004 | Nazwy elementów, atrybutów i celów PI muszą pasować do [5] Name, a przy przetwarzaniu przestrzeni nazw do QName z niepustymi częściami; inna nazwa jest błędem. Cel PI `xml` w dowolnej wielkości liter jest błędem ([17]). |
| WR-005 | Każdy znak tekstu, wartości atrybutu, komentarza, PI i CDATA musi pasować do [2] Char, a dane muszą być poprawnym UTF-8; inaczej wywołanie kończy się błędem i niczego nie zapisuje. Pisarz nie usuwa i nie zamienia znaków. |
| WR-006 | Tekst: `&`, `<`, `>` są zapisywane jako `&amp;`, `&lt;`, `&gt;`, a CR jako `&#13;`; pozostałe znaki dosłownie. Odczyt daje te same bajty danych, także gdy tekst zawiera `]]>`. |
| WR-007 | Wartość atrybutu jest zapisywana w cudzysłowie `"`; `&`, `<`, `"` jako `&amp;`, `&lt;`, `&quot;`, a TAB, LF i CR jako `&#9;`, `&#10;`, `&#13;`, tak żeby normalizacja 3.3.3 przy odczycie oddała tę samą wartość. |
| WR-008 | Atrybut o tej samej nazwie rozwiniętej (przestrzeń, nazwa lokalna) albo tej samej nazwie kwalifikowanej nie może wystąpić dwa razy w jednym znaczniku; atrybuty są zapisywane w kolejności podania. Atrybut podany po rozpoczęciu treści elementu jest błędem. |
| WR-009 | `End` zamyka ostatni otwarty element; bez otwartego elementu jest błędem. Element bez treści zapisany przez `Start` i `End` daje `<a></a>`, a przez wywołanie elementu pustego (albo token z `SelfClosing`) daje `<a/>` (ADR 0002). |
| WR-010 | CDATA: dane zawierające `]]>` są błędem (pisarz nie dzieli sekcji po cichu). Komentarz: `--` w danych albo `-` na końcu jest błędem ([15]). PI: `?>` w danych jest błędem ([16]). |
| WR-011 | Tryb jawny przestrzeni nazw: deklaracje podaje wywołujący (albo token) i są zapisywane w podanej kolejności przed atrybutami; użycie prefiksu bez deklaracji w zakresie, ponowna deklaracja tego samego prefiksu w jednym znaczniku, deklaracja prefiksu `xmlns`, zmiana wiązania `xml` i wyłączenie prefiksu (`xmlns:p=""`) są błędami (Namespaces, rozdziały 3 i 5). |
| WR-012 | Tryb automatyczny: gdy nazwa elementu albo atrybutu wskazuje przestrzeń niezadeklarowaną w zakresie, pisarz dodaje deklarację w tym znaczniku. Prefiks bierze z nazwy, jeśli jest podany i wolny, inaczej nadaje kolejny z ustalonego ciągu (`ns1`, `ns2`, …) pomijając zajęte. Atrybut w przestrzeni nazw nigdy nie dostaje prefiksu pustego. Wynik zależy tylko od ciągu wywołań. |
| WR-013 | `WriteToken` zapisuje token czytnika: początek elementu z deklaracjami i atrybutami (także `Specified=false`), koniec, tekst (jako CDATA, gdy token ma `CDATA`), komentarz, PI. Token `KindDoctype` jest pomijany, o ile opcja nie każe zwrócić błędu. Dane tokenu nie są zatrzymywane po powrocie. |
| WR-014 | Błędy pisarza to `*Error` ze stałym kodem z katalogu (`docs/errors.md`), ścieżką otwartych elementów i komunikatem po angielsku. Po błędzie walidacji wywołania pisarz nie zapisał nic z tego wywołania i może przyjmować dalsze wywołania. |
| WR-015 | Pisarz nie alokuje pamięci na wywołanie po rozgrzaniu (poza wzrostem stosu otwartych elementów i tablicy deklaracji); test mierzy alokacje na dokumencie wzorcowym i wymaga liczby niezależnej od rozmiaru dokumentu. |
| WR-016 | Opcja wcięć (pytanie 6), domyślnie wyłączona: białe znaki są dodawane wyłącznie między znacznikami elementów, w których nie zapisano tekstu; element z tekstem i jego potomkowie są zapisywani bez dodatków. |
| WR-017 | Limity pisarza: głębokość otwartych elementów i długość nazwy mają te same wartości domyślne i kody co w czytniku, tak żeby pisarz nie zapisał dokumentu, którego czytnik z ustawieniami domyślnymi nie przeczyta. |
