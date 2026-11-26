# Changelog

All release tags of this repository are signed (`git tag -s`); check one with `git tag -v vX.Y.Z`.

## Unreleased (to be v0.1.0)

The XML reader moves here from goxsdgo.

- `xmlreader`: the streaming XML 1.0 reader of goxsdgo v0.3.0, with the same API, the same error codes and the
  same behaviour; only the import path changes (`github.com/polestorm/goxmlgo/xmlreader`). W3C XML Conformance:
  1725 of the 1727 cases in scope.
- `xmlchars`: the name character classes of XML 1.0 Fifth Edition, public (in goxsdgo it was
  `internal/xmlchars`).
- Error codes are listed in `docs/errors.md`; the public API of each package is recorded in `api/`.
