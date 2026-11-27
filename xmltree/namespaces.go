package xmltree

// unboundOnly to kontekst miejsca bez żadnej deklaracji: sama informacja, że przestrzeń domyślna nie jest związana.
var unboundOnly = []Binding{{}}

// LookupNamespace returns the namespace bound to prefix in the scope of the node: from the declarations of the
// node, of its ancestors, and from the namespaces an element remembers from the place it was taken from. The
// empty prefix is the default namespace; it is not bound (false) outside any declaration and after xmlns="". The
// prefix xml is always bound.
func (n *Node) LookupNamespace(prefix string) (string, bool) {
	if prefix == "xml" {
		return nsXML, true
	}
	for e := n; e != nil; e = e.parent {
		if e.el == nil {
			continue
		}
		ns := e.el.ns
		if ns == nil {
			continue
		}
		for i := len(ns.decls) - 1; i >= 0; i-- {
			if b := &ns.decls[i]; b.Prefix == prefix {
				return b.URI, b.URI != ""
			}
		}
		for i := range ns.context {
			if b := &ns.context[i]; b.Prefix == prefix {
				return b.URI, b.URI != ""
			}
		}
	}
	return "", false
}

// InScopeNamespaces returns the namespaces in scope of the node, as LookupNamespace sees them: one binding for
// each bound prefix, sorted by prefix, the default namespace (empty prefix) first. A default namespace undeclared
// by xmlns="" is not in the result, and neither is the prefix xml.
func (n *Node) InScopeNamespaces() []Binding {
	all := n.visible(true)
	out := all[:0]
	for _, b := range all {
		if b.URI != "" && b.Prefix != "xml" {
			out = append(out, b)
		}
	}
	sortBindings(out)
	return out
}

// scopeAbove zwraca kontekst węzła: wiązania widoczne nad nim (bez jego własnych deklaracji), posortowane, zawsze
// z wpisem dla przestrzeni domyślnej (pusty URI, gdy nie jest związana).
func (n *Node) scopeAbove() []Binding {
	out := n.visible(false)
	if len(out) == 0 {
		return unboundOnly
	}
	keep, hasDefault := out[:0], false
	for _, b := range out {
		if b.Prefix == "" {
			hasDefault = true
		} else if b.URI == "" || b.Prefix == "xml" {
			continue // prefiksu nie da się znieść, a xml jest związany zawsze: takie wpisy nic nie znaczą
		}
		keep = append(keep, b)
	}
	if !hasDefault {
		keep = append(keep, Binding{})
	}
	sortBindings(keep)
	return keep
}

// visible zbiera po jednym wiązaniu na prefiks, od najbliższej deklaracji: własne deklaracje n (gdy own), jego
// kontekst, potem przodkowie. Wpis z pustym URI oznacza prefiks zniesiony albo przestrzeń domyślną bez wiązania.
func (n *Node) visible(own bool) []Binding {
	var out []Binding
	add := func(b Binding) {
		for i := range out {
			if out[i].Prefix == b.Prefix {
				return
			}
		}
		out = append(out, b)
	}
	for e := n; e != nil; e = e.parent {
		if e.el == nil {
			continue
		}
		if own || e != n {
			d := e.el.decls()
			for i := len(d) - 1; i >= 0; i-- {
				add(d[i])
			}
		}
		for _, b := range e.el.ctx() {
			add(b)
		}
	}
	return out
}

func sortBindings(b []Binding) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j].Prefix < b[j-1].Prefix; j-- {
			b[j], b[j-1] = b[j-1], b[j]
		}
	}
}

// fixer ustala deklaracje, które trzeba dopisać do znacznika elementu, żeby w miejscu zapisu jego nazwy i jego
// zapamiętany kontekst znaczyły to samo co w drzewie. lookup odpowiada o zakresie nad elementem w miejscu zapisu.
// Kolejność wyniku jest stała: kontekst według prefiksu, potem nazwa elementu, potem atrybuty w ich kolejności.
type fixer struct {
	lookup func(prefix string) (string, bool)
	extra  []Binding
}

func (f *fixer) element(n *Node) []Binding {
	e := n.el
	f.extra = f.extra[:0]
	if e.ns != nil {
		for i := range e.ns.context {
			f.need(e, e.ns.context[i].Prefix, e.ns.context[i].URI)
		}
	}
	if name := e.name; name != nil && (name.Space != "" || name.Prefix == "") {
		f.need(e, name.Prefix, name.Space)
	}
	for i := range e.attrs {
		if a := e.attrs[i].name; a.Space != "" && a.Prefix != "" {
			f.need(e, a.Prefix, a.Space)
		}
	}
	return f.extra
}

// need dopisuje deklarację prefix → uri, jeśli ani element, ani zakres nad nim jej nie daje. Deklaracja własna
// elementu rozstrzyga zawsze: niezgodność nazwy z własną deklaracją zgłasza pisarz, drzewo jej nie naprawia.
func (f *fixer) need(e *elem, prefix, uri string) {
	if prefix == "xml" || prefix == "xmlns" {
		return
	}
	if e.ns != nil {
		for i := range e.ns.decls {
			if e.ns.decls[i].Prefix == prefix {
				return
			}
		}
	}
	for i := range f.extra {
		if f.extra[i].Prefix == prefix {
			return
		}
	}
	cur, bound := f.lookup(prefix)
	if !bound {
		cur = ""
	}
	if cur == uri || (uri == "" && prefix != "") {
		return
	}
	f.extra = append(f.extra, Binding{Prefix: prefix, URI: uri})
}
