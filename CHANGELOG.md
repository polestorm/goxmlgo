# Changelog

All release tags of this repository are signed (`git tag -s`); check one with `git tag -v vX.Y.Z`.

## Unreleased (to be v0.3.0)

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
