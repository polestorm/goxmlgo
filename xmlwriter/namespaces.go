package xmlwriter

import "strconv"

// linearAttrs: do tylu atrybutów powtórzenie wykrywa przegląd listy, powyżej zbiór (inaczej czas byłby
// kwadratowy względem liczby atrybutów).
const linearAttrs = 32

// attrKey to nazwa rozwinięta atrybutu (Namespaces in XML 1.0, 2.1); bez przestrzeni nazw sama nazwa.
type attrKey struct{ space, local string }

// attrSet to atrybuty jednego znacznika początku: do wykrywania powtórzeń i do limitu liczby atrybutów.
type attrSet struct {
	keys   []attrKey
	prefix []string
	set    map[attrKey]string // nazwa → prefiks, z jakim ją zapisano; używany powyżej linearAttrs
	n      int                // atrybuty i deklaracje przestrzeni nazw znacznika
}

func (a *attrSet) reset() {
	a.keys, a.prefix, a.n = a.keys[:0], a.prefix[:0], 0
	if len(a.set) > 0 {
		clear(a.set)
	}
}

func (a *attrSet) find(k attrKey) (string, bool) {
	if len(a.set) > 0 {
		p, ok := a.set[k]
		return p, ok
	}
	for i := range a.keys {
		if a.keys[i] == k {
			return a.prefix[i], true
		}
	}
	return "", false
}

func (a *attrSet) add(k attrKey, prefix string) {
	a.n++
	if len(a.set) == 0 {
		if len(a.keys) < linearAttrs {
			a.keys, a.prefix = append(a.keys, k), append(a.prefix, prefix)
			return
		}
		if a.set == nil {
			a.set = map[attrKey]string{}
		}
		for i, key := range a.keys {
			a.set[key] = a.prefix[i]
		}
	}
	a.set[k] = prefix
}

// lookup zwraca przestrzeń nazw związaną z prefiksem w zakresie otwartych elementów (i znacznika właśnie
// budowanego). Prefiks pusty to przestrzeń domyślna; po xmlns="" i bez deklaracji nie jest związany.
func (w *Writer) lookup(prefix string) (string, bool) {
	if prefix == "xml" {
		return nsXML, true
	}
	for i := len(w.bindings) - 1; i >= 0; i-- {
		if w.bindings[i].Prefix == prefix {
			return w.bindings[i].URI, w.bindings[i].URI != ""
		}
	}
	return "", false
}

// declaredFrom mówi, czy prefiks ma deklarację wśród w.bindings[mark:].
func (w *Writer) declaredFrom(prefix string, mark int) bool {
	for _, b := range w.bindings[mark:] {
		if b.Prefix == prefix {
			return true
		}
	}
	return false
}

// prefixFor szuka w zakresie niepustego prefiksu związanego z przestrzenią space, od najbliższej deklaracji;
// pomija prefiksy przesłonięte bliższą deklaracją.
func (w *Writer) prefixFor(space string) string {
	for i := len(w.bindings) - 1; i >= 0; i-- {
		b := w.bindings[i]
		if b.URI != space || b.Prefix == "" {
			continue
		}
		if !w.declaredFrom(b.Prefix, i+1) {
			return b.Prefix
		}
	}
	return ""
}

var generated = func() (t [32]string) {
	for i := range t {
		t[i] = "ns" + strconv.Itoa(i+1)
	}
	return t
}()

// freePrefix zwraca pierwszy z prefiksów ns1, ns2, …, który nie ma deklaracji w żadnym otwartym elemencie.
func (w *Writer) freePrefix() string {
	for i := 0; ; i++ {
		var p string
		if i < len(generated) {
			p = generated[i]
		} else {
			p = "ns" + strconv.Itoa(i+1)
		}
		if !w.declaredFrom(p, 0) {
			return p
		}
	}
}

// checkDecl sprawdza deklarację przestrzeni nazw znacznika, którego deklaracje zaczynają się w w.bindings[mark]
// (Namespaces in XML 1.0, rozdział 3 i ograniczenia przestrzeni nazw).
func (w *Writer) checkDecl(d Binding, mark int) error {
	switch d.Prefix {
	case "":
		if d.URI == nsXML || d.URI == nsXMLNS {
			return w.fail(CodeReservedPrefixes, "the namespaces of xml and xmlns cannot be the default namespace")
		}
	case "xmlns":
		return w.fail(CodeReservedPrefixes, "the prefix xmlns cannot be declared")
	case "xml":
		if d.URI != nsXML {
			return w.fail(CodeReservedPrefixes, "the prefix xml cannot be bound to another namespace")
		}
	default:
		switch nameKind(d.Prefix) {
		case nameBad:
			return w.fail(CodeName, "namespace prefix is not a Name")
		case nameColon:
			return w.fail(CodeQName, "namespace prefix contains a colon")
		}
		if len(d.Prefix)+len("xmlns:") > w.lim.MaxNameLength {
			return w.fail(CodeLimitNameLength, "namespace prefix longer than the limit")
		}
		switch d.URI {
		case "":
			return w.fail(CodeNoPrefixUndeclaring, "the declaration of prefix "+d.Prefix+" has an empty namespace name")
		case nsXML, nsXMLNS:
			return w.fail(CodeReservedPrefixes, "the namespaces of xml and xmlns cannot be bound to another prefix")
		}
	}
	if err := w.checkSpace(d.URI); err != nil {
		return err
	}
	if w.declaredFrom(d.Prefix, mark) {
		return w.fail(CodeUniqueAttSpec, "namespace prefix declared twice in one tag")
	}
	return nil
}

// elemPrefix ustala prefiks nazwy elementu, którego deklaracje jawne są już w w.bindings[mark:]. declare mówi,
// że pisarz musi dopisać deklarację (prefiks, name.Space) do tego znacznika (tryb automatyczny).
func (w *Writer) elemPrefix(name Name, mark int) (prefix string, declare bool, err error) {
	switch {
	case name.Prefix == "xmlns" || name.Space == nsXMLNS:
		return "", false, w.fail(CodeReservedPrefixes, "an element cannot use the prefix xmlns or its namespace")
	case name.Prefix == "xml" && name.Space != nsXML:
		return "", false, w.fail(CodeNamespaceName, "the prefix xml is bound to "+nsXML)
	case name.Space == nsXML:
		if name.Prefix == "xml" || w.auto {
			return "xml", false, nil
		}
		return "", false, w.fail(CodeNamespaceName, "the XML namespace needs the prefix xml")
	}
	uri, bound := w.lookup(name.Prefix)
	if uri == name.Space && (bound || name.Prefix == "") {
		return name.Prefix, false, nil
	}
	if !w.auto {
		if name.Prefix != "" && !bound {
			return "", false, w.fail(CodePrefixDeclared, "prefix "+name.Prefix+" is not declared")
		}
		return "", false, w.mismatch("element", name, uri)
	}
	if name.Prefix != "" && name.Space == "" {
		return "", false, w.fail(CodeNamespaceName, "an element name with a prefix needs a namespace")
	}
	if !w.declaredFrom(name.Prefix, mark) {
		return name.Prefix, true, nil
	}
	if name.Space == "" {
		return "", false, w.fail(CodeNamespaceName, "an element without a namespace in a tag that declares a default namespace")
	}
	if p := w.prefixFor(name.Space); p != "" {
		return p, false, nil
	}
	return w.freePrefix(), true, nil
}

// attrPrefix ustala prefiks nazwy atrybutu w otwartym znaczniku. Atrybut bez prefiksu nie należy do żadnej
// przestrzeni nazw (Namespaces in XML 1.0, 6.2), więc atrybut w przestrzeni nazw zawsze dostaje prefiks niepusty.
func (w *Writer) attrPrefix(name Name) (prefix string, declare bool, err error) {
	switch {
	case name.Space == "":
		if name.Prefix != "" {
			return "", false, w.fail(CodeNamespaceName, "an attribute name with a prefix needs a namespace")
		}
		return "", false, nil
	case name.Space == nsXMLNS:
		return "", false, w.fail(CodeReservedPrefixes, "an attribute cannot be in the namespace of xmlns")
	case name.Prefix == "xml" && name.Space != nsXML:
		return "", false, w.fail(CodeNamespaceName, "the prefix xml is bound to "+nsXML)
	case name.Space == nsXML:
		if name.Prefix == "xml" || w.auto {
			return "xml", false, nil
		}
		return "", false, w.fail(CodeNamespaceName, "the XML namespace needs the prefix xml")
	}
	if name.Prefix != "" {
		uri, bound := w.lookup(name.Prefix)
		if bound && uri == name.Space {
			return name.Prefix, false, nil
		}
		if !w.auto {
			if !bound {
				return "", false, w.fail(CodePrefixDeclared, "prefix "+name.Prefix+" is not declared")
			}
			return "", false, w.mismatch("attribute", name, uri)
		}
		// Prefiks wolno zadeklarować w tym znaczniku tylko wtedy, gdy nie ma go w zakresie: inaczej zmieniłby
		// znaczenie nazwy elementu albo wcześniejszych atrybutów tego znacznika.
		if !w.declaredFrom(name.Prefix, 0) {
			return name.Prefix, true, nil
		}
	} else if !w.auto {
		return "", false, w.fail(CodeNamespaceName, "an attribute in a namespace needs a prefix")
	}
	if p := w.prefixFor(name.Space); p != "" {
		return p, false, nil
	}
	return w.freePrefix(), true, nil
}

func (w *Writer) mismatch(what string, name Name, bound string) error {
	q := name.Local
	if name.Prefix != "" {
		q = name.Prefix + ":" + q
	}
	return w.fail(CodeNamespaceName, what+" "+q+": Name.Space is "+strconv.Quote(name.Space)+
		", but the prefix is bound to "+strconv.Quote(bound))
}

// checkSpace sprawdza nazwę przestrzeni nazw jako wartość atrybutu deklaracji: znaki i długość. Dotyczy deklaracji
// podanych przez wywołującego i tych, które pisarz dopisuje sam z Name.Space (tryb automatyczny).
func (w *Writer) checkSpace(uri string) error {
	if len(uri) > w.lim.MaxTextSize {
		return w.fail(CodeLimitTextSize, "namespace name longer than the limit")
	}
	if _, code := check(uri, cAttr, 0); code != "" {
		return w.dataErr(code, "a namespace name")
	}
	return nil
}
