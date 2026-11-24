package emit

import (
	"regexp"
	"strings"
)

// The emitter writes Rust the way rustfmt (default settings, edition 2024)
// would format it, so the crate is rustfmt-clean as written. Expressions
// are built as small trees and laid out flat when they fit, and broken
// over lines, as rustfmt breaks them, when they do not.

const (
	maxWidth       = 100 // max_width
	fnCallWidth    = 60  // fn_call_width: arguments of a call on one line
	structLitWidth = 18  // struct_lit_width: fields of a struct literal on one line
	arrayWidth     = 60  // array_width: elements of vec![...] on one line
	chainWidth     = 60  // chain_width: a chain of method calls on one line
	indentWidth    = 4
)

// Rust operator precedence, higher binds tighter.
const (
	precAssign  = 1
	precRange   = 3
	precOr      = 4
	precAnd     = 5
	precCmp     = 6
	precBitOr   = 7
	precBitXor  = 8
	precBitAnd  = 9
	precShift   = 10
	precAdd     = 11
	precMul     = 12
	precCast    = 13
	precUnary   = 14
	precPostfix = 16
	precAtom    = 17
)

// A node is an expression to lay out.
type node interface {
	flat() string
	prec() int
}

// atom is text that is never broken.
type atom struct {
	s string
	p int
}

func (a atom) flat() string { return a.s }
func (a atom) prec() int    { return a.p }

func text(s string) node { return atom{s, precAtom} }

// call is f(args), a macro call m!(args) or m![args], or a method call
// recv.f(args).
type call struct {
	head        string // "f", "println!", "vec!" or ".f" for a method
	recv        node   // the receiver of a method call, or nil
	args        []node
	open, close string
	array       bool // laid out as an array (vec![...])
}

// specialMacros are the format-like macros rustfmt lays out specially: the
// first n arguments (the writer and the format string) on their own lines,
// and the rest filled onto lines when they are all simple.
var specialMacros = map[string]int{
	"format!": 1, "print!": 1, "println!": 1, "eprint!": 1, "eprintln!": 1, "panic!": 1,
	"write!": 2, "writeln!": 2,
}

func (c *call) flat() string {
	var b strings.Builder
	if c.recv != nil {
		b.WriteString(paren(c.recv, precPostfix))
	}
	b.WriteString(c.head)
	b.WriteString(c.open)
	b.WriteString(joinFlat(c.args))
	b.WriteString(c.close)
	return b.String()
}

func (c *call) prec() int { return precPostfix }

func joinFlat(ns []node) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = n.flat()
	}
	return strings.Join(parts, ", ")
}

// structLit is Name { field: value, ..base }.
type structLit struct {
	name   string
	fields []fieldInit
	base   string
}

type fieldInit struct {
	name  string
	value node
}

func (s *structLit) fieldsFlat() string {
	parts := make([]string, 0, len(s.fields)+1)
	for _, f := range s.fields {
		parts = append(parts, f.flatField())
	}
	if s.base != "" {
		parts = append(parts, ".."+s.base)
	}
	return strings.Join(parts, ", ")
}

func (f fieldInit) flatField() string {
	if v := f.value.flat(); v != f.name {
		return f.name + ": " + v
	}
	return f.name
}

func (s *structLit) flat() string {
	if len(s.fields) == 0 && s.base == "" {
		return s.name + " {}"
	}
	return s.name + " { " + s.fieldsFlat() + " }"
}

func (s *structLit) prec() int { return precAtom }

// addrOf is &x or &mut x of an expression that may need breaking, or a
// closure without parameters, || x (a static's initializer, RFC 0010),
// whose body needs no parentheses.
type addrOf struct {
	prefix string // "&", "&mut " or "|| "
	x      node
}

func (a *addrOf) flat() string {
	if a.prefix == "|| " {
		return a.prefix + a.x.flat()
	}
	return a.prefix + paren(a.x, precUnary)
}
func (a *addrOf) prec() int { return precUnary }

// binary is l op r.
type binary struct {
	op       string
	l, r     node
	p        int
	nonAssoc bool // a comparison: an operand of the same precedence needs parentheses
}

func (b *binary) flat() string {
	l := paren(b.l, b.p)
	if (b.nonAssoc && b.l.prec() == b.p) || unclear(b.p, b.l) {
		l = "(" + b.l.flat() + ")"
	}
	r := parenRight(b.r, b.p)
	if unclear(b.p, b.r) {
		r = "(" + b.r.flat() + ")"
	}
	return l + " " + b.op + " " + r
}

// unclear reports whether operand n of a bitwise or shift operator of
// precedence p is arithmetic: Rust needs no parentheses there, but clippy's
// precedence lint asks for them, and they say what the Go source said.
func unclear(p int, n node) bool {
	return p >= precBitOr && p <= precShift && (n.prec() == precAdd || n.prec() == precMul)
}

func (b *binary) prec() int { return b.p }

// paren renders n, parenthesised if it binds less tightly than p.
func paren(n node, p int) string {
	if n.prec() < p {
		return "(" + n.flat() + ")"
	}
	return n.flat()
}

// parenRight renders the right operand of a left-associative operator.
func parenRight(n node, p int) string {
	if n.prec() <= p {
		return "(" + n.flat() + ")"
	}
	return n.flat()
}

// lay renders n starting at column col (the width already used on its
// line) and at indentation ind, breaking it over lines where it does not
// fit. The result's first line continues the current line.
func lay(n node, col, ind int) string {
	return layWith(n, col, ind, 0)
}

// layWith is lay with suffix: the width that follows n on its last line
// (such as ";" or " {").
func layWith(n node, col, ind, suffix int) string {
	f := n.flat()
	if c, ok := n.(*call); fits(n, f, col, suffix) || (ok && len(c.args) == 0 && !longChain(c)) {
		return f // a call without arguments has nothing to break
	}
	return layBroken(n, col, ind, suffix)
}

// layBroken renders n broken over lines, as rustfmt does when it does not
// fit on one.
func layBroken(n node, col, ind, suffix int) string {
	f := n.flat()
	switch n := n.(type) {
	case *addrOf:
		return n.prefix + layBroken(n.x, col+len(n.prefix), ind, suffix)
	case *call:
		if root, links := chainOf(n); len(links) >= 2 {
			return chainBroken(root, links, ind)
		}
		return n.broken(col, ind, suffix)
	case *structLit:
		return n.broken(ind)
	case *binary:
		return n.broken(col, ind, suffix)
	}
	return f
}

// fits reports whether the flat form of n fits on the line and within the
// width rustfmt allows for its kind.
func fits(n node, f string, col, suffix int) bool {
	if col+len(f)+suffix > maxWidth {
		return false
	}
	switch n := n.(type) {
	case *call:
		if longChain(n) {
			return false
		}
		w := len(joinFlat(n.args))
		if n.array {
			return w <= arrayWidth
		}
		if len(n.args) == 1 && !overflows(n.args[0]) {
			return true // rustfmt keeps a lone simple argument on the line
		}
		// rustfmt holds the arguments to their own widths too: a struct
		// literal in a call in a call is broken when its fields are long.
		for _, a := range n.args {
			if !fits(a, a.flat(), 0, 0) {
				return false
			}
		}
		return w <= fnCallWidth
	case *structLit:
		return len(n.fieldsFlat()) <= structLitWidth
	case *addrOf:
		return fits(n.x, n.x.flat(), col+len(n.prefix), suffix)
	case *binary:
		// Each operand keeps its own width (a struct literal compared).
		return fits(n.l, n.l.flat(), 0, 0) && fits(n.r, n.r.flat(), 0, 0)
	}
	return true
}

func (c *call) broken(col, ind, suffix int) string {
	var b strings.Builder
	if c.recv != nil {
		b.WriteString(paren(c.recv, precPostfix))
	}
	b.WriteString(c.head)
	b.WriteString(c.open)
	if len(c.args) == 1 && !c.array && overflows(c.args[0]) {
		// rustfmt lets a lone argument that is itself a call or a struct
		// literal overflow: it stays on this line and breaks inside. A
		// method chain that breaks into one link per line does not: it
		// goes on its own line, as any other argument.
		laid := layBroken(c.args[0], col+len(b.String()), ind, suffix+len(c.close))
		if m, chain := c.args[0].(*call); !chain || m.recv == nil || !linksBroken(laid) {
			b.WriteString(laid)
			b.WriteString(c.close)
			return b.String()
		}
	}
	inner := ind + indentWidth
	pad := strings.Repeat(" ", inner)
	if c.array && shortItems(c.args) {
		// rustfmt fills lines with short array elements.
		fill(&b, c.args, inner)
		b.WriteString("\n" + strings.Repeat(" ", ind) + c.close)
		return b.String()
	}
	if n, ok := specialMacros[c.head]; ok && len(c.args) > n && allSimple(c.args[n:]) && inner+len(joinFlat(c.args[n:]))+1 <= maxWidth {
		for _, a := range c.args[:n] {
			b.WriteString("\n" + pad + layWith(a, inner, inner, 1) + ",")
		}
		fill(&b, c.args[n:], inner)
		b.WriteString("\n" + strings.Repeat(" ", ind) + c.close)
		return b.String()
	}
	for _, a := range c.args {
		b.WriteString("\n" + pad)
		b.WriteString(layWith(a, inner, inner, 1))
		b.WriteString(",")
	}
	b.WriteString("\n" + strings.Repeat(" ", ind) + c.close)
	return b.String()
}

// linksBroken reports whether laid, a laid-out expression, breaks a
// method chain into one link per line: a line after the first starts with
// a link.
func linksBroken(laid string) bool {
	lines := strings.Split(laid, "\n")
	for _, l := range lines[1:] {
		if t := strings.TrimLeft(l, " "); strings.HasPrefix(t, ".") && !strings.HasPrefix(t, "..") {
			return true
		}
	}
	return false
}

// overflows reports whether n may overflow as the only argument of a call.
func overflows(n node) bool {
	switch n := n.(type) {
	case *call, *structLit:
		return true
	case *addrOf:
		return overflows(n.x)
	}
	return false
}

// fill writes items on lines starting at column ind, as many on each line
// as fit, each followed by a comma.
func fill(b *strings.Builder, items []node, ind int) {
	pad := strings.Repeat(" ", ind)
	b.WriteString("\n" + pad)
	lineLen := ind
	for i, a := range items {
		s := a.flat() + ","
		if i > 0 {
			if lineLen+1+len(s) > maxWidth {
				b.WriteString("\n" + pad)
				lineLen = ind
			} else {
				b.WriteString(" ")
				lineLen++
			}
		}
		b.WriteString(s)
		lineLen += len(s)
	}
}

// simpleExpr matches what rustfmt calls a simple expression: a literal, or
// a one-segment path with field accesses, indexes, casts, & and unary
// operators.
var simpleExpr = regexp.MustCompile(`^[&*!-]*(?:(?:r#)?[A-Za-z_][A-Za-z0-9_]*|[0-9][0-9A-Za-z_.]*|b?'[^']*'|"(?:[^"\\]|\\.)*")(?:\.[A-Za-z_0-9]+|\[[A-Za-z0-9_]+(?: as usize)?\])*(?: as [a-z0-9]+)?$`)

func allSimple(ns []node) bool {
	for _, n := range ns {
		if c, ok := n.(*call); ok && longChain(c) || !simpleExpr.MatchString(n.flat()) {
			return false
		}
	}
	return true
}

// shortItems reports whether every element is short enough for rustfmt to
// put several on a line (short_array_element_width_threshold).
func shortItems(ns []node) bool {
	for _, n := range ns {
		if len(n.flat()) > 10 {
			return false
		}
	}
	return true
}

func (s *structLit) broken(ind int) string {
	var b strings.Builder
	inner := ind + indentWidth
	pad := strings.Repeat(" ", inner)
	b.WriteString(s.name + " {")
	for _, f := range s.fields {
		b.WriteString("\n" + pad)
		v := layWith(f.value, inner+len(f.name)+2, inner, 1)
		if v == f.name {
			b.WriteString(f.name)
		} else {
			b.WriteString(f.name + ": " + v)
		}
		b.WriteString(",")
	}
	if s.base != "" {
		b.WriteString("\n" + pad + ".." + s.base)
	}
	b.WriteString("\n" + strings.Repeat(" ", ind) + "}")
	return b.String()
}

// broken breaks a chain of binary operators before each operator.
func (b *binary) broken(col, ind, suffix int) string {
	var operands []node
	var ops []string
	var walk func(n node)
	walk = func(n node) {
		if x, ok := n.(*binary); ok && x.p == b.p && x.op == b.op {
			walk(x.l)
			ops = append(ops, x.op)
			operands = append(operands, x.r)
			return
		}
		operands = append(operands, n)
	}
	walk(b)
	inner := ind + indentWidth
	var out strings.Builder
	out.WriteString(layOperand(operands[0], b.p, col, ind, false))
	for i, op := range ops {
		out.WriteString("\n" + strings.Repeat(" ", inner) + op + " ")
		out.WriteString(layOperand(operands[i+1], b.p, inner+len(op)+1, inner, true))
	}
	return out.String()
}

func layOperand(n node, p, col, ind int, right bool) string {
	if (right && n.prec() <= p) || (!right && n.prec() < p) || unclear(p, n) {
		return "(" + lay(n, col+1, ind) + ")"
	}
	return lay(n, col, ind)
}

// longChain reports whether c is a chain rustfmt breaks over lines for
// its width alone.
func longChain(c *call) bool {
	_, links := chainOf(c)
	return len(links) >= 2 && len(c.flat()) > chainWidth // root included (measured)
}

// chainOf splits a method call chain, a.b(..).c(..), into its root, a,
// and its links, each a method call without its receiver.
func chainOf(c *call) (node, []*call) {
	var links []*call
	var n node = c
	for {
		x, ok := n.(*call)
		if !ok || x.recv == nil || !strings.HasPrefix(x.head, ".") {
			break
		}
		links = append([]*call{{head: x.head, open: x.open, close: x.close, args: x.args}}, links...)
		n = x.recv
	}
	return n, links
}

// chainBroken renders a chain over lines as rustfmt does: each link on its
// own line, indented, the first one kept on the root's line when the root
// is no wider than an indent.
func chainBroken(root node, links []*call, ind int) string {
	var b strings.Builder
	b.WriteString(paren(root, precPostfix))
	pad := "\n" + strings.Repeat(" ", ind+indentWidth)
	for i, l := range links {
		if i > 0 || len(root.flat()) > indentWidth {
			b.WriteString(pad)
		}
		b.WriteString(l.flat())
	}
	return b.String()
}
