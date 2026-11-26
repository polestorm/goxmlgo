# goxmlgo — instrukcje dla Claude Code

Projekt: wspólna warstwa XML rodziny go*go w czystym Go. Trzy pakiety na jednym typie tokenu: czytnik strumieniowy
(`xmlreader`), pisarz strumieniowy (`xmlwriter`) i modyfikowalne drzewo w pamięci (`xmltree`), plus klasy znaków
(`xmlchars`). Cele: zgodność z rekomendacjami W3C (XML 1.0 Fifth Edition, Namespaces in XML 1.0 Third Edition), zero cgo
i zero zależności poza biblioteką standardową, praca strumieniowa w stałej pamięci także dla plików kilku GB, zapis
powrotny bez cichej straty, bezpieczny z konstrukcji (bez sieci, bez encji zewnętrznych, limity), błędy ze stałym
kodem i miejscem, wynik powtarzalny co do bajta, czas liniowy także dla danych złośliwych. Projekt jest agnostyczny
wobec odbiorców.

Zacznij od `README.md`, potem `docs/roadmap.md`, `docs/adr/0001-scope-and-acceptance.md`,
`docs/adr/0002-repository-token-and-fidelity.md`, `docs/design.md`,
`docs/migration.md`, `docs/provenance.md` i `docs/adr/open-questions.md`.

## Po co i w jakim układzie (stan 2026-11-26)

- Czytnik XML rodziny powstał w `goxsdgo` jako `xmlreader` (`../goxsdgo/xmlreader`, ADR 0004 tamże; publiczny od
  v0.3.0). Ma już konsumentów bez związku z XSD: `../goxadesgo` (własne drzewo `internal/xmltree` na czytniku)
  i `../godocxgo` (ADR 0002 tamże). Kandydaci: SVG w `../gopdfgo` (dziś `encoding/xml`), przyszły czytnik XLSX.
- Potrzeby wykraczają poza walidację XSD: pomijanie poddrzew, ponowne użycie czytnika, limit liczby elementów,
  zapis XML (DOCX, XLSX, SVG, JPK, podpis XAdES), modyfikacja i zapis powrotny, przetwarzanie plików kilku GB.
  Dlatego czytnik przechodzi tutaj i rośnie niezależnie od `goxsdgo`.
- **Kolejność u konsumentów** (`docs/migration.md`): po etapie 1 (v0.1.0) przełącza się `goxsdgo` i `goxadesgo`,
  po etapie 2 (v0.2.0) rusza etap 1 `godocxgo`. Od chwili przeniesienia czytnik w `goxsdgo` jest zamrożony:
  poprawki wchodzą tylko tutaj.
- Rodzina: `../goxsdgo`, `../gomdgo`, `../gomermaidgo`, `../gopdfgo`, `../godocxgo` powstają tak samo (od zera ze
  specyfikacji, czarna skrzynka jako odniesienie, korpus, wymagania, datowane pomiary). **Najbliższy wzór to
  `../goxsdgo`**: jego `docs/testing.md`, `requirements/`, `tools/fetch`, `tools/fuzz`, `tools/evidence`,
  `tools/bench`, test pokrycia wymagań, pliki zapadek (`*.ratchet`), `docs/methodology.md` i `docs/results/`.
  Narzędzia z `goxsdgo` są nasze: wolno je przenieść i dostosować.

## Kryteria gotowości (skrót ADR 0001)

| # | Kryterium | Gdzie mierzone |
|---|---|---|
| K1 | czytnik: zestaw W3C XML Conformance, ≥ 1 725 z 1 727 przypadków w zakresie; każde wyłączenie i każda porażka opisane w `docs/differences.md`; liczba nie spada | `TestXMLConformance`, zapadka |
| K2 | zapis powrotny: dla każdego dobrze uformowanego dokumentu zestawu W3C i korpusu odczyt → zapis → odczyt daje ten sam strumień tokenów; zapis jest idempotentny co do bajta | `TestRoundTrip` |
| K3 | drzewo: `Parse` → `WriteTo` daje te same bajty co czytnik → pisarz; poddrzewo można zmaterializować w środku strumienia; każda operacja zmiany ma wymaganie i test | `TestTreeRoundTrip`, `TestSubtreeInStream` |
| K4 | każde wymaganie z `requirements/` ma test; publiczne API zgodne z zapisanym wzorcem | `TestRequirementsCovered`, `TestAPISnapshot` |
| K5 | fuzzing ≥ 24 h łącznie bez paniki i bez naruszenia niezmienników (to, co pisarz zapisał bez błędu, czytnik czyta bez błędu i z tymi samymi tokenami) | `FuzzReader`, `FuzzRoundTrip`, `FuzzWriter`, `FuzzTree` |
| K6 | bezpieczeństwo: bez sieci i plików, encje zewnętrzne nigdy nie czytane, limity domyślnie włączone; wrogie wejścia odrzucone w < 1 s i < 64 MB; czas liniowy | `TestSafeByDefault`, `TestHostileInputs`, `TestPathological` |
| K7 | wydajność i pamięć: czytnik nie wolniejszy niż 0,95× czytnika z `goxsdgo` v0.3.0 w tym samym przebiegu; plik 1 GB przez czytnik → pisarz w < 64 MB; stała liczba alokacji na dokument | `tools/bench`, `TestStreamingMemory` |
| K8 | powtarzalność i konsumenci: te same bajty w 10 przebiegach i na każdym systemie z CI; `goxsdgo` i `goxadesgo` po przełączeniu przechodzą swoje testy bez zmiany swoich liczb | `TestDeterministic`, u konsumentów (etap 6) |
| K9 | pakiet dowodów: wymagania → testy → wynik ostatniego przebiegu, tabela K1–K9 ze źródłem każdej liczby | `tools/evidence` |

Po każdym etapie dopisz wiersz do tabeli pomiarów w `docs/testing.md` (data, commit, liczby K1–K3) i zaktualizuj
kolumnę „Stan” w `docs/roadmap.md`.

## Jak pracujemy

- etapy w kolejności z `docs/roadmap.md`; etapy 0 (narzędzia i szkielet testów) i 1 (przeniesienie czytnika bez
  zmiany zachowania) wykonane 2026-11-26 (`docs/adr/0003-reader-transfer.md`, dowody w `docs/testing.md`);
  **następny krok: etap 2**;
- pytania 1–3 są rozstrzygnięte (ADR 0002: repozytorium publiczne od v0.1.0, deklaracje przestrzeni nazw
  w tokenie, zapis powrotny zachowuje treść, nie bajty), więc etapy 0–2 nie mają blokad decyzyjnych; pozostałe
  pytania (`docs/adr/open-questions.md`) rozstrzygamy w etapach, z rekomendacją;
- wymagania w `requirements/` są szkicami do czasu rozpoczęcia ich etapu (`requirements/README.md`); etap zaczyna
  się od potwierdzenia swoich wymagań;
- najpierw test (przypadek W3C, plik korpusu albo przypadek z wymagania), potem kod; liczba zaliczonych przypadków
  W3C nie spada między commitami bez wpisu, dlaczego;
- repozytorium `github.com/polestorm/goxmlgo` (publiczne, do założenia; tagi wydań podpisane); commit i push tylko na prośbę właściciela.

Zasady:
- **czysty rodowód** (`docs/provenance.md`): piszemy z rekomendacji W3C. Nie czytaj, nie kopiuj i nie przenoś kodu
  innych parserów, pisarzy i bibliotek drzew XML (libxml2, expat, Xerces, Saxon, lxml, etree, quick-xml i podobnych,
  także pisanych w Go). Wolno uruchamiać je jako czarną skrzynkę. `encoding/xml` z biblioteki standardowej wolno
  czytać i używać w testach jako punkt odniesienia; biblioteka go nie importuje;
- **zero zależności i zero cgo**: `go.mod` w katalogu głównym nie ma sekcji `require`; narzędzia (`tools/*`)
  w osobnych modułach;
- **zgodność API czytnika**: `goxmlgo/xmlreader` po etapie 1 jest źródłowo zgodny z `goxsdgo/xmlreader` v0.3.0;
  potem API tylko rośnie. Każda zmiana publicznego API zmienia plik wzorca (`TestAPISnapshot`) i ma wpis w ADR;
- **żadnej cichej straty przy zapisie**: pisarz nie poprawia i nie pomija niczego po cichu. Znak niedozwolony
  w XML, zła nazwa, niezadeklarowany prefiks, niezamknięty element to błąd ze stałym kodem, nie „naprawa”;
- **powtarzalność**: żadnej iteracji po mapie w drodze do wyniku, żadnego czasu ani losowości w wyniku; ten sam
  ciąg wywołań daje te same bajty na każdym systemie;
- **porównania z innymi** (`encoding/xml` i każdy inny kandydat): wsad publiczny albo generowany deterministycznie,
  narzędzie pomiaru w repozytorium, metoda w `docs/methodology.md`, surowy wynik w `docs/results/`; liczba bez
  odsyłacza nie trafia do README;
- **fuzzing** (`docs/fuzzing.md`): kampanie na Synology przez wspólną kolejkę, na Macu najwyżej w nocy na zasilaniu;
  lokalnie tylko krótki przebieg sprawdzający poprawkę (≤ 2 min, `-parallel 2`). W poleceniach dla podagentów
  przekazuj tę regułę wprost;
- **żadnych danych prywatnych w repozytorium**: korpus to dokumenty syntetyczne i publiczne zestawy testowe;
- nazwy plików, pakietów i identyfikatorów po angielsku; komentarze dokumentacyjne publicznego API, `README.md`,
  `docs/provenance.md` i `docs/methodology.md` po angielsku (pokazuje je pkg.go.dev albo prowadzi do nich README);
  komentarze wewnętrzne, ADR, plan, wymagania i pomiary po polsku; komunikaty biblioteki po angielsku, ze stałym
  kodem do tłumaczenia przez wywołującego;
- każda decyzja = plik w `docs/adr/`; pytania bez decyzji w `docs/adr/open-questions.md`;
- liczby są datowane i podpisane sposobem pomiaru;
- opisy commitów po angielsku; przed commitem muszą przechodzić `gofmt`, `go vet ./...` i `go test ./...`.
