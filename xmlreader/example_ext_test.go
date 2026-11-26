package xmlreader_test

import (
	"fmt"
	"io"
	"strings"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// Skip passes over an element the caller does not need; its content is still checked.
func ExampleReader_Skip() {
	doc := `<order><meta><audit user="x"><step/><step/></audit></meta><item>pen</item><item>ink</item></order>`
	r := xmlreader.New(strings.NewReader(doc), xmlreader.Options{})
	for {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		switch {
		case t.Kind == xmlreader.KindStartElement && t.Name.Local == "meta":
			if err := r.Skip(); err != nil {
				fmt.Println(err)
				return
			}
		case t.Kind == xmlreader.KindText:
			fmt.Printf("%s\n", t.Data)
		}
	}
	// Output:
	// pen
	// ink
}

// One Reader reads many small documents; Reset keeps its buffers.
func ExampleReader_Reset() {
	var r xmlreader.Reader
	for _, doc := range []string{`<a n="1"/>`, `<b><c/></b>`, `<broken>`} {
		r.Reset(strings.NewReader(doc), xmlreader.Options{})
		elements := 0
		for {
			t, err := r.Next()
			if err == io.EOF {
				fmt.Println(elements, "elements")
				break
			}
			if err != nil {
				fmt.Println(err)
				break
			}
			if t.Kind == xmlreader.KindStartElement {
				elements++
			}
		}
	}
	// Output:
	// 1 elements
	// 2 elements
	// 1:9: xml-eof: element <broken> not closed
}

// A function that sees only tokens keeps track of the namespaces in scope with a NamespaceScope.
func ExampleNamespaceScope() {
	doc := `<doc xmlns:t="urn:types"><field type="t:money"><inner xmlns:t="urn:other" type="t:money"/></field></doc>`
	var src xmlreader.TokenReader = xmlreader.New(strings.NewReader(doc), xmlreader.Options{})
	var scope xmlreader.NamespaceScope
	for {
		t, err := src.Next()
		if err != nil {
			break
		}
		scope.Update(t)
		if t.Kind != xmlreader.KindStartElement {
			continue
		}
		for _, a := range t.Attrs {
			if a.Name.Local == "type" {
				prefix, local, _ := strings.Cut(a.Value, ":")
				uri, _ := scope.Lookup(prefix)
				fmt.Printf("%s: {%s}%s\n", t.Name.Local, uri, local)
			}
		}
	}
	// Output:
	// field: {urn:types}money
	// inner: {urn:other}money
}

// The XML declaration is available before the first token.
func ExampleReader_Declaration() {
	r := xmlreader.New(strings.NewReader(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><a/>`), xmlreader.Options{})
	d, err := r.Declaration()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(d.Version, d.Encoding, d.Standalone, d.InputEncoding, d.BOM)
	// Output:
	// 1.0 UTF-8 yes UTF-8 false
}
