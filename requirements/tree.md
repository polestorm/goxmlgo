# Wymagania: drzewo

Status: szkic

Na początku etapu 4 uzupełnić o wynik przeglądu `../../goxadesgo/internal/xmltree` i
`../../goxsdgo/internal/schema/dom.go`: każda ich funkcja ma tu wymaganie albo wpis, dlaczego zostaje u konsumenta.

| Id | Wymaganie |
|---|---|
| TR-001 | `Parse(io.Reader)` i `ReadDocument(TokenReader)` budują dokument: węzły przed korzeniem (komentarze, PI), korzeń, węzły po korzeniu, treść deklaracji XML. Błąd źródła jest zwracany bez zmiany kodu i pozycji. |
| TR-002 | `ReadElement(src, start)` buduje poddrzewo elementu, którego token początku właśnie zwróciło źródło, i zostawia źródło w stanie tuż za jego znacznikiem końca. Reszta strumienia jest taka sama, jak gdyby element pominięto przez `Skip`. |
| TR-003 | Węzeł elementu ma nazwę (z prefiksem), atrybuty w kolejności ze źródła (z `Specified` i `Type`), deklaracje przestrzeni nazw w kolejności ze źródła, dzieci w kolejności dokumentu i rodzica. Węzły tekstu zachowują podział i znacznik CDATA; komentarze i PI są węzłami. Dane węzłów są kopiami i nie zależą od źródła. |
| TR-004 | Węzeł ze źródła ma wiersz i kolumnę swojego tokenu; węzeł utworzony w pamięci ma zera. |
| TR-005 | `LookupNamespace(prefix)` i `InScopeNamespaces()` na węźle dają to samo, co czytnik w tym miejscu dokumentu. Dla poddrzewa z `ReadElement` uwzględniają deklaracje przodków spoza poddrzewa, zapamiętane w chwili budowy. |
| TR-006 | Limit węzłów (`Options.MaxNodes`, domyślnie 2 000 000; zero to wartość domyślna, wartość ujemna brak limitu) obejmuje elementy, teksty, komentarze i PI; przekroczenie daje `limit-nodes` z pozycją węzła, który przekroczył limit, a zbudowana część jest odrzucana. |
| TR-007 | Operacje zmiany: dodanie dziecka na końcu, wstawienie przed wskazanym dzieckiem, usunięcie, zastąpienie; ustawienie i usunięcie atrybutu (ustawienie istniejącego zachowuje jego miejsce, nowy trafia na koniec); ustawienie tekstu elementu; tworzenie węzłów każdego rodzaju. Po każdej operacji wskaźniki na rodzica i kolejność dzieci są spójne. |
| TR-008 | Węzeł ma najwyżej jednego rodzica: wstawienie węzła, który ma rodzica, jest błędem (trzeba go najpierw usunąć albo skopiować); wstawienie węzła do własnego potomka jest błędem. Głęboka kopia poddrzewa daje węzły bez rodzica. |
| TR-009 | Operacja, która dałaby dokument źle uformowany, jest błędem ze stałym kodem najpóźniej przy zapisie: drugi element główny, tekst inny niż białe znaki poza korzeniem, nazwa niebędąca nazwą, powtórzony atrybut. |
| TR-010 | `WriteTo(*xmlwriter.Writer)` zapisuje węzeł i jego potomków. Dla drzewa niezmienionego wynik jest co do bajta równy zapisowi strumienia, z którego drzewo powstało (K3). |
| TR-011 | Węzeł przeniesiony albo skopiowany w miejsce o innych deklaracjach przestrzeni nazw zapisuje się poprawnie: brakujące deklaracje są dodawane na korzeniu zapisywanego poddrzewa albo w miejscu użycia, deterministycznie; nazwy rozwinięte elementów i atrybutów po odczycie są takie same jak przed przeniesieniem. Wartości tekstowe zawierające prefiksy (QName w treści) nie są zmieniane; deklaracje, które węzeł miał w zakresie w źródle, są dla niego zachowywane. |
| TR-012 | `NewReader(node)` daje `TokenReader` zwracający tokeny poddrzewa w kolejności dokumentu, z deklaracjami przestrzeni nazw; dla drzewa niezmienionego strumień jest równy strumieniowi źródła (według definicji z `docs/design.md`). |
| TR-013 | Rejestr identyfikatorów (opcjonalny): atrybuty typu `ID` z ATTLIST oraz atrybuty o nazwach podanych w opcjach tworzą indeks `ByID`; powtórzony identyfikator jest błędem ze stałym kodem i pozycjami obu wystąpień. Po zmianach drzewa indeks jest aktualny albo jawnie przebudowywany (do decyzji w etapie 4). |
| TR-014 | Budowa i zapis drzewa są liniowe względem liczby węzłów; pamięć na węzeł jest mierzona i zapisana w `docs/testing.md`. |
