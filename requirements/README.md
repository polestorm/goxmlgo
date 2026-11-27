# Wymagania

Każde wymaganie ma identyfikator `PREFIKS-nnn`. Testy i pliki korpusu odwołują się do nich przez `REQ: PREFIKS-nnn`;
test `TestRequirementsCovered` sprawdza pokrycie w obie strony.

| Prefiks | Plik | Zakres | Etap | Status |
|---|---|---|---|---|
| `XML` | `xml.md` | czytnik: dobra forma, przestrzenie nazw, pozycje, DOCTYPE, limity (001…018 z `goxsdgo`), rozszerzenia (019…) | 1, 2 | 001…018 przyjęte (etap 1); 019…029 przyjęte (etap 2); 030, 031 przyjęte (etap 4) |
| `CH` | `chars.md` | klasy znaków XML 1.0 | 1, 2 | 001, 003, 004 przyjęte (etap 1); 002, 005 szkic |
| `WR` | `writer.md` | pisarz: wywołania, tokeny, escapowanie, przestrzenie nazw, dobra forma wyjścia, kody | 3 | 001…018 przyjęte (etap 3); 019, 020 przyjęte (etap 4) |
| `TR` | `tree.md` | drzewo: budowa, poddrzewo ze strumienia, zmiany, zapis, źródło tokenów, limit | 4 | 001…015 przyjęte (etap 4) |
| `STR` | `stream.md` | łańcuchy: przelot, filtr, wyspa; zapis powrotny | 3, 5 | 001, 002, 003, 006, 007 przyjęte (etap 3); 004, 005 szkic |
| `SAFE` | `safety.md` | brak sieci i plików, encje zewnętrzne, limity, wrogie wejścia, czas liniowy | 5 | 001…006 przyjęte (2026-11-27); 007 szkic |
| `DET` | `determinism.md` | powtarzalność, niezależność od systemu, pamięć i alokacje | 3, 5 | 002, 004, 007 przyjęte (etap 3); reszta szkic |
| `API` | `api.md` | zgodność i stabilność API, wzorce, kody błędów | 0, 1 | 001, 002, 003, 004, 007 przyjęte (etapy 0–2); 005, 006 szkic |
| `EVD` | `evidence.md` | pakiet dowodów (K9) | 5 | szkic |

## Szkic a wymaganie przyjęte

Wiersz `Status: szkic` albo `Status: przyjęte` dotyczy tabel pod nim, do następnego takiego wiersza: plik może
mieć część przyjętą i część w szkicu (np. `xml.md`: 001…018 przyjęte, rozszerzenia w szkicu). Szkice zapisano przy
planowaniu (2026-11-26), żeby sesja produkcyjna znała zamiar; etap zaczyna się od przeglądu swoich szkiców,
poprawienia ich według rekomendacji i decyzji z `../docs/adr/`, i zmiany statusu. `TestRequirementsCovered` wymaga
testu tylko dla wymagań przyjętych, ale odwołanie `REQ:` do nieistniejącego identyfikatora jest błędem zawsze,
a wymaganie przed pierwszym wierszem statusu jest błędem. Identyfikator raz przyjęty
nie zmienia znaczenia; wymaganie wycofane zostaje w pliku z dopiskiem.

Wymaganie dla rekomendacji wskazuje rozdział i regułę (np. „3.3.3”, „[10] AttValue”, „WFC: Unique Att Spec”),
a nie pojedynczy przypadek testowy; przypadki W3C są testami tego wymagania.
