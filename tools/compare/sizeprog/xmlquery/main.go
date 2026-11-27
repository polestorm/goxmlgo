// Program pomiaru rozmiaru: dokument z wejścia do drzewa antchfx/xmlquery i z powrotem na wyjście.
package main

import (
	"io"
	"os"

	"github.com/antchfx/xmlquery"
)

func main() {
	doc, err := xmlquery.Parse(os.Stdin)
	if err != nil {
		os.Exit(1)
	}
	if _, err := io.WriteString(os.Stdout, doc.OutputXML(true)); err != nil {
		os.Exit(1)
	}
}
