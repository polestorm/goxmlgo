// Program pomiaru rozmiaru: dokument z wejścia do drzewa beevik/etree i z powrotem na wyjście.
package main

import (
	"os"

	"github.com/beevik/etree"
)

func main() {
	doc := etree.NewDocument()
	if _, err := doc.ReadFrom(os.Stdin); err != nil {
		os.Exit(1)
	}
	if _, err := doc.WriteTo(os.Stdout); err != nil {
		os.Exit(1)
	}
}
