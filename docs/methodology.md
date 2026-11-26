# Methodology of the comparisons

Every number in the README files of this repository that compares goxmlgo with something else leads here. This
file says what was measured, on what input, with which versions, on which machine and with which commands, so the
measurement can be repeated by anyone. Raw results are in [`results/`](results/).

Status: the reader is measured (stage 1, 2026-11-26). The sections for the writer (stage 3) and for the pipeline
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

Not measured yet. At v0.1.0 the code of the reader is the code of goxsdgo v0.3.0 (the difference is an import path
and comments, [`provenance.md`](provenance.md)), so a ratio would measure only the noise of the machine. The
measurement starts with stage 2, the first change to the reader, and is repeated at every release: the same
document, the same run, both versions built into `tools/bench` (goxsdgo v0.3.0 as a module dependency of that
tool, not of the library).

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
- One document shape (many short elements, two namespaces, little attribute data). Other shapes are measured in
  stage 5 with the generators in `internal/testkit`.
