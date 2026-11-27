# Methodology of the comparisons

Every number in the README files of this repository that compares goxmlgo with something else leads here. This
file says what was measured, on what input, with which versions, on which machine and with which commands, so the
measurement can be repeated by anyone. Raw results are in [`results/`](results/).

Status: the reader is measured (stages 1 and 2, 2026-11-26; the published numbers are from the stage 2 code) and so is the writer (stage 3, 2026-11-27). The sections for the pipeline
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

**Result** of 2026-11-27 ([`results/reader-vs-goxsdgo.txt`](results/reader-vs-goxsdgo.txt), commit of stage
3, `247615e`): 1.23 on the JPK-like document and 1.20 on the WordprocessingML-like one (1.22 and 1.19 on the commit of stage
2, `96c7949`).

The same module has `TestSameAsGoxsdgo` (`go -C tools/bench test ./...`), which reads every file of the corpus and
of the fetched W3C suite with both readers and requires the same tokens and the same error; the one deliberate
difference is listed in [`differences.md`](differences.md).

## Writer

**Round trip (criterion K2).** `go test ./xmlwriter -run TestRoundTrip -v`. Input: the 13 documents of
`testdata/corpus` and every case of the W3C suite in scope whose type is not `not-wf` and which the reader accepts
(775 documents; cases of XML 1.0 are read and written without namespace processing, cases of Namespaces 1.0 with
it, as in the conformance test). For each document the test reads it, writes the tokens with `WriteToken`, reads
the output, and requires the same stream of tokens under the definition in [`design.md`](design.md) (kinds, names
with prefixes, namespace declarations, attributes, data, CDATA, white-space and empty-element flags; not
positions, not the DOCTYPE token); it then writes the output again and requires the same bytes. A failure is any
difference or any error of the writer. The ratchet `testdata/w3c/roundtrip.ratchet` keeps the number from falling.
Result of 2026-11-27: 13 of 13 and 775 of 775.

**Output read by encoding/xml.** `go test ./xmlwriter -run TestStdlibWriter -v`. The output of the writer for the
same documents is read by `encoding/xml` (`Decoder`, default settings) and compared with what xmlreader reads from
the source: expanded names, attribute values, text (adjacent character data joined, because encoding/xml does not
tell CDATA sections from text), comments and PIs. Documents that xmlreader does not read with namespace processing
are left out, and documents that encoding/xml rejects are counted separately: its name characters are those of the
earlier editions of XML 1.0. Result of 2026-11-27: 789 documents, 767 the same, 13 rejected by encoding/xml, 9
left out, no difference in a document both read.

**Pass-through speed.** The same tool, run and statistics as the reader comparison above
(`tools/bench`, [`results/reader-vs-goxsdgo.txt`](results/reader-vs-goxsdgo.txt)), with two more candidates that
read the document and write it at once to `io.Discard`: `xmlreader` → `xmlwriter` (`Next`, `WriteToken`, `Close`,
default options) and `encoding/xml` `Decoder` → `Encoder` (`RawToken`, `EncodeToken`, `Flush`, default settings).
The two do not do the same work: xmlwriter checks every name, character and namespace binding and keeps prefixes,
CDATA sections and empty-element tags; `Encoder` rewrites namespace prefixes and checks less
([`differences.md`](differences.md)). **Threshold** (K7, since 2026-11-27): on every document the pass-through of
goxmlgo is at least 0.55 of the speed of its reader alone and not slower than the encoding/xml pass-through; the
tool exits with status 1 below that. The threshold is relative because a speed in MB/s depends on the machine and
on the document. Result of 2026-11-27 (commit of stage 3, `247615e`): 164 MB/s on the JPK-like document and 83 MB/s on the WordprocessingML-like one, that is 0.62 and 0.67 of
the speed of the reader alone and 1.83 and 1.20 times the speed of the encoding/xml pass-through.

**Allocations.** `go test ./xmlwriter -run TestWriterAllocations -v`: a document written with calls allocates
nothing after warm-up at two sizes, and the pass-through allocates no more than the reader alone on the same
document.

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
