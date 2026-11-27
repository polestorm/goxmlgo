# goxmlgo

XML in pure Go, written from the W3C recommendations (XML 1.0 Fifth Edition, Namespaces in XML 1.0 Third Edition):
a streaming reader, a streaming writer and an editable in-memory tree that share one token type. No cgo, no
dependencies outside the Go standard library, and no use of `encoding/xml`.

- [Comparison](#comparison) · [Status](#status) · [Packages](#packages) · [Use](#use) ·
  [What it is for](#what-it-is-for) · [Corpus and tests](#corpus-and-tests) ·
  [How the comparison was measured](#how-the-comparison-was-measured) · [Licence](#licence)

## Comparison

etree and xmlquery are the two usual tree libraries for XML in Go; both are built on `encoding/xml` of the
standard library. They were only run as programs: goxmlgo is written from the specifications, not from the code
of other parsers ([docs/provenance.md](docs/provenance.md)).

| | goxmlgo | etree 1.8.1 | xmlquery 1.5.2 |
|---|:---:|:---:|:---:|
| Well-formed documents of the W3C suite read[^conf] | 775 of 776 🥇 | 712 of 776 | 711 of 776 |
| Ill-formed documents of the W3C suite rejected | 951 of 951 🥇 | 410 of 951 | 432 of 951 |
| Dependencies[^deps] | 0 🥇 | 0 🥇 | 3 |
| Lines of code[^size] | 6,227 | 3,059 🥇 | 180,877 |
| Added to a program | 439 kB 🥇 | 475 kB | 3,177 kB |
| Time to build a tree, 8.8 MB of records[^bench] | 47 ms 🥇 | 105 ms | 156 ms |
| Time to build a tree, 9.3 MB of markup with many attributes | 99 ms 🥇 | 127 ms | 209 ms |
| Time to write the tree, 8.8 MB / 9.3 MB | 15.5 🥇 / 28.4 ms 🥇 | 15.5 🥇 / 31.3 ms | 36.2 / 80.3 ms |
| Allocations to build a tree, 8.8 MB | 1,566 🥇 | 1,660,102 | 2,980,119 |
| Memory held by the tree, 8.8 MB / 9.3 MB[^mem] | 42 / 67 MB | 38 / 61 MB 🥇 | 59 / 78 MB |
| Reading without a tree, in constant memory[^feat] | yes: 1 GB in 14 MB | no | yes, a stream parser |
| Writing without a tree | yes | no | no |
| One element of a stream as a tree, the rest passed through | yes | no | not in our probe |
| Tree can be changed and written back | yes | yes | yes |
| Path or XPath queries | no | yes, paths | yes, XPath |
| Line and column of every node and attribute | yes | no | not in our probe |
| Errors with a fixed code, line and column | yes | no | no |
| Limits on depth, sizes, entity expansion, on by default | yes | no | no |
| The same bytes for the same input, documented and tested | yes | not documented | not documented |
| Licence | Apache 2.0 | BSD 2-Clause | MIT |
| GitHub popularity[^pop] | new | 1,673 stars | 490 stars |

[^conf]: The 1,727 cases of the W3C XML Conformance Test Suite (20130923) that apply to XML 1.0 Fifth Edition and
    Namespaces 1.0 without external entities: 776 well-formed, 951 not. A well-formed document should give a
    tree, an ill-formed one an error. The one well-formed document goxmlgo does not read is described in
    [docs/differences.md](docs/differences.md). Raw result:
    [docs/results/tree-conformance.tsv](docs/results/tree-conformance.tsv).
[^deps]: Modules required in the `go.mod` of each library.
[^size]: Lines of the packages outside the standard library that a "document from standard input into a tree and
    out again" program pulls in, without tests. etree and xmlquery parse with `encoding/xml`, which is part of the
    standard library and is not counted in their lines; goxmlgo has its own parser. "Added to a program" is the
    size of that program minus a program that only copies its input (`-trimpath -ldflags="-s -w"`, macOS arm64),
    and does count everything. Raw result: [docs/results/tree-sizes.tsv](docs/results/tree-sizes.tsv).
[^bench]: Two documents made by generators of this repository: 20,000 records in the style of a tax ledger
    (8.8 MB, text in elements, few attributes) and 30,000 paragraphs in the style of a word-processor document
    (9.3 MB, many attributes with prefixes). Median of 9 rounds, one goroutine, Apple M4, Go 1.26.6, measured on
    2026-11-27. 🥇 marks the best value in a row; values within 1% of each other share it. Raw result:
    [docs/results/tree-vs-others.txt](docs/results/tree-vs-others.txt).
[^mem]: Heap in use by the built tree after a collection. goxmlgo keeps the line and column of every node and
    attribute, attribute types from the DTD and names with prefix and namespace name; in v0.4.0 its trees took
    60 and 107 MB.
[^feat]: The rows from here down are not measurements. Those about goxmlgo are covered by its tests; those about
    the other two come from their documentation and were not tested, and "not in our probe" means we did not
    find it there. The 1 GB figure: [docs/results/streaming-memory.txt](docs/results/streaming-memory.txt).
[^pop]: Stars on 2026-11-27.

Against `encoding/xml` itself, reading token by token: goxmlgo reads the same two documents at 254 and 122 MB/s,
`encoding/xml` at 90 and 81 MB/s, and of the 951 ill-formed documents `encoding/xml` rejects 434
([xmlreader/README.md](xmlreader/README.md)).

## Status

Version 0.5.0. The API may still change before 1.0. What is left before 1.0 is a 24-hour fuzzing campaign and the move of the first consumers
([docs/roadmap.md](docs/roadmap.md), [docs/CHANGELOG.md](docs/CHANGELOG.md)). Requires Go 1.26 or newer.

## Packages

| Package | What it does |
|---|---|
| `xmlreader` | pull reader: every well-formedness and namespace constraint checked, line and column of every token and error, fixed error codes, limits on by default, no external entities, constant memory |
| `xmlwriter` | streaming writer: calls or tokens in, escaping, namespace declarations as given or added automatically, output that is well-formed or an error with a fixed code, the same bytes for the same calls, no allocation per call |
| `xmltree` | in-memory tree that can be changed and written back: a whole document, or one element taken out of a stream; nodes can be moved and copied between places and documents and are written with the namespace declarations they need |
| `xmlchars` | the name character classes of XML 1.0 |

## Use

Reading a document, changing a part of it and writing it back, with one row in memory at a time:

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
		row, err := xmltree.ReadElement(r, t, xmltree.Options{}) // only this subtree is held in memory
		if err != nil {
			return err
		}
		row.SetAttr(xmlreader.Name{Local: "checked"}, "yes")
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

A whole document as a tree: `doc, err := xmltree.Parse(in, xmltree.Options{})`, then `doc.Root()`, `Append`,
`InsertBefore`, `Remove`, `SetText`, `SetAttr`, and `doc.WriteTo(w)`. More examples are in the documentation of
each package (`Example*`).

## What it is for

- **one token type** for reading, writing and the tree, so the stages of a pipeline (a reader, a validating
  reader, a filter, a writer) fit together;
- **documents of any size**: reading and writing take memory that depends on the depth and the longest token, not
  on the size of the document; a tree only where the caller asks for one;
- **no silent loss**: what is read and written back gives the same tokens when read again; the writer reports
  what it cannot write instead of repairing it. What is deliberately not kept (quotes, white space inside tags,
  the DOCTYPE declaration) is listed in [docs/design.md](docs/design.md);
- **input that is not trusted**: no network, no files, external entities never read, limits on depth, sizes,
  entity expansion and namespace declarations in scope, on by default; hostile inputs are rejected with a fixed
  code in well under a second ([docs/errors.md](docs/errors.md));
- **the same bytes** for the same input, on every platform.

Not in scope: XML 1.1, DTD validation, XPath, XSLT, XSD validation (that is
[goxsdgo](https://github.com/polestorm/goxsdgo)), decoding into Go structs.

## Corpus and tests

| What | Input | Result |
|---|---|---|
| Conformance of the reader | W3C XML Conformance Test Suite 20130923, the 1,727 cases in scope | 1,725 pass; the two that do not are described in [docs/differences.md](docs/differences.md) |
| Round trip: read, write, read | the 775 documents of the suite that the reader accepts, and the 13 documents of the corpus | the same tokens for every document; a second write gives the same bytes |
| Tree: build, write | the same 788 documents | the same bytes as the reader to writer pass-through; 1,156 single elements built as trees in the middle of a stream without changing the output |
| Hostile and pathological input | 26 families made by the test (entity expansion, a million nested elements or attributes, tokens over the limits, piled-up namespace declarations) and 15 families for linear time | each rejected with the right code, the slowest in 0.18 s; doubling the size multiplies the time by at most 2.4 (the limit is 2.5) |
| Memory of a stream | 1 GB generated while it is read | 14 MB for the process, 1.2 MB of heap |
| Fuzzing | four fuzzers: reader, round trip, writer calls, tree changes | the journal with every run is [docs/fuzzing.md](docs/fuzzing.md) |
| Requirements | 100 accepted requirements in [requirements/](requirements/) | each has a test that names it; the public API is compared with a recorded snapshot |

The corpus (`testdata/corpus`) is 13 synthetic documents, written for this repository, each with its expected
tokens and expected output; the generators of the large documents are in `internal/testkit`. The W3C suite is not
part of the repository: `go -C tools/fetch run . -root ../..` (or `python tools/fetch/fetch.py`) downloads it and
checks its checksums, and then `go test ./...` runs everything above except the 1 GB run and fuzzing. How each
kind of test works: [docs/testing.md](docs/testing.md).
`go -C tools/evidence run . -root ../..` builds the evidence package: every requirement with its tests and their
results, and the acceptance criteria with the source of each number.

## How the comparison was measured

Every number in this file comes from a tool of this repository and a raw result kept in
[docs/results](docs/results/README.md); versions of the candidates, their settings, the commands and the
statistics are in [docs/methodology.md](docs/methodology.md). To repeat the comparison:

```sh
go -C tools/fetch run . -root ../..                                  # the W3C suite
go -C tools/compare run . -root ../..                                # speed, allocations, memory
go -C tools/compare test . -run 'TestCandidatesConformance|TestSizes' -v
```

The documents are generated, so nothing private and nothing large is needed. The machine was a laptop also used
for other work; between sessions absolute times move by a few percent, the order of the candidates did not.

## Licence

Apache 2.0.
