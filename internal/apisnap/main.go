// Command apisnap wypisuje wzorzec publicznego API pakietu z podanego katalogu (testkit.APISnapshot). Służy do
// zrobienia wzorca dla kodu spoza tego modułu, np. czytnika goxsdgo w commicie źródłowym (docs/provenance.md):
//
//	go run ./internal/apisnap <katalog> <ścieżka importu>
package main

import (
	"fmt"
	"os"

	"github.com/polestorm/goxmlgo/internal/testkit"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: apisnap DIR IMPORTPATH")
		os.Exit(2)
	}
	s, err := testkit.APISnapshot(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "apisnap:", err)
		os.Exit(1)
	}
	fmt.Print(s)
}
