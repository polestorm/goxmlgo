# Changelog

All release tags of this repository are signed (`git tag -s`); check one with `git tag -v vX.Y.Z`.

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
