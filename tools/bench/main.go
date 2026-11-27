// Command bench mierzy czytnik goxmlgo wobec czytnika goxsdgo v0.3.0 (kryterium K7: nie wolniejszy niż 0,95×)
// na tych samych dokumentach, w tym samym przebiegu, na przemian, a od etapu 3 także przelot czytnik → pisarz
// goxmlgo i, dla porównania, przelot Decoder → Encoder z encoding/xml. Metoda: docs/methodology.md.
//
// Użycie (z katalogu głównego repozytorium):
//
//	go -C tools/bench run . -root ../..                                   # wynik na wyjście
//	go -C tools/bench run . -root ../.. -out ../../docs/results/reader-vs-goxsdgo.txt
//
// Narzędzie jest osobnym modułem: goxsdgo v0.3.0 jest zależnością pomiaru, nie biblioteki. Dokumenty powstają
// w pamięci z deterministycznych generatorów (internal/testkit), więc pomiar nie potrzebuje żadnych plików.
package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/polestorm/goxmlgo/internal/testkit"
	newreader "github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmlwriter"
	oldreader "github.com/polestorm/goxsdgo/xmlreader"
)

// threshold to granica K7: szybkość nowego czytnika podzielona przez szybkość czytnika goxsdgo v0.3.0.
const threshold = 0.95

type candidate struct {
	name string
	read func(data []byte) (tokens int, err error)
}

var candidates = []candidate{
	{"goxsdgo v0.3.0", func(data []byte) (int, error) {
		r := oldreader.New(bytes.NewReader(data), oldreader.Options{})
		for n := 0; ; n++ {
			if _, err := r.Next(); err != nil {
				if err == io.EOF {
					return n, nil
				}
				return n, err
			}
		}
	}},
	{"goxmlgo", func(data []byte) (int, error) {
		r := newreader.New(bytes.NewReader(data), newreader.Options{})
		for n := 0; ; n++ {
			if _, err := r.Next(); err != nil {
				if err == io.EOF {
					return n, nil
				}
				return n, err
			}
		}
	}},
}

// passThrough to kandydaci przelotu: każdy czyta dokument i zapisuje go od razu do io.Discard. Liczba tokenów
// nie jest porównywana między nimi (encoding/xml dzieli tekst inaczej).
var passThrough = []candidate{
	{"goxmlgo reader+writer", func(data []byte) (int, error) {
		r := newreader.New(bytes.NewReader(data), newreader.Options{})
		w := xmlwriter.New(io.Discard, xmlwriter.Options{})
		for n := 0; ; n++ {
			t, err := r.Next()
			if err == io.EOF {
				return n, w.Close()
			}
			if err != nil {
				return n, err
			}
			if err := w.WriteToken(t); err != nil {
				return n, err
			}
		}
	}},
	{"encoding/xml Decoder+Encoder", func(data []byte) (int, error) {
		d := xml.NewDecoder(bytes.NewReader(data))
		e := xml.NewEncoder(io.Discard)
		for n := 0; ; n++ {
			t, err := d.RawToken()
			if err == io.EOF {
				return n, e.Flush()
			}
			if err != nil {
				return n, err
			}
			if err := e.EncodeToken(t); err != nil {
				return n, err
			}
		}
	}},
}

type document struct {
	name string
	gen  func() io.Reader
}

var documents = []document{
	{"GenJPK(20000)", func() io.Reader { return testkit.GenJPK(20000) }},
	{"GenWordML(30000)", func() io.Reader { return testkit.GenWordML(30000) }},
}

func main() {
	root := flag.String("root", ".", "repository root (for the commit in the header)")
	rounds := flag.Int("rounds", 15, "rounds; in each round every candidate reads every document once")
	out := flag.String("out", "", "also write the result to this file")
	flag.Parse()

	var b strings.Builder
	fmt.Fprintf(&b, "# go -C tools/bench run . -root ../.. -rounds %d\n", *rounds)
	fmt.Fprintf(&b, "# date: %s\n", time.Now().UTC().Format("2006-01-02T15:04Z"))
	fmt.Fprintf(&b, "# goxmlgo: %s\n", commit(*root))
	fmt.Fprintf(&b, "# goxsdgo: v0.3.0 (module dependency of tools/bench, go.sum)\n")
	fmt.Fprintf(&b, "# %s %s/%s, %d CPU, one goroutine\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	fmt.Fprintf(&b, "# %s\n", machine())
	fmt.Fprintf(&b, "# per document: median of %d rounds, candidates alternate within a round; ratio = goxmlgo MB/s / goxsdgo MB/s; K7 needs >= %.2f\n", *rounds, threshold)
	fmt.Fprintf(&b, "# pass-through candidates read the document and write it to io.Discard; encoding/xml uses RawToken and EncodeToken\n")
	fmt.Fprintf(&b, "document\tbytes\ttokens\tcandidate\tmedian_ms\tMB/s\tmin_ms\tmax_ms\tallocs\tbytes_allocated\n")

	failed := false
	var ratios []string
	for _, doc := range documents {
		data, err := io.ReadAll(doc.gen())
		if err != nil {
			fail(err)
		}
		candidates := append(append([]candidate(nil), candidates...), passThrough...)
		times := make([][]time.Duration, len(candidates))
		tokens := make([]int, len(candidates))
		for round := -1; round < *rounds; round++ { // runda -1 to rozgrzewka, nie jest liczona
			for k := range candidates {
				i := (k + max(round, 0)) % len(candidates) // kolejność kandydatów zmienia się co rundę
				start := time.Now()
				n, err := candidates[i].read(data)
				d := time.Since(start)
				if err != nil {
					fail(fmt.Errorf("%s on %s: %v", candidates[i].name, doc.name, err))
				}
				tokens[i] = n
				if round >= 0 {
					times[i] = append(times[i], d)
				}
			}
		}
		if tokens[0] != tokens[1] {
			fail(fmt.Errorf("%s: %d tokens from %s, %d from %s", doc.name, tokens[0], candidates[0].name, tokens[1], candidates[1].name))
		}
		speed := make([]float64, len(candidates))
		for i, c := range candidates {
			sort.Slice(times[i], func(a, b int) bool { return times[i][a] < times[i][b] })
			med := times[i][len(times[i])/2]
			speed[i] = float64(len(data)) / 1e6 / med.Seconds()
			allocs, bytesAllocated := allocations(func() { c.read(data) })
			fmt.Fprintf(&b, "%s\t%d\t%d\t%s\t%.2f\t%.1f\t%.2f\t%.2f\t%d\t%d\n", doc.name, len(data), tokens[i], c.name,
				ms(med), speed[i], ms(times[i][0]), ms(times[i][len(times[i])-1]), allocs, bytesAllocated)
		}
		ratio := speed[1] / speed[0]
		verdict := "ok"
		if ratio < threshold {
			verdict, failed = "BELOW THE THRESHOLD", true
		}
		ratios = append(ratios, fmt.Sprintf("# ratio %s: %.3f (%s)", doc.name, ratio, verdict))
		ratios = append(ratios, fmt.Sprintf("# pass-through %s: goxmlgo %.1f MB/s, %.2f of its reader alone, %.2f of encoding/xml Decoder+Encoder",
			doc.name, speed[2], speed[2]/speed[1], speed[2]/speed[3]))
	}
	b.WriteString(strings.Join(ratios, "\n") + "\n")
	fmt.Print(b.String())
	if *out != "" {
		if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
			fail(err)
		}
	}
	if failed {
		os.Exit(1)
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// allocations zwraca liczbę alokacji i bajty zaalokowane przez jedno wykonanie f.
func allocations(f func()) (uint64, uint64) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.Mallocs - before.Mallocs, after.TotalAlloc - before.TotalAlloc
}

func commit(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "working tree without a commit"
	}
	c := strings.TrimSpace(string(out))
	if st, err := exec.Command("git", "-C", root, "status", "--porcelain").Output(); err != nil || len(st) > 0 {
		c += " (with uncommitted changes)"
	}
	return c
}

// machine opisuje procesor i system, o ile system to podaje.
func machine() string {
	var parts []string
	for _, c := range [][]string{{"sysctl", "-n", "machdep.cpu.brand_string"}, {"sw_vers", "-productVersion"}, {"uname", "-sr"}} {
		if out, err := exec.Command(c[0], c[1:]...).Output(); err == nil {
			parts = append(parts, strings.TrimSpace(string(out)))
		}
	}
	if len(parts) == 0 {
		return "machine: unknown"
	}
	return "machine: " + strings.Join(parts, ", ")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "bench:", err)
	os.Exit(1)
}
