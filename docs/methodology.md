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

**Result** of 2026-11-27 ([`results/reader-vs-goxsdgo.txt`](results/reader-vs-goxsdgo.txt), commit `445e58e`,
v0.4.0 and the tests of stage 5): 1.24 on the JPK-like document and 1.19 on the WordprocessingML-like one (1.22 and 1.19 on the commit of stage
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
on the document. Result of 2026-11-27 (commit `445e58e`): 158 MB/s on the JPK-like document and 82 MB/s on the
WordprocessingML-like one, that is 0.62 and 0.67 of the speed of the reader alone and 1.84 and 1.21 times the speed
of the encoding/xml pass-through.

**Allocations.** `go test ./xmlwriter -run TestWriterAllocations -v`: a document written with calls allocates
nothing after warm-up at two sizes, and the pass-through allocates no more than the reader alone on the same
document.

## Tree and streaming memory

**Tree round trip (criterion K3).** `go test ./xmltree -run 'TestTreeRoundTrip|TestSubtreeInStream' -v`. Input:
the corpus and the documents of the W3C suite that the reader accepts, as for K2. A document counts when
`Parse` → `WriteTo` gives the bytes of the reader → writer pass-through, the tree reader gives the token stream of
the source and a deep copy is written with the same bytes. The second test builds single elements as trees in the
middle of a pass-through and requires the output not to change.

**Memory of a tree.** `go test ./xmltree -run TestTreeLinear -v` logs the heap held by a tree per node and per
byte of the document, and the allocations of building it, on the two generated documents at two sizes.

**Streaming memory (criterion K7).** `GOXMLGO_STREAM_MB=1024 go test . -run '^TestStreamingMemory$' -count=1 -v`.
The document (the JPK-like generator, 1 GB) is produced while it is read and is not held in memory; it goes from
the reader to the writer and to a writer that only counts. Two numbers are taken: the peak of the Go heap above
its size before the run, sampled every 200 µs with the collector set to keep the heap near the live data, and the
memory the whole test process obtained from the operating system (`runtime.MemStats.Sys`) at the end. The test
fails above 8 MB of heap or 64 MB for the process. Result of 2026-11-27
([`results/streaming-memory.txt`](results/streaming-memory.txt), commit `445e58e`): 1030 MB read, 1.2 MB peak
heap, 14 MB for the process. Without the variable the same test runs on 100 MB in every `go test`.

**Hostile and pathological inputs (criterion K6).** `go test . -run 'TestHostileInputs|TestPathological' -v`.
Inputs are generated by the test; the table in the log gives, for each, the error code, the time and the peak
heap. Time and memory bounds are not checked under the race detector.

## Tree compared with other Go libraries

**Candidates.** `xmltree` of this repository; `github.com/beevik/etree` and `github.com/antchfx/xmlquery` at the
versions in `tools/compare/go.sum` (printed in the header of the raw file), fetched by the Go toolchain and used
through their documented entry points only: `etree.Document.ReadFrom` and `WriteTo`, `xmlquery.Parse` and
`Node.OutputXML`. Their code was not read ([`provenance.md`](provenance.md)). All with default settings, except
that the node limit of xmltree is turned off. `xmlreader` and `encoding/xml.Decoder.Token` are measured next to
them, without a tree, to show what reading alone costs.

**Input.** The two generated documents of the reader comparison (`GenJPK(20000)`, `GenWordML(30000)`), and for
conformance the 1727 cases of the W3C suite in scope (see the first section).

**Commands**, from the repository root:

```sh
go -C tools/compare run . -root ../.. -out ../../docs/results/tree-vs-others.txt
go -C tools/compare test . -run TestCandidatesConformance -conformance-out ../../docs/results/tree-conformance.tsv
```

**Statistics.** One warm-up round, then 9 rounds; in each round every candidate builds a tree of every document
and writes it to `io.Discard`, the order of the candidates changing every round, a collection before each build.
The table has the median. Allocations and bytes are those of one build with the collector off; "held" is the
heap in use after a collection while the tree is alive.

**What the candidates check.** A faster parser that accepts ill-formed documents is not doing the same work, so
the second command counts verdicts: a well-formed document should give a tree, an ill-formed one an error.
xmltree reads the cases of the XML 1.0 recommendation without namespace processing, as its own conformance test
does; the other two have no such setting.

**Result** of 2026-11-27 ([`results/tree-vs-others.txt`](results/tree-vs-others.txt),
[`results/tree-conformance.tsv`](results/tree-conformance.tsv); commit `445e58e`):

| | xmltree | etree | xmlquery |
|---|---|---|---|
| build, JPK-like document | 189 MB/s | 84 MB/s | 57 MB/s |
| build, WordprocessingML-like document | 100 MB/s | 74 MB/s | 45 MB/s |
| write, JPK-like / WordprocessingML-like | 565 / 327 MB/s | 570 / 297 MB/s | 219 / 115 MB/s |
| allocations per build, JPK-like | 1 557 | 1 660 102 | 2 980 122 |
| heap held by the tree, JPK-like / WordprocessingML-like | 6.8× / 11.5× the document | 4.3× / 6.5× | 6.7× / 8.4× |
| well-formed documents accepted (of 776) | 775 | 712 | 711 |
| ill-formed documents rejected (of 951) | 951 | 410 | 432 |

xmltree builds faster and checks more, and it holds more memory: a node keeps its line and column, every
attribute its type, position and namespace name, and names are kept with prefix and namespace name.

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
