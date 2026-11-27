# 0001 Zakres i kryteria gotowości

Status: przyjęta (robocza) · 2026-11-26

## Kontekst

Czytnik XML rodziny go*go (`goxsdgo/xmlreader`, ADR 0004 w `goxsdgo`) powstał na potrzeby walidacji XSD. Stan
z 2026-11-26:

- konsumentów bez związku z XSD jest dwóch: `goxadesgo` importuje moduł `goxsdgo` wyłącznie dla czytnika, `godocxgo`
  zdecydował tak samo (ADR 0002 tamże); kandydaci to SVG w `gopdfgo` (dziś `encoding/xml` na wejściu niezaufanym)
  i czytnik XLSX;
- te same rzeczy powstają dwa razy: drzewo dokumentu jest w `goxsdgo/internal/schema/dom.go` i osobno
  w `goxadesgo/internal/xmltree` (z własnym limitem węzłów, którego czytnik nie ma);
- brakuje funkcji, których XSD nie potrzebował: pomijania poddrzewa, ponownego użycia czytnika, limitu liczby
  elementów, zapisu XML, zmiany dokumentu i zapisu powrotnego;
- wersje są sprzężone: zmiana API walidatora podnosi wersję modułu konsumentom czytnika i odwrotnie.

Dwie potrzeby wyglądają na sprzeczne: modyfikacja z zapisem powrotnym (DOCX, XLSX, SVG, podpis) i praca na plikach
kilku GB (JPK). Godzi je jeden typ tokenu dla czytnika, pisarza i drzewa: całość płynie strumieniem, a drzewo
powstaje tylko dla poddrzewa, które trzeba zmienić (`../design.md`).

## Decyzja

1. **Osobny moduł `github.com/polestorm/goxmlgo`** z pakietami `xmlchars`, `xmlreader`, `xmlwriter`, `xmltree`.
   Zależności między pakietami tylko w tę stronę. Moduł nie ma zależności poza biblioteką standardową.
2. **Źródła:** Extensible Markup Language (XML) 1.0 Fifth Edition, Namespaces in XML 1.0 Third Edition, XML
   Information Set Second Edition (pojęcia: co jest treścią dokumentu, a co zapisem). Zakres czytnika bez zmian
   względem ADR 0004 w `goxsdgo`: pełna dobra forma i przestrzenie nazw, podzbiór wewnętrzny DTD w zakresie
   procesora bez walidacji, encje zewnętrzne nigdy nie czytane, limity domyślnie włączone.
3. **Czytnik przechodzi bez zmiany zachowania** (etap 1): ten sam kod, te same testy, te same kody błędów,
   API źródłowo zgodne z `goxsdgo/xmlreader` v0.3.0. Rozszerzenia dopiero w etapie 2 i tylko jako dodatki.
4. **Model zdarzeń: pull.** Jedynym modelem strumieniowym jest `Next()` zwracające token. Obsługę zdarzeniową
   (SAX) buduje się na nim pętlą; odwrotnie się nie da, więc osobnego pakietu zdarzeń nie będzie. Wspólny interfejs
   źródła tokenów (`xmlreader.TokenReader`) implementują czytnik, drzewo, filtry i czytniki walidujące innych modułów.
5. **Pisarz strumieniowy** (`xmlwriter`): przyjmuje wywołania (`Start`, `Attr`, `Text`, `End`, …) albo tokeny
   czytnika (`WriteToken`). Wyjście jest zawsze dobrze uformowane albo wywołanie kończy się błędem ze stałym kodem:
   pisarz niczego nie poprawia po cichu. Kodowanie wyjścia UTF-8. Escapowanie jest takie, żeby ponowny odczyt dał
   te same wartości (znaki TAB, LF, CR w wartościach atrybutów i CR w tekście jako odwołania znakowe).
6. **Poziom wierności zapisu powrotnego: treść, nie bajty** (potwierdzone w ADR 0002). Zachowane: nazwy z prefiksami, kolejność
   atrybutów, deklaracje przestrzeni nazw w miejscu i kolejności, tekst, podział na sekcje CDATA, komentarze,
   instrukcje przetwarzania, białe znaki w treści. Niezachowane: deklaracja XML poza wersją i `standalone`, rodzaj
   cudzysłowu, odstępy wewnątrz znacznika, zapis odwołań (encje są rozwinięte), deklaracja DOCTYPE (domyślnie
   pomijana; atrybuty z wartości domyślnych są zapisywane jawnie). Dokładna definicja równości strumieni tokenów
   jest w `../design.md` i jest podstawą K2.
7. **Drzewo** (`xmltree`): węzły z polami publicznymi, wskaźnik na rodzica, pozycja w źródle. Budowane z dowolnego
   źródła tokenów: cały dokument (`Parse`, `ReadDocument`) albo jedno poddrzewo w środku strumienia (`ReadElement`).
   Operacje zmiany na węzłach, zapis przez pisarza (`WriteTo`), odczyt jako źródło tokenów (żeby zmienione drzewo
   dało się zwalidować przed zapisem). Limit liczby węzłów domyślnie włączony. Zakres musi pokryć potrzeby obu
   istniejących drzew, tak żeby dało się je zastąpić.
8. **Bezpieczeństwo.** Limity czytnika jak dotąd oraz limit łącznej liczby elementów (domyślnie wyłączony, bo
   strumień nie trzyma dokumentu w pamięci) i limit węzłów drzewa (domyślnie włączony). Żaden pakiet nie otwiera
   plików ani połączeń.
9. **Poza zakresem:** XML 1.1 i Namespaces 1.1, walidacja DTD, XPath i XSLT, walidacja XSD (`goxsdgo`), dekodowanie
   do struktur Go (`Unmarshal`), Canonical XML (dziś `goxadesgo/c14n`; pytanie 8), kopiowanie surowych bajtów
   poddrzewa (pytanie 4), kodowania wyjścia inne niż UTF-8.

## Kryteria gotowości

Wszystkie muszą być spełnione jednocześnie, zmierzone i zapisane z datą w `../testing.md`:

| # | Kryterium | Miara |
|---|---|---|
| K1 | zgodność czytnika | zestaw W3C XML Conformance 20130923: ≥ 1 725 z 1 727 przypadków w zakresie (stan przy przeniesieniu); każde wyłączenie z powodem i każda porażka opisane w `../differences.md`; plik zapadki nie pozwala liczbie spaść |
| K2 | zapis powrotny | dla każdego dobrze uformowanego dokumentu z zakresu K1 i każdego dokumentu korpusu: tokeny(odczyt(zapis(odczyt(x)))) = tokeny(odczyt(x)) według definicji z `../design.md`, 100%; zapis(odczyt(zapis(odczyt(x)))) = zapis(odczyt(x)) co do bajta, 100%; wyjątki tylko wymienione z powodem |
| K3 | drzewo | dla dokumentów z K2: `Parse` → `WriteTo` daje te same bajty co czytnik → pisarz; `ReadElement` w środku strumienia zostawia strumień w stanie tuż za elementem i nie zmienia wyniku całości; każda operacja zmiany ma wymaganie i test; oba istniejące drzewa (`goxadesgo`, `goxsdgo`) dają się zastąpić (sprawdzane w etapie 6) |
| K4 | wymagania i API | każde przyjęte wymaganie ma test i każde `REQ:` wskazuje istniejące wymaganie; publiczne API każdego pakietu równe wzorcowi w `api/*.txt` |
| K5 | odporność | fuzzery bez paniki i bez naruszenia niezmienników przez ≥ 24 h łącznie na kandydacie do wydania. Niezmienniki: czytnik po błędzie zwraca stale ten sam błąd; to, co pisarz zapisał bez błędu, czytnik czyta bez błędu i z tymi samymi tokenami; drzewo zbudowane i zapisane daje to samo co strumień |
| K6 | bezpieczeństwo | brak dostępu do sieci i plików (test importów pakietów); encje zewnętrzne nigdy nie czytane; zestaw wrogich wejść (rozwijanie encji, zagnieżdżenie 10⁶ poziomów, nazwa 1 GB, 10⁶ atrybutów, token tekstu ponad limit) odrzucony w < 1 s i < 64 MB każde; podwojenie rozmiaru wejścia patologicznego zwiększa czas najwyżej 2,5× |
| K7 | wydajność i pamięć | czytnik ≥ 0,95× szybkości `goxsdgo/xmlreader` v0.3.0 na tym samym dokumencie, w tym samym przebiegu, na tej samej maszynie; czytnik → pisarz ≥ 0,55× szybkości samego czytnika na każdym dokumencie wzorcowym i nie wolniej niż przelot `encoding/xml`, w tym samym przebiegu (od 2026-11-27, ADR 0005 pkt 13; wcześniej cel roboczy „≥ 100 MB/s”); plik 1 GB przez czytnik → pisarz w < 64 MB pamięci procesu; liczba alokacji czytnika i pisarza nie rośnie z rozmiarem dokumentu |
| K8 | powtarzalność i konsumenci | 10 przebiegów daje te same bajty; wzorce korpusu zgodne na wszystkich systemach z CI (Linux, macOS, Windows); `goxsdgo` po przełączeniu przechodzi swoje testy z tymi samymi liczbami K1–K3, `goxadesgo` przechodzi swoje; pakiet aliasów `goxsdgo/xmlreader` kompiluje dotychczasowych konsumentów bez zmian |
| K9 | dowód | pakiet dowodów: wymagania → testy → wynik ostatniego przebiegu, tabela K1–K9 ze źródłem każdej liczby, powtarzalny poza datą |

## Konsekwencje

+ jedna implementacja XML dla całej rodziny, z własnym zestawem zgodności, fuzzingiem i pakietem dowodów;
+ zapis i edycja powstają raz, a nie w każdym konsumencie osobno;
+ czytnik rośnie według potrzeb wszystkich konsumentów, a jego wersja nie zależy od wydań walidatora;
− `goxsdgo` traci „zero zależności”: zyskuje jedną, własną, bez dalszych zależności;
− dodatkowe repozytorium i kolejność wydań: `goxmlgo`, potem `goxsdgo`, potem pozostali;
− `goxmlgo` musi być publiczne, zanim przełączy się publiczne `goxsdgo`: jest publiczne od v0.1.0 (ADR 0002);
− do zakończenia etapu 6 istnieją dwie kopie czytnika; tę w `goxsdgo` zamrażamy od chwili przeniesienia.
