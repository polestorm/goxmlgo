# Plan

Szacunki z 2026-11-26, w godzinach pracy z asystentem. Kryteria zakończenia całości: ADR 0001, kryteria K1–K9.

| Etap | Zakres | Szacunek | Wydanie | Stan |
|---|---|---|---|---|
| 0 | infrastruktura: `tools/fetch` z manifestem `testdata/sources.json` (zestaw W3C XML Conformance, przeniesione z `goxsdgo`), `TestRequirementsCovered`, `TestAPISnapshot` z wzorcami `api/*.txt`, szkielet `tools/fuzz` i `docs/fuzzing.md`, CI z pobraniem danych, korpus i jego generator (`testdata/README.md`) | 3–5 h | — | wykonany 2026-11-26 (przed pierwszym commitem) |
| 1 | **przeniesienie czytnika bez zmiany zachowania**: `xmlreader` i `xmlchars` z `goxsdgo` (commit źródłowy zapisany w `docs/provenance.md`), testy, wejścia regresji fuzzingu, wymagania XML-001…018 z tymi samymi identyfikatorami, `docs/errors.md`, `docs/differences.md`, porównanie z `encoding/xml` powtórzone tutaj (`docs/methodology.md`, `docs/results/`) | 3–5 h | v0.1.0 | wykonany 2026-11-26 (przed pierwszym commitem; dowody w `testing.md`); wydanie v0.1.0 na prośbę właściciela |
| 2 | rozszerzenia czytnika: `Skip`, `Reset`, `Limits.MaxElements`, interfejs `TokenReader`, deklaracje przestrzeni nazw w tokenie i typ śledzący zakres (ADR 0002), deklaracja XML, gwarancja trwałości napisów; wszystko jako dodatki do API; początek `tools/bench` i porównania z czytnikiem `goxsdgo` v0.3.0 (ADR 0003, pkt 6) | 5–8 h | v0.2.0 | wykonany 2026-11-26 (przed pierwszym commitem; ADR 0004, dowody w `testing.md`); wydanie na prośbę właściciela |
| 3 | `xmlwriter`: wywołania i `WriteToken`, escapowanie, tryb jawny i automatyczny przestrzeni nazw, kontrola dobrej formy wyjścia, kody błędów, `TestRoundTrip` (K2), `FuzzRoundTrip`, `FuzzWriter`, pomiar szybkości | 8–12 h | v0.3.0 | wykonany 2026-11-27 (ADR 0005, dowody w `testing.md`); wydany jako v0.3.0 |
| 4 | `xmltree`: przegląd potrzeb obu istniejących drzew, `Parse`, `ReadDocument`, `ReadElement`, operacje zmiany, `WriteTo`, `NewReader`, limit węzłów, rejestr identyfikatorów, `TestTreeRoundTrip` (K3), `FuzzTree` | 10–14 h | v0.4.0 | wykonany 2026-11-27 (ADR 0006 i 0007, dowody w `testing.md`); wydany jako v0.4.0 |
| 5 | strumień i utwardzanie: wzorce przelot, filtr, wyspa z przykładami; plik 1 GB (K7), wrogie wejścia i przypadki patologiczne (K6), kampania fuzzingu 24 h na Synology (K5), `tools/bench` i publikacja pomiarów, `tools/evidence` (K9) | 6–10 h | v0.5.0 | w toku od 2026-11-27 (ADR 0007 i 0008): K6, K7, wzorce, porównanie i pakiet dowodów zrobione; brakuje fuzzingu 24 h (K5) |
| 6 | konsumenci (w ich sesjach, `migration.md`): `goxsdgo` i `goxadesgo` na `goxmlgo`, pakiet aliasów `goxsdgo/xmlreader`, zastąpienie obu drzew przez `xmltree`, pomiar K8, pakiet dowodów | 4–6 h | v1.0.0 po K1–K9 | — |
| | **razem** | **39–60 h** | | |

Rozmiar docelowy: 6–8 tys. linii kodu bez testów (czytnik i klasy znaków ok. 2,9 tys. już istnieją).

## Kolejność i bramki

- repozytorium jest publiczne od v0.1.0 (ADR 0002), bo publiczne `goxsdgo` nie może zależeć od modułu
  prywatnego; założenie repozytorium i pierwsze wypchnięcie na prośbę właściciela;
- `goxsdgo` i `goxadesgo` przełączają import czytnika na v0.2.0 (pierwsza część etapu 6; nie na v0.1.0, która nie
  ma poprawki deklaracji XML, `migration.md`). Warto to zrobić od razu: do tej chwili istnieją dwie kopie czytnika;
- **po etapie 2 (v0.2.0)** rusza etap 1 `godocxgo` (potrzebuje `Skip`, `Reset`, limitu elementów);
- etapy 3 i 4 w tej kolejności: drzewo zapisuje się przez pisarza;
- druga część etapu 6 (zastąpienie drzew) potrzebuje etapu 4.

## Wydania

Tagi podpisane (`git tag -s vX.Y.Z -m vX.Y.Z`, sprawdzenie `git tag -v` przed wypchnięciem), `CHANGELOG.md` od
v0.1.0 z informacją, że wszystkie tagi są podpisane. Wersje 0.x: API może rosnąć, zmiany niezgodne tylko z wpisem
w ADR i w `CHANGELOG.md`. Wersja 1.0.0 po spełnieniu K1–K9 i przełączeniu konsumentów.
