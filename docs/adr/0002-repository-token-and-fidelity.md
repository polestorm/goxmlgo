# 0002 Repozytorium publiczne, deklaracje przestrzeni nazw w tokenie, wierność zapisu powrotnego

Status: przyjęta · 2026-11-26 (decyzja właściciela; zamyka pytania 1, 2 i 3)

## Kontekst

Trzy pytania blokowały etapy 1 i 2 (`open-questions.md`): czy repozytorium jest publiczne, jak ogniwa łańcucha
poza czytnikiem dowiadują się o deklaracjach przestrzeni nazw, i co dokładnie ma wracać z zapisu powrotnego.
Właściciel przyjął rekomendacje.

## Decyzja

1. **Repozytorium `github.com/polestorm/goxmlgo` jest publiczne od v0.1.0.** Publiczne `goxsdgo` ma od niego
   zależeć, a czytnik jest już opublikowany w `goxsdgo` v0.3.0, więc niczego nowego nie ujawniamy. Skutki: README,
   komentarze publicznego API, `docs/provenance.md` i `docs/methodology.md` po angielsku; tagi wydań podpisane;
   porównania z innymi tylko z publicznym wsadem, narzędziem w repozytorium i surowym wynikiem.
2. **Token początku elementu niesie deklaracje przestrzeni nazw tego elementu** (pole z wycinkiem `Binding`,
   w kolejności z dokumentu, ważne do następnego `Next`, bez alokacji; obejmuje deklaracje z wartości domyślnych
   ATTLIST). Zakres przestrzeni nazw poza czytnikiem śledzi mały typ pomocniczy zasilany tokenami, wspólny dla
   pisarza, drzewa i filtrów. Metody czytnika `DeclaredNamespaces` i `LookupNamespace` zostają bez zmian: to
   dodatek do API, nie zmiana. Wymagania: XML-025, XML-026.
3. **Zapis powrotny zachowuje treść i prefiksy, nie bajty** (ADR 0001, pkt 6; definicja równości strumieni
   w `../design.md`). Szczegóły:
   - `<a/>` i `<a></a>` są rozróżniane: `SelfClosing` tokenu jest zachowywany przy zapisie i porównywany w K2
     (wymaganie WR-009);
   - pisarz w 1.0 nie zapisuje deklaracji DOCTYPE: token `KindDoctype` jest pomijany (albo, według opcji, jest
     błędem), a atrybuty z wartości domyślnych są zapisywane jawnie (WR-013);
   - bajty wejścia nie wracają: rodzaj cudzysłowu, odstępy w znacznikach, zapis odwołań, kodowanie i BOM,
     końce wierszy CR LF. Fragmentu objętego podpisem nie przepisuje się przez tokeny; kopiowanie surowych bajtów
     poddrzewa zostaje pytaniem 4.

## Konsekwencje

+ etapy 1 i 2 nie mają już blokad decyzyjnych;
+ pisarz i filtry działają na samych tokenach, więc źródłem może być czytnik, drzewo albo czytnik walidujący;
− token rośnie o jedno pole; czytnik musi je wypełniać bez alokacji (pomiar w etapie 2, K7);
− dokument, którego treść zależy od DOCTYPE (encje bez rozwinięcia, typy atrybutów), po zapisie nie ma deklaracji:
  takie przypadki są wyjątkami K2 wymienionymi w `../differences.md`.
