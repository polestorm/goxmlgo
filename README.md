# goxmlgo

XML in pure Go, written from the W3C recommendations (XML 1.0 Fifth Edition, Namespaces in XML 1.0 Third Edition):
a streaming reader, a streaming writer and an editable in-memory tree that share one token type. No cgo, no
dependencies outside the Go standard library.

Status: the reader and the writer are here. The reader ([`xmlreader`](xmlreader/README.md), with a comparison
with `encoding/xml`) was moved from [`goxsdgo/xmlreader`](https://github.com/polestorm/goxsdgo/tree/main/xmlreader)
v0.3.0 with the same API ([`docs/provenance.md`](docs/provenance.md)) and has since gained `Skip`, `Reset` and
namespace declarations in the token. The writer (`xmlwriter`) writes calls or the tokens of a reader. The tree is
planned ([`docs/roadmap.md`](docs/roadmap.md)).

Packages:

| Package | What it does |
|---|---|
| `xmlreader` | pull reader: every well-formedness and namespace constraint checked, line and column of every token and error, fixed error codes, limits on by default, no external entities, constant memory |
| `xmlwriter` | streaming writer: calls or tokens in, escaping, namespace declarations as given or added automatically, output that is always well-formed or an error with a fixed code, the same bytes for the same calls, no allocation per call |
| `xmltree` (planned) | in-memory tree that can be changed and written back; a whole document or one subtree taken out of a stream |
| `xmlchars` | the name character classes of XML 1.0 |

Reading, filtering and writing back in constant memory works today (`Example*` in `xmlwriter`). Planned, with the
tree, changing a part of a document:

```go
r := xmlreader.New(in, xmlreader.Options{})
w := xmlwriter.New(out, xmlwriter.Options{})
for {
	t, err := r.Next()
	if err == io.EOF {
		break
	}
	if err != nil {
		return err // *xmlreader.Error with Code, Line, Column
	}
	if t.Kind == xmlreader.KindStartElement && t.Name.Local == "Row" {
		row, err := xmltree.ReadElement(r, t) // only this subtree is held in memory
		if err != nil {
			return err
		}
		fix(row)
		if err := row.WriteTo(w); err != nil {
			return err
		}
		continue
	}
	if err := w.WriteToken(t); err != nil {
		return err
	}
}
return w.Close()
```

Goals:

- **one token type** for reading, writing and the tree, so the stages of a pipeline (a reader, a validating
  reader, a filter, a writer) fit together;
- **constant memory** for documents of any size; a tree only where the caller asks for one;
- **no silent loss**: what is read and written back gives the same tokens when read again; the writer reports
  what it cannot write instead of repairing it;
- **safe by default**: no network, no files, external entities never read, limits on depth, sizes and entity
  expansion;
- **the same bytes** for the same input, on every platform.

Read and written back, every document of the W3C XML Conformance Test Suite that the reader accepts (775) and every
document of the corpus gives the same tokens, and a second pass gives the same bytes; what is deliberately not kept
(quotes, white space inside tags, the DOCTYPE declaration) is listed in [`docs/design.md`](docs/design.md). How
this and the speed of the pass-through are measured: [`docs/methodology.md`](docs/methodology.md), raw results in
[`docs/results/`](docs/results/README.md).

Not in scope: XML 1.1, DTD validation, XPath, XSLT, XSD validation (that is
[goxsdgo](https://github.com/polestorm/goxsdgo)), decoding into Go structs.

Tests need the W3C XML Conformance Test Suite, which is fetched, not committed:
`go -C tools/fetch run . -root ../..` (or `python tools/fetch/fetch.py`), then `go test ./...`.

Licence: Apache 2.0.
