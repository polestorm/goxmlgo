# Wymagania: pakiet dowodów

Status: szkic

Kryterium K9 (ADR 0001). Narzędzie `tools/evidence` przenosimy z `goxsdgo` i dostosowujemy; wymagania EVD-001…005
tamże są wzorem.

| Id | Wymaganie |
|---|---|
| EVD-001 | Pakiet zawiera surowy wynik `go test -count=1 -json ./...` modułu głównego i modułów narzędzi oraz wynik `tools/fetch -verify`, z commitem (i oznaczeniem zmian w drzewie roboczym), wersją Go, systemem i datą. |
| EVD-002 | Pakiet zawiera macierz śledzenia: każde przyjęte wymaganie z `requirements/*.md` z testami, które się na nie powołują przez `REQ:`, i z wynikiem tych testów w przebiegu. Wymaganie bez testu albo z testem pominiętym jest wykazane jako niespełnione. |
| EVD-003 | Pakiet zawiera tabelę K1–K9: wartość, cel, stan i źródło. Wartości z przebiegów testów liczy z ich wyjścia; pomiary spoza `go test` (K5, K7, K8 u konsumentów) bierze z plików w `docs/` z datą i commitem pomiaru. |
| EVD-004 | Dla tego samego commitu i tych samych danych raport i jego postać maszynowa różnią się tylko datą wygenerowania; czasy przebiegów są wyłącznie w plikach surowych. |
| EVD-005 | Narzędzie nie sięga do sieci, nie uruchamia fuzzingu i nie czyta danych spoza repozytorium. Kończy się kodem 1, gdy K4 albo K9 nie są spełnione; pakiet zapisuje także wtedy. |
