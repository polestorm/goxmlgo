package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var sizesOut = flag.String("sizes-out", "", "write the result of TestSizes to this file")

// TestSizes mierzy rozmiar kandydatów tak, jak odczuje go program, który ich użyje: liczbę modułów wymaganych
// w go.mod kandydata, liczbę wierszy kodu pakietów spoza biblioteki standardowej, które wciąga program „dokument
// z wejścia do drzewa i na wyjście” (sizeprog/<kandydat>), i przyrost rozmiaru tego programu wobec programu, który
// tylko kopiuje wejście (sizeprog/base), przy -trimpath -ldflags="-s -w". Test niczego nie wymaga od kandydatów;
// zapisuje liczby (flaga -sizes-out).
func TestSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("builds four programs")
	}
	tmp := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("go", args...)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
		}
		return string(out)
	}
	size := func(prog string) int64 {
		t.Helper()
		bin := filepath.Join(tmp, prog)
		run("build", "-trimpath", "-ldflags=-s -w", "-o", bin, "./sizeprog/"+prog)
		st, err := os.Stat(bin)
		if err != nil {
			t.Fatal(err)
		}
		return st.Size()
	}
	base := size("base")
	var b strings.Builder
	fmt.Fprintf(&b, "# go -C tools/compare test . -run TestSizes -sizes-out ../../docs/results/tree-sizes.tsv\n")
	fmt.Fprintf(&b, "# goxmlgo: %s; candidates: %s\n", commit("../.."), versions())
	fmt.Fprintf(&b, "# %s %s/%s, CGO_ENABLED=0, go build -trimpath -ldflags=\"-s -w\"; the program reads a document from standard input into a tree and writes the tree to standard output (tools/compare/sizeprog); the base program copies standard input to standard output: %d bytes\n",
		runtime.Version(), runtime.GOOS, runtime.GOARCH, base)
	fmt.Fprintf(&b, "candidate\tmodules_required\tpackages_outside_std\tlines_of_code\tprogram_bytes\tadded_bytes\n")
	for _, c := range []struct{ name, prog, module string }{
		{"goxmlgo xmltree", "goxmlgo", "github.com/polestorm/goxmlgo"},
		{"beevik/etree", "etree", "github.com/beevik/etree"},
		{"antchfx/xmlquery", "xmlquery", "github.com/antchfx/xmlquery"},
	} {
		// pakiety spoza biblioteki standardowej, bez samego programu
		pkgs, lines := 0, 0
		list := run("list", "-deps", "-f", `{{if not .Standard}}{{.ImportPath}}|{{.Dir}}|{{join .GoFiles ","}}{{end}}`, "./sizeprog/"+c.prog)
		for _, l := range strings.Split(strings.TrimSpace(list), "\n") {
			f := strings.Split(l, "|")
			if len(f) != 3 || strings.Contains(f[0], "/sizeprog/") {
				continue
			}
			pkgs++
			for _, name := range strings.Split(f[2], ",") {
				data, err := os.ReadFile(filepath.Join(f[1], name))
				if err != nil {
					t.Fatal(err)
				}
				lines += bytes.Count(data, []byte("\n"))
			}
		}
		// moduły wymagane wprost i pośrednio w go.mod kandydata
		gomod := strings.TrimSpace(run("list", "-m", "-f", "{{.GoMod}}", c.module))
		required := 0
		if data, err := os.ReadFile(gomod); err == nil {
			inBlock := false
			for _, l := range strings.Split(string(data), "\n") {
				l = strings.TrimSpace(l)
				switch {
				case l == "require (":
					inBlock = true
				case l == ")":
					inBlock = false
				case inBlock && l != "" && !strings.HasPrefix(l, "//"):
					required++
				case strings.HasPrefix(l, "require ") && !strings.HasSuffix(l, "("):
					required++
				}
			}
		} else {
			t.Fatal(err)
		}
		s := size(c.prog)
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\t%d\t%d\n", c.name, required, pkgs, lines, s, s-base)
	}
	t.Log("\n" + b.String())
	if *sizesOut != "" {
		if err := os.WriteFile(*sizesOut, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
