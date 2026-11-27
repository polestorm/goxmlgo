package xmlwriter_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

// A document written with calls. The namespace declarations are given to Start; values are escaped by the
// writer.
func ExampleWriter() {
	const ns = "urn:example:invoice"
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	w.Declaration("")
	w.Start(xmlwriter.Name{Space: ns, Local: "invoice"}, xmlwriter.Binding{Prefix: "", URI: ns})
	w.Attr(xmlwriter.Name{Local: "number"}, `FV/2026/11/"7"`)
	w.Start(xmlwriter.Name{Space: ns, Local: "buyer"})
	w.Text("Żółw & Synowie <sp. z o.o.>")
	w.End()
	w.Start(xmlwriter.Name{Space: ns, Local: "paid"})
	w.EndEmpty()
	w.End()
	if err := w.Close(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <?xml version="1.0" encoding="UTF-8"?>
	// <invoice xmlns="urn:example:invoice" number="FV/2026/11/&quot;7&quot;"><buyer>Żółw &amp; Synowie &lt;sp. z o.o.&gt;</buyer><paid/></invoice>
}

// With AutoNamespaces the writer declares namespaces where they are first used, and with Indent it puts tags
// on lines of their own.
func ExampleOptions() {
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{AutoNamespaces: true, Indent: "  "})
	w.Start(xmlwriter.Name{Space: "urn:example:a", Local: "list"})
	w.Start(xmlwriter.Name{Space: "urn:example:b", Local: "item", Prefix: "b"})
	w.Attr(xmlwriter.Name{Space: "urn:example:b", Local: "id"}, "1")
	w.Text("first")
	w.End()
	w.Start(xmlwriter.Name{Space: "urn:example:a", Local: "note"})
	w.EndEmpty()
	w.End()
	if err := w.Close(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <list xmlns="urn:example:a">
	//   <b:item xmlns:b="urn:example:b" b:id="1">first</b:item>
	//   <note/>
	// </list>
}

// A pass-through with a filter: the tokens of a reader go to the writer, one element is dropped with Skip and
// one attribute value is changed. Memory does not depend on the size of the document.
func ExampleWriter_WriteToken() {
	const doc = `<?xml version="1.0"?>
<order id="7" xmlns:x="urn:example:x"><item x:state="new">tea &amp; milk</item><debug>internal</debug><item x:state="new"/></order>`

	r := xmlreader.New(strings.NewReader(doc), xmlreader.Options{})
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
		if t.Kind == xmlreader.KindStartElement {
			if t.Name.Local == "debug" {
				if err := r.Skip(); err != nil {
					fmt.Println(err)
					return
				}
				continue
			}
			for i := range t.Attrs {
				if t.Attrs[i].Name.Local == "state" {
					t.Attrs[i].Value = "sent"
				}
			}
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
	// <order xmlns:x="urn:example:x" id="7"><item x:state="sent">tea &amp; milk</item><item x:state="sent"/></order>
}

// The writer does not repair its input. A call that would break the document fails with a code, writes nothing,
// and the Writer goes on.
func ExampleError() {
	w := xmlwriter.New(os.Stdout, xmlwriter.Options{})
	w.Start(xmlwriter.Name{Local: "log"})
	err := w.Text("bell: \x07")
	var e *xmlwriter.Error
	if errors.As(err, &e) {
		fmt.Fprintln(os.Stderr, e.Code, e.Path)
	}
	w.Text("bell removed by the caller")
	w.End()
	w.Close()
	// Output:
	// <log>bell removed by the caller</log>
}
