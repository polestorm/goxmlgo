package xmltree_test

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmltree"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

// A document read into a tree, changed and written back. What was not changed is written as it was read.
func ExampleParse() {
	const src = `<?xml version="1.0"?>
<order xmlns="urn:example:order" id="7"><item qty="1">pen</item><!-- more to come --><note/></order>`

	doc, err := xmltree.Parse(strings.NewReader(src), xmltree.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	order := doc.Root()
	item := order.FirstChild()
	item.SetAttr(xmltree.Name{Local: "qty"}, "3")

	second := xmltree.NewElement(xmltree.Name{Space: "urn:example:order", Local: "item"})
	second.SetAttr(xmltree.Name{Local: "qty"}, "2")
	second.SetText("ink & paper")
	if err := order.InsertBefore(second, item.NextSibling()); err != nil {
		fmt.Println(err)
		return
	}
	order.Remove(order.LastChild()) // <note/>

	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	if err := doc.WriteTo(w); err != nil {
		fmt.Println(err)
		return
	}
	if err := w.Close(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <?xml version="1.0" encoding="UTF-8"?>
	// <order xmlns="urn:example:order" id="7"><item qty="3">pen</item><item qty="2">ink &amp; paper</item><!-- more to come --></order>
}

// One element at a time in memory: the document passes from the reader to the writer, and only the rows are
// built as trees, changed and written. The memory needed does not grow with the document.
func ExampleReadElement() {
	const src = `<rows xmlns:t="urn:example:types"><row n="1"><v type="t:int">10</v></row><skip/><row n="2"><v type="t:int">20</v></row></rows>`

	r := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	for {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		if t.Kind == xmlreader.KindStartElement && t.Name.Local == "row" {
			row, err := xmltree.ReadElement(r, t, xmltree.Options{})
			if err != nil {
				fmt.Println(err)
				return
			}
			// the row sees the namespaces declared above it
			uri, _ := row.LookupNamespace("t")
			row.SetAttr(xmltree.Name{Local: "types"}, uri)
			v := row.FirstChild()
			v.SetText(v.Text() + "0")
			if err := row.WriteTo(w); err != nil {
				fmt.Println(err)
				return
			}
			continue
		}
		if err := w.WriteToken(t); err != nil {
			fmt.Println(err)
			return
		}
	}
	if err := w.Close(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <rows xmlns:t="urn:example:types"><row n="1" types="urn:example:types"><v type="t:int">100</v></row><skip/><row n="2" types="urn:example:types"><v type="t:int">200</v></row></rows>
}

// A copy of an element written into another document: the namespace declarations it needs are added where it
// is written, and prefixes used in its text keep their meaning.
func ExampleNode_Clone() {
	src, _ := xmltree.Parse(strings.NewReader(
		`<lib xmlns:b="urn:example:book" xmlns:x="urn:example:types"><b:book b:lang="pl" kind="x:novel">Lalka</b:book></lib>`), xmltree.Options{})
	dst, _ := xmltree.Parse(strings.NewReader(`<shelf/>`), xmltree.Options{})

	book := src.Root().FirstChild().Clone()
	if err := dst.Root().Append(book); err != nil {
		fmt.Println(err)
		return
	}
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	if err := dst.WriteTo(w); err != nil {
		fmt.Println(err)
		return
	}
	w.Close()
	// Output:
	// <shelf><b:book xmlns:b="urn:example:book" xmlns:x="urn:example:types" b:lang="pl" kind="x:novel">Lalka</b:book></shelf>
}

// The index of identifiers.
func ExampleDocument_ByID() {
	doc, err := xmltree.Parse(strings.NewReader(`<doc><part Id="intro">Hello</part><part Id="body">World</part></doc>`),
		xmltree.Options{IDAttributes: []string{"Id"}})
	if err != nil {
		fmt.Println(err)
		return
	}
	if part, ok := doc.ByID("body"); ok {
		fmt.Println(part.Text(), part.Line(), part.Column())
	}
	_, err = xmltree.Parse(strings.NewReader(`<doc><a Id="x"/><b Id="x"/></doc>`), xmltree.Options{IDAttributes: []string{"Id"}})
	fmt.Println(err)
	// Output:
	// World 1 35
	// 1:20: id-duplicate: the identifier "x" is used by two elements
}

// An island: the document flows from the reader to the writer, and only the element to change is built as a
// tree. The memory needed is that of the largest island, not of the document.
func Example_island() {
	const src = `<list xmlns:p="urn:example:p"><p:item n="1"><p:v>10</p:v></p:item><other/><p:item n="2"><p:v>20</p:v></p:item></list>`

	r := xmlreader.New(strings.NewReader(src), xmlreader.Options{})
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	for {
		t, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		if t.Kind == xmlreader.KindStartElement && t.Name.Local == "item" {
			item, err := xmltree.ReadElement(r, t, xmltree.Options{})
			if err != nil {
				fmt.Println(err)
				return
			}
			v := item.FirstChild()
			sum := xmltree.NewElement(xmltree.Name{Space: "urn:example:p", Local: "double", Prefix: "p"})
			n, _ := strconv.Atoi(v.Text())
			sum.SetText(strconv.Itoa(2 * n))
			item.Append(sum)
			if err := item.WriteTo(w); err != nil {
				fmt.Println(err)
				return
			}
			continue
		}
		if err := w.WriteToken(t); err != nil {
			fmt.Println(err)
			return
		}
	}
	if err := w.Close(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <list xmlns:p="urn:example:p"><p:item n="1"><p:v>10</p:v><p:double>20</p:double></p:item><other/><p:item n="2"><p:v>20</p:v><p:double>40</p:double></p:item></list>
}
