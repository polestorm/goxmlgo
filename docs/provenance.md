# Code provenance

`goxmlgo` is written from scratch, from the W3C recommendations. This file says what we may use and what we may
not, and where the code that was moved here came from.

## Sources we write from

- Extensible Markup Language (XML) 1.0 Fifth Edition;
- Namespaces in XML 1.0 Third Edition;
- XML Information Set Second Edition, for the terms (what is content and what is spelling);
- Unicode, for the character classes the recommendations refer to.

The recommendations state the grammar, the well-formedness constraints and the namespace constraints. They are the
specification and may be applied directly.

## Code moved from goxsdgo

The reader (`xmlreader`) and the character classes (`xmlchars`) were written in
[goxsdgo](https://github.com/polestorm/goxsdgo) under the same rules (its `docs/provenance.md` and ADR 0004) and
were moved here in stage 1, on 2026-11-26, from a clean commit (`git archive`, not the working tree).

Source commit: goxsdgo `698affa5c57817c0c29be42b6493a2a6a3736382`. The moved files are byte-identical there and in
the release v0.3.0 (`cfb54cf`) and v0.4.0 (`4749d94`): `git diff v0.3.0 698affa -- xmlreader internal/xmlchars`
prints nothing.

| What | Where it was in goxsdgo | What changed in the move |
|---|---|---|
| `xmlreader/*.go` (code) | `xmlreader/` | the import path of `xmlchars` and one internal comment next to it (`chars.go`), the last sentence of the package comment (`reader.go`); nothing else |
| `xmlchars/xmlchars.go` | `internal/xmlchars/` | comments only: the package is public here, so its documentation is in English |
| `xmlreader/*_test.go`, `xmlreader/testdata/fuzz/` | `xmlreader/` | import paths; the seed directories of `FuzzReader`; one `REQ:` reference (`DET-001` there is `SAFE-004` here); `corpus_test.go` is new |
| `xmlchars/xmlchars_test.go` | `internal/xmlchars/` | the `REQ:` reference (`DT-019` there is `CH-001` here); `TestNoAllocations` is new |
| `requirements/xml.md`, XML-001…018 | `requirements/xml.md` | the table is unchanged |
| `internal/testkit` (ratchet, seed sampling) | `internal/testkit/` | the parts used only by the XSD tests removed; `api.go`, `gen.go` are new |
| `tools/fetch`, `tools/fuzz` | the same | module path, texts, the list of data directories |

The difference in the library code, to be repeated by anyone with both repositories side by side:

```sh
for f in chars.go dtd.go errors.go input.go reader.go; do
  git -C ../goxsdgo show 698affa:xmlreader/$f | diff - xmlreader/$f
done
git -C ../goxsdgo show 698affa:internal/xmlchars/xmlchars.go | diff - xmlchars/xmlchars.go | grep '^[<>]' | grep -v '^[<>] *//'
```

The first command prints three hunks (the import, the comment under it, the package comment); `dtd.go`, `errors.go`
and `input.go` are identical. The second prints nothing: every changed line of `xmlchars.go` is a comment line.
The other three proofs that the move changed nothing (the same conformance results case by case, the same public
API, the tests of goxsdgo passing on the moved reader) are recorded in [`testing.md`](testing.md).

From the move on, the reader in goxsdgo is frozen: fixes go here.

`tools/evidence` and `tools/bench` of goxsdgo will be copied and adapted in stage 5; they are ours as well.

## What we do not do

We do not read, copy or port the code of XML parsers, writers and tree libraries: libxml2, expat, Xerces (C and
Java), Saxon, lxml, ElementTree, quick-xml, xml-rs, or their ports, including those written in Go. The list is not
closed.

The reason is not the licence but three goals: code that is entirely ours and easy to show to be so, an evidence
package that ties the code to the rules of the recommendation, and the recommendation, not someone else's
implementation, as the reference.

## What we may do

- run other parsers and writers (for example `xmllint`) as black boxes and compare results;
- read their user documentation;
- read `encoding/xml` from the Go standard library and use it in tests as a reference point; the library packages
  do not import it.

## Test data

The W3C XML Conformance Test Suite is test data, not code. Sources, licences and checksums:
[`../testdata/README.md`](../testdata/README.md).
