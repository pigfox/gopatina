package subset

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"

	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/fma"
)

// checkFusion matches the float operations gc fuses for the target
// (pkg.Fused, from package fma) to the program's source, and records in
// pkg.Fusions how the emitter writes each: x.mul_add(y, a) and its
// negated forms, giving gc's single rounding. A site it cannot match
// exactly is refused, never written unfused; when gc's report is not
// available on a target that fuses (pkg.FusedErr), every float + and - is
// refused, since gc may fuse any of them.
func (c *checker) checkFusion() {
	pkg := c.pkg
	if pkg.FusedErr != nil {
		c.eachFloatAddSub(func(e *ast.BinaryExpr) {
			c.unsupported(e, fmt.Sprintf("a float %s, which gc may fuse with a multiply for this target, while %v,", e.Op, pkg.FusedErr))
		})
		return
	}
	if pkg.Fused == nil {
		return // the target never fuses
	}
	ops := map[fma.Pos]*ast.BinaryExpr{}
	c.eachFloatAddSub(func(e *ast.BinaryExpr) { ops[c.fmaPos(e.OpPos)] = e })
	pkg.Fusions = map[*ast.BinaryExpr]fma.Fusion{}
	for _, s := range pkg.Fused.List {
		if !c.emitted(s.Pos) {
			continue // in a package GoPatina provides, or code the program does not reach
		}
		e := ops[s.Pos]
		where := c.fmaTarget()
		switch {
		case e == nil:
			c.unsupportedAt(s.Pos, fmt.Sprintf("an operation gc fuses into a float multiply-add here for %s, which is not a float + or -,", where))
		case len(s.Inlined) > 0:
			c.unsupported(e, fmt.Sprintf("a float multiply-add gc fuses for %s where it inlines this function (at %s)", where, s.Inlined[0]))
		case c.pkg.Info.Types[e].Type.Underlying().(*types.Basic).Kind() == types.Float32:
			c.unsupported(e, fmt.Sprintf("a float32 multiply-add gc fuses for %s", where))
		default:
			f, ok := c.fusionOf(e)
			if !ok {
				c.unsupported(e, fmt.Sprintf("a float %s that gc fuses with a multiply for %s, whose multiply is not one of its operands,", e.Op, where))
				continue
			}
			pkg.Fusions[e] = f
		}
	}
}

// fusionOf returns how gc fuses e, a float + or - it reports fused: which
// operand is the multiply, by the order its rules try them (ARM64.rules:
// FADD a (FMUL x y) with a the left operand first, then the right; FSUB
// a (FMUL x y) before FSUB (FMUL x y) a; AMD64.rules: ADDSD (MULSD x y) z,
// the left operand first, and no subtraction).
func (c *checker) fusionOf(e *ast.BinaryExpr) (fma.Fusion, bool) {
	arm := c.pkg.Fused.Target.GOARCH == "arm64"
	var tries []fma.Fusion
	switch {
	case e.Op == token.ADD && arm:
		tries = []fma.Fusion{{Mul: c.mulOf(e.Y), Addend: e.X, Form: fma.Add}, {Mul: c.mulOf(e.X), Addend: e.Y, Form: fma.Add}}
	case e.Op == token.ADD:
		tries = []fma.Fusion{{Mul: c.mulOf(e.X), Addend: e.Y, Form: fma.Add}, {Mul: c.mulOf(e.Y), Addend: e.X, Form: fma.Add}}
	case arm: // token.SUB
		tries = []fma.Fusion{{Mul: c.mulOf(e.Y), Addend: e.X, Form: fma.SubMul}, {Mul: c.mulOf(e.X), Addend: e.Y, Form: fma.MulSub}}
	}
	for _, f := range tries {
		if f.Mul != nil {
			return f, true
		}
	}
	return fma.Fusion{}, false
}

// mulOf returns e, through parentheses, when it is a float multiply that
// gc keeps a multiply: not one it folds (both operands constant) or
// rewrites (x*2 is x+x, x*-1 is -x, x*1 is x), and not behind an explicit
// conversion, which rounds and so keeps it from fusing.
func (c *checker) mulOf(e ast.Expr) *ast.BinaryExpr {
	m, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || m.Op != token.MUL || c.pkg.Info.Types[m].Value != nil {
		return nil
	}
	for _, x := range []ast.Expr{m.X, m.Y} {
		v := c.pkg.Info.Types[x].Value
		if v == nil {
			continue
		}
		for _, k := range []int64{2, 1, -1} {
			if constant.Compare(constant.ToFloat(v), token.EQL, constant.MakeInt64(k)) {
				return nil // generic.rules: x*2 is x+x, x*1 is x, x*-1 is -x
			}
		}
	}
	return m
}

// eachFloatAddSub calls f for each float + and - of the program's code
// that is not a constant.
func (c *checker) eachFloatAddSub(f func(*ast.BinaryExpr)) {
	for _, file := range c.pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			e, ok := n.(*ast.BinaryExpr)
			if !ok || (e.Op != token.ADD && e.Op != token.SUB) || c.pkg.Info.Types[e].Value != nil {
				return true
			}
			if b, ok := c.pkg.Info.Types[e].Type.Underlying().(*types.Basic); ok && b.Info()&types.IsFloat != 0 {
				f(e)
			}
			return true
		})
	}
}

// fmaPos is p as package fma writes positions.
func (c *checker) fmaPos(p token.Pos) fma.Pos {
	q := c.pkg.Fset.Position(p)
	return fma.Pos{File: filepath.Clean(q.Filename), Line: q.Line, Col: q.Column}
}

// emitted reports whether p is in a declaration GoPatina writes: one of
// the program's translated files, inside a declaration it keeps.
func (c *checker) emitted(p fma.Pos) bool {
	for _, file := range c.pkg.Files {
		for _, d := range file.Decls {
			start, end := c.pkg.Fset.Position(d.Pos()), c.pkg.Fset.Position(d.End())
			if filepath.Clean(start.Filename) != p.File {
				break
			}
			if (start.Line < p.Line || start.Line == p.Line && start.Column <= p.Col) &&
				(p.Line < end.Line || p.Line == end.Line && p.Col < end.Column) {
				return true
			}
		}
	}
	return false
}

// fmaTarget names the target gc fused for.
func (c *checker) fmaTarget() string { return c.pkg.Fused.Target.String() }

// unsupportedAt reports, at p, a construct patina mode does not support.
func (c *checker) unsupportedAt(p fma.Pos, what string) {
	d := diag.Errorf(token.Position{Filename: p.File, Line: p.Line, Column: p.Col}, "%s is not supported in patina mode yet", what)
	c.diags = append(c.diags, d.With(seeCompat))
}
