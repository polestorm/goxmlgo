package xmlreader

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// chunked oddaje dane porcjami po n bajtów.
type chunked struct {
	data []byte
	n    int
}

func (c *chunked) Read(p []byte) (int, error) {
	if len(c.data) == 0 {
		return 0, io.EOF
	}
	k := min(c.n, len(c.data), len(p))
	copy(p, c.data[:k])
	c.data = c.data[k:]
	return k, nil
}

// safeDump to dumpTokensOpts, który panikę zamienia na opis (żeby test pokazał wszystkie przypadki naraz).
func safeDump(src io.Reader, opts Options) (out string) {
	defer func() {
		if p := recover(); p != nil {
			out = fmt.Sprintf("PANIC: %v", p)
		}
	}()
	return dumpTokensOpts(src, opts)
}

// TestChunkedInput: strumień tokenów i błędów nie zależy od podziału wejścia na odczyty. Każdy dokument korpusu
// i każdy przypadek W3C w zakresie jest czytany w całości, po jednym bajcie i porcjami po 7 bajtów.
// REQ: DET-003
func TestChunkedInput(t *testing.T) {
	bad := 0
	for _, d := range testDocs(t) {
		want := safeDump(bytes.NewReader(d.data), d.opts)
		for name, src := range map[string]io.Reader{
			"one byte at a time": iotest.OneByteReader(bytes.NewReader(d.data)),
			"7 bytes at a time":  &chunked{data: d.data, n: 7},
		} {
			if got := safeDump(src, d.opts); got != want {
				bad++
				if bad <= 15 {
					t.Errorf("%s, %s: stream differs from reading the whole input\n%s", d.id, name, firstDiff(got, want))
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d differences in total", bad)
	}
}

// TestXMLDeclSmallDocument: regresja błędu z v0.1.0 (i goxsdgo v0.3.0). Deklaracja XML z białym znakiem przed
// "?>" w dokumencie, którego reszta jest krótsza niż nazwa szukanego pseudoatrybutu, była odrzucana jako
// xml-eof, bo próba doczytania danych przesuwała bufor, a czytnik wracał do zapamiętanej w nim pozycji.
// REQ: XML-009
func TestXMLDeclSmallDocument(t *testing.T) {
	for doc, want := range map[string]string{
		`<?xml version="1.0" ?><a/>`:                  "eof",
		`<?xml version="1.0" ?><abc/>`:                "eof",
		`<?xml version="1.0" encoding="UTF-8" ?><a/>`: "eof",
		"<?xml version='1.0'\n\t?><a/>":               "eof",
		`<?xml version="1.0" ?>`:                      "error xml-no-root 1:23",
		`<?xml version="1.0" `:                        "error xml-eof 1:21",
		`<?xml version="1.0" x?><a/>`:                 "error xml-decl 1:21",
		`<?xml ?><a/>`:                                "error xml-decl 1:6",
	} {
		for name, src := range map[string]io.Reader{
			"whole":       bytes.NewReader([]byte(doc)),
			"one byte":    iotest.OneByteReader(bytes.NewReader([]byte(doc))),
			"three bytes": &chunked{data: []byte(doc), n: 3},
		} {
			got := safeDump(src, Options{})
			if last := got[strings.LastIndexByte(got[:len(got)-1], '\n')+1:]; last != want+"\n" {
				t.Errorf("%q (%s): %q, want %q", doc, name, last, want)
			}
		}
	}
}
