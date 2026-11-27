package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Requirement to wiersz tabeli wymagań z requirements/*.md. Accepted: wiersz stoi pod „Status: przyjęte”
// (requirements/README.md); tylko takie wymaganie musi mieć test.
type Requirement struct {
	ID       string `json:"id"`
	File     string `json:"file"`
	Text     string `json:"text"`
	Accepted bool   `json:"accepted"`
}

// Ref to odwołanie „REQ:” w teście albo w pliku korpusu. Test jest pusty, gdy odwołania nie da się przypisać
// do funkcji testowej (np. komentarz przy funkcji pomocniczej).
type Ref struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Package string `json:"package,omitempty"`
	Test    string `json:"test,omitempty"`
	Corpus  bool   `json:"corpus,omitempty"`
}

// Key to klucz wyniku testu: ścieżka importu pakietu i nazwa testu najwyższego poziomu.
func (r Ref) Key() string { return r.Package + "." + r.Test }

// Te same reguły co w TestRequirementsCovered (requirements_test.go w katalogu głównym): prefiksy z tabeli
// w requirements/README.md, a identyfikatorem jest wszystko, co tak wygląda, także z prefiksem spoza tabeli.
var (
	prefixRow = regexp.MustCompile("^\\| `([A-Z]{2,5})` \\|")
	anyID     = regexp.MustCompile(`\b[A-Z]{2,5}-\d{3}\b`)
)

const refMark = "REQ" + ":"

// parseRequirements czyta prefiksy z tabeli w requirements/README.md i wymagania ze wszystkich pozostałych plików
// requirements/*.md, w kolejności plików i wierszy. Wiersz „Status: szkic” albo „Status: przyjęte” dotyczy tabel
// pod nim, do następnego takiego wiersza. Drugi wynik to błędy w samych wymaganiach (nieznany status, prefiks
// spoza tabeli, wymaganie przed pierwszym wierszem statusu, identyfikator zdefiniowany dwa razy): pakiet powstaje
// mimo nich, ale K4 nie jest wtedy spełnione.
func parseRequirements(root string) ([]Requirement, []string, error) {
	readme, err := os.ReadFile(filepath.Join(root, "requirements", "README.md"))
	if err != nil {
		return nil, nil, err
	}
	prefixes := map[string]bool{}
	for _, line := range strings.Split(string(readme), "\n") {
		if m := prefixRow.FindStringSubmatch(line); m != nil {
			prefixes[m[1]] = true
		}
	}
	if len(prefixes) == 0 {
		return nil, nil, fmt.Errorf("no prefixes in requirements/README.md")
	}
	files, _ := filepath.Glob(filepath.Join(root, "requirements", "*.md"))
	sort.Strings(files)
	var reqs []Requirement
	var problems []string
	seen := map[string]bool{}
	for _, f := range files {
		if filepath.Base(f) == "README.md" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		name := rel(root, f)
		status := ""
		for _, line := range strings.Split(string(b), "\n") {
			if s, ok := strings.CutPrefix(line, "Status: "); ok {
				status = strings.TrimSpace(s)
				if status != "szkic" && status != "przyjęte" {
					problems = append(problems, fmt.Sprintf("%s: nieznany status „%s”", name, status))
				}
				continue
			}
			id := anyID.FindString(line)
			if id == "" || !strings.HasPrefix(line, "| "+id+" |") {
				continue
			}
			if !prefixes[id[:strings.IndexByte(id, '-')]] {
				problems = append(problems, fmt.Sprintf("%s: %s ma prefiks spoza requirements/README.md", name, id))
			}
			if status == "" {
				problems = append(problems, fmt.Sprintf("%s: %s stoi przed pierwszym wierszem statusu", name, id))
			}
			if seen[id] {
				problems = append(problems, fmt.Sprintf("%s: %s zdefiniowane drugi raz", name, id))
				continue
			}
			seen[id] = true
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "| "+id+" |")), "|"))
			reqs = append(reqs, Requirement{ID: id, File: name, Text: text, Accepted: status == "przyjęte"})
		}
	}
	return reqs, problems, nil
}

// skipDirs to katalogi robocze, których nie przeszukujemy (jak w TestRequirementsCovered); katalogi z kropką
// i katalogi testdata też są pomijane.
var skipDirs = map[string]bool{"out": true, "ref": true, "node_modules": true}

// collectRefs zbiera odwołania „REQ:” z komentarzy w plikach *_test.go (każdego modułu w repozytorium) i z plików
// testdata/corpus/*. Identyfikatory to wszystko, co wygląda jak identyfikator, w tym samym wierszu za „REQ:”.
// Komentarz wewnątrz funkcji należy do tej funkcji; komentarz poza funkcją — do najbliższej funkcji poniżej.
// Funkcje Test*, Fuzz*, Example* i Benchmark* są dowodem; inne (pomocnicze) — dowodem wspierającym.
func collectRefs(root string) ([]Ref, error) {
	var refs []Ref
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root && (skipDirs[name] || name == "testdata" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fileRefs, err := refsInGoFile(root, p)
		refs = append(refs, fileRefs...)
		return err
	})
	if err != nil {
		return nil, err
	}
	corpus, _ := filepath.Glob(filepath.Join(root, "testdata", "corpus", "*"))
	sort.Strings(corpus)
	for _, f := range corpus {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			if i := strings.Index(line, refMark); i >= 0 {
				for _, id := range anyID.FindAllString(line[i:], -1) {
					refs = append(refs, Ref{ID: id, File: rel(root, f), Corpus: true})
				}
			}
		}
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].ID != refs[j].ID {
			return refs[i].ID < refs[j].ID
		}
		return refs[i].File+refs[i].Test < refs[j].File+refs[j].Test
	})
	return dedupRefs(refs), nil
}

func refsInGoFile(root, p string) ([]Ref, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel(root, p), err)
	}
	pkg, err := importPath(root, filepath.Dir(p))
	if err != nil {
		return nil, err
	}
	var funcs []*ast.FuncDecl
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
			funcs = append(funcs, fn)
		}
	}
	var refs []Ref
	for _, cg := range f.Comments {
		var ids []string
		for _, line := range strings.Split(cg.Text(), "\n") {
			if i := strings.Index(line, refMark); i >= 0 {
				ids = append(ids, anyID.FindAllString(line[i:], -1)...)
			}
		}
		if len(ids) == 0 {
			continue
		}
		var owner *ast.FuncDecl
		for _, fn := range funcs {
			if cg.Pos() >= fn.Pos() && cg.End() <= fn.End() {
				owner = fn
				break
			}
		}
		if owner == nil {
			for _, fn := range funcs {
				if fn.Pos() > cg.End() {
					owner = fn
					break
				}
			}
		}
		test := ""
		if owner != nil && isTestName(owner.Name.Name) {
			test = owner.Name.Name
		}
		for _, id := range ids {
			refs = append(refs, Ref{ID: id, File: rel(root, p), Package: pkg, Test: test})
		}
	}
	return refs, nil
}

func isTestName(name string) bool {
	for _, prefix := range []string{"Test", "Fuzz", "Example", "Benchmark"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func dedupRefs(refs []Ref) []Ref {
	seen := map[Ref]bool{}
	out := refs[:0]
	for _, r := range refs {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// importPath ustala ścieżkę importu katalogu z najbliższego go.mod w górę drzewa (do root włącznie).
func importPath(root, dir string) (string, error) {
	for d := dir; ; d = filepath.Dir(d) {
		b, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil {
			mod := ""
			for _, line := range strings.Split(string(b), "\n") {
				if m, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					mod = strings.TrimSpace(m)
					break
				}
			}
			if mod == "" {
				return "", fmt.Errorf("%s/go.mod: no module line", d)
			}
			r, err := filepath.Rel(d, dir)
			if err != nil {
				return "", err
			}
			if r == "." {
				return mod, nil
			}
			return mod + "/" + filepath.ToSlash(r), nil
		}
		if d == root || d == filepath.Dir(d) {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
	}
}

// TestRun to wynik jednego przebiegu `go test -json`.
type TestRun struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Skipped int    `json:"skipped"`
	// FailedTests i SkippedTests to klucze testów najwyższego poziomu (pakiet.Test), posortowane.
	FailedTests  []string `json:"failed_tests,omitempty"`
	SkippedTests []string `json:"skipped_tests,omitempty"`
	// BuildFailed to pakiety, które się nie skompilowały albo zakończyły bez wyniku testu.
	BuildFailed []string `json:"build_failed,omitempty"`
	results     map[string]string
	output      map[string][]string
}

type testEvent struct {
	Action, Package, Test, Output string
}

// parseTestEvents czyta strumień `go test -json` i zbiera wyniki oraz wyjście testów najwyższego poziomu.
func parseTestEvents(name, command string, r io.Reader) (*TestRun, error) {
	run := &TestRun{Name: name, Command: command, results: map[string]string{}, output: map[string][]string{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	pkgFailed := map[string]bool{}
	pkgHasTests := map[string]bool{}
	for sc.Scan() {
		var e testEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.Test == "" {
			if e.Action == "fail" {
				pkgFailed[e.Package] = true
			}
			continue
		}
		if strings.Contains(e.Test, "/") {
			continue
		}
		key := e.Package + "." + e.Test
		pkgHasTests[e.Package] = true
		switch e.Action {
		case "pass", "fail", "skip":
			run.results[key] = e.Action
		case "output":
			run.output[key] = append(run.output[key], strings.TrimRight(e.Output, "\n"))
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	// Mapy służą tylko do zliczenia; listy są niżej sortowane, więc wynik nie zależy od kolejności iteracji.
	for key, action := range run.results {
		switch action {
		case "pass":
			run.Passed++
		case "fail":
			run.Failed++
			run.FailedTests = append(run.FailedTests, key)
		case "skip":
			run.Skipped++
			run.SkippedTests = append(run.SkippedTests, key)
		}
	}
	for pkg := range pkgFailed {
		if !pkgHasTests[pkg] {
			run.BuildFailed = append(run.BuildFailed, pkg)
		}
	}
	sort.Strings(run.FailedTests)
	sort.Strings(run.SkippedTests)
	sort.Strings(run.BuildFailed)
	return run, nil
}

// Result zwraca wynik testu (pass, fail, skip) albo "" dla nieznanego.
func (r *TestRun) Result(key string) string { return r.results[key] }

// Output zwraca wiersze wyjścia testu.
func (r *TestRun) Output(key string) []string { return r.output[key] }

// Trace to wiersz macierzy śledzenia: wymaganie, jego dowody i wynik.
type Trace struct {
	Requirement
	Evidence []string `json:"evidence"`
	// Status: pass (wszystkie testy przeszły), manual (dowód uruchamiany na żądanie: benchmark), skip (test
	// pominięty, np. bez pobranych danych), unresolved (tylko funkcje pomocnicze albo test nieznany w przebiegu),
	// fail, missing (brak dowodu), draft (wymaganie w szkicu: test nie jest wymagany, dowody są tylko wypisane).
	Status string `json:"status"`
}

var statusRank = map[string]int{"pass": 0, "manual": 1, "skip": 2, "unresolved": 3, "fail": 4, "missing": 5}

// buildTraces łączy wymagania, odwołania i wyniki przebiegów w macierz śledzenia; zwraca też odwołania do
// nieistniejących wymagań. Odwołanie przy funkcji pomocniczej (np. sprawdzeniu wołanym przez kilka testów) jest
// dowodem wspierającym: nie pogarsza stanu, ale samo nie wystarcza.
func buildTraces(reqs []Requirement, refs []Ref, runs []*TestRun) ([]Trace, []Ref) {
	defined := map[string]bool{}
	for _, r := range reqs {
		defined[r.ID] = true
	}
	byID := map[string][]Ref{}
	var unknown []Ref
	for _, ref := range refs {
		if !defined[ref.ID] {
			unknown = append(unknown, ref)
			continue
		}
		byID[ref.ID] = append(byID[ref.ID], ref)
	}
	lookup := func(key string) string {
		for _, run := range runs {
			if a := run.Result(key); a != "" {
				return a
			}
		}
		return ""
	}
	var traces []Trace
	for _, req := range reqs {
		t := Trace{Requirement: req, Status: "pass"}
		resolved, supporting := 0, 0
		worse := func(s string) {
			if statusRank[s] > statusRank[t.Status] {
				t.Status = s
			}
		}
		for _, ref := range byID[req.ID] {
			switch {
			case ref.Corpus:
				t.Evidence = append(t.Evidence, ref.File)
				resolved++
			case ref.Test == "":
				t.Evidence = append(t.Evidence, ref.File+" (funkcja pomocnicza)")
				supporting++
			case strings.HasPrefix(ref.Test, "Benchmark"):
				t.Evidence = append(t.Evidence, ref.Key()+" (benchmark, na żądanie)")
				resolved++
				worse("manual")
			default:
				resolved++
				switch lookup(ref.Key()) {
				case "pass":
					t.Evidence = append(t.Evidence, ref.Key())
				case "skip":
					t.Evidence = append(t.Evidence, ref.Key()+" (pominięty)")
					worse("skip")
				case "fail":
					t.Evidence = append(t.Evidence, ref.Key()+" (nie przeszedł)")
					worse("fail")
				default:
					t.Evidence = append(t.Evidence, ref.Key()+" (brak w przebiegu)")
					worse("unresolved")
				}
			}
		}
		switch {
		case !req.Accepted:
			t.Status = "draft"
		case resolved == 0 && supporting > 0:
			t.Status = "unresolved"
		case resolved == 0:
			t.Status = "missing"
		}
		traces = append(traces, t)
	}
	return traces, unknown
}

// W3CRow to wiersz raportu TestXMLConformance dla jednej grupy zestawu (albo „all”).
type W3CRow struct {
	Group   string `json:"group"`
	Passed  int    `json:"passed"`
	InScope int    `json:"in_scope"`
}

// W3CExclusion to liczba przypadków wyłączonych z danego powodu.
type W3CExclusion struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

var (
	w3cRowRe      = regexp.MustCompile(`^\s*([A-Za-z][\w.-]*)\s+(\d+)/(\d+)\s*$`)
	w3cExcludedRe = regexp.MustCompile(`^\s*excluded\s+(\d+): (.+)$`)
)

// parseW3C odczytuje tabelę zaliczeń i wyłączeń z wyjścia TestXMLConformance.
func parseW3C(lines []string) ([]W3CRow, []W3CExclusion) {
	var rows []W3CRow
	var excl []W3CExclusion
	for _, l := range lines {
		for _, part := range strings.Split(l, "\n") {
			if m := w3cExcludedRe.FindStringSubmatch(part); m != nil {
				c, _ := strconv.Atoi(m[1])
				excl = append(excl, W3CExclusion{strings.TrimSpace(m[2]), c})
			} else if m := w3cRowRe.FindStringSubmatch(part); m != nil {
				n := atoi(m[2:])
				rows = append(rows, W3CRow{m[1], n[0], n[1]})
			}
		}
	}
	return rows, excl
}

func atoi(ss []string) []int {
	out := make([]int, len(ss))
	for i, s := range ss {
		out[i], _ = strconv.Atoi(s)
	}
	return out
}

// findCount szuka w wierszach wyjścia pierwszego dopasowania wyrażenia z dwiema liczbami (x z y).
func findCount(lines []string, re *regexp.Regexp) (int, int, bool) {
	for _, l := range lines {
		if m := re.FindStringSubmatch(l); m != nil {
			n := atoi(m[1:3])
			return n[0], n[1], true
		}
	}
	return 0, 0, false
}

// findNumber szuka w wierszach wyjścia pierwszego dopasowania wyrażenia z jedną liczbą.
func findNumber(lines []string, re *regexp.Regexp) (int, bool) {
	for _, l := range lines {
		if m := re.FindStringSubmatch(l); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n, true
		}
	}
	return 0, false
}

// countCorpus liczy dokumenty korpusu tak jak testy (testdata/corpus/*.xml bez wzorców zapisu *.out.xml).
func countCorpus(root string) int {
	files, _ := filepath.Glob(filepath.Join(root, "testdata", "corpus", "*.xml"))
	n := 0
	for _, f := range files {
		if !strings.HasSuffix(f, ".out.xml") {
			n++
		}
	}
	return n
}

// Ratchet to nagłówek pliku zapadki.
type Ratchet struct {
	File    string   `json:"file"`
	InScope int      `json:"in_scope"`
	Passing int      `json:"passing"`
	Mode    string   `json:"mode"`
	Listed  int      `json:"listed"`
	IDs     []string `json:"-"`
}

var ratchetCountsRe = regexp.MustCompile(`^# in scope (\d+), passing (\d+)`)

// readRatchets czyta nagłówki wszystkich plików *.ratchet w testdata (bez pobranych katalogów).
func readRatchets(root string) ([]Ratchet, error) {
	var out []Ratchet
	err := filepath.WalkDir(filepath.Join(root, "testdata"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(p, ".ratchet") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		r := Ratchet{File: rel(root, p), Mode: "passing"}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			switch {
			case ratchetCountsRe.MatchString(line):
				n := atoi(ratchetCountsRe.FindStringSubmatch(line)[1:3])
				r.InScope, r.Passing = n[0], n[1]
			case strings.HasPrefix(line, "# mode: "):
				r.Mode = strings.TrimPrefix(line, "# mode: ")
			case line == "" || strings.HasPrefix(line, "#"):
			default:
				r.IDs = append(r.IDs, line)
			}
		}
		r.Listed = len(r.IDs)
		out = append(out, r)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, err
}

// NamedCase to przypadek W3C, który nie przechodzi, i informacja, czy differences.md go nazywa.
type NamedCase struct {
	ID    string `json:"id"`
	Named bool   `json:"named"`
}

// namedInDifferences sprawdza dla każdego niezaliczonego przypadku W3C, czy differences.md wymienia jego
// identyfikator z zapadki dosłownie, w odwrotnych apostrofach (np. `eduni/rmt-e3e-13`).
func namedInDifferences(ids []string, differences string) []NamedCase {
	var out []NamedCase
	for _, id := range ids {
		out = append(out, NamedCase{id, strings.Contains(differences, "`"+id+"`")})
	}
	return out
}

// FuzzRun to wiersz dziennika fuzzingu z docs/fuzzing.md (tabela „Przebiegi”).
type FuzzRun struct {
	Date     string `json:"date"`
	Commit   string `json:"commit"`
	Host     string `json:"host"`
	Fuzzer   string `json:"fuzzer"`
	Duration string `json:"duration"`
	Execs    string `json:"execs"`
	Result   string `json:"result"`
	// Dirty: przebieg na kodzie z niezatwierdzonymi zmianami (oznaczenie „(zmiany)”); nie liczy się do K5.
	Dirty bool `json:"dirty,omitempty"`
	// Counted: wiersz liczy się do K5 (wynik „ok”, czas zapisany jako liczba, commit kandydata albo bez zmian
	// biblioteki, bez „(zmiany)”, bez późniejszego przebiegu z błędem na tym samym kodzie).
	Counted bool `json:"counted"`
	seconds time.Duration
}

var (
	hexRe      = regexp.MustCompile(`[0-9a-f]{7,40}`)
	fuzzDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
)

// parseFuzzJournal czyta tabelę pod nagłówkiem „## Przebiegi” w docs/fuzzing.md: wiersze z datą, commitem i nazwą
// fuzzera. Wiersz kampanii zleconej albo wycofanej (czas i wykonania opisane słowami) zostaje na liście z czasem
// zerowym, więc nie liczy się do K5.
func parseFuzzJournal(text string) []FuzzRun {
	var out []FuzzRun
	inRuns := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			inRuns = line == "## Przebiegi"
			continue
		}
		if !inRuns || !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 7 {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if !fuzzDateRe.MatchString(cells[0]) || !strings.Contains(cells[3], "Fuzz") {
			continue
		}
		r := FuzzRun{Date: cells[0], Commit: hexRe.FindString(cells[1]), Host: cells[2], Fuzzer: strings.ReplaceAll(cells[3], "`", ""),
			Duration: cells[4], Execs: cells[5], Result: strings.Join(cells[6:], "|")}
		r.Dirty = strings.Contains(cells[1], "(zmiany)")
		if d, err := time.ParseDuration(r.Duration); err == nil {
			r.seconds = d
		}
		out = append(out, r)
	}
	return out
}

// countFuzzHours oznacza wiersze liczone do K5 i zwraca ich łączny czas. same mówi, czy commit ma ten sam kod
// biblioteki co kandydat. Przebieg zakończony błędem na kodzie kandydata zeruje licznik (docs/fuzzing.md, „Jak
// liczymy godziny do K5”): liczą się tylko wiersze po nim, a drugi wynik mówi, że taki przebieg był.
func countFuzzHours(runs []FuzzRun, same func(commit string) bool) (time.Duration, bool) {
	var total time.Duration
	failed := false
	for i := range runs {
		r := &runs[i]
		r.Counted = false
		if r.Dirty || r.seconds <= 0 || r.Commit == "" || !same(r.Commit) {
			continue
		}
		if !strings.HasPrefix(r.Result, "ok") {
			failed = true
			total = 0
			for j := range runs[:i] {
				runs[j].Counted = false
			}
			continue
		}
		r.Counted = true
		total += r.seconds
	}
	return total, failed
}

// benchFile to surowy wynik tools/bench, z którego bierzemy liczby K7.
const benchFile = "docs/results/reader-vs-goxsdgo.txt"

// Bench to liczby K7 z pliku wyniku tools/bench.
type Bench struct {
	File   string `json:"file"`
	Found  bool   `json:"found"`
	Date   string `json:"date,omitempty"`
	Commit string `json:"commit,omitempty"`
	// SameLibrary: commit pomiaru ma ten sam kod biblioteki co kandydat; inaczej pomiar jest nieaktualny.
	SameLibrary bool        `json:"same_library_as_candidate"`
	Documents   []BenchLine `json:"documents,omitempty"`
}

// BenchLine to stosunki dla jednego dokumentu wzorcowego: czytnik wobec goxsdgo v0.3.0 oraz przelot czytnik →
// pisarz wobec samego czytnika i wobec przelotu encoding/xml. Zero oznacza brak wiersza w pliku.
type BenchLine struct {
	Document     string  `json:"document"`
	ReaderRatio  float64 `json:"reader_vs_goxsdgo"`
	PassOfReader float64 `json:"pass_through_of_reader"`
	PassOfStdlib float64 `json:"pass_through_of_encoding_xml"`
}

var (
	benchRatioRe = regexp.MustCompile(`^# ratio (.+): ([0-9.]+)`)
	benchPassRe  = regexp.MustCompile(`^# pass-through (.+): goxmlgo [0-9.]+ MB/s, ([0-9.]+) of its reader alone, ([0-9.]+) of encoding/xml`)
)

// parseBench czyta nagłówek (data, commit) i wiersze stosunków z końca pliku wyniku tools/bench.
func parseBench(text string) Bench {
	b := Bench{File: benchFile}
	find := func(doc string) *BenchLine {
		for i := range b.Documents {
			if b.Documents[i].Document == doc {
				return &b.Documents[i]
			}
		}
		b.Documents = append(b.Documents, BenchLine{Document: doc})
		return &b.Documents[len(b.Documents)-1]
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if s, ok := strings.CutPrefix(line, "# date: "); ok {
			b.Date = strings.TrimSpace(s)
		} else if s, ok := strings.CutPrefix(line, "# goxmlgo: "); ok {
			b.Commit = hexRe.FindString(s)
		} else if m := benchRatioRe.FindStringSubmatch(line); m != nil {
			l := find(m[1])
			l.ReaderRatio, _ = strconv.ParseFloat(m[2], 64)
		} else if m := benchPassRe.FindStringSubmatch(line); m != nil {
			l := find(m[1])
			l.PassOfReader, _ = strconv.ParseFloat(m[2], 64)
			l.PassOfStdlib, _ = strconv.ParseFloat(m[3], 64)
		}
	}
	b.Found = len(b.Documents) > 0
	return b
}

// memoryFile to surowy wynik TestStreamingMemory na pliku 1 GB (K7).
const memoryFile = "docs/results/streaming-memory.txt"

// StreamMemory to pomiar przelotu dużego pliku przez czytnik → pisarz: z pliku wyniku albo z tego przebiegu.
type StreamMemory struct {
	File   string `json:"file,omitempty"`
	Found  bool   `json:"found"`
	Date   string `json:"date,omitempty"`
	Commit string `json:"commit,omitempty"`
	// Uncommitted: nagłówek mówi, że pomiar zrobiono z drzewa ze zmianami; taki pomiar nie spełnia kryterium.
	Uncommitted bool `json:"uncommitted,omitempty"`
	// SameLibrary: commit pomiaru ma ten sam kod biblioteki co kandydat.
	SameLibrary bool    `json:"same_library_as_candidate"`
	SizeMB      float64 `json:"size_mb,omitempty"`
	HeapMB      float64 `json:"peak_heap_mb,omitempty"`
	ProcessMB   float64 `json:"process_mb,omitempty"`
}

var streamRe = regexp.MustCompile(`pass-through of (\d+) MB .*peak heap ([0-9.]+) MB, memory obtained from the system by the test process ([0-9.]+) MB`)

// parseStreamLine szuka w wierszach wyjścia TestStreamingMemory rozmiaru dokumentu, szczytu sterty i pamięci
// pobranej od systemu przez proces testu.
func parseStreamLine(lines []string) (size, heap, process float64, ok bool) {
	for _, l := range lines {
		if m := streamRe.FindStringSubmatch(l); m != nil {
			size, _ = strconv.ParseFloat(m[1], 64)
			heap, _ = strconv.ParseFloat(m[2], 64)
			process, _ = strconv.ParseFloat(m[3], 64)
			return size, heap, process, true
		}
	}
	return
}

// parseStreamingMemory czyta zapisany wynik TestStreamingMemory: nagłówek (data, commit) i wiersz pomiaru.
func parseStreamingMemory(text string) StreamMemory {
	s := StreamMemory{File: memoryFile}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "# date: "); ok {
			s.Date = strings.TrimSpace(v)
		} else if v, ok := strings.CutPrefix(line, "# goxmlgo: "); ok {
			s.Commit = hexRe.FindString(v)
			s.Uncommitted = strings.Contains(v, "uncommitted") || strings.Contains(v, "(zmiany)")
		}
	}
	s.SizeMB, s.HeapMB, s.ProcessMB, s.Found = parseStreamLine(lines)
	return s
}

// ADR to decyzja z docs/adr.
type ADR struct {
	File   string `json:"file"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func readADRs(root string) []ADR {
	files, _ := filepath.Glob(filepath.Join(root, "docs", "adr", "[0-9]*.md"))
	sort.Strings(files)
	var out []ADR
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		a := ADR{File: rel(root, f)}
		for _, line := range strings.Split(string(b), "\n") {
			if a.Title == "" && strings.HasPrefix(line, "# ") {
				a.Title = strings.TrimPrefix(line, "# ")
			}
			if s, ok := strings.CutPrefix(line, "Status: "); ok && a.Status == "" {
				a.Status = strings.TrimSpace(s)
			}
		}
		out = append(out, a)
	}
	return out
}

// DiffSection to sekcja docs/differences.md z liczbą wierszy tabel.
type DiffSection struct {
	Title string `json:"title"`
	Rows  int    `json:"rows"`
}

// diffSections liczy w każdej sekcji wiersze danych tabel (bez nagłówków: wiersz, po którym stoi „|---”).
func diffSections(text string) []DiffSection {
	var out []DiffSection
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "## "):
			out = append(out, DiffSection{Title: strings.TrimPrefix(line, "## ")})
		case len(out) > 0 && strings.HasPrefix(line, "| ") && !strings.HasPrefix(line, "|---"):
			if i+1 < len(lines) && strings.HasPrefix(lines[i+1], "|---") {
				continue
			}
			out[len(out)-1].Rows++
		}
	}
	return out
}

// SourceSet to zestaw danych testowych z testdata/sources.json.
type SourceSet struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Dest       string `json:"dest"`
	Files      int    `json:"files"`
	SHA256     string `json:"sha256,omitempty"`
	TreeSHA256 string `json:"tree_sha256,omitempty"`
	LocalOnly  int    `json:"local_only,omitempty"`
}

func readSources(root string) ([]SourceSet, string, error) {
	b, err := os.ReadFile(filepath.Join(root, "testdata", "sources.json"))
	if err != nil {
		return nil, "", err
	}
	var m struct {
		Sets []struct {
			Name, Kind, Dest string
			SHA256           string `json:"sha256"`
			TreeSHA256       string `json:"tree_sha256"`
			FileCount        int    `json:"file_count"`
			Files            []struct {
				LocalOnly bool `json:"local_only"`
			}
		}
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, "", err
	}
	var out []SourceSet
	for _, s := range m.Sets {
		set := SourceSet{Name: s.Name, Kind: s.Kind, Dest: s.Dest, SHA256: s.SHA256, TreeSHA256: s.TreeSHA256, Files: s.FileCount}
		if s.Kind == "files" {
			set.Files = len(s.Files)
			for _, f := range s.Files {
				if f.LocalOnly {
					set.LocalOnly++
				}
			}
		}
		out = append(out, set)
	}
	sum := sha256.Sum256(b)
	return out, hex.EncodeToString(sum[:]), nil
}

// Measurements to opcjonalny plik docs/measurements.json: pomiary, których nie daje `go test` ani plik
// w docs/results. Każdy wpis ma datę i commit pomiaru. Brak pliku albo wpisu oznacza „nie zmierzono”.
type Measurements struct {
	// Found: plik istnieje.
	Found     bool `json:"found"`
	Candidate struct {
		Commit string `json:"commit,omitempty"`
		Note   string `json:"note,omitempty"`
	} `json:"candidate"`
	// K3Consumers: oba istniejące drzewa (goxadesgo, goxsdgo) dają się zastąpić (ADR 0001, K3; etap 6).
	K3Consumers ExternalResult `json:"k3_consumers"`
	// K8: macierz CI (te same bajty na Linux, macOS, Windows) i testy konsumentów po przełączeniu.
	K8 ExternalResult `json:"k8"`
}

// ExternalResult to wynik sprawdzenia poza tym repozytorium. Status: met, not_met albo pending.
type ExternalResult struct {
	Status string `json:"status,omitempty"`
	Date   string `json:"date,omitempty"`
	Commit string `json:"commit,omitempty"`
	Result string `json:"result,omitempty"`
	Note   string `json:"note,omitempty"`
}

func readMeasurements(root string) (*Measurements, error) {
	var m Measurements
	b, err := os.ReadFile(filepath.Join(root, "docs", "measurements.json"))
	if os.IsNotExist(err) {
		return &m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("docs/measurements.json: %w", err)
	}
	m.Found = true
	return &m, nil
}

func rel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(r)
}
