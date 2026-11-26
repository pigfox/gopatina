package emit

import (
	"go/ast"
	"go/constant"
)

// mathCall renders a call of a provided math function (subset.StdFuncs).
// Each is the Rust that gives Go's result bit for bit, NaNs included: the
// same IEEE operation (Sqrt, the same instruction on amd64 and arm64;
// Floor, Ceil and Trunc, with Go's quiet NaN) or the same bits (Abs,
// Copysign, Signbit, Float64bits).
func (g *gen) mathCall(e *ast.CallExpr, name string) node {
	a := e.Args
	// A constant receiver needs its type: 2.0_f64.sqrt().
	method := func(m string, args ...node) node {
		return &call{recv: atom{paren(g.receiverValue(a[0]), precPostfix), precPostfix}, head: "." + m, open: "(", close: ")", args: args}
	}
	switch name {
	case "math.Sqrt":
		return method("sqrt")
	case "math.Abs":
		return method("abs")
	case "math.Floor", "math.Ceil", "math.Trunc":
		// Rust's floor leaves a signaling NaN signaling on amd64; Go's
		// result is quiet (rt::floor).
		helper := map[string]string{"math.Floor": "floor", "math.Ceil": "ceil", "math.Trunc": "trunc"}[name]
		g.rt[helper] = true
		return &call{head: "rt::" + helper, open: "(", close: ")", args: []node{g.value(a[0])}}
	case "math.Copysign":
		return method("copysign", g.value(a[1]))
	case "math.Signbit":
		return method("is_sign_negative")
	case "math.IsNaN":
		return method("is_nan")
	case "math.Float64bits":
		return method("to_bits")
	case "math.Float64frombits":
		return &call{head: "f64::from_bits", open: "(", close: ")", args: []node{g.value(a[0])}}
	case "math.NaN":
		// Go's NaN, not Rust's f64::NAN: their payloads differ.
		return text("f64::from_bits(0x7ff8_0000_0000_0001)")
	case "math.Inf":
		if s, ok := g.constSign(a[0]); ok {
			if s < 0 {
				return text("f64::NEG_INFINITY")
			}
			return text("f64::INFINITY")
		}
		g.rt["inf"] = true
		return &call{head: "rt::inf", open: "(", close: ")", args: []node{g.value(a[0])}}
	}
	// math.IsInf
	if s, ok := g.constSign(a[1]); ok {
		switch {
		case s > 0:
			return atom{paren(g.value(a[0]), precCmp+1) + " == f64::INFINITY", precCmp}
		case s < 0:
			return atom{paren(g.value(a[0]), precCmp+1) + " == f64::NEG_INFINITY", precCmp}
		}
		return method("is_infinite")
	}
	g.rt["is_inf"] = true
	return &call{head: "rt::is_inf", open: "(", close: ")", args: []node{g.value(a[0]), g.value(a[1])}}
}

// constSign returns the sign of e, an int argument, when it is constant.
func (g *gen) constSign(e ast.Expr) (int, bool) {
	tv := g.info.Types[e]
	if tv.Value == nil {
		return 0, false
	}
	return constant.Sign(tv.Value), true
}
