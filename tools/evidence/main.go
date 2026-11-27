// Command evidence składa pakiet dowodów goxmlgo (kryterium K9, ADR 0001, requirements/evidence.md): wynik
// ostatniego przebiegu testów, macierz śledzenia wymagania → testy → wynik, tabelę K1–K9 ze źródłami, sumy danych
// testowych, stan zapadek, dziennik fuzzingu, listę decyzji i różnic. Przeniesione z goxsdgo (tools/evidence)
// i dostosowane.
//
//	go -C tools/evidence run . -root ../..                     # pakiet w out/evidence/<data>-<commit>/
//	go -C tools/evidence run . -root ../.. -tar                # także archiwum .tar.gz
//	go -C tools/evidence run . -root ../.. -candidate v0.4.0   # godziny fuzzingu i pomiary liczone dla tego commitu
//	go -C tools/evidence run . -root ../.. -stream-mb 1024     # plik 1 GB (K7) zmierzony osobnym przebiegiem testu
//
// Narzędzie jest osobnym modułem bez zależności: uruchamia go test, git i python3, czyli kod, który nie należy do
// biblioteki. Nie sięga do sieci (przebiegi mają GOPROXY=off), nie uruchamia fuzzingu i nie czyta danych spoza
// repozytorium (EVD-005). Kończy się kodem 1, gdy K4 albo K9 nie są spełnione; pakiet zapisuje także wtedy.
//
// Pomiary, których nie daje `go test` ani plik w docs/results (zastąpienie drzew konsumentów w K3, macierz CI
// i konsumenci w K8), narzędzie czyta z opcjonalnego pliku docs/measurements.json (typ Measurements). Gdy pliku
// albo wpisu nie ma, kryterium jest wykazane jako niezmierzone, nigdy jako spełnione.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	root := flag.String("root", ".", "goxmlgo repository root")
	outBase := flag.String("out", "", "directory for packages (default <root>/out/evidence)")
	makeTar := flag.Bool("tar", false, "also write <package>.tar.gz")
	candidate := flag.String("candidate", "", "release candidate commit for K5 and K7 (default: candidate from docs/measurements.json, else HEAD)")
	streamMB := flag.Int("stream-mb", 0, "also run TestStreamingMemory alone on a document of this size (GOXMLGO_STREAM_MB), e.g. 1024 for K7")
	flag.Parse()

	abs, err := filepath.Abs(*root)
	if err != nil {
		fail(err)
	}
	if *outBase == "" {
		*outBase = filepath.Join(abs, "out", "evidence")
	}
	now := time.Now()
	e, raw, err := build(abs, *candidate, *streamMB, now)
	if err != nil {
		fail(err)
	}
	dir := filepath.Join(*outBase, now.Format("2006-01-02")+"-"+e.Commit)
	if e.Dirty {
		dir += "-dirty"
	}
	if err := write(dir, e, raw); err != nil {
		fail(err)
	}
	if *makeTar {
		if err := tarDir(dir, dir+".tar.gz"); err != nil {
			fail(err)
		}
	}
	fmt.Printf("evidence package: %s\n", dir)
	for _, c := range e.Criteria {
		fmt.Printf("  %-3s %-8s %s\n", c.ID, c.Status, c.Value)
	}
	if blocking(e.Criteria) {
		os.Exit(1)
	}
}

// blocking mówi, czy narzędzie kończy się kodem 1: K4 albo K9 nie są spełnione (EVD-005).
func blocking(criteria []Criterion) bool {
	for _, c := range criteria {
		if (c.ID == "K4" || c.ID == "K9") && c.Status == "not_met" {
			return true
		}
	}
	return false
}

// testRuns zwraca przebiegi `go test`: moduł główny i każdy moduł narzędzia (tools/*/go.mod), w kolejności nazw.
func testRuns(root string) []runSpec {
	specs := []runSpec{{"moduł główny", ".", "go-test-module.json"}}
	mods, _ := filepath.Glob(filepath.Join(root, "tools", "*", "go.mod"))
	sort.Strings(mods)
	for _, m := range mods {
		name := filepath.Base(filepath.Dir(m))
		specs = append(specs, runSpec{"tools/" + name, "tools/" + name, "go-test-" + name + ".json"})
	}
	return specs
}

type runSpec struct{ name, dir, file string }

// build uruchamia przebiegi i składa pakiet. raw to surowe wyniki przebiegów (nazwa pliku → treść); zawierają czasy,
// więc nie wchodzą do części powtarzalnej.
func build(root, candidate string, streamMB int, now time.Time) (*Evidence, map[string][]byte, error) {
	reqs, problems, err := parseRequirements(root)
	if err != nil {
		return nil, nil, err
	}
	refs, err := collectRefs(root)
	if err != nil {
		return nil, nil, err
	}
	meas, err := readMeasurements(root)
	if err != nil {
		return nil, nil, err
	}
	raw := map[string][]byte{}
	var runs []*TestRun
	for _, spec := range testRuns(root) {
		out := runCmd(filepath.Join(root, spec.dir), testEnv(), "go", "test", "-count=1", "-json", "./...")
		raw[spec.file] = out
		cmd := "go test -count=1 -json ./..."
		if spec.dir != "." {
			cmd = "go -C " + spec.dir + " test -count=1 -json ./..."
		}
		run, err := parseTestEvents(spec.name, cmd, bytes.NewReader(out))
		if err != nil {
			return nil, nil, err
		}
		runs = append(runs, run)
	}
	// Plik 1 GB (K7) mierzy osobny przebieg samego TestStreamingMemory: test sprawdza pamięć pobraną od systemu
	// przez cały proces, więc inne testy w tym samym procesie zawyżyłyby wynik.
	var streamRun *TestRun
	if streamMB > 0 {
		size := "GOXMLGO_STREAM_MB=" + strconv.Itoa(streamMB)
		out := runCmd(root, append(testEnv(), size), "go", "test", ".", "-run", "^TestStreamingMemory$", "-count=1", "-json")
		raw["go-test-stream.json"] = out
		streamRun, err = parseTestEvents("plik "+strconv.Itoa(streamMB)+" MB (K7)", size+" go test . -run '^TestStreamingMemory$' -count=1 -json", bytes.NewReader(out))
		if err != nil {
			return nil, nil, err
		}
		runs = append(runs, streamRun)
	}
	py := PythonRun{Command: "python3 -I -B -m unittest discover -s tools/fetch -v"}
	pyOut := runCmd(root, testEnv(), "python3", "-I", "-B", "-m", "unittest", "discover", "-s", "tools/fetch", "-v")
	raw["python-fetch.txt"] = pyOut
	if m := regexp.MustCompile(`Ran (\d+) tests?`).FindSubmatch(pyOut); m != nil {
		py.Ran, _ = strconv.Atoi(string(m[1]))
	}
	lines := strings.Split(strings.TrimSpace(string(pyOut)), "\n")
	py.Summary = strings.TrimSpace(lines[len(lines)-1])
	py.OK = py.Ran > 0 && strings.HasPrefix(py.Summary, "OK")
	if py.Ran > 0 {
		py.Summary = fmt.Sprintf("%d testów, %s", py.Ran, py.Summary)
	}

	fv := FetchVerify{Command: "go -C tools/fetch run . -root ../.. -verify"}
	verifyOut, verifyErr := runCmdErr(filepath.Join(root, "tools", "fetch"), testEnv(), "go", "run", ".", "-root", "../..", "-verify")
	raw["fetch-verify.txt"] = verifyOut
	fv.OK = verifyErr == nil
	for _, l := range strings.Split(strings.TrimSpace(string(verifyOut)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			fv.Lines = append(fv.Lines, l)
		}
	}

	commit := strings.TrimSpace(string(runCmd(root, nil, "git", "rev-parse", "--short", "HEAD")))
	// Plik nieśledzony (np. nowy test) też zmienia przebieg, więc liczy się jako zmiana w drzewie roboczym.
	dirty := strings.TrimSpace(string(runCmd(root, nil, "git", "status", "--porcelain"))) != ""
	candidateSource := "flaga -candidate"
	if candidate == "" {
		candidate, candidateSource = meas.Candidate.Commit, "docs/measurements.json"
	}
	if candidate == "" {
		candidate, candidateSource = commit, "HEAD"
	}

	traces, unknown := buildTraces(reqs, refs, runs)
	in := Inputs{Runs: runs, PythonTests: py, Traces: traces, UnknownRefs: unknown, RequirementProblems: problems,
		Measurements: meas, FetchVerify: fv, Candidate: candidate, StreamMB: streamMB, StreamRun: streamRun}
	w3cRows, w3cExcl := parseW3C(in.output(pkgReader + ".TestXMLConformance"))
	in.W3C = w3cRows

	ratchets, err := readRatchets(root)
	if err != nil {
		return nil, nil, err
	}
	diffText, _ := os.ReadFile(filepath.Join(root, "docs", "differences.md"))
	for i := range ratchets {
		if ratchets[i].File == "testdata/w3c/xmlconf.ratchet" && ratchets[i].Mode == "failing" {
			in.W3CRatchet = &ratchets[i]
			in.W3CFailing = namedInDifferences(ratchets[i].IDs, string(diffText))
		}
	}
	in.CorpusDocuments = countCorpus(root)

	same := sameLibraryFunc(root, candidate)
	journal, _ := os.ReadFile(filepath.Join(root, "docs", "fuzzing.md"))
	fuzz := parseFuzzJournal(string(journal))
	in.FuzzHours, in.FuzzFailed = countFuzzHours(fuzz, same)

	benchText, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(benchFile)))
	bench := parseBench(string(benchText))
	bench.SameLibrary = bench.Commit != "" && same(bench.Commit)
	in.Bench = bench
	memText, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(memoryFile)))
	memory := parseStreamingMemory(string(memText))
	memory.SameLibrary = memory.Commit != "" && same(memory.Commit)
	in.Memory = memory

	sources, sourcesSHA, err := readSources(root)
	if err != nil {
		return nil, nil, err
	}
	goVersion := strings.TrimSpace(string(runCmd(root, nil, "go", "env", "GOVERSION")))
	python := strings.TrimSpace(string(runCmd(root, nil, "python3", "--version")))

	e := &Evidence{
		Generated:           now.Format("2006-01-02 15:04"),
		Commit:              commit,
		Dirty:               dirty,
		GoVersion:           goVersion,
		Platform:            runtime.GOOS + "/" + runtime.GOARCH,
		Python:              python,
		Candidate:           candidate,
		CandidateSource:     candidateSource,
		Criteria:            buildCriteria(in),
		Runs:                runs,
		PythonTests:         py,
		Traces:              traces,
		UnknownRefs:         unknown,
		RequirementProblems: problems,
		W3C:                 w3cRows,
		W3CExcluded:         w3cExcl,
		W3CFailing:          in.W3CFailing,
		Ratchets:            ratchets,
		Fuzz:                fuzz,
		FuzzHours:           in.FuzzHours.Hours(),
		Bench:               bench,
		Memory:              memory,
		StreamMB:            streamMB,
		Measurements:        meas,
		Sources:             sources,
		SourcesSHA:          sourcesSHA,
		FetchVerify:         fv,
		ADRs:                readADRs(root),
		Deps:                readDeps(root),
		Differences:         diffSections(string(diffText)),
	}
	for name := range raw {
		e.Files = append(e.Files, name)
	}
	e.Files = append(e.Files, "evidence.json", "report.md")
	sort.Strings(e.Files)
	return e, raw, nil
}

// testEnv to środowisko przebiegów: bez zmiennych GOXMLGO_*, które mogłyby zmienić testy albo wskazać dane spoza
// repozytorium (rozmiar dla TestStreamingMemory ustawia wyłącznie flaga -stream-mb), i bez dostępu polecenia go do sieci (moduły narzędzi muszą być już w pamięci podręcznej modułów;
// brakujący moduł to nieudany przebieg, nie pobranie). EVD-005.
func testEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GOXMLGO_") || name == "GOPROXY" || name == "GOTOOLCHAIN" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GOPROXY=off", "GOTOOLCHAIN=local")
}

// sameLibraryFunc zwraca funkcję, która mówi, czy commit ma ten sam kod biblioteki co kandydat: różnica poza
// testami, danymi testowymi (w każdym katalogu testdata), dokumentami (docs, requirements i pliki .md),
// narzędziami, CI i ustawieniami Claude jest pusta (docs/fuzzing.md, „Jak liczymy godziny do K5”). W pathspec bez
// „glob” gwiazdka obejmuje też „/”. Commit, którego git nie zna, nie jest tym samym kodem.
func sameLibraryFunc(root, candidate string) func(commit string) bool {
	memo := map[string]bool{}
	return func(commit string) bool {
		if candidate == "" || commit == "" {
			return false
		}
		if v, ok := memo[commit]; ok {
			return v
		}
		cmd := exec.Command("git", append([]string{"diff", "--quiet", candidate, commit, "--", "."}, libraryExcludes...)...)
		cmd.Dir = root
		memo[commit] = cmd.Run() == nil
		return memo[commit]
	}
}

// libraryExcludes: ścieżki, które nie są kodem biblioteki (sameLibraryFunc).
var libraryExcludes = []string{":!*_test.go", ":!*testdata/*", ":!docs", ":!requirements", ":!*.md", ":!tools", ":!.github", ":!.claude"}

func runCmd(dir string, env []string, name string, args ...string) []byte {
	out, _ := runCmdErr(dir, env, name, args...)
	return out
}

func runCmdErr(dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

func write(dir string, e *Evidence, raw map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, b := range raw {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return err
		}
	}
	js, err := marshal(e)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence.json"), js, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report.md"), []byte(renderReport(e)), 0o644)
}

func marshal(e *Evidence) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(e)
	return buf.Bytes(), err
}

func tarDir(dir, target string) error {
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	base := filepath.Base(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		b, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: base + "/" + ent.Name(), Mode: 0o644, Size: int64(len(b)), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err := io.Copy(tw, bytes.NewReader(b)); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "evidence:", err)
	os.Exit(1)
}

// readDeps czyta zależności modułu biblioteki: go list -m all (bez modułu głównego) i sumy h1 z go.sum. Biblioteka
// ma ich nie mieć (ADR 0001, pkt 1); pusta lista w pakiecie jest tego dowodem.
func readDeps(root string) []Dependency {
	sums := map[string]string{}
	b, _ := os.ReadFile(filepath.Join(root, "go.sum"))
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) == 3 && !strings.HasSuffix(f[1], "/go.mod") {
			sums[f[0]+" "+f[1]] = f[2]
		}
	}
	var deps []Dependency
	out := runCmd(root, testEnv(), "go", "list", "-m", "-f", "{{if not .Main}}{{.Path}} {{.Version}}{{end}}", "all")
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		deps = append(deps, Dependency{Path: f[0], Version: f[1], Sum: sums[f[0]+" "+f[1]]})
	}
	return deps
}
