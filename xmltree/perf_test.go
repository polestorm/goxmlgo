package xmltree

import (
	"bytes"
	"io"
	"runtime"
	"testing"
	"unsafe"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

func countNodes(root *Node) int {
	n := 0
	for it := newWalk(root); ; {
		c, enter := it.next()
		if c == nil {
			return n - 1 // bez węzła dokumentu
		}
		if enter {
			n++
		}
	}
}

// TestTreeLinear: budowa i zapis drzewa są liniowe względem liczby węzłów. Miarą jest liczba alokacji i bajtów
// (powtarzalna, w odróżnieniu od czasu): dokument dwa razy większy potrzebuje ich dwa razy więcej, z tolerancją
// 10%, a zapis drzewa nie alokuje na węzeł. Pamięć zajęta przez drzewo na węzeł trafia do dziennika testu
// i do docs/testing.md.
// REQ: TR-014
func TestTreeLinear(t *testing.T) {
	type measure struct {
		nodes               int
		allocs, bytes, held uint64
		writeAllocs         float64
	}
	run := func(src []byte) measure {
		var before, after, kept runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		doc, err := Parse(bytes.NewReader(src), Options{})
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		runtime.ReadMemStats(&kept)
		m := measure{nodes: countNodes(doc.Node()), allocs: after.Mallocs - before.Mallocs, bytes: after.TotalAlloc - before.TotalAlloc}
		if kept.HeapAlloc > before.HeapAlloc {
			m.held = kept.HeapAlloc - before.HeapAlloc
		}
		w := xmlwriter.New(io.Discard, xmlwriter.Options{})
		write := func() {
			w.Reset(io.Discard, xmlwriter.Options{})
			if err := doc.WriteTo(w); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
		}
		write()
		m.writeAllocs = testing.AllocsPerRun(3, write)
		runtime.KeepAlive(doc)
		return m
	}
	for _, gen := range []struct {
		name string
		make func(n int) io.Reader
		n    int
	}{{"GenJPK", testkit.GenJPK, 2000}, {"GenWordML", testkit.GenWordML, 3000}} {
		small, _ := io.ReadAll(gen.make(gen.n))
		large, _ := io.ReadAll(gen.make(2 * gen.n))
		a, b := run(small), run(large)
		for what, r := range map[string]float64{
			"nodes":       float64(b.nodes) / float64(a.nodes),
			"allocations": float64(b.allocs) / float64(a.allocs),
			"bytes":       float64(b.bytes) / float64(a.bytes),
		} {
			// alokacji jest tak mało, że część stała waży: dla nich liczy się tylko granica górna
			if r > 2.2 || (r < 1.8 && what != "allocations") {
				t.Errorf("%s: a document twice as large needs %.2f times the %s", gen.name, r, what)
			}
		}
		if a.writeAllocs > 8 || b.writeAllocs > 8 {
			t.Errorf("%s: WriteTo allocates %v and %v times, want a constant", gen.name, a.writeAllocs, b.writeAllocs)
		}
		t.Logf("%s(%d): %d bytes of XML, %d nodes; build: %.2f allocations and %.0f bytes allocated per node, %.0f bytes held per node (%.1f× the document); WriteTo: %v allocations per document",
			gen.name, 2*gen.n, len(large), b.nodes, float64(b.allocs)/float64(b.nodes), float64(b.bytes)/float64(b.nodes),
			float64(b.held)/float64(b.nodes), float64(b.held)/float64(len(large)), b.writeAllocs)
	}
	t.Logf("a Node is %d bytes", unsafe.Sizeof(Node{}))
}

func BenchmarkParse(b *testing.B) {
	src, _ := io.ReadAll(testkit.GenJPK(20000))
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Parse(bytes.NewReader(src), Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteTo(b *testing.B) {
	src, _ := io.ReadAll(testkit.GenJPK(20000))
	doc, err := Parse(bytes.NewReader(src), Options{})
	if err != nil {
		b.Fatal(err)
	}
	w := xmlwriter.New(io.Discard, xmlwriter.Options{})
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Reset(io.Discard, xmlwriter.Options{})
		if err := doc.WriteTo(w); err != nil {
			b.Fatal(err)
		}
		w.Close()
	}
}
