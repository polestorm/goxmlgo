# Methodology of the comparisons

Every number in the README files of this repository that compares goxmlgo with something else leads here. This
file says what was measured, on what input, with which versions, on which machine and with which commands, so the
measurement can be repeated by anyone. Raw results are in [`results/`](results/).

Status: the reader is measured (stages 1 and 2, 2026-11-26; the published numbers are from the stage 2 code). The sections for the writer (stage 3) and for the pipeline
and the tree (stage 5) are templates. A number is published only when all of its section is filled in.

## Rules

- **Input** is public or generated: the W3C XML Conformance Test Suite fetched by `tools/fetch` according to
  `testdata/sources.json` (URL, checksum, file count), or a document generated in memory by a deterministic
  function in the repository (its name is given). No private files.
- **The tool** that measures is in this repository (`go test` benchmarks and `tools/bench`, a separate module).
- **Candidates** are named with their exact versions and how they were obtained; their settings are stated.
- **Statistics**: the median of 5 runs unless stated otherwise; what counts as a failure is stated per table.
- **Raw output** of the run the numbers were taken from is committed under `results/`, with the date, the commit,
  the Go version, the operating system and the machine.
- When the input, a candidate's version, the method or the result changes, the measurement is run again with the
  same tool. Numbers are not edited by hand.

## Reader against encoding/xml

The comparison of [`xmlreader`](../xmlreader/README.md) with `encoding/xml` is run by the tests of the package.
The numbers below come from a run in this repository; the earlier run in goxsdgo was not copied.

**Candidates.** `xmlreader` of this repository at the commit named in the raw files, `Options{}` (default limits,
namespaces on; for the cases of the XML 1.0 recommendation, as opposed to Namespaces, `NoNamespaces: true`, as the
suite intends). `encoding/xml` of the Go standard library, the version of the Go toolchain named in the raw files,
`xml.NewDecoder` with default settings (`Strict` is true), no `CharsetReader`, no `Entity` map.

**Input.**

- Conformance: the W3C XML Conformance Test Suite 20130923, fetched and checked by
  `go -C tools/fetch run . -root ../..` (`w3c-xmlconf` in [`testdata/sources.json`](../testdata/sources.json): URL,
  SHA-256 of the archive and of the file tree, 3386 files). In scope are the 1727 cases that apply to XML 1.0 Fifth
  Edition and Namespaces 1.0 without external entities; the exclusion rules are the function `exclusion` in
  `xmlreader/conformance_test.go`, their counts and reasons are in [`differences.md`](differences.md) and are printed
  by `go test ./xmlreader -run TestXMLConformance -v`. Both readers get the same cases.
- Speed and memory: a document of 20,000 JPK-like rows (8.8 MB) generated in memory by the deterministic function
  `jpkLike` in `xmlreader/reader_test.go`; the same bytes on every run.

**Commands**, from the repository root:

```sh
go -C tools/fetch run . -root ../..
go test ./xmlreader -run TestStdlibConformance -v -stdlib-out ../docs/results/xmlreader-conformance.tsv
go test ./xmlreader -run '^$' -bench 'BenchmarkJPK' -benchmem -count 5
```

**What counts.**

- Conformance: a well-formed case (`valid` or `invalid` in the suite; DTD validity is not checked by either
  reader) passes when the reader reads the whole document without an error; for xmlreader the output must also
  equal the suite's canonical form where the suite has one, as in `TestXMLConformance`. A `not-wf` case passes
  when the reader rejects the document. encoding/xml is read with `Token`, which resolves namespaces. The rows of
  the table are the sums printed at the top of the raw file; each case is a line in it.
- Speed and memory: `BenchmarkJPK` (xmlreader), `BenchmarkJPKStdlib` (`Token`) and `BenchmarkJPKStdlibRaw`
  (`RawToken`, without namespace resolution) read the same document, one goroutine, every token to the end. The
  table gives the median of the 5 `MB/s` values of each benchmark, and `B/op` and `allocs/op` of the same lines.

**Raw results** of the published run: [`results/xmlreader-conformance.tsv`](results/xmlreader-conformance.tsv)
(each case, both readers) and [`results/xmlreader-bench.txt`](results/xmlreader-bench.txt) (the `go test` output;
its header has the date, the commit, the Go version, the system and the load of the machine; the CPU is in the
`cpu:` line).

## Reader against the reader of goxsdgo v0.3.0 (criterion K7)

At v0.1.0 the code of the reader is the code of goxsdgo v0.3.0 (the difference is an import path and comments,
[`provenance.md`](provenance.md)), so the measurement starts with stage 2, the first change to the reader, and is
repeated at every release.

**Candidates.** `github.com/polestorm/goxsdgo/xmlreader` v0.3.0, fetched by the Go toolchain as a module
dependency of `tools/bench` (version and checksum in `tools/bench/go.mod` and `go.sum`; the library module itself
has no dependencies), and `xmlreader` of this repository. Both with `Options{}`, both reading every token to the
end from a `bytes.Reader`, in one process, one goroutine.

**Input.** Two documents generated in memory by deterministic functions of this repository
(`internal/testkit/gen.go`; `TestGeneratorsAreDeterministic` pins their checksums): `GenJPK(20000)`, 8.8 MB of
JPK-like rows (many short elements with text, two namespaces), and `GenWordML(30000)`, 9.3 MB of
WordprocessingML-like paragraphs (many prefixed attributes, empty elements, character references).

**Command**, from the repository root:

```sh
go -C tools/bench run . -root ../.. -out ../../docs/results/reader-vs-goxsdgo.txt
```

**Statistics.** One warm-up round, then 15 rounds; in each round every candidate reads the document once, and the
order of the candidates changes every round. The table gives the median time of the 15 and the speed computed
from it, the fastest and the slowest round, and the allocations of one more read. The ratio is the speed of
goxmlgo divided by the speed of goxsdgo v0.3.0 on the same document; K7 requires at least 0.95 for every
document, and the tool exits with status 1 below that.

**Result** of 2026-11-26 ([`results/reader-vs-goxsdgo.txt`](results/reader-vs-goxsdgo.txt)): 1.22 on the JPK-like
document and 1.19 on the WordprocessingML-like one.

The same module has `TestSameAsGoxsdgo` (`go -C tools/bench test ./...`), which reads every file of the corpus and
of the fetched W3C suite with both readers and requires the same tokens and the same error; the one deliberate
difference is listed in [`differences.md`](differences.md).

## Writer

To be filled in at stage 3: throughput of reader → writer on the reference document, allocations, and, if a
comparison with `encoding/xml` (`Encoder`) is published, its settings and what it does not check.

## Tree and streaming memory

To be filled in at stages 4 and 5: memory per node, the 1 GB pass-through, how process memory is measured.

## Known limits of the numbers

- The conformance rows count verdicts (and, for xmlreader, the canonical form), not error codes or positions.
- Speed and memory come from one machine, a laptop also used for other work; the load average at the start of the
  run is in the header of the raw file. In the published run the 5 values of each benchmark differ by up to 9%
  (one slower run of `BenchmarkJPKStdlib`); between sessions absolute numbers move more than that, the order of the
  candidates did not.
- The comparison with encoding/xml uses one document shape (many short elements, two namespaces, little attribute
  data); the comparison with goxsdgo v0.3.0 adds an attribute-heavy one. Neither has long text or large
  attribute values.
- The ratio to goxsdgo v0.3.0 is taken within one process, alternating, so load on the machine affects both
  candidates; the spread between the fastest and the slowest of the 15 rounds was under 7% for each candidate.
