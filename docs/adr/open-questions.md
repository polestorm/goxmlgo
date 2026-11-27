# Pytania otwarte

Kolejność według etapu, w którym pytanie blokuje pracę.

1. ~~Widoczność repozytorium.~~ Rozstrzygnięte 2026-11-26: publiczne od v0.1.0
   ([ADR 0002](0002-repository-token-and-fidelity.md)).
2. ~~Deklaracje przestrzeni nazw w tokenie.~~ Rozstrzygnięte 2026-11-26: pole w tokenie początku elementu i typ
   śledzący zakres ([ADR 0002](0002-repository-token-and-fidelity.md)).
3. ~~Poziom wierności zapisu powrotnego.~~ Rozstrzygnięte 2026-11-26: treść i prefiksy, nie bajty; `SelfClosing`
   zachowany; DOCTYPE nie jest zapisywany w 1.0 ([ADR 0002](0002-repository-token-and-fidelity.md)).
4. **Kopiowanie surowych bajtów poddrzewa.** Przy edycji DOCX nieznane elementy powinny przejść nietknięte, co do
   bajta. Wymaga od czytnika wydania zakresu bajtów elementu; komplikuje się przy wejściu w UTF-16 i przy
   prefiksach zadeklarowanych powyżej poddrzewa. Rekomendacja: po 1.0, gdy powstanie pierwszy konsument piszący
   (edytor DOCX albo XLSX), z osobnym ADR.
5. **Kodowania.** Wejście: dodać ISO-8859-2 i windows-1250 (stare pliki polskie)? Rekomendacja z ADR 0004
   w `goxsdgo` zostaje: gdy pojawi się odbiorca, z tablic Unicode. Wyjście: tylko UTF-8 (rekomendacja), UTF-16
   na żądanie konsumenta.
6. ~~Wcięcia w pisarzu.~~ Rozstrzygnięte 2026-11-27: reguła wykonalna dla pisarza strumieniowego, a jej algorytm
   jest częścią kontraktu powtarzalności ([ADR 0005](0005-writer.md), pkt 10).
7. ~~Gdzie mieszkają typy wspólne.~~ Rozstrzygnięte 2026-11-27: zostają w `xmlreader`, `xmlwriter` ma aliasy;
   osobnego pakietu na typy na razie nie wydzielamy ([ADR 0005](0005-writer.md), pkt 2).
8. **Canonical XML.** `goxadesgo/c14n` to serializacja drzewa według normy W3C, czyli rzecz ogólna. Rekomendacja:
   osobna decyzja po etapie 4, gdy `goxadesgo` przejdzie na `xmltree`; do tego czasu poza zakresem.
9. ~~Kształt API drzewa.~~ Rozstrzygnięte 2026-11-27 przez właściciela: węzeł dostępny wyłącznie przez metody
   ([ADR 0006](0006-tree.md), pkt 1).
10. **Wybieranie węzłów.** Proste ścieżki (`a/b/@c`) albo podzbiór XPath. `goxsdgo` ma własny podzbiór dla
    ograniczeń tożsamości, `goxadesgo` potrzebuje wyszukiwania po identyfikatorze. Rekomendacja: poza zakresem,
    dopóki nie ma dwóch konsumentów tej samej funkcji; w 1.0 tylko iteracja po dzieciach i `ByID`.
11. **Najniższa wersja Go.** `goxsdgo` deklaruje 1.22, `goxadesgo` 1.24. Iteratory (`range` po funkcji) wymagają
    1.23. Rekomendacja: 1.22 i API bez iteratorów, żeby nie podnosić wymagań konsumentom.
12. **Jak długo żyje pakiet aliasów `goxsdgo/xmlreader`.** Decyzja należy do `goxsdgo`; rekomendacja: do `goxsdgo`
    1.0, oznaczony jako przestarzały od pierwszego wydania po przełączeniu.
13. ~~Cel szybkości przelotu (K7).~~ Rozstrzygnięte 2026-11-27: próg względny zamiast „≥ 100 MB/s”: przelot
    czytnik → pisarz ≥ 0,55× szybkości samego czytnika na każdym dokumencie wzorcowym i nie wolniejszy niż przelot
    `encoding/xml`, w jednym przebiegu `tools/bench` ([ADR 0005](0005-writer.md), pkt 13).
14. ~~Pamięć drzewa.~~ Rozstrzygnięte 2026-11-27: priorytety to bezpieczeństwo, szybkość, pamięć; układ węzła
    jest prywatny, więc można go zmieniać także po 1.0 ([ADR 0006](0006-tree.md), pkt 15).
15. ~~Deklaracja `xmlns:xml` w zapisie powrotnym.~~ Rozstrzygnięte 2026-11-27: czytnik oddaje ją jak każdą inną
    deklarację, bez wyjątku ([ADR 0006](0006-tree.md), pkt 16).
16. **Porównanie drzewa z innymi bibliotekami.** Kandydaci w Go uruchamiani jako czarna skrzynka w `tools/bench`
    (np. `beevik/etree`, `antchfx/xmlquery`), z liczbą przypadków W3C, które każdy przyjmuje błędnie; ewentualnie
    `pugixml` i `libxml2` z zastrzeżeniem o innym języku i mniejszym zakresie kontroli. Pomiar z OXml (Delphi,
    biblioteka komercyjna) nie jest powtarzalny z zewnątrz, więc najwyżej wewnętrzny. Do decyzji przed etapem 5.
17. ~~Limit deklaracji przestrzeni nazw w zakresie.~~ Potwierdzony przez właściciela 2026-11-27, z wartością
    domyślną 256 zamiast 1024 ([ADR 0007](0007-hostile-inputs.md)).
18. **Pamięć drzewa przy limicie domyślnym** (2 mln węzłów to do ok. 420 MB dla 8 MB wejścia) i koszt prefiksów
    automatycznych pisarza: [ADR 0007](0007-hostile-inputs.md), „Co zostaje do etapu 5”.
