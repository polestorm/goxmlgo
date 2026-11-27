// Program odniesienia dla pomiaru rozmiaru: kopiuje wejście na wyjście, bez żadnej biblioteki XML.
package main

import (
	"io"
	"os"
)

func main() {
	if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
		os.Exit(1)
	}
}
