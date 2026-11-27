package emit

import (
	"go/ast"
	"go/constant"
	"go/token"

	"github.com/pigfox/gopatina/internal/fma"
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
		if g.pkg.NaNExact[e] {
			helper := "sqrt_nan" + g.f32Suffix(e)
			g.rt[helper] = true
			return &call{head: "rt::" + helper, open: "(", close: ")", args: []node{g.value(a[0])}}
		}
		return method("sqrt")
	case "math.Abs":
		return method("abs")
	case "math.Exp":
		g.rt["exp"] = true
		g.rt[g.fmaUsable()] = true
		return &call{head: "rt::exp", open: "(", close: ")", args: []node{g.value(a[0])}}
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

// fused renders a float + or - that gc fuses with its multiply operand
// for the target (subset.checkFusion), as gc's instruction computes it
// (rt::fma_arm64, rt::fma_amd64): one rounding, and its NaN rules.
// FMSUB and FNMSUB negate an operand first, which is exact.
func (g *gen) fused(f fma.Fusion) node {
	x, y, a := g.value(f.Mul.X), g.value(f.Mul.Y), g.value(f.Addend)
	neg := func(n node) node { return atom{"-" + paren(n, precUnary), precUnary} }
	suffix := ""
	if g.rustType(g.typeOf(f.Mul)) == "f32" {
		suffix = "_f32"
	}
	if g.pkg.Fused.Target.GOARCH != "arm64" {
		// VFMADD231SD z, x, y: additions only.
		g.rt["fma_amd64"+suffix] = true
		return &call{head: "rt::fma_amd64" + suffix, open: "(", close: ")", args: []node{x, y, a}}
	}
	// gc writes FMADD (FMULD x y) as Ra = a, Rm = x, Rn = y
	// (cmd/compile/internal/arm64/ssa.go), and Arm computes Ra + Rn*Rm,
	// FMSUB negating Rn and FNMSUB Ra: the NaN rules see the operands in
	// that order and sign.
	switch f.Form {
	case fma.SubMul: // a - x*y = a + (-y)*x
		y = neg(y)
	case fma.MulSub: // x*y - a = (-a) + y*x
		a = neg(a)
	}
	g.rt["fma_arm64"+suffix] = true
	return &call{head: "rt::fma_arm64" + suffix, open: "(", close: ")", args: []node{y, x, a}}
}

// nanOp renders x op y, a float + - * / whose NaN a bit-level read may see
// (subset.checkNaNBits), with gc's NaN on the target (rt::add_nan, ...).
func (g *gen) nanOp(op token.Token, x, y ast.Expr) node {
	helper := map[token.Token]string{token.ADD: "add_nan", token.SUB: "sub_nan", token.MUL: "mul_nan", token.QUO: "div_nan"}[op] + g.f32Suffix(x)
	g.rt[helper] = true
	return &call{head: "rt::" + helper, open: "(", close: ")", args: []node{g.receiverValue(x), g.receiverValue(y)}}
}

// f32Suffix is "_f32" when e is a float32, the suffix of its helpers.
func (g *gen) f32Suffix(e ast.Expr) string {
	if g.rustType(g.typeOf(e)) == "f32" {
		return "_f32"
	}
	return ""
}

// fmaUsable names the runtime item that says whether Go's math uses FMA
// on amd64: GODEBUG may turn it off below GOAMD64=v3, not from v3 on.
func (g *gen) fmaUsable() string {
	if (fma.Target{GOARCH: "amd64", GOAMD64: g.pkg.Target.GOAMD64}).Fuses() {
		return "fma_usable_v3"
	}
	return "fma_usable"
}
