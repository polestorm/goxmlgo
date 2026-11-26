# Wymagania: łańcuchy i zapis powrotny

Status: szkic

Definicja równości strumieni tokenów: `../docs/design.md`.

| Id | Wymaganie |
|---|---|
| STR-001 | Zapis powrotny: dla każdego dokumentu przyjętego przez czytnik strumień tokenów po odczyt → zapis → odczyt jest równy strumieniowi pierwszego odczytu. Wyjątki są wymienione w `docs/differences.md` z powodem. |
| STR-002 | Idempotencja: zapis(odczyt(y)) = y co do bajta dla każdego y będącego wynikiem zapisu. |
| STR-003 | Przelot czytnik → pisarz działa w pamięci niezależnej od rozmiaru dokumentu (zależnej od głębokości i najdłuższego tokenu) i bez alokacji na token. |
| STR-004 | Filtr: usunięcie elementu (przez `Skip` zamiast zapisu), zmiana wartości atrybutu albo tekstu i dopisanie elementów w strumieniu dają dokument dobrze uformowany; przykłady w `Example*`. |
| STR-005 | Wyspa: `ReadElement` → zmiana → `WriteTo` w środku przelotu daje dokument, który poza zmienionym poddrzewem ma strumień tokenów równy wejściu; pamięć zależy od największego zmaterializowanego poddrzewa, nie od rozmiaru dokumentu. |
| STR-006 | Dokument w UTF-16 albo ISO-8859-1 po przelocie jest zapisany w UTF-8 z tą samą treścią; deklaracja XML na wyjściu nie podaje kodowania wejścia. |
| STR-007 | Źródła tokenów inne niż czytnik (drzewo, filtr wywołującego) dają przez pisarza ten sam wynik co czytnik dla tego samego strumienia: pisarz polega wyłącznie na tokenach, nie na typie źródła. |
