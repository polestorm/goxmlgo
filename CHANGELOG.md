# Changelog

All release tags of this repository are signed (`git tag -s`); check one with `git tag -v vX.Y.Z`.

## Unreleased

Hardening. The API is that of v0.4.0.

- The module needs Go 1.26 (v0.4.0 and earlier: Go 1.22). A module that depends on it has to declare at least
  the same version.
- `xmltree` takes less memory: an element shares the record of its name with the other elements of that name
  and keeps namespace data only when it has any. On the generated documents a tree takes 4.8 and 8.1 times the
  size of the document (v0.4.0: 6.8 and 11.5). No change of behaviour.

- Tests and examples of the stream patterns: a filter between a reader and a writer, and an island (one element
  built as a tree in the middle of a pass-through) (`Example_filter`, `Example_island`).
- Measured: 1 GB from the reader to the writer in 14 MB of process memory, 1.2 MB of heap
  (`docs/results/streaming-memory.txt`).
- Tests of determinism (10 runs, input split in any way) and of parallel use of separate readers, writers and
  trees.
- `tools/compare`: the tree next to `beevik/etree` and `antchfx/xmlquery` on generated documents and on the W3C
  suite; method in `docs/methodology.md`, raw results in `docs/results/`.
- `tools/evidence`: the evidence package (requirements, tests, the table of criteria K1–K9).

## v0.4.0 (2026-11-27)

The tree.

- `xmltree`: an in-memory tree that can be changed and written back. `Parse` and `ReadDocument` build a whole
  document, `ReadElement` one element in the middle of a stream, leaving the stream right after it; `WriteTo`
  writes through an `xmlwriter.Writer`, `NewReader` gives the tokens of a tree. A tree that was not changed is
  written with the same bytes as the stream it was built from, for every document of the W3C XML Conformance
  Test Suite that the reader accepts (775) and of the corpus.
- A node is read and changed through methods only, so the links between nodes are always consistent; adding,
  inserting and removing a child take constant time. `Append`, `InsertBefore`, `Remove`, `Replace`, `SetText`,
  `SetAttr`, `RemoveAttr`, `Clone` and setters for the data of a node. A node moved or copied to a place with
  other namespace declarations is written with the declarations it needs.
- Nodes, attributes and text of a tree built from a source are allocated in blocks: about 0.01 allocations per
  node on a document without attributes.
- A limit on the number of nodes, on by default (`Options.MaxNodes`); an optional index of identifiers
  (`Document.ByID`); errors with fixed codes (`docs/errors.md`).
- Added to `xmlreader`: `Reader.InScopeNamespaces`. Added to `xmlwriter`: `Writer.LookupNamespace`.
- Changed in `xmlreader` and `xmlwriter`: a new limit, `Limits.MaxNamespaces` (256 by default), on the number of
  namespace declarations in scope at one time, with the code `limit-namespaces`. Without it a document within
  all other default limits could slow the reader down about 150 times (2 million declarations in scope, searched
  for every prefix). A document with more declarations in scope, accepted until v0.3.0, is now rejected unless
  the limit is raised or turned off.
- Changed in `xmlreader`: the declaration `xmlns:xml="http://www.w3.org/XML/1998/namespace"` is returned among the
  namespace declarations of its element (`Token.Namespaces`, `DeclaredNamespaces`) like any other; until v0.3.0
  it was left out, so it was lost when a document was read and written back. The scope does not change.
- Tests of hostile and pathological inputs for the reader, the writer and the tree (`safety_test.go`).

## v0.3.0 (2026-11-27)

The writer.

- `xmlwriter`: a streaming writer. `Start`, `Attr`, `Text`, `CDATA`, `Comment`, `ProcInst`, `End`, `EndEmpty`
  write a document call by call; `WriteToken` writes the tokens of a reader, so reader → writer is a pass-through
  in constant memory. The output is well-formed or the call fails with a fixed code (`docs/errors.md`) and
  writes nothing; characters, names and namespace bindings are checked, never repaired.
- Namespaces as given, or declared by the writer where they are first used (`Options.AutoNamespaces`);
  indentation as an option (`Options.Indent`); limits with the defaults of the reader.
- Round trip: every document of the W3C XML Conformance Test Suite that the reader accepts (775) and of the corpus
  reads, writes and reads back to the same tokens, and a second write gives the same bytes.
- Changed in `xmlreader`: a reference to an entity that gives no character before a tag (an empty entity, or one
  that starts with a tag) no longer produces a text token with empty data. A text token is now never empty,
  except for an empty CDATA section. See `docs/differences.md`.

## v0.2.0 (2026-11-26)

Additions to the reader; a program written for v0.1.0 compiles and behaves as before.

- `Reader.Skip` passes over the rest of an element; its content is checked as when it is read.
- `Reader.Reset` reuses a Reader and its buffers for the next document.
- `Limits.MaxElements` limits the number of elements in a document (off by default); new code `limit-elements`.
- `Token.Namespaces` holds the namespace declarations of a start tag; `NamespaceScope` tracks namespaces in scope
  from tokens alone; the interface `TokenReader`.
- `Reader.Declaration` returns the XML declaration and the encoding of the input.
- The documentation states which parts of a token may be kept (strings) and which are reused (slices).
- New code `usage-skip` for `Skip` called in the wrong state.
- Fixed: a panic or a false `xml-decl` error when the XML declaration arrived in small reads, and a false
  `xml-eof` for very small documents with white space before `?>` in the declaration (both present in v0.1.0 and
  in goxsdgo v0.3.0). The error code for an illegal ASCII character right after a name no longer depends on
  where a read ended. See `docs/differences.md`.
- The reader is about 20% faster than in v0.1.0 (`docs/results/reader-vs-goxsdgo.txt`).

## v0.1.0 (2026-11-26)

Tagged on commit `0648973`, where this file still says "Unreleased".

The XML reader moves here from goxsdgo.

- `xmlreader`: the streaming XML 1.0 reader of goxsdgo v0.3.0, with the same API, the same error codes and the
  same behaviour; only the import path changes (`github.com/polestorm/goxmlgo/xmlreader`). W3C XML Conformance:
  1725 of the 1727 cases in scope.
- `xmlchars`: the name character classes of XML 1.0 Fifth Edition, public (in goxsdgo it was
  `internal/xmlchars`).
- Error codes are listed in `docs/errors.md`; the public API of each package is recorded in `api/`.
