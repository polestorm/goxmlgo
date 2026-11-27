package xmltree

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/polestorm/goxmlgo/internal/testkit"
	"github.com/polestorm/goxmlgo/xmlreader"
	"github.com/polestorm/goxmlgo/xmlwriter"
)

// Słowniki FuzzTree: nazwy, przestrzenie nazw i dane, z których powstają węzły dodawane do drzewa.
var (
	fuzzLocals   = []string{"a", "b", "c", "żółw", "item", "Id", "id", "lang", "x:y", "", "1a"}
	fuzzPrefixes = []string{"", "", "", "p", "q", "xml", "ns1", "xmlns"}
	fuzzSpaces   = []string{"", "", "urn:a", "urn:b", "urn:c", nsXML}
	fuzzData     = []string{"", "v", "text", " ", "\n", "a\r\nb", "<&>\"'", "]]>", "--", "?>", " lead", "ż \U0001F600", "\x00", "\xff"}
)

type fuzzInput struct {
	data []byte
	i    int
}

func (in *fuzzInput) byte() byte {
	if in.i >= len(in.data) {
		return 0
	}
	in.i++
	return in.data[in.i-1]
}

func (in *fuzzInput) pick(table []string) string { return table[int(in.byte())%len(table)] }

func (in *fuzzInput) name() Name {
	return Name{Space: in.pick(fuzzSpaces), Local: in.pick(fuzzLocals), Prefix: in.pick(fuzzPrefixes)}
}

// treeNodes zwraca węzły drzewa w kolejności dokumentu i sprawdza po drodze spójność: każde dziecko wskazuje
// rodzica, żaden węzeł nie występuje dwa razy.
func treeNodes(t *testing.T, root *Node) []*Node {
	var nodes []*Node
	seen := map[*Node]bool{}
	for it := newWalk(root); ; {
		n, enter := it.next()
		if n == nil {
			return nodes
		}
		if !enter {
			continue
		}
		if seen[n] {
			t.Fatalf("a node is in the tree twice")
		}
		seen[n] = true
		var prev *Node
		for c := n.FirstChild(); c != nil; prev, c = c, c.NextSibling() {
			if c.Parent() != n || c.PrevSibling() != prev {
				t.Fatalf("broken links between a node and its children")
			}
		}
		if n.LastChild() != prev {
			t.Fatalf("LastChild is not the last child")
		}
		if k := n.Kind(); k != KindElement && k != KindDocument && n.FirstChild() != nil {
			t.Fatalf("a node of kind %d has children", k)
		}
		nodes = append(nodes, n)
	}
}

// FuzzTree sprawdza dwie rzeczy. Dla wejścia, które czytnik przyjmuje: drzewo zbudowane i zapisane daje te same
// bajty co strumień, a czytnik drzewa ten sam strumień tokenów (K3); wejście odrzucone przez czytnik odrzuca też
// Parse, z tym samym błędem. Potem ciąg operacji zmiany z drugiego argumentu: żadna nie psuje wskaźników na
// rodzica, każdy błąd operacji to *Error, a drzewo po zmianach albo zapisuje się z błędem pisarza o stałym kodzie,
// albo daje dokument, który czytnik czyta i z którego odtwarza strumień czytnika drzewa.
// REQ: TR-007 TR-008 TR-009 TR-010 TR-011 TR-012
func FuzzTree(f *testing.F) {
	for _, s := range []string{
		"<a/>",
		"<?xml version='1.0' standalone='yes'?><a x='1' xmlns:p='urn:a'><p:b p:c='&#10;'>t&amp;&#13;<![CDATA[c]]></p:b><!--c--><?pi d?></a>",
		`<!DOCTYPE a [<!ENTITY e "x<b>&f;</b>"><!ENTITY f ""><!ATTLIST a d CDATA "v" i ID #IMPLIED>]><a i="k">&e;&f;</a>`,
		"<a xmlns='urn:a'><b xmlns=''><c xmlns='urn:b' xmlns:q='urn:c' q:x='1'/></b><d xml:space='preserve'> </d></a>",
	} {
		for _, ops := range []string{"", "\x00\x01\x02\x03\x04\x05\x06\x07", "\x02\x01\x00\x00\x03\x02\x01\x05\x01\x02\x03\x04\x04\x00\x02\x06\x01\x07\x02\x01"} {
			f.Add([]byte(s), []byte(ops), false)
		}
		f.Add([]byte(s), []byte{}, true)
	}
	for _, b := range testkit.SampleFiles("../testdata/corpus", []string{".xml"}, 30, 64<<10) {
		f.Add(b, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}, false)
	}
	for _, b := range testkit.SampleFiles("../testdata/w3c/xmlconf", []string{".xml"}, 100, 64<<10) {
		f.Add(b, []byte{}, false)
	}
	f.Fuzz(func(t *testing.T, data, ops []byte, noNS bool) {
		ro := xmlreader.Options{NoNamespaces: noNS}
		if _, rerr := dumpBytes(data, noNS); rerr != nil {
			_, err := Parse(bytes.NewReader(data), Options{Reader: ro})
			if err == nil || err.Error() != rerr.Error() {
				t.Fatalf("the reader says %v, Parse says %v", rerr, err)
			}
			return
		}
		if why := treeRoundTrip(data, noNS); why != "" {
			t.Fatal(why)
		}
		doc, err := Parse(bytes.NewReader(data), Options{Reader: ro})
		if err != nil {
			t.Fatal(err)
		}
		in := &fuzzInput{data: ops}
		var pool []*Node // węzły bez rodzica: usunięte, skopiowane, nowe
		take := func() *Node {
			switch b := in.byte(); {
			case len(pool) > 0 && b%2 == 0:
				i := int(in.byte()) % len(pool)
				n := pool[i]
				pool = append(pool[:i], pool[i+1:]...)
				return n
			case b%8 == 1:
				return NewText(in.pick(fuzzData))
			case b%8 == 3:
				return NewCDATA(in.pick(fuzzData))
			case b%8 == 5:
				return NewComment(in.pick(fuzzData))
			case b%16 == 7:
				return NewProcInst(in.pick(fuzzLocals), in.pick(fuzzData))
			}
			return NewElement(in.name())
		}
		for step := 0; step < 48 && in.i < len(in.data); step++ {
			nodes := treeNodes(t, doc.Node())
			if len(nodes) > 5000 {
				break // kopie wstawiane do własnego źródła podwajają drzewo; model nie ma mierzyć rozmiaru
			}
			n := nodes[int(in.byte())%len(nodes)]
			var kid *Node
			if c := n.Children(); len(c) > 0 {
				kid = c[int(in.byte())%len(c)]
			}
			var err error
			switch in.byte() % 9 {
			case 0:
				c := take()
				if err = n.Append(c); err != nil {
					pool = append(pool, c)
				}
			case 1:
				c := take()
				if err = n.InsertBefore(c, kid); err != nil {
					pool = append(pool, c)
				}
			case 2:
				if kid != nil {
					if err = n.Remove(kid); err == nil {
						pool = append(pool, kid)
					}
				}
			case 3:
				if kid != nil {
					c := take()
					if err = n.Replace(kid, c); err == nil {
						pool = append(pool, kid)
					} else {
						pool = append(pool, c)
					}
				}
			case 4:
				err = n.SetAttr(in.name(), in.pick(fuzzData))
			case 5:
				n.RemoveAttr(in.pick(fuzzSpaces), in.pick(fuzzLocals))
			case 6:
				err = n.SetText(in.pick(fuzzData))
			case 7:
				if n.Kind() != KindDocument {
					pool = append(pool, n.Clone())
				}
			case 8:
				if n.Kind() == KindElement {
					n.SetNamespaces(append(n.Namespaces(), Binding{Prefix: in.pick(fuzzPrefixes), URI: in.pick(fuzzSpaces)}))
				}
			}
			var te *Error
			if err != nil && (!errors.As(err, &te) || te.Code == "") {
				t.Fatalf("step %d: an error that is not *Error: %v", step, err)
			}
			if len(pool) > 16 {
				pool = pool[1:]
			}
		}
		treeNodes(t, doc.Node())
		for _, p := range pool {
			if p.Parent() != nil {
				t.Fatal("a node in the pool has a parent")
			}
			treeNodes(t, p)
		}
		out, err := writeDoc(doc, xmlwriter.Options{NoNamespaces: noNS})
		if err != nil {
			var we *xmlwriter.Error
			if !errors.As(err, &we) || we.Code == "" {
				t.Fatalf("WriteTo: an error that is not *xmlwriter.Error: %v", err)
			}
		} else {
			got, err := dumpTokens(xmlreader.New(bytes.NewReader(out), ro), true)
			if err != nil {
				t.Fatalf("the reader rejects what the tree wrote: %v\n%s", err, out)
			}
			want, err := dumpTokens(NewReader(doc.Node()), true)
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}
			if got != want {
				t.Fatalf("the written tree reads back as another stream than the tree gives\n%s\noutput: %s", firstDiff(got, want), out)
			}
			again, err := writeDoc(doc, xmlwriter.Options{NoNamespaces: noNS})
			if err != nil || !bytes.Equal(again, out) {
				t.Fatalf("the second write of the same tree gives other bytes (%v)", err)
			}
		}
		// tryb automatyczny pisarza: bez paniki, a zapis bez błędu daje dokument dobrze uformowany
		if !noNS {
			if out, err := writeDoc(doc, xmlwriter.Options{AutoNamespaces: true}); err == nil {
				if _, err := dumpBytes(out, false); err != nil {
					t.Fatalf("the reader rejects what the tree wrote with AutoNamespaces: %v\n%s", err, out)
				}
			} else if !strings.Contains(err.Error(), ":") {
				t.Fatalf("AutoNamespaces: %v", err)
			}
		}
	})
}
