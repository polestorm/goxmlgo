// Program pomiaru rozmiaru: dokument z wejścia do drzewa goxmlgo i z powrotem na wyjście.
package main

import (
	"os"

	"github.com/polestorm/goxmlgo/xmltree"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

func main() {
	doc, err := xmltree.Parse(os.Stdin, xmltree.Options{})
	if err != nil {
		os.Exit(1)
	}
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	if doc.WriteTo(w) != nil || w.Close() != nil {
		os.Exit(1)
	}
}
