package emit

import (
	"go/ast"
	"strings"
)

// intrinsic renders a call of math/bits function name, one of
// load.Intrinsics, as the Rust that gives gc's result (RFC 0009).
func (g *gen) intrinsic(e *ast.CallExpr, name string) node {
	arg := func(i int) node { return g.value(e.Args[i]) }
	// A constant receiver of a method needs its type: 1_u64.rotate_left.
	method := func(m string, args ...node) node {
		return &call{recv: atom{paren(g.receiverValue(e.Args[0]), precPostfix), precPostfix}, head: "." + m, open: "(", close: ")", args: args}
	}
	asInt := func(n node) node { return atom{paren(n, precCast) + " as i64", precCast} }
	width := strings.TrimLeft(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	if width == "" {
		width = "64" // uint is 64 bits on every target
	}
	switch strings.TrimRight(name, "0123456789") {
	case "Add", "Sub", "Mul":
		// Two results, as a tuple; the carry rule is in the runtime.
		helper := strings.ToLower(strings.TrimRight(name, "0123456789")) + "64"
		g.rt[helper] = true
		args := []node{arg(0), arg(1)}
		if helper != "mul64" {
			args = append(args, arg(2))
		}
		return &call{head: "rt::" + helper, open: "(", close: ")", args: args}
	case "RotateLeft":
		// k mod the width; a negative k rotates right, as Go defines it.
		k := atom{paren(g.receiverValue(e.Args[1]), precPostfix) + ".rem_euclid(" + width + ") as u32", precCast}
		return method("rotate_left", k)
	case "LeadingZeros":
		return asInt(method("leading_zeros"))
	case "TrailingZeros":
		return asInt(method("trailing_zeros"))
	case "OnesCount":
		return asInt(method("count_ones"))
	case "Len":
		return atom{"(" + width + " - " + method("leading_zeros").flat() + ") as i64", precCast}
	}
	return method("swap_bytes") // ReverseBytes
}
