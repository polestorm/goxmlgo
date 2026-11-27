package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRepo buduje w katalogu tymczasowym minimalne repozytorium: wymagania (przyjęte i szkic), moduł główny
// i moduł narzędzia z testami z odwołaniami do wymagań, plik korpusu, zapadkę, różnice, dziennik fuzzingu i wynik
// tools/bench. Znacznik odwołania jest w treści zapisany jako „@@”, żeby test pokrycia wymagań w katalogu głównym
// (który czyta każdy wiersz plików *_test.go) nie brał zmyślonych identyfikatorów za odwołania.
func fakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"requirements/README.md": "| Prefiks | Plik | Zakres |\n|---|---|---|\n| `AB` | `a.md` | a |\n| `CDE` | `c.md` | c |\n",
		"requirements/a.md": "Status: przyjęte\n\n| Id | Wymaganie |\n|---|---|\n| AB-001 | pierwsze |\n| AB-002 | drugie, z \\| w treści |\n| AB-003 | tylko pomocnicza |\n| AB-004 | benchmark |\n| AB-005 | z pliku korpusu |\n| AB-006 | test w module narzędzia |\n" +
			"\nStatus: szkic\n\n| Id | Wymaganie |\n|---|---|\n| AB-007 | szkic bez testu |\n",
		"requirements/c.md": "Status: przyjęte\n\n| Id | Wymaganie |\n|---|---|\n| CDE-001 | bez testu |\n",
		"go.mod":            "module example.com/m\n\ngo 1.22\n",
		"p/p_test.go": `package p

import "testing"

// TestOne sprawdza pierwsze. @@ AB-001
// Drugi wiersz komentarza wspomina CDE-001, ale nie jest odwołaniem.
func TestOne(t *testing.T) {}

func TestTwo(t *testing.T) {
	// @@ AB-002, XY-001
}

// @@ AB-002, AB-003
func helper() {}

// @@ AB-004
func BenchmarkFour(b *testing.B) {}
`,
		"tools/x/go.mod":                "module example.com/m/tools/x\n\ngo 1.22\n",
		"tools/x/x_test.go":             "package main\n\nimport \"testing\"\n\n// @@ AB-006\nfunc TestTool(t *testing.T) {}\n",
		"tools/a/go.mod":                "module example.com/m/tools/a\n\ngo 1.22\n",
		"out/skipped/s_test.go":         "package s\n\n// @@ ZZ-999\nfunc TestIgnored() {}\n",
		"testdata/corpus/doc.xml":       "<!-- @@ AB-005 -->\n<a/>\n",
		"testdata/corpus/doc.out.xml":   "<a/>\n",
		"testdata/corpus/other.xml":     "<b/>\n",
		"testdata/w3c/xmlconf.ratchet":  "# header\n# in scope 10, passing 8\n# mode: failing\neduni/rmt-e3e-13\nibm/ibm-valid-P29-ibm29v01.xml\n",
		"testdata/w3c/xmlconf/skip.txt": "pobrane dane\n",
		"docs/differences.md":           "## A\n\n| x | y |\n|---|---|\n| `eduni/rmt-e3e-13` | z |\n| inny | z |\n\n| u | w |\n|---|---|\n| 1 | 2 |\n\n## B\n\nnic\n",
		"docs/fuzzing.md": "## Fuzzery i niezmienniki\n\n| Fuzzer | Pakiet | Niezmiennik | Od etapu |\n|---|---|---|---|\n| `FuzzA` | `x` | bez paniki | 1 |\n\n## Przebiegi\n\n" +
			"| Data | Commit | Host i sprzęt | Fuzzer | Czas zegarowy | Wykonania | Wynik |\n|---|---|---|---|---|---|---|\n" +
			"| 2026-11-26 | `0000aaa` | NAS | `FuzzA` | zlecone 23:37, nie rozpoczęte | — | wycofane z kolejki |\n" +
			"| 2026-11-27 | `abc1234` | NAS | `FuzzA`, `FuzzB` | zlecone 08:21, 3 × 475 min | — | w kolejce |\n" +
			"| 2026-11-27 09:56 | `abc1234` (zmiany) | Mac | `FuzzA` | 2h0m0s | 10 | ok; przebieg z drzewa ze zmianami |\n" +
			"| 2026-11-27 11:03 | `abc1234` | lokalnie | `FuzzA` (./x) | 15m3s | 10 | ok |\n" +
			"| 2026-11-27 11:20 | `def5678` | lokalnie | `FuzzB` (./x) | 10m0s | 10 | ok |\n" +
			"| 2026-11-27 11:40 | `9999999` | lokalnie | `FuzzB` (./x) | 3h0m0s | 10 | ok |\n" +
			"\n## Znalezione błędy\n\n| Data | Fuzzer | Błąd | Poprawka | Wejście regresji |\n|---|---|---|---|---|\n| 2026-11-27 | `FuzzA` | a | b | c |\n",
		"docs/results/reader-vs-goxsdgo.txt": "# go -C tools/bench run . -root ../.. -rounds 15\n# date: 2026-11-27T07:20Z\n# goxmlgo: abc1234\n" +
			"GenA(1)\t1\t1\tgoxmlgo\t1\t1\n" +
			"# ratio GenA(1): 1.228 (ok)\n# pass-through GenA(1): goxmlgo 163.8 MB/s, 0.62 of its reader alone, 1.83 of encoding/xml Decoder+Encoder\n" +
			"# ratio GenB(2): 1.197 (ok)\n# pass-through GenB(2): goxmlgo 83.2 MB/s, 0.67 of its reader alone, 1.20 of encoding/xml Decoder+Encoder (ok)\n",
	}
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.ReplaceAll(body, "@@", refMark)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const fakeEvents = `{"Action":"output","Package":"example.com/m/p","Test":"TestOne","Output":"=== RUN TestOne\n"}
{"Action":"pass","Package":"example.com/m/p","Test":"TestOne"}
{"Action":"pass","Package":"example.com/m/p","Test":"TestOne/sub"}
{"Action":"skip","Package":"example.com/m/p","Test":"TestTwo"}
{"Action":"pass","Package":"example.com/m/p"}
{"Action":"fail","Package":"example.com/m/q"}
`

func events(lines ...string) *TestRun {
	run, _ := parseTestEvents("m", "go test", strings.NewReader(strings.Join(lines, "\n")))
	return run
}

func pass(pkg, test string, output ...string) string {
	var b strings.Builder
	for _, o := range output {
		js, _ := json.Marshal(o + "\n")
		b.WriteString(`{"Action":"output","Package":"` + pkg + `","Test":"` + test + `","Output":` + string(js) + "}\n")
	}
	return b.String() + `{"Action":"pass","Package":"` + pkg + `","Test":"` + test + `"}`
}

func statuses(cs []Criterion) map[string]string {
	got := map[string]string{}
	for _, c := range cs {
		got[c.ID] = c.Status
	}
	return got
}

// Pakiet ma surowy wynik przebiegu modułu głównego i każdego modułu narzędzia, wynik tools/fetch -verify, commit
// z oznaczeniem zmian, wersję Go, system i datę.
// REQ: EVD-001
func TestPackageContents(t *testing.T) {
	root := fakeRepo(t)
	var names []string
	for _, s := range testRuns(root) {
		names = append(names, s.dir+"="+s.file)
	}
	if got := strings.Join(names, " "); got != ".=go-test-module.json tools/a=go-test-a.json tools/x=go-test-x.json" {
		t.Errorf("test runs: %s", got)
	}

	e := &Evidence{Generated: "2026-11-27 12:00", Commit: "abc1234", Dirty: true, GoVersion: "go1.26.6", Platform: "linux/amd64",
		Python: "Python 3.12", Candidate: "abc1234", CandidateSource: "HEAD",
		FetchVerify: FetchVerify{Command: "go -C tools/fetch run . -root ../.. -verify", OK: true, Lines: []string{"w3c-xmlconf: ok (3386 files, present)"}}}
	raw := map[string][]byte{"go-test-module.json": []byte("{}\n"), "go-test-a.json": []byte("{}\n"), "fetch-verify.txt": []byte("ok\n")}
	dir := filepath.Join(t.TempDir(), "pkg")
	if err := write(dir, e, raw); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go-test-module.json", "go-test-a.json", "fetch-verify.txt", "evidence.json", "report.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("package file: %v", err)
		}
	}
	report, _ := os.ReadFile(filepath.Join(dir, "report.md"))
	for _, want := range []string{"Commit `abc1234` (ze zmianami w drzewie roboczym", "go1.26.6", "linux/amd64", "2026-11-27 12:00", "w3c-xmlconf: ok (3386 files, present)"} {
		if !strings.Contains(string(report), want) {
			t.Errorf("report.md lacks %q", want)
		}
	}
	if err := tarDir(dir, dir+".tar.gz"); err != nil {
		t.Error(err)
	}
}

// REQ: EVD-002
func TestTraceabilityMatrix(t *testing.T) {
	root := fakeRepo(t)
	reqs, problems, err := parseRequirements(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 8 || reqs[1].Text != `drugie, z \| w treści` || !reqs[0].Accepted || reqs[6].ID != "AB-007" || reqs[6].Accepted || len(problems) != 0 {
		t.Fatalf("requirements: %+v, problems %v", reqs, problems)
	}
	refs, err := collectRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	run, err := parseTestEvents("m", "go test", strings.NewReader(fakeEvents))
	if err != nil {
		t.Fatal(err)
	}
	if run.Passed != 1 || run.Skipped != 1 || len(run.BuildFailed) != 1 || run.BuildFailed[0] != "example.com/m/q" {
		t.Fatalf("run: %+v", run)
	}
	tool := events(pass("example.com/m/tools/x", "TestTool"))
	traces, unknown := buildTraces(reqs, refs, []*TestRun{run, tool})
	want := map[string]string{"AB-001": "pass", "AB-002": "skip", "AB-003": "unresolved", "AB-004": "manual",
		"AB-005": "pass", "AB-006": "pass", "AB-007": "draft", "CDE-001": "missing"}
	for _, tr := range traces {
		if tr.Status != want[tr.ID] {
			t.Errorf("%s: status %s, want %s (evidence %v)", tr.ID, tr.Status, want[tr.ID], tr.Evidence)
		}
	}
	// AB-002 ma dwa dowody: TestTwo (pominięty) i funkcję pomocniczą (dowód wspierający, nie pogarsza stanu).
	if len(traces[1].Evidence) != 2 {
		t.Errorf("AB-002 evidence: %v", traces[1].Evidence)
	}
	if ev := traces[4].Evidence; len(ev) != 1 || ev[0] != "testdata/corpus/doc.xml" {
		t.Errorf("AB-005 evidence: %v", ev)
	}
	// Identyfikator za znacznikiem, którego nikt nie zdefiniował, jest odwołaniem do nieznanego wymagania, także
	// z prefiksem spoza tabeli; katalog out/ nie jest przeszukiwany.
	if len(unknown) != 1 || unknown[0].ID != "XY-001" || unknown[0].Test != "TestTwo" {
		t.Errorf("unknown refs: %+v", unknown)
	}

	// Wymaganie bez testu albo z testem pominiętym nie daje spełnionego K4.
	in := Inputs{Runs: []*TestRun{run, tool}, Traces: traces, UnknownRefs: unknown}
	if got := statuses(buildCriteria(in))["K4"]; got != "not_met" {
		t.Errorf("K4 with a missing test: %s", got)
	}
	only := []Trace{traces[0], traces[1]}
	root2 := events(pass(pkgRoot, "TestRequirementsCovered"), pass(pkgRoot, "TestAPISnapshot"))
	if got := statuses(buildCriteria(Inputs{Runs: []*TestRun{root2}, Traces: only}))["K4"]; got != "pending" {
		t.Errorf("K4 with a skipped test: %s", got)
	}
	if got := statuses(buildCriteria(Inputs{Runs: []*TestRun{root2}, Traces: only[:1]}))["K4"]; got != "met" {
		t.Errorf("K4 with every accepted requirement passing: %s", got)
	}
}

// REQ: EVD-002
func TestRequirementProblems(t *testing.T) {
	root := fakeRepo(t)
	bad := "| Id | Wymaganie |\n|---|---|\n| AB-010 | przed statusem |\n\nStatus: gotowe\n\n| QQ-001 | prefiks spoza tabeli |\n| AB-001 | drugi raz |\n"
	if err := os.WriteFile(filepath.Join(root, "requirements", "b.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	_, problems, err := parseRequirements(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 4 {
		t.Fatalf("problems: %q", problems)
	}
	if got := statuses(buildCriteria(Inputs{RequirementProblems: problems}))["K4"]; got != "not_met" {
		t.Errorf("K4 with broken requirement files: %s", got)
	}
}

// REQ: EVD-003
func TestFuzzHoursCountOnlyCleanRunsOfTheCandidate(t *testing.T) {
	root := fakeRepo(t)
	b, _ := os.ReadFile(filepath.Join(root, "docs", "fuzzing.md"))
	runs := parseFuzzJournal(string(b))
	if len(runs) != 6 {
		t.Fatalf("%d runs: %+v", len(runs), runs)
	}
	same := func(c string) bool { return c == "abc1234" || c == "def5678" }
	total, failed := countFuzzHours(runs, same)
	if total != 25*time.Minute+3*time.Second || failed {
		t.Errorf("counted %v, failed %v; rows %+v", total, failed, runs)
	}
	// Wiersze kampanii zleconych (czas opisany słowami), wiersz „(zmiany)” i commit z innym kodem biblioteki nie
	// są liczone.
	if runs[0].Counted || runs[1].Counted || !runs[2].Dirty || runs[2].Counted || !runs[3].Counted || !runs[4].Counted || runs[5].Counted {
		t.Errorf("flags: %+v", runs)
	}
	if runs[3].Fuzzer != "FuzzA (./x)" || runs[3].Host != "lokalnie" {
		t.Errorf("row: %+v", runs[3])
	}
	// Przebieg z błędem na kodzie kandydata zeruje licznik.
	runs = append(runs, FuzzRun{Commit: "abc1234", Result: "FAIL: panic", seconds: time.Minute},
		FuzzRun{Commit: "abc1234", Result: "ok", seconds: 2 * time.Minute})
	total, failed = countFuzzHours(runs, same)
	if total != 2*time.Minute || !failed || runs[3].Counted || !runs[7].Counted {
		t.Errorf("after a failure: %v, %v", total, failed)
	}
}

// REQ: EVD-003
func TestBenchResult(t *testing.T) {
	root := fakeRepo(t)
	b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(benchFile)))
	bench := parseBench(string(b))
	if !bench.Found || bench.Commit != "abc1234" || bench.Date != "2026-11-27T07:20Z" || len(bench.Documents) != 2 {
		t.Fatalf("bench: %+v", bench)
	}
	if d := bench.Documents[1]; d.Document != "GenB(2)" || d.ReaderRatio != 1.197 || d.PassOfReader != 0.67 || d.PassOfStdlib != 1.20 {
		t.Errorf("document: %+v", d)
	}
	if parseBench("# date: x\n").Found {
		t.Error("a file without ratio lines counts as a measurement")
	}
}

// REQ: EVD-003
func TestStreamingMemoryResult(t *testing.T) {
	text := "# GOXMLGO_STREAM_MB=1024 go test . -run '^TestStreamingMemory$' -count=1 -v\n# date: 2026-11-27T10:49Z\n" +
		"# goxmlgo: d45fa04 with uncommitted changes (the test itself is new)\n=== RUN   TestStreamingMemory\n" +
		"    stream_test.go:363: pass-through of 1030 MB (2445881 rows): 1080478088 bytes written, peak heap 1.20 MB, memory obtained from the system by the test process 18.0 MB\n"
	s := parseStreamingMemory(text)
	if !s.Found || s.Commit != "d45fa04" || !s.Uncommitted || s.Date != "2026-11-27T10:49Z" || s.SizeMB != 1030 || s.HeapMB != 1.2 || s.ProcessMB != 18 {
		t.Errorf("recorded result: %+v", s)
	}
	if s := parseStreamingMemory(strings.Replace(text, " with uncommitted changes (the test itself is new)", "", 1)); s.Uncommitted || s.Commit != "d45fa04" {
		t.Errorf("clean result: %+v", s)
	}
	if parseStreamingMemory("# date: x\n").Found {
		t.Error("a file without the measurement line counts as a measurement")
	}

	// Flaga -stream-mb: liczba z osobnego przebiegu testu zastępuje zapisaną; bez flagi wyjście testu z pełnego
	// przebiegu (100 MB, wspólny proces) nie jest pomiarem K7.
	line := "    stream_test.go:363: pass-through of 1030 MB (2445881 rows): 1080478088 bytes written, peak heap 1.20 MB, memory obtained from the system by the test process 18.0 MB"
	in := goodInputs()
	in.Memory = StreamMemory{}
	in.Runs = append([]*TestRun{events(pass(pkgRoot, "TestStreamingMemory", line))}, in.Runs...)
	if got := statuses(buildCriteria(in))["K7"]; got != "pending" {
		t.Errorf("K7 without the flag and without a recorded result: %s", got)
	}
	in.StreamMB, in.StreamRun = 1024, events(pass(pkgRoot, "TestStreamingMemory", line))
	cs := buildCriteria(in)
	if cs[6].Status != "met" || !strings.Contains(cs[6].Value, "plik 1030 MB: pamięć procesu 18.0 MB") {
		t.Errorf("K7 measured in a dedicated run: %+v", cs[6])
	}
	in.StreamRun = events(strings.Replace(pass(pkgRoot, "TestStreamingMemory", line), `"Action":"pass"`, `"Action":"fail"`, 1))
	if got := statuses(buildCriteria(in))["K7"]; got != "not_met" {
		t.Errorf("K7 with a failed dedicated run: %s", got)
	}
}

// goodInputs to dane, przy których każde kryterium jest spełnione.
func goodInputs() Inputs {
	conformance := []string{"    conformance_test.go:83: ", "        W3C XML Conformance: passed/in scope", "        eduni        476/477  ",
		"        ibm          526/527  ", "        all         1725/1727 ", "        excluded   273: XML 1.1"}
	run := events(
		pass(pkgReader, "TestXMLConformance", conformance...),
		pass(pkgWriter, "TestRoundTrip", "    roundtrip_test.go:248: round trip on the W3C XML Conformance suite: 775 of 775 documents the reader accepts (2 well-formed cases in scope are rejected by the reader)"),
		pass(pkgTree, "TestTreeRoundTrip", "    roundtrip_test.go:298: tree round trip on the W3C XML Conformance suite: 775 of 775 documents the reader accepts"),
		pass(pkgTree, "TestSubtreeInStream", "    roundtrip_test.go:368: 1156 elements read as a subtree in the middle of a stream"),
		pass(pkgRoot, "TestRequirementsCovered"), pass(pkgRoot, "TestAPISnapshot"),
		pass(pkgRoot, "TestSafeByDefault"), pass(pkgRoot, "TestHostileInputs"), pass(pkgRoot, "TestPathological"), pass(pkgRoot, "TestNoIOImports"),
		pass(pkgReader, "TestStreamingMemory"),
	)
	m := &Measurements{Found: true}
	m.K3Consumers = ExternalResult{Status: "met", Result: "oba drzewa zastąpione"}
	m.K8 = ExternalResult{Status: "met", Result: "CI zielone"}
	rows, _ := parseW3C(run.Output(pkgReader + ".TestXMLConformance"))
	trace := func(id string) Trace { return Trace{Requirement: Requirement{ID: id, Accepted: true}, Status: "pass"} }
	return Inputs{Runs: []*TestRun{run}, Measurements: m, W3C: rows, CorpusDocuments: 13,
		W3CFailing:  []NamedCase{{ID: "eduni/rmt-e3e-13", Named: true}, {ID: "ibm/ibm-valid-P29-ibm29v01.xml", Named: true}},
		PythonTests: PythonRun{OK: true}, FetchVerify: FetchVerify{OK: true}, FuzzHours: 25 * time.Hour, Candidate: "abc1234",
		Traces: []Trace{trace("TR-001"), trace("DET-002"), trace("DET-007")},
		Memory: StreamMemory{Found: true, Commit: "abc1234", SameLibrary: true, SizeMB: 1030, HeapMB: 1.2, ProcessMB: 18},
		Bench:  Bench{Found: true, Commit: "abc1234", SameLibrary: true, Documents: []BenchLine{{"GenA(1)", 1.23, 0.62, 1.83}}}}
}

// REQ: EVD-003
func TestCriteriaFromInputs(t *testing.T) {
	cs := buildCriteria(goodInputs())
	if len(cs) != 9 {
		t.Fatalf("%d criteria", len(cs))
	}
	for i, c := range cs {
		if c.ID != "K"+string(rune('1'+i)) || c.Status != "met" || c.Value == "" || c.Target == "" || c.Source == "" {
			t.Errorf("criterion %d: %+v", i+1, c)
		}
	}
	if !strings.HasPrefix(cs[0].Value, "1725 / 1727 (eduni 476/477, ibm 526/527)") {
		t.Errorf("K1 value: %s", cs[0].Value)
	}
	if !strings.Contains(cs[1].Value, "W3C 775 / 775") || !strings.Contains(cs[2].Value, "1156 elementów") {
		t.Errorf("K2: %s; K3: %s", cs[1].Value, cs[2].Value)
	}

	// Każda zmiana niżej psuje jedno kryterium; żadne nie może zostać „met”.
	cases := []struct {
		id, want string
		change   func(in *Inputs)
	}{
		{"K1", "not_met", func(in *Inputs) { in.W3CFailing[0].Named = false }},
		{"K1", "not_met", func(in *Inputs) { in.W3C[len(in.W3C)-1].Passed = 1724 }},
		{"K1", "pending", func(in *Inputs) { in.W3C = nil }},
		{"K2", "not_met", func(in *Inputs) { in.CorpusDocuments = 0 }},
		{"K3", "pending", func(in *Inputs) { in.Measurements.K3Consumers = ExternalResult{} }},
		{"K3", "not_met", func(in *Inputs) { in.Traces[0].Status = "fail" }},
		{"K4", "not_met", func(in *Inputs) { in.UnknownRefs = []Ref{{ID: "XY-001"}} }},
		{"K5", "pending", func(in *Inputs) { in.FuzzHours = 23 * time.Hour }},
		{"K5", "not_met", func(in *Inputs) { in.FuzzFailed = true }},
		{"K7", "pending", func(in *Inputs) { in.Bench.SameLibrary = false }},
		{"K7", "pending", func(in *Inputs) { in.Bench = Bench{} }},
		{"K7", "pending", func(in *Inputs) { in.Memory = StreamMemory{} }},
		{"K7", "pending", func(in *Inputs) { in.Memory.SizeMB = 100 }},
		{"K7", "pending", func(in *Inputs) { in.Memory.Uncommitted = true }},
		{"K7", "pending", func(in *Inputs) { in.Memory.SameLibrary = false }},
		{"K7", "not_met", func(in *Inputs) { in.Memory.ProcessMB = 64 }},
		{"K7", "not_met", func(in *Inputs) { in.Bench.Documents[0].ReaderRatio = 0.94 }},
		{"K7", "not_met", func(in *Inputs) { in.Bench.Documents[0].PassOfStdlib = 0.99 }},
		{"K7", "pending", func(in *Inputs) { in.Bench.Documents[0].PassOfReader = 0 }},
		{"K7", "not_met", func(in *Inputs) { in.Traces[2].Status = "fail" }},
		{"K8", "pending", func(in *Inputs) { in.Measurements = nil }},
		{"K8", "not_met", func(in *Inputs) { in.Traces[1].Status = "fail" }},
		{"K9", "not_met", func(in *Inputs) { in.PythonTests.OK = false }},
		{"K9", "pending", func(in *Inputs) { in.FetchVerify.OK = false }},
	}
	for i, c := range cases {
		in := goodInputs()
		c.change(&in)
		if got := statuses(buildCriteria(in))[c.id]; got != c.want {
			t.Errorf("case %d: %s is %s, want %s", i, c.id, got, c.want)
		}
	}

	// Bez żadnego przebiegu i bez pomiarów nic nie jest spełnione: brak liczby to nie zaliczenie.
	for _, c := range buildCriteria(Inputs{}) {
		if c.Status == "met" || c.Status == "" {
			t.Errorf("%s without any input: %q (%s)", c.ID, c.Status, c.Value)
		}
	}
	// Test pominięty z braku danych (albo pod detektorem wyścigów) nie spełnia kryterium.
	skipped := events(`{"Action":"skip","Package":"`+pkgReader+`","Test":"TestXMLConformance"}`,
		`{"Action":"skip","Package":"`+pkgWriter+`","Test":"TestRoundTrip"}`,
		`{"Action":"skip","Package":"`+pkgRoot+`","Test":"TestPathological"}`,
		pass(pkgRoot, "TestSafeByDefault"), pass(pkgRoot, "TestHostileInputs"), pass(pkgRoot, "TestNoIOImports"))
	got := statuses(buildCriteria(Inputs{Runs: []*TestRun{skipped}}))
	if got["K1"] != "pending" || got["K2"] != "pending" || got["K6"] != "pending" {
		t.Errorf("skipped tests: %v", got)
	}
}

// REQ: EVD-001, EVD-003
func TestRatchetsAndDifferences(t *testing.T) {
	root := fakeRepo(t)
	rs, err := readRatchets(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].InScope != 10 || rs[0].Passing != 8 || rs[0].Mode != "failing" || rs[0].Listed != 2 {
		t.Fatalf("ratchets: %+v", rs)
	}
	diff, _ := os.ReadFile(filepath.Join(root, "docs", "differences.md"))
	named := namedInDifferences(rs[0].IDs, string(diff))
	if !named[0].Named || named[1].Named {
		t.Errorf("named: %+v", named)
	}
	sections := diffSections(string(diff))
	if len(sections) != 2 || sections[0].Rows != 3 || sections[1].Rows != 0 {
		t.Errorf("sections: %+v", sections)
	}
	if n := countCorpus(root); n != 2 {
		t.Errorf("corpus documents: %d", n)
	}
}

// REQ: EVD-003
func TestMeasurementsFile(t *testing.T) {
	root := fakeRepo(t)
	m, err := readMeasurements(root)
	if err != nil || m.Found {
		t.Fatalf("without the file: %+v, %v", m, err)
	}
	body := `{"candidate":{"commit":"abc1234"},"k8":{"status":"met","result":"ok","date":"2026-12-01","commit":"abc1234"}}`
	if err := os.WriteFile(filepath.Join(root, "docs", "measurements.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err = readMeasurements(root)
	if err != nil || !m.Found || m.Candidate.Commit != "abc1234" || m.K8.Date != "2026-12-01" || m.K8.Status != "met" || m.K3Consumers.Status != "" {
		t.Fatalf("with the file: %+v, %v", m, err)
	}
}

// REQ: EVD-004
func TestReportIsDeterministic(t *testing.T) {
	root := fakeRepo(t)
	build := func(when string) (string, string) {
		reqs, problems, err := parseRequirements(root)
		if err != nil {
			t.Fatal(err)
		}
		refs, err := collectRefs(root)
		if err != nil {
			t.Fatal(err)
		}
		in := goodInputs()
		in.Traces, in.UnknownRefs = buildTraces(reqs, refs, in.Runs)
		in.RequirementProblems = problems
		journal, _ := os.ReadFile(filepath.Join(root, "docs", "fuzzing.md"))
		fuzz := parseFuzzJournal(string(journal))
		in.FuzzHours, in.FuzzFailed = countFuzzHours(fuzz, func(string) bool { return true })
		ratchets, _ := readRatchets(root)
		e := &Evidence{Generated: when, Commit: "abc1234", Candidate: "abc1234", CandidateSource: "HEAD", Criteria: buildCriteria(in),
			Runs: in.Runs, Traces: in.Traces, UnknownRefs: in.UnknownRefs, W3C: in.W3C, W3CFailing: in.W3CFailing, Ratchets: ratchets,
			Fuzz: fuzz, FuzzHours: in.FuzzHours.Hours(), Bench: in.Bench, Measurements: in.Measurements,
			Files: []string{"evidence.json", "report.md"}}
		js, err := marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		return renderReport(e), string(js)
	}
	r1, j1 := build("2026-11-25 10:00")
	for i := 0; i < 10; i++ {
		r2, j2 := build("2026-11-26 11:00")
		if strings.Replace(r1, "2026-11-25 10:00", "2026-11-26 11:00", 1) != r2 {
			t.Fatal("report differs beyond the generation time")
		}
		if strings.Replace(j1, "2026-11-25 10:00", "2026-11-26 11:00", 1) != j2 {
			t.Fatal("evidence.json differs beyond the generation time")
		}
	}
	if !strings.Contains(r1, `drugie, z \\| w treści`) {
		t.Error("table cell not escaped")
	}
	// Czasy przebiegów nie wchodzą do raportu: wyjście testu z czasem zostaje w pliku surowym.
	in := goodInputs()
	in.Runs = append(in.Runs, events(pass(pkgRoot, "TestHostileInputs", "    safety_test.go:256: entity expansion  reject  limit-entity-expansion  123.4 ms  48.0 MB")))
	for _, c := range buildCriteria(in) {
		if strings.Contains(c.Value, "123.4") {
			t.Errorf("%s carries a timing: %s", c.ID, c.Value)
		}
	}
}

// REQ: EVD-005
func TestEnvironmentWithoutNetworkAndOutsideData(t *testing.T) {
	t.Setenv("GOXMLGO_PRIVATE_CORPUS", "/secret")
	t.Setenv("GOXMLGO_STREAM_MB", "1024") // rozmiar ustawia tylko flaga -stream-mb
	t.Setenv("GOPROXY", "https://proxy.golang.org,direct")
	t.Setenv("GOTOOLCHAIN", "auto")
	seen := map[string]string{}
	for _, kv := range testEnv() {
		name, value, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GOXMLGO_") {
			t.Errorf("test environment keeps %s", kv)
		}
		if _, dup := seen[name]; dup && (name == "GOPROXY" || name == "GOTOOLCHAIN") {
			t.Errorf("%s set twice", name)
		}
		seen[name] = value
	}
	if seen["GOPROXY"] != "off" || seen["GOTOOLCHAIN"] != "local" {
		t.Errorf("GOPROXY=%q GOTOOLCHAIN=%q", seen["GOPROXY"], seen["GOTOOLCHAIN"])
	}
	// Przebiegi to zwykłe `go test`, bez fuzzingu.
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), `"-fuzz`) || strings.Contains(string(src), `"-test.fuzz`) {
		t.Error("the tool passes a fuzzing flag to go test")
	}
}

// Kod wyjścia 1 tylko wtedy, gdy K4 albo K9 nie są spełnione; stan „oczekuje” i inne kryteria go nie dają.
// REQ: EVD-005
func TestExitCode(t *testing.T) {
	for _, c := range []struct {
		criteria []Criterion
		want     bool
	}{
		{[]Criterion{{ID: "K4", Status: "met"}, {ID: "K9", Status: "met"}}, false},
		{[]Criterion{{ID: "K4", Status: "not_met"}, {ID: "K9", Status: "met"}}, true},
		{[]Criterion{{ID: "K4", Status: "met"}, {ID: "K9", Status: "not_met"}}, true},
		{[]Criterion{{ID: "K4", Status: "pending"}, {ID: "K5", Status: "not_met"}, {ID: "K9", Status: "pending"}}, false},
	} {
		if got := blocking(c.criteria); got != c.want {
			t.Errorf("%+v: exit 1 is %v, want %v", c.criteria, got, c.want)
		}
	}
}
