# Wymagania: klasy znaków

Źródło: XML 1.0 Fifth Edition, produkcje [2] Char, [3] S, [4] NameStartChar, [4a] NameChar.

## Przeniesione z `goxsdgo` (etap 1)

Status: przyjęte

| Id | Wymaganie |
|---|---|
| CH-001 | `IsNameStart5` i `IsNameChar5` zwracają dla każdego punktu kodowego Unicode (0…0x10FFFF) wynik zgodny z produkcjami [4] i [4a] piątego wydania; tablice przedziałów `NameStart5` i `NameCharExtra5`, przepisane z rekomendacji osobno, opisują te same zbiory (test porównuje obie postacie dla całej przestrzeni punktów kodowych). |
| CH-003 | Pakiet nie ma zależności (także od biblioteki standardowej) i jego funkcje nie alokują pamięci. |
| CH-004 | Zachowanie funkcji i zawartość tablic przeniesionych z `goxsdgo/internal/xmlchars` są identyczne z commitem źródłowym: różnica plików ogranicza się do komentarzy (`../docs/provenance.md`), a przeniesiony test przechodzi bez zmian. |

## Rozszerzenia (etap 2 albo później)

Status: szkic

Przeniesiony pakiet ma tylko klasy znaków nazw. Czytnik trzyma własne `isChar` i `isSpace` (pakiet `xmlreader`,
nieeksportowane); przeniesienie ich do `xmlchars` i funkcje na napisach to dodatki do API, potrzebne pisarzowi
(etap 3), do potwierdzenia razem z nazwami (dziś przyrostek `5` oznacza wydanie).

| Id | Wymaganie |
|---|---|
| CH-002 | Funkcje sprawdzające Name, NCName, QName i Nmtoken na napisie dają wynik zgodny z produkcjami [5], [7] oraz [4], [7] z Namespaces in XML; napis pusty i niepoprawny UTF-8 nie są nazwą. |
| CH-005 | Funkcje dla produkcji [2] Char i [3] S zwracają wynik zgodny z rekomendacją dla każdego punktu kodowego i dla wartości spoza zakresu; czytnik i pisarz używają tych samych funkcji. |
