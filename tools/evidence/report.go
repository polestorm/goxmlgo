package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Evidence to cała zawartość pakietu poza surowymi wynikami przebiegów. Dla tego samego commitu i tych samych
// danych różni się tylko polem Generated (EVD-004).
type Evidence struct {
	Generated           string         `json:"generated"`
	Commit              string         `json:"commit"`
	Dirty               bool           `json:"dirty"`
	GoVersion           string         `json:"go_version"`
	Platform            string         `json:"platform"`
	Python              string         `json:"python"`
	Candidate           string         `json:"candidate"`
	CandidateSource     string         `json:"candidate_source"`
	Criteria            []Criterion    `json:"criteria"`
	Runs                []*TestRun     `json:"runs"`
	PythonTests         PythonRun      `json:"python_tests"`
	Traces              []Trace        `json:"traces"`
	UnknownRefs         []Ref          `json:"unknown_refs,omitempty"`
	RequirementProblems []string       `json:"requirement_problems,omitempty"`
	W3C                 []W3CRow       `json:"w3c"`
	W3CExcluded         []W3CExclusion `json:"w3c_excluded"`
	W3CFailing          []NamedCase    `json:"w3c_failing"`
	Ratchets            []Ratchet      `json:"ratchets"`
	Fuzz                []FuzzRun      `json:"fuzz"`
	FuzzHours           float64        `json:"fuzz_hours_counted"`
	Bench               Bench          `json:"bench"`
	Memory              StreamMemory   `json:"streaming_memory"`
	StreamMB            int            `json:"stream_mb,omitempty"`
	Measurements        *Measurements  `json:"measurements"`
	Sources             []SourceSet    `json:"sources"`
	SourcesSHA          string         `json:"sources_json_sha256"`
	FetchVerify         FetchVerify    `json:"fetch_verify"`
	ADRs                []ADR          `json:"adrs"`
	Deps                []Dependency   `json:"dependencies"`
	Differences         []DiffSection  `json:"differences"`
	Files               []string       `json:"files"`
}

// Dependency to moduł wymagany przez bibliotekę (go list -m all bez modułu głównego) z sumą z go.sum.
type Dependency struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"go_sum"`
}

// PythonRun to wynik testów tools/fetch w Pythonie.
type PythonRun struct {
	Command string `json:"command"`
	Ran     int    `json:"ran"`
	OK      bool   `json:"ok"`
	Summary string `json:"summary"`
}

// FetchVerify to wynik `tools/fetch -verify`: czy pobrane dane mają sumy z manifestu.
type FetchVerify struct {
	Command string   `json:"command"`
	OK      bool     `json:"ok"`
	Lines   []string `json:"lines"`
}

// Criterion to jeden wiersz tabeli K1–K9.
type Criterion struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Target string `json:"target"`
	// Status: met, not_met, pending (nie zmierzono albo pomiar niepełny; nigdy nie znaczy „spełnione”).
	Status string `json:"status"`
	Source string `json:"source"`
}

// Inputs to dane, z których liczymy kryteria.
type Inputs struct {
	Runs                []*TestRun
	PythonTests         PythonRun
	Traces              []Trace
	UnknownRefs         []Ref
	RequirementProblems []string
	W3C                 []W3CRow
	W3CFailing          []NamedCase
	W3CRatchet          *Ratchet
	CorpusDocuments     int
	FuzzHours           time.Duration
	FuzzFailed          bool
	Bench               Bench
	Memory              StreamMemory
	StreamMB            int
	// StreamRun: osobny przebieg samego TestStreamingMemory przy fladze -stream-mb; jest też ostatni w Runs.
	StreamRun    *TestRun
	Measurements *Measurements
	FetchVerify  FetchVerify
	Candidate    string
}

const (
	pkgRoot   = "github.com/polestorm/goxmlgo"
	pkgReader = pkgRoot + "/xmlreader"
	pkgWriter = pkgRoot + "/xmlwriter"
	pkgTree   = pkgRoot + "/xmltree"
)

// Progi z ADR 0001 (tabela kryteriów gotowości).
const (
	k1MinPassed      = 1725
	k1InScope        = 1727
	k5Hours          = 24
	k7ReaderRatio    = 0.95
	k7PassOfReader   = 0.55
	k7PassOfStdlib   = 1.0
	k7MemoryFileMB   = 1000
	k7MemoryLimitMB  = 64
	notMeasured      = "nie zmierzono"
	notMeasuredStage = "nie zmierzono (etap 6)"
)

var (
	roundTripRe = regexp.MustCompile(`round trip on the W3C XML Conformance suite: (\d+) of (\d+) documents`)
	subtreeRe   = regexp.MustCompile(`(\d+) elements read as a subtree`)
)

func (in Inputs) result(key string) string {
	for _, r := range in.Runs {
		if a := r.Result(key); a != "" {
			return a
		}
	}
	return ""
}

func (in Inputs) output(key string) []string {
	for _, r := range in.Runs {
		if o := r.Output(key); len(o) > 0 {
			return o
		}
	}
	return nil
}

// byPrefix liczy przyjęte wymagania o danym prefiksie i te z nich, których testy przeszły; failed mówi, czy
// któreś nie przeszło albo nie ma testu.
func (in Inputs) byPrefix(prefix string) (accepted, passed int, failed bool) {
	for _, t := range in.Traces {
		if !t.Accepted || !strings.HasPrefix(t.ID, prefix+"-") {
			continue
		}
		accepted++
		switch t.Status {
		case "pass":
			passed++
		case "fail", "missing":
			failed = true
		}
	}
	return
}

// worst zwraca najgorszy stan z podanych wyników testów: fail → not_met, wszystkie pass → met, inaczej pending.
func worst(results ...string) string {
	status := "met"
	for _, r := range results {
		switch r {
		case "fail":
			return "not_met"
		case "pass":
		default:
			status = "pending"
		}
	}
	return status
}

// buildCriteria liczy tabelę K1–K9 (EVD-003). Wartości z przebiegów testów bierze z ich wyjścia; K5 z dziennika
// fuzzingu, K7 z plików wyniku w docs/results (tools/bench i TestStreamingMemory na 1 GB), pomiary u konsumentów
// z docs/measurements.json. Liczba, której nie da się uzyskać, daje stan pending albo not_met, nigdy met. Czasy
// z przebiegów nie wchodzą do wartości; jedynym pomiarem z przebiegu jest pamięć z osobnego przebiegu przy fladze -stream-mb (EVD-004).
func buildCriteria(in Inputs) []Criterion {
	m := in.Measurements
	if m == nil {
		m = &Measurements{}
	}
	var out []Criterion

	// K1
	k1 := Criterion{ID: "K1", Name: "zgodność czytnika (W3C XML Conformance)",
		Target: fmt.Sprintf("≥ %d z %d przypadków w zakresie; każde wyłączenie i każda porażka opisane w docs/differences.md; liczba nie spada", k1MinPassed, k1InScope),
		Source: "TestXMLConformance (xmlreader), testdata/w3c/xmlconf.ratchet, docs/differences.md"}
	var all *W3CRow
	var groups []string
	for i := range in.W3C {
		if in.W3C[i].Group == "all" {
			all = &in.W3C[i]
		} else {
			groups = append(groups, fmt.Sprintf("%s %d/%d", in.W3C[i].Group, in.W3C[i].Passed, in.W3C[i].InScope))
		}
	}
	switch res := in.result(pkgReader + ".TestXMLConformance"); {
	case res == "skip":
		k1.Value, k1.Status = "brak pobranego zestawu W3C (test pominięty)", "pending"
	case all == nil:
		k1.Value, k1.Status = "brak wyniku TestXMLConformance w przebiegu: "+orNone(res), "pending"
		if res == "fail" {
			k1.Status = "not_met"
		}
	default:
		failing := all.InScope - all.Passed
		named := 0
		for _, c := range in.W3CFailing {
			if c.Named {
				named++
			}
		}
		k1.Value = fmt.Sprintf("%d / %d", all.Passed, all.InScope)
		if len(groups) > 0 {
			k1.Value += " (" + strings.Join(groups, ", ") + ")"
		}
		k1.Value += fmt.Sprintf("; niezaliczone %d, w zapadce %d, nazwane w differences.md %d", failing, len(in.W3CFailing), named)
		if res == "pass" && all.Passed >= k1MinPassed && len(in.W3CFailing) == failing && named == failing {
			k1.Status = "met"
		} else {
			k1.Status = "not_met"
		}
	}
	out = append(out, k1)

	// K2
	k2 := Criterion{ID: "K2", Name: "zapis powrotny", Target: "każdy dokument przyjęty przez czytnik (zestaw W3C i korpus): odczyt → zapis → odczyt daje ten sam strumień tokenów; zapis idempotentny co do bajta",
		Source: "TestRoundTrip (xmlwriter), testdata/w3c/roundtrip.ratchet, testdata/corpus"}
	rtRes := in.result(pkgWriter + ".TestRoundTrip")
	rtOK, rtIn, rtFound := findCount(in.output(pkgWriter+".TestRoundTrip"), roundTripRe)
	switch {
	case rtRes == "fail":
		k2.Value, k2.Status = "TestRoundTrip nie przeszedł", "not_met"
		if rtFound {
			k2.Value += fmt.Sprintf("; W3C %d / %d", rtOK, rtIn)
		}
	case rtRes == "skip":
		k2.Value, k2.Status = "brak pobranego zestawu W3C (test pominięty po części korpusu)", "pending"
	case rtRes != "pass" || !rtFound:
		k2.Value, k2.Status = "brak wyniku TestRoundTrip w przebiegu: "+orNone(rtRes), "pending"
	default:
		k2.Value = fmt.Sprintf("W3C %d / %d dokumentów przyjmowanych przez czytnik; korpus %d / %d", rtOK, rtIn, in.CorpusDocuments, in.CorpusDocuments)
		k2.Status = "not_met"
		if rtOK == rtIn && rtIn > 0 && in.CorpusDocuments > 0 {
			k2.Status = "met"
		}
	}
	out = append(out, k2)

	// K3
	k3 := Criterion{ID: "K3", Name: "drzewo", Target: "Parse → WriteTo daje te same bajty co czytnik → pisarz; poddrzewo w środku strumienia nie zmienia wyniku; każda operacja zmiany ma wymaganie i test; drzewa goxadesgo i goxsdgo dają się zastąpić",
		Source: "TestTreeRoundTrip, TestSubtreeInStream (xmltree), testdata/w3c/tree.ratchet, wymagania TR w macierzy śledzenia, docs/measurements.json (k3_consumers)"}
	trRes, subRes := in.result(pkgTree+".TestTreeRoundTrip"), in.result(pkgTree+".TestSubtreeInStream")
	trOK, trIn, trFound := findCount(in.output(pkgTree+".TestTreeRoundTrip"), roundTripRe)
	elements, elFound := findNumber(in.output(pkgTree+".TestSubtreeInStream"), subtreeRe)
	trAccepted, trPassed, trFailed := in.byPrefix("TR")
	k3.Value = fmt.Sprintf("TestTreeRoundTrip %s", orNone(trRes))
	if trFound {
		k3.Value += fmt.Sprintf(": W3C %d / %d, korpus %d dokumentów", trOK, trIn, in.CorpusDocuments)
	}
	k3.Value += fmt.Sprintf("; TestSubtreeInStream %s", orNone(subRes))
	if elFound {
		k3.Value += fmt.Sprintf(": %d elementów jako poddrzewo", elements)
	}
	k3.Value += fmt.Sprintf("; wymagania TR z zaliczonym testem: %d z %d; zastąpienie drzew konsumentów: %s", trPassed, trAccepted, external(m.K3Consumers, notMeasuredStage))
	k3.Status = worst(trRes, subRes)
	switch {
	case k3.Status == "not_met" || trFailed || m.K3Consumers.Status == "not_met" || (trFound && trOK != trIn):
		k3.Status = "not_met"
	case k3.Status == "met" && trFound && elFound && trAccepted > 0 && trPassed == trAccepted && m.K3Consumers.Status == "met":
	default:
		k3.Status = "pending"
	}
	out = append(out, k3)

	// K4
	k4 := Criterion{ID: "K4", Name: "wymagania i API", Target: "każde przyjęte wymaganie ma test, testy przechodzą; każde REQ: wskazuje istniejące wymaganie; publiczne API równe wzorcom api/*.txt",
		Source: "requirements/*.md, odwołania REQ: w testach i korpusie, TestRequirementsCovered, TestAPISnapshot (pakiet główny), macierz śledzenia w tym pakiecie"}
	counts := map[string]int{}
	accepted := 0
	for _, t := range in.Traces {
		counts[t.Status]++
		if t.Accepted {
			accepted++
		}
	}
	covered, snapshot := in.result(pkgRoot+".TestRequirementsCovered"), in.result(pkgRoot+".TestAPISnapshot")
	k4.Value = fmt.Sprintf("%d wymagań przyjętych: przeszło %d, na żądanie %d, pominięte %d, bez testu %d, nieprzypisane %d, nie przeszło %d; w szkicu %d; odwołania do nieznanych wymagań: %d; błędy w plikach wymagań: %d; TestRequirementsCovered %s; TestAPISnapshot %s",
		accepted, counts["pass"], counts["manual"], counts["skip"], counts["missing"], counts["unresolved"], counts["fail"], counts["draft"],
		len(in.UnknownRefs), len(in.RequirementProblems), orNone(covered), orNone(snapshot))
	switch {
	case counts["fail"] > 0 || counts["missing"] > 0 || len(in.UnknownRefs) > 0 || len(in.RequirementProblems) > 0 || covered == "fail" || snapshot == "fail":
		k4.Status = "not_met"
	case counts["skip"] > 0 || counts["unresolved"] > 0 || covered != "pass" || snapshot != "pass" || accepted == 0:
		k4.Status = "pending"
	default:
		k4.Status = "met"
	}
	out = append(out, k4)

	// K5
	k5 := Criterion{ID: "K5", Name: "odporność (fuzzing)", Target: fmt.Sprintf("fuzzing ≥ %d h łącznie na kandydacie bez paniki i bez naruszenia niezmienników", k5Hours),
		Source: fmt.Sprintf("docs/fuzzing.md, tabela „Przebiegi” (kandydat %s i commity bez zmian kodu biblioteki; bez przebiegów z drzewa ze zmianami)", orDash(in.Candidate))}
	k5.Value = fmt.Sprintf("%s z %d h", fmtHours(in.FuzzHours), k5Hours)
	switch {
	case in.FuzzFailed:
		k5.Value += "; dziennik ma przebieg zakończony błędem na kodzie kandydata"
		k5.Status = "not_met"
	case in.FuzzHours.Hours() >= k5Hours:
		k5.Status = "met"
	default:
		k5.Status = "pending"
	}
	out = append(out, k5)

	// K6
	k6 := Criterion{ID: "K6", Name: "bezpieczeństwo", Target: "bez sieci i plików; encje zewnętrzne nigdy nie czytane; limity domyślnie włączone; wrogie wejścia odrzucone w < 1 s i < 64 MB; czas liniowy (podwojenie rozmiaru najwyżej 2,5×)",
		Source: "TestSafeByDefault, TestHostileInputs, TestPathological, TestNoIOImports (pakiet główny); granice czasu i pamięci sprawdza sam test, czasy są w surowym wyniku przebiegu"}
	var k6Results, k6Parts []string
	for _, name := range []string{"TestSafeByDefault", "TestHostileInputs", "TestPathological", "TestNoIOImports"} {
		res := in.result(pkgRoot + "." + name)
		k6Results = append(k6Results, res)
		k6Parts = append(k6Parts, name+" "+orNone(res))
	}
	k6.Value = strings.Join(k6Parts, "; ")
	k6.Status = worst(k6Results...)
	out = append(out, k6)

	// K7
	k7 := Criterion{ID: "K7", Name: "wydajność i pamięć",
		Target: fmt.Sprintf("czytnik ≥ %.2f× czytnika goxsdgo v0.3.0; przelot czytnik → pisarz ≥ %.2f× samego czytnika i nie wolniej niż przelot encoding/xml; plik 1 GB przez czytnik → pisarz w < %d MB; liczba alokacji nie rośnie z rozmiarem dokumentu",
			k7ReaderRatio, k7PassOfReader, k7MemoryLimitMB),
		Source: benchFile + " (tools/bench), " + memoryFile + " (GOXMLGO_STREAM_MB=1024), TestStreamingMemory (pakiet główny), wymaganie DET-007 w macierzy śledzenia"}
	streamKey, streamName := pkgRoot+".TestStreamingMemory", "TestStreamingMemory"
	if in.result(streamKey) == "" && in.result(pkgReader+".TestStreamingMemory") != "" {
		// przed etapem 5 test był tylko w xmlreader (sam czytnik)
		streamKey, streamName = pkgReader+".TestStreamingMemory", "TestStreamingMemory (xmlreader, sam czytnik)"
	}
	stream := in.result(streamKey)
	allocs := ""
	for _, t := range in.Traces {
		if t.ID == "DET-007" {
			allocs = t.Status
		}
	}
	benchBad, benchComplete := false, in.Bench.Found
	var k7Parts []string
	if !in.Bench.Found {
		k7Parts = append(k7Parts, "szybkość: "+notMeasured+" (brak wierszy „# ratio” w "+benchFile+")")
	} else {
		for _, d := range in.Bench.Documents {
			k7Parts = append(k7Parts, fmt.Sprintf("%s: czytnik %s, przelot %s czytnika i %s encoding/xml", d.Document, ratio(d.ReaderRatio), ratio(d.PassOfReader), ratio(d.PassOfStdlib)))
			if d.ReaderRatio == 0 || d.PassOfReader == 0 || d.PassOfStdlib == 0 {
				benchComplete = false
			}
			if (d.ReaderRatio > 0 && d.ReaderRatio < k7ReaderRatio) || (d.PassOfReader > 0 && d.PassOfReader < k7PassOfReader) || (d.PassOfStdlib > 0 && d.PassOfStdlib < k7PassOfStdlib) {
				benchBad = true
			}
		}
		stamp := fmt.Sprintf("pomiar %s, commit %s", orDash(in.Bench.Date), orDash(in.Bench.Commit))
		if !in.Bench.SameLibrary {
			stamp += fmt.Sprintf(": kod biblioteki inny niż u kandydata %s, pomiar do powtórzenia", orDash(in.Candidate))
			benchComplete = false
		}
		k7Parts = append(k7Parts, stamp)
	}
	memoryOK := false
	mem, memStamp := in.Memory, ""
	if in.StreamRun != nil {
		// pomiar z osobnego przebiegu (flaga -stream-mb) ma pierwszeństwo przed zapisanym; test sam sprawdza granicę
		key := pkgRoot + ".TestStreamingMemory"
		size, heap, process, ok := parseStreamLine(in.StreamRun.Output(key))
		mem = StreamMemory{Found: ok, SizeMB: size, HeapMB: heap, ProcessMB: process, SameLibrary: true}
		memStamp = fmt.Sprintf("osobny przebieg testu w tym pakiecie, GOXMLGO_STREAM_MB=%d: %s", in.StreamMB, orNone(in.StreamRun.Result(key)))
		if res := in.StreamRun.Result(key); res != "pass" {
			mem.SameLibrary = false // bez zaliczonego testu pomiar nie spełnia kryterium
			if res == "fail" {
				benchBad = true
			}
		}
	} else if mem.Found {
		memStamp = fmt.Sprintf("pomiar %s, commit %s", orDash(mem.Date), orDash(mem.Commit))
		switch {
		case mem.Uncommitted:
			memStamp += ": z drzewa ze zmianami, pomiar do powtórzenia na commicie"
		case !mem.SameLibrary:
			memStamp += fmt.Sprintf(": kod biblioteki inny niż u kandydata %s, pomiar do powtórzenia", orDash(in.Candidate))
		}
	}
	if mem.Found {
		k7Parts = append(k7Parts, fmt.Sprintf("plik %.0f MB: pamięć procesu %.1f MB, szczyt sterty %.2f MB (%s)", mem.SizeMB, mem.ProcessMB, mem.HeapMB, memStamp))
		if mem.ProcessMB >= k7MemoryLimitMB {
			benchBad = true
		}
		memoryOK = mem.SizeMB >= k7MemoryFileMB && mem.ProcessMB > 0 && mem.ProcessMB < k7MemoryLimitMB && !mem.Uncommitted && mem.SameLibrary
	} else {
		k7Parts = append(k7Parts, "plik 1 GB: "+notMeasured)
	}
	k7Parts = append(k7Parts, streamName+" "+orNone(stream), "alokacje (DET-007): "+statusWord(allocs))
	k7.Value = strings.Join(k7Parts, "; ")
	switch {
	case benchBad || stream == "fail" || allocs == "fail":
		k7.Status = "not_met"
	case benchComplete && memoryOK && stream == "pass" && allocs == "pass":
		k7.Status = "met"
	default:
		k7.Status = "pending"
	}
	out = append(out, k7)

	// K8
	k8 := Criterion{ID: "K8", Name: "powtarzalność i konsumenci", Target: "te same bajty w 10 przebiegach i na każdym systemie z CI; goxsdgo i goxadesgo po przełączeniu przechodzą swoje testy bez zmiany swoich liczb",
		Source: "wymagania DET w macierzy śledzenia (testy powtarzalności w tym przebiegu), docs/measurements.json (k8: macierz CI i konsumenci)"}
	detAccepted, detPassed, detFailed := in.byPrefix("DET")
	k8.Value = fmt.Sprintf("wymagania DET z zaliczonym testem: %d z %d przyjętych (ten system); macierz CI i konsumenci: %s", detPassed, detAccepted, external(m.K8, notMeasuredStage))
	switch {
	case detFailed || m.K8.Status == "not_met":
		k8.Status = "not_met"
	case m.K8.Status == "met" && detAccepted > 0 && detPassed == detAccepted:
		k8.Status = "met"
	default:
		k8.Status = "pending"
	}
	out = append(out, k8)

	// K9
	failed := 0
	for _, r := range in.Runs {
		failed += r.Failed + len(r.BuildFailed)
	}
	k9 := Criterion{ID: "K9", Name: "pakiet dowodów", Target: "wymagania → testy → wynik ostatniego przebiegu; tabela K1–K9 ze źródłem każdej liczby; dane testowe zgodne z manifestem",
		Source: "ten pakiet (tools/evidence, requirements/evidence.md)"}
	k9.Value = fmt.Sprintf("przebiegi go test: %d, nieudanych testów i pakietów: %d; testy Pythona: %s; tools/fetch -verify: %s; macierz: %d wymagań",
		len(in.Runs), failed, okWord(in.PythonTests.OK), okWord(in.FetchVerify.OK), len(in.Traces))
	switch {
	case failed > 0 || len(in.Runs) == 0 || !in.PythonTests.OK || counts["fail"] > 0 || counts["missing"] > 0:
		k9.Status = "not_met"
	case !in.FetchVerify.OK:
		k9.Status = "pending"
	default:
		k9.Status = "met"
	}
	out = append(out, k9)
	return out
}

// external opisuje wynik sprawdzenia spoza repozytorium albo mówi, że go nie ma.
func external(r ExternalResult, missing string) string {
	if r.Status == "" && r.Result == "" {
		return missing
	}
	s := orDash(r.Result)
	if r.Date != "" || r.Commit != "" {
		s += fmt.Sprintf(" (%s, commit %s)", orDash(r.Date), orDash(r.Commit))
	}
	return s
}

func ratio(v float64) string {
	if v == 0 {
		return "brak"
	}
	return fmt.Sprintf("%.2f×", v)
}

func fmtHours(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%.1f h", d.Hours())
}

func orNone(s string) string {
	if s == "" {
		return "brak w przebiegu"
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func okWord(ok bool) string {
	if ok {
		return "ok"
	}
	return "błąd"
}

var statusPL = map[string]string{"met": "spełnione", "not_met": "**niespełnione**", "pending": "oczekuje (nie zmierzono albo pomiar niepełny)",
	"pass": "przeszedł", "fail": "**nie przeszedł**", "skip": "pominięty (brak danych)", "missing": "**brak testu**",
	"unresolved": "**nieprzypisany**", "manual": "na żądanie (benchmark)", "draft": "szkic (test niewymagany)"}

func statusWord(status string) string {
	if status == "" {
		return "brak w macierzy"
	}
	if s, ok := statusPL[status]; ok {
		return strings.Trim(s, "*")
	}
	return status
}

// renderReport składa report.md (po polsku).
func renderReport(e *Evidence) string {
	var b strings.Builder
	p := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	dirty := ""
	if e.Dirty {
		dirty = " (ze zmianami w drzewie roboczym — pakiet nie dotyczy czystego commitu)"
	}
	p("# Pakiet dowodów goxmlgo\n\n")
	p("Commit `%s`%s; %s, %s; %s. Wygenerowano %s przez `go -C tools/evidence run . -root ../..` (requirements/evidence.md).\n\n",
		e.Commit, dirty, e.GoVersion, e.Platform, e.Python, e.Generated)
	p("Kandydat do wydania: `%s` (%s).\n\n", e.Candidate, e.CandidateSource)

	p("## Kryteria gotowości (ADR 0001)\n\n| # | Kryterium | Wynik | Cel | Stan | Źródło |\n|---|---|---|---|---|---|\n")
	for _, c := range e.Criteria {
		p("| %s | %s | %s | %s | %s | %s |\n", c.ID, c.Name, cell(c.Value), cell(c.Target), statusPL[c.Status], cell(c.Source))
	}

	p("\n## Przebiegi testów\n\n| Przebieg | Polecenie | Przeszło | Nie przeszło | Pominięte |\n|---|---|---|---|---|\n")
	for _, r := range e.Runs {
		p("| %s | `%s` | %d | %d | %d |\n", r.Name, r.Command, r.Passed, r.Failed+len(r.BuildFailed), r.Skipped)
	}
	p("| Python | `%s` | %s | | |\n\n", e.PythonTests.Command, cell(e.PythonTests.Summary))
	for _, r := range e.Runs {
		for _, t := range r.FailedTests {
			p("- nie przeszedł: `%s`\n", t)
		}
		for _, t := range r.BuildFailed {
			p("- pakiet bez wyniku (błąd kompilacji?): `%s`\n", t)
		}
		for _, t := range r.SkippedTests {
			p("- pominięty: `%s`\n", t)
		}
	}

	p("\n## Macierz śledzenia (K4)\n")
	file := ""
	for _, t := range e.Traces {
		if t.File != file {
			file = t.File
			p("\n### %s\n\n| Wymaganie | Treść | Dowód | Wynik |\n|---|---|---|---|\n", file)
		}
		var ev []string
		for _, x := range t.Evidence {
			ev = append(ev, "`"+x+"`")
		}
		p("| %s | %s | %s | %s |\n", t.ID, cell(t.Text), strings.Join(ev, "<br>"), statusPL[t.Status])
	}
	if len(e.UnknownRefs) > 0 {
		p("\nOdwołania do nieistniejących wymagań:\n\n")
		for _, r := range e.UnknownRefs {
			p("- `%s` w `%s` (%s)\n", r.ID, r.File, orDash(r.Test))
		}
	}
	if len(e.RequirementProblems) > 0 {
		p("\nBłędy w plikach wymagań:\n\n")
		for _, s := range e.RequirementProblems {
			p("- %s\n", s)
		}
	}

	p("\n## Zestaw W3C XML Conformance (K1)\n\n| Grupa | Zaliczone / w zakresie |\n|---|---|\n")
	for _, r := range e.W3C {
		p("| %s | %d / %d |\n", r.Group, r.Passed, r.InScope)
	}
	if len(e.W3CExcluded) > 0 {
		p("\n| Wyłączone | Powód |\n|---|---|\n")
		for _, x := range e.W3CExcluded {
			p("| %d | %s |\n", x.Count, cell(x.Reason))
		}
	}
	if len(e.W3CFailing) > 0 {
		p("\nNiezaliczone przypadki w zakresie (z `testdata/w3c/xmlconf.ratchet`) i to, czy `docs/differences.md` je nazywa:\n\n| Przypadek | Nazwany w differences.md |\n|---|---|\n")
		for _, c := range e.W3CFailing {
			named := "tak"
			if !c.Named {
				named = "**nie**"
			}
			p("| `%s` | %s |\n", c.ID, named)
		}
	}

	p("\n## Zapadki\n\n| Plik | W zakresie | Przechodzi | Tryb | Na liście |\n|---|---|---|---|---|\n")
	for _, r := range e.Ratchets {
		p("| `%s` | %d | %d | %s | %d |\n", r.File, r.InScope, r.Passing, r.Mode, r.Listed)
	}

	p("\n## Fuzzing (K5)\n\nLiczone do K5: %s (kandydat `%s` i commity bez zmian kodu biblioteki; reguły w docs/fuzzing.md).\n\n", fmtHours(time.Duration(e.FuzzHours*float64(time.Hour))), e.Candidate)
	p("| Data | Commit | Host i sprzęt | Fuzzer | Czas zegarowy | Wykonania | Wynik | Liczony |\n|---|---|---|---|---|---|---|---|\n")
	for _, f := range e.Fuzz {
		counted := "nie"
		if f.Counted {
			counted = "tak"
		}
		commit := "`" + f.Commit + "`"
		if f.Dirty {
			commit += " (zmiany)"
		}
		p("| %s | %s | %s | %s | %s | %s | %s | %s |\n", f.Date, commit, cell(f.Host), cell(f.Fuzzer), cell(f.Duration), cell(f.Execs), cell(f.Result), counted)
	}

	p("\n## Wydajność (K7)\n\n")
	if !e.Bench.Found {
		p("Brak wierszy stosunków w `%s`.\n", e.Bench.File)
	} else {
		same := "ten sam kod biblioteki co u kandydata"
		if !e.Bench.SameLibrary {
			same = "**kod biblioteki inny niż u kandydata**"
		}
		p("`%s`: pomiar %s, commit `%s` (%s). Metoda: docs/methodology.md.\n\n", e.Bench.File, orDash(e.Bench.Date), e.Bench.Commit, same)
		p("| Dokument | Czytnik wobec goxsdgo v0.3.0 (≥ %.2f) | Przelot wobec czytnika (≥ %.2f) | Przelot wobec encoding/xml (≥ %.2f) |\n|---|---|---|---|\n", k7ReaderRatio, k7PassOfReader, k7PassOfStdlib)
		for _, d := range e.Bench.Documents {
			p("| %s | %s | %s | %s |\n", cell(d.Document), ratio(d.ReaderRatio), ratio(d.PassOfReader), ratio(d.PassOfStdlib))
		}
	}

	if e.Memory.Found {
		state := "ten sam kod biblioteki co u kandydata"
		switch {
		case e.Memory.Uncommitted:
			state = "**z drzewa ze zmianami**"
		case !e.Memory.SameLibrary:
			state = "**kod biblioteki inny niż u kandydata**"
		}
		p("\n`%s`: pomiar %s, commit `%s` (%s): plik %.0f MB, pamięć procesu %.1f MB, szczyt sterty %.2f MB (cel: < %d MB).\n",
			e.Memory.File, orDash(e.Memory.Date), e.Memory.Commit, state, e.Memory.SizeMB, e.Memory.ProcessMB, e.Memory.HeapMB, k7MemoryLimitMB)
	} else {
		p("\nBrak pomiaru w `%s`.\n", e.Memory.File)
	}
	if e.StreamMB > 0 {
		p("\nTestStreamingMemory uruchomiono też osobno z GOXMLGO_STREAM_MB=%d; wynik jest w wierszu K7 i w `go-test-stream.json`.\n", e.StreamMB)
	}

	p("\n## Pomiary spoza przebiegu (docs/measurements.json)\n\n")
	if e.Measurements == nil || !e.Measurements.Found {
		p("Pliku nie ma: zastąpienie drzew konsumentów (K3) oraz macierz CI i konsumenci (K8) są wykazane jako niezmierzone.\n")
	} else {
		m := e.Measurements
		p("| Pomiar | Stan | Wynik | Data | Commit |\n|---|---|---|---|---|\n")
		p("| K3: zastąpienie drzew konsumentów | %s | %s | %s | %s |\n", orDash(m.K3Consumers.Status), cell(orDash(m.K3Consumers.Result)), orDash(m.K3Consumers.Date), orDash(m.K3Consumers.Commit))
		p("| K8: macierz CI i konsumenci | %s | %s | %s | %s |\n", orDash(m.K8.Status), cell(orDash(m.K8.Result)), orDash(m.K8.Date), orDash(m.K8.Commit))
	}

	p("\n## Dane testowe\n\nManifest `testdata/sources.json`, SHA-256 `%s`.\n\n| Zestaw | Rodzaj | Katalog | Pliki | SHA-256 archiwum | SHA-256 drzewa | Tylko lokalnie |\n|---|---|---|---|---|---|---|\n", e.SourcesSHA)
	for _, s := range e.Sources {
		p("| %s | %s | `%s` | %d | %s | %s | %d |\n", s.Name, s.Kind, s.Dest, s.Files, orDash(s.SHA256), orDash(s.TreeSHA256), s.LocalOnly)
	}
	p("\n`%s`: %s\n\n```\n%s\n```\n", e.FetchVerify.Command, okWord(e.FetchVerify.OK), strings.Join(e.FetchVerify.Lines, "\n"))

	p("\n## Zależności biblioteki\n\nModuły z `go list -m all` (bez modułu głównego), sumy z `go.sum`. Biblioteka ma nie mieć żadnych (ADR 0001, pkt 1).\n\n| Moduł | Wersja | go.sum |\n|---|---|---|\n")
	for _, d := range e.Deps {
		p("| `%s` | %s | `%s` |\n", d.Path, d.Version, d.Sum)
	}
	if len(e.Deps) == 0 {
		p("| — | | |\n")
	}
	p("\n## Decyzje (docs/adr)\n\n| Plik | Tytuł | Status |\n|---|---|---|\n")
	for _, a := range e.ADRs {
		p("| `%s` | %s | %s |\n", a.File, cell(a.Title), cell(a.Status))
	}

	p("\n## Różnice (docs/differences.md)\n\n| Sekcja | Wiersze |\n|---|---|\n")
	for _, d := range e.Differences {
		p("| %s | %d |\n", cell(d.Title), d.Rows)
	}

	p("\n## Pliki pakietu\n\n")
	for _, f := range e.Files {
		p("- `%s`\n", f)
	}
	return b.String()
}

func cell(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "|", "\\|")
}
