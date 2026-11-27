// Command compare mierzy drzewo goxmlgo (xmltree) obok innych bibliotek drzewa XML w Go, uruchamianych jako czarna
// skrzynka: budowa drzewa z dokumentu, budowa i zapis, pamięć zajęta przez drzewo. Ile dokumentów zestawu W3C XML
// Conformance każdy kandydat przyjmuje i odrzuca (szybkość zależy od tego, co kandydat sprawdza), liczy test
// TestCandidatesConformance tego modułu. Metoda: docs/methodology.md, sekcja „Tree compared with other Go libraries”.
//
// Użycie (z katalogu głównego repozytorium):
//
//	go -C tools/compare run . -root ../..                                        # wynik na wyjście
//	go -C tools/compare run . -root ../.. -out ../../docs/results/tree-vs-others.txt
//
// Osobny moduł: biblioteki porównywane są zależnościami pomiaru, nie biblioteki. Kodu kandydatów nie czytamy
// (docs/provenance.md); używamy wyłącznie ich publicznego API według dokumentacji.
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
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/beevik/etree"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmltree"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

// candidate to biblioteka drzewa: build buduje drzewo z dokumentu i zwraca je (żeby zmierzyć pamięć), write
// zapisuje zbudowane drzewo do io.Discard.
type candidate struct {
	name  string
	build func(data []byte) (tree any, err error)
	write func(tree any) error
}

var candidates = []candidate{
	{"goxmlgo xmltree", func(data []byte) (any, error) {
		return xmltree.Parse(bytes.NewReader(data), xmltree.Options{MaxNodes: -1})
	}, func(tree any) error {
		w := xmlwriter.New(io.Discard, xmlwriter.Options{})
		if err := tree.(*xmltree.Document).WriteTo(w); err != nil {
			return err
		}
		return w.Close()
	}},
	{"beevik/etree", func(data []byte) (any, error) {
		doc := etree.NewDocument()
		_, err := doc.ReadFrom(bytes.NewReader(data))
		return doc, err
	}, func(tree any) error {
		_, err := tree.(*etree.Document).WriteTo(io.Discard)
		return err
	}},
	{"antchfx/xmlquery", func(data []byte) (any, error) {
		return xmlquery.Parse(bytes.NewReader(data))
	}, func(tree any) error {
		_, err := io.WriteString(io.Discard, tree.(*xmlquery.Node).OutputXML(true))
		return err
	}},
}

// readers to czytniki bez drzewa, jako punkt odniesienia dla kosztu samego czytania.
var readers = []struct {
	name string
	read func(data []byte) error
}{
	{"goxmlgo xmlreader (no tree)", func(data []byte) error {
		r := xmlreader.New(bytes.NewReader(data), xmlreader.Options{})
		for {
			if _, err := r.Next(); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
		}
	}},
	{"encoding/xml Decoder.Token (no tree)", func(data []byte) error {
		d := xml.NewDecoder(bytes.NewReader(data))
		for {
			if _, err := d.Token(); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
		}
	}},
}

var documents = []struct {
	name string
	gen  func() io.Reader
}{
	{"GenJPK(20000)", func() io.Reader { return testkit.GenJPK(20000) }},
	{"GenWordML(30000)", func() io.Reader { return testkit.GenWordML(30000) }},
}

func main() {
	root := flag.String("root", ".", "repository root (for the commit in the header)")
	rounds := flag.Int("rounds", 9, "rounds; in each round every candidate handles every document once")
	out := flag.String("out", "", "also write the result to this file")
	flag.Parse()

	var b strings.Builder
	fmt.Fprintf(&b, "# go -C tools/compare run . -root ../.. -rounds %d\n", *rounds)
	fmt.Fprintf(&b, "# date: %s\n", time.Now().UTC().Format("2006-01-02T15:04Z"))
	fmt.Fprintf(&b, "# goxmlgo: %s\n", commit(*root))
	fmt.Fprintf(&b, "# candidates: %s\n", versions())
	fmt.Fprintf(&b, "# %s %s/%s, %d CPU, one goroutine\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	fmt.Fprintf(&b, "# %s\n", machine())
	fmt.Fprintf(&b, "# per document: median of %d rounds, candidates alternate within a round; held = heap in use by the built tree after a collection\n", *rounds)
	fmt.Fprintf(&b, "document\tbytes\tcandidate\tbuild_ms\tbuild_MB/s\tbuild_allocs\tbuild_bytes\theld_bytes\theld/document\twrite_ms\twrite_MB/s\n")
	for _, doc := range documents {
		data, err := io.ReadAll(doc.gen())
		if err != nil {
			fail(err)
		}
		n := len(candidates)
		build, write := make([][]time.Duration, n), make([][]time.Duration, n)
		for round := -1; round < *rounds; round++ { // runda -1 to rozgrzewka
			for k := 0; k < n; k++ {
				i := (k + max(round, 0)) % n
				runtime.GC()
				start := time.Now()
				tree, err := candidates[i].build(data)
				d := time.Since(start)
				if err != nil {
					fail(fmt.Errorf("%s on %s: %v", candidates[i].name, doc.name, err))
				}
				start = time.Now()
				if err := candidates[i].write(tree); err != nil {
					fail(fmt.Errorf("%s writing %s: %v", candidates[i].name, doc.name, err))
				}
				dw := time.Since(start)
				if round >= 0 {
					build[i], write[i] = append(build[i], d), append(write[i], dw)
				}
			}
		}
		for i, c := range candidates {
			allocs, bytesAllocated, held := memory(func() any {
				tree, _ := c.build(data)
				return tree
			})
			bm, wm := median(build[i]), median(write[i])
			fmt.Fprintf(&b, "%s\t%d\t%s\t%.2f\t%.1f\t%d\t%d\t%d\t%.1f\t%.2f\t%.1f\n", doc.name, len(data), c.name,
				ms(bm), mbps(len(data), bm), allocs, bytesAllocated, held, float64(held)/float64(len(data)), ms(wm), mbps(len(data), wm))
		}
		for _, r := range readers {
			var times []time.Duration
			for round := -1; round < *rounds; round++ {
				start := time.Now()
				if err := r.read(data); err != nil {
					fail(fmt.Errorf("%s on %s: %v", r.name, doc.name, err))
				}
				if round >= 0 {
					times = append(times, time.Since(start))
				}
			}
			m := median(times)
			fmt.Fprintf(&b, "%s\t%d\t%s\t%.2f\t%.1f\t-\t-\t-\t-\t-\t-\n", doc.name, len(data), r.name, ms(m), mbps(len(data), m))
		}
	}
	fmt.Print(b.String())
	if *out != "" {
		if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
			fail(err)
		}
	}
}

// memory zwraca alokacje i bajty jednej budowy oraz pamięć sterty zajętą przez zbudowane drzewo.
func memory(build func() any) (allocs, bytesAllocated, held uint64) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1)) // bez odśmiecania w trakcie: liczymy przydziały i stan po
	var before, after, kept runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	tree := build()
	runtime.ReadMemStats(&after)
	runtime.GC()
	runtime.ReadMemStats(&kept)
	runtime.KeepAlive(tree)
	if kept.HeapAlloc > before.HeapAlloc {
		held = kept.HeapAlloc - before.HeapAlloc
	}
	return after.Mallocs - before.Mallocs, after.TotalAlloc - before.TotalAlloc, held
}

func median(d []time.Duration) time.Duration {
	sort.Slice(d, func(a, b int) bool { return d[a] < d[b] })
	return d[len(d)/2]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func mbps(n int, d time.Duration) float64 { return float64(n) / 1e6 / d.Seconds() }

func versions() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	var out []string
	for _, m := range info.Deps {
		if m.Path == "github.com/beevik/etree" || m.Path == "github.com/antchfx/xmlquery" {
			out = append(out, m.Path+" "+m.Version)
		}
	}
	return strings.Join(out, ", ") + ", encoding/xml of " + runtime.Version() + " (go.sum of tools/compare)"
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
	fmt.Fprintln(os.Stderr, "compare:", err)
	os.Exit(1)
}
