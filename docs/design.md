# Projekt

Szkic architektury z 2026-11-26. Sygnatury są planem do potwierdzenia testami w etapach; wiążące są wymagania
w `../requirements/` i decyzje w `adr/`.

## Pakiety

```
xmlchars   klasy znaków XML 1.0 (Char, NameStartChar, NameChar, S); bez zależności
   ↑
xmlreader  czytnik strumieniowy; typy wspólne: Token, Name, Attr, Binding, Kind; interfejs TokenReader
   ↑
xmlwriter  pisarz strumieniowy; przyjmuje wywołania albo Token
   ↑
xmltree    drzewo; czyta z TokenReader, pisze przez xmlwriter, samo jest TokenReader
```

Typy wspólne zostają w `xmlreader`, bo tak wymaga zgodność źródłowa z `goxsdgo/xmlreader` v0.3.0. Osobny pakiet
na same typy byłby możliwy tylko przez aliasy; nie widać dziś potrzeby (pytanie 7).

## Jeden token, trzy role

```go
// xmlreader
type TokenReader interface {
	Next() (Token, error) // io.EOF na końcu; Token ważny do następnego Next
}
```

Implementują go: `*xmlreader.Reader`, czytnik drzewa (`xmltree.NewReader(node)`), filtry pisane przez wywołującego,
a poza modułem czytnik walidujący `goxsdgo` (ten sam strumień, ale już sprawdzony względem schematu).

Żeby ogniwo łańcucha nie potrzebowało dostępu do konkretnego czytnika, token musi nieść wszystko, co potrzebne do
zapisu i do śledzenia zakresu przestrzeni nazw. Dziś deklaracje przestrzeni nazw są dostępne tylko metodą czytnika
(`DeclaredNamespaces`), a `LookupNamespace` tylko na nim. Decyzja (ADR 0002): token początku elementu dostaje
pole z deklaracjami tego elementu (wycinek ważny do następnego `Next`, bez alokacji), a śledzenie zakresu poza
czytnikiem robi mały typ pomocniczy (stos deklaracji), wspólny dla pisarza, drzewa i filtrów.

## Co wraca z zapisu: definicja równości strumieni

Podstawa K2. Dwa strumienie tokenów są równe, gdy mają tę samą liczbę tokenów i dla każdej pary:

| Pole | Porównywane |
|---|---|
| `Kind` | tak |
| `Name` (Space, Local, Prefix) | tak, także dla atrybutów |
| atrybuty | kolejność, nazwy i wartości po normalizacji; `Specified` i `Type` nie (po zapisie atrybut domyślny jest jawny, a deklaracji ATTLIST nie ma) |
| deklaracje przestrzeni nazw elementu | kolejność, prefiks, nazwa przestrzeni |
| `Data` | tak, co do bajta |
| `CDATA`, `Whitespace`, `SelfClosing` | `CDATA` i `Whitespace` tak; `SelfClosing` też (ADR 0002) |
| pozycje (`Line`, `Column`, `EndLine`, `EndColumn`) | nie |
| token `KindDoctype` | pomijany po obu stronach (pisarz domyślnie nie pisze DOCTYPE) |

Świadomie niezachowane: bajty wejścia (cudzysłowy, odstępy w znacznikach, zapis odwołań znakowych i encji,
kodowanie wejścia, BOM, końce wierszy CR LF), deklaracja XML poza `version` i `standalone`. Stąd reguła dla
konsumentów: fragmentu objętego podpisem nie przepisuje się przez tokeny.

## Czytnik: rozszerzenia (etap 2)

| Dodatek | Po co | Uwagi |
|---|---|---|
| `(*Reader).Skip() error` | pominięcie reszty bieżącego elementu (elementy nieznane, `mc:Fallback`, treść z symbolu `skip` w XSD) | sprawdza dobrą formę i przestrzenie nazw jak zwykle, ale nie buduje atrybutów ani tokenów tekstu; po powrocie następny token to pierwszy za znacznikiem końca |
| `(*Reader).Reset(r io.Reader, opts Options)` | ponowne użycie buforów przy wielu małych dokumentach (części DOCX, faktury) | stan po `Reset` nieodróżnialny od `New` |
| `Limits.MaxElements` | ochrona konsumentów, którzy budują coś na element | domyślnie bez limitu (strumień), kod `limit-elements` |
| deklaracje przestrzeni nazw w tokenie | łańcuchy bez dostępu do czytnika, pisarz | ADR 0002 |
| deklaracja XML | pisarz ma odtworzyć `version` i `standalone` | metoda czytnika albo token; do decyzji w etapie 2 |
| gwarancja trwałości napisów | konsument wie, co musi kopiować | `Attr.Value`, `Name.*`, `Binding.*` zostają ważne po `Next`; `Token.Data` i wycinki nie. Zapis w dokumentacji i test |

Wszystko jako dodatki: program napisany dla v0.1.0 kompiluje się i działa tak samo.

## Pisarz (etap 3)

```go
w := xmlwriter.New(out, xmlwriter.Options{})
w.Declaration("1.0", "")            // opcjonalnie
w.Start(xmlwriter.Name{...})        // albo StartNS z deklaracjami
w.Attr(name, value)
w.Text(data) / w.CDATA(data) / w.Comment(data) / w.ProcInst(target, data)
w.End()
w.WriteToken(tok)                   // przelot z czytnika
w.Close()                           // błąd, gdy zostały otwarte elementy; opróżnia bufor
```

Zasady:

- **wyjście zawsze dobrze uformowane.** Pisarz sprawdza to, co czytnik by odrzucił: nazwy ([5] Name, QName), znaki
  ([2] Char), jeden korzeń, pary znaczników, powtórzony atrybut, prefiks bez deklaracji w zakresie, `--`
  w komentarzu, `]]>` w CDATA, cel PI `xml`. Naruszenie to `*xmlwriter.Error` ze stałym kodem; nic nie jest
  usuwane ani zamieniane po cichu;
- **escapowanie jednoznaczne**: w tekście `&`, `<`, `>` i CR (`&#13;`); w wartości atrybutu `&`, `<`, `"` oraz TAB,
  LF i CR jako odwołania znakowe, żeby normalizacja przy odczycie (3.3.3) ich nie zamieniła na spacje. Atrybuty
  zawsze w cudzysłowie `"`;
- **dwa tryby przestrzeni nazw**: jawny (deklaracje podaje wywołujący albo token; tryb przelotu) i automatyczny
  (pisarz sam deklaruje brakujące przestrzenie w miejscu pierwszego użycia, z prefiksami nadawanymi w ustalonej
  kolejności). Automatyczny służy generowaniu dokumentów i zapisowi przeniesionych węzłów drzewa;
- **powtarzalność**: żadnych map w drodze do wyjścia; ten sam ciąg wywołań to te same bajty;
- **bez alokacji na token**, bufor wyjścia o stałym rozmiarze; pamięć zależy od głębokości, nie od rozmiaru;
- UTF-8 bez BOM; końce wierszy takie, jakie podał wywołujący; wcięcia tylko jako opcja, domyślnie wyłączona
  (pytanie 6), bo zmieniają treść mieszaną.

## Drzewo (etap 4)

```go
doc, err := xmltree.Parse(r, xmltree.Options{})        // cały dokument z io.Reader
doc, err := xmltree.ReadDocument(src, opts)            // z dowolnego TokenReader
el, err  := xmltree.ReadElement(src, startToken, opts) // jedno poddrzewo w środku strumienia
err = el.WriteTo(w)                                    // przez xmlwriter
src2 := xmltree.NewReader(el)                          // drzewo jako źródło tokenów
```

- węzeł: rodzaj (element, tekst, komentarz, PI), nazwa, atrybuty w kolejności, deklaracje przestrzeni nazw
  elementu, dane, rodzic, dzieci, wiersz i kolumna ze źródła (zero dla węzłów utworzonych w pamięci);
- zmiany: dodanie, wstawienie przed, usunięcie i zastąpienie dziecka; ustawienie i usunięcie atrybutu; ustawienie
  tekstu; tworzenie węzłów. Operacja, która dałaby dokument źle uformowany (drugi korzeń, zła nazwa), jest błędem
  w chwili wywołania albo najpóźniej przy zapisie, zawsze ze stałym kodem;
- przestrzenie nazw: węzeł pamięta deklaracje ze źródła; `LookupNamespace` i `InScopeNamespaces` idą w górę po
  rodzicach. Węzeł przeniesiony w inne miejsce zapisuje się poprawnie, bo zapis poddrzewa uzupełnia brakujące
  deklaracje (tryb automatyczny pisarza);
- limit węzłów domyślnie włączony (wartość domyślna jak dziś w `goxadesgo`: 2 000 000), kod `limit-nodes`;
- potrzeby istniejących drzew, które zakres musi pokryć: rejestr identyfikatorów (atrybuty typu ID z ATTLIST
  i z listy nazw podanej w opcjach, błąd przy powtórzeniu), przestrzenie nazw w zakresie każdego węzła, pozycje.
  Lista do uzupełnienia przeglądem `../goxadesgo/internal/xmltree` i `../goxsdgo/internal/schema/dom.go` na
  początku etapu 4.

## Strumień i drzewo razem (etap 5)

Trzy wzorce, każdy z testem i przykładem (`Example*`):

1. **przelot**: czytnik → pisarz; stała pamięć, K7;
2. **filtr**: czytnik → funkcja wywołującego na tokenach → pisarz (zmiana wartości, usunięcie elementu przez
   `Skip`, dopisanie elementów);
3. **wyspa**: w strumieniu, na wybranym tokenie początku, `ReadElement` buduje drzewo jednego poddrzewa, wywołujący
   je zmienia, `WriteTo` oddaje je pisarzowi, strumień płynie dalej. Pamięć zależy od największej wyspy.

Obsługa zdarzeniowa (SAX) to wzorzec 2 bez pisarza: pętla `Next` z instrukcją `switch`. Osobnego API nie będzie.

## Kody błędów

Każdy pakiet ma własny typ `Error` ze stałym kodem i pozycją (czytnik: wiersz i kolumna wejścia; pisarz: ścieżka
otwartych elementów i numer wywołania; drzewo: kod i pozycja węzła). Kody czytnika zostają bez zmian. Katalog kodów
wszystkich pakietów: `docs/errors.md` (powstaje w etapie 1, rośnie z etapami); kod raz wydany nie zmienia znaczenia.
