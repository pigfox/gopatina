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
	funcOf := map[*ast.BinaryExpr]*ast.FuncDecl{}
	for _, file := range pkg.Files {
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				ast.Inspect(fd, func(n ast.Node) bool {
					if b, ok := n.(*ast.BinaryExpr); ok {
						funcOf[b] = fd
					}
					if s, ok := n.(*ast.AssignStmt); ok && pkg.OpAssigns[s] != nil {
						funcOf[pkg.OpAssigns[s]] = fd
					}
					return true
				})
			}
		}
	}
	pkg.Fusions = map[*ast.BinaryExpr]fma.Fusion{}
	pkg.Kept = map[*ast.BinaryExpr]bool{}
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
		default:
			c.fusing = funcOf[e]
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
		tries = []fma.Fusion{{Mul: c.operandMul(e.Y), Addend: e.X, Form: fma.Add}, {Mul: c.operandMul(e.X), Addend: e.Y, Form: fma.Add}}
	case e.Op == token.ADD:
		tries = []fma.Fusion{{Mul: c.operandMul(e.X), Addend: e.Y, Form: fma.Add}, {Mul: c.operandMul(e.Y), Addend: e.X, Form: fma.Add}}
	case arm: // token.SUB
		tries = []fma.Fusion{{Mul: c.operandMul(e.Y), Addend: e.X, Form: fma.SubMul}, {Mul: c.operandMul(e.X), Addend: e.Y, Form: fma.MulSub}}
	}
	for _, f := range tries {
		if f.Mul != nil {
			return f, true
		}
	}
	return fma.Fusion{}, false
}

// operandMul returns the multiply gc fuses for operand e of a + or -: e
// itself, or the multiply a local holds, assigned it once (v := x*y),
// whose value gc's SSA carries to the + across statements. Its operands
// are then kept as they were (load.Package.Kept), since the program may
// change them before the + runs.
func (c *checker) operandMul(e ast.Expr) *ast.BinaryExpr {
	if m := c.mulOf(e); m != nil {
		return m
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	v, ok := c.pkg.Info.Uses[id].(*types.Var)
	if !ok || v.Parent() == nil || v.Parent() == v.Pkg().Scope() || c.fusing == nil {
		return nil
	}
	m := c.onlyMul(c.fusing, v)
	if m != nil {
		c.pkg.Kept[m] = true
	}
	return m
}

// onlyMul returns the multiply local v is set to, when the function fd
// sets it exactly once, by v := x*y or var v = x*y, and never takes its
// address or changes it otherwise.
func (c *checker) onlyMul(fd *ast.FuncDecl, v *types.Var) *ast.BinaryExpr {
	info := c.pkg.Info
	is := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		return ok && (info.Defs[id] == v || info.Uses[id] == v)
	}
	var def ast.Expr
	var defStmt ast.Node
	sets := 0
	inBlock := map[ast.Node]bool{} // statements of a block, not an if's or a for's own
	ast.Inspect(fd, func(n ast.Node) bool {
		switch b := n.(type) {
		case *ast.BlockStmt:
			for _, s := range b.List {
				inBlock[s] = true
			}
		case *ast.CaseClause:
			for _, s := range b.Body {
				inBlock[s] = true
			}
		}
		return true
	})
	ast.Inspect(fd, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.DeclStmt:
			if gd, ok := s.Decl.(*ast.GenDecl); ok {
				for _, spec := range gd.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range vs.Names {
							if is(name) {
								defStmt = s
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			for i, l := range s.Lhs {
				if is(l) {
					sets++
					if s.Tok == token.DEFINE && len(s.Lhs) == len(s.Rhs) {
						def, defStmt = s.Rhs[i], s
					}
				}
			}
		case *ast.ValueSpec:
			for i, name := range s.Names {
				if is(name) {
					sets++
					if len(s.Values) == len(s.Names) {
						def = s.Values[i]
					}
				}
			}
		case *ast.IncDecStmt:
			if is(s.X) {
				sets++
			}
		case *ast.RangeStmt:
			if (s.Key != nil && is(s.Key)) || (s.Value != nil && is(s.Value)) {
				sets++
			}
		case *ast.UnaryExpr:
			if s.Op == token.AND && is(s.X) {
				sets += 2 // its address: it may change through it
			}
		case *ast.FuncLit:
			sets += 2 // a closure may change what it captures
		}
		return true
	})
	if sets != 1 || def == nil || !inBlock[defStmt] {
		return nil
	}
	return c.mulOf(def)
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
			if s, ok := n.(*ast.AssignStmt); ok {
				if e := c.opAssign(s); e != nil {
					f(e)
				}
				return true
			}
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

// opAssign returns float x += y or x -= y at s as the binary expression
// x + y or x - y, at the operator, where gc reports a fusion of it (issue
// #223); the emitter then writes x = the fused expression. It is recorded
// in pkg.OpAssigns. A target with a call in it is left out: written twice,
// its call would run twice.
func (c *checker) opAssign(s *ast.AssignStmt) *ast.BinaryExpr {
	op := map[token.Token]token.Token{token.ADD_ASSIGN: token.ADD, token.SUB_ASSIGN: token.SUB}[s.Tok]
	if op == token.ILLEGAL {
		return nil // Go's op= has one target and one value
	}
	b, ok := c.pkg.Info.Types[s.Lhs[0]].Type.Underlying().(*types.Basic)
	if !ok || b.Info()&types.IsFloat == 0 || hasCall(s.Lhs[0]) {
		return nil
	}
	e := &ast.BinaryExpr{X: s.Lhs[0], OpPos: s.TokPos, Op: op, Y: s.Rhs[0]}
	if c.pkg.OpAssigns == nil {
		c.pkg.OpAssigns = map[*ast.AssignStmt]*ast.BinaryExpr{}
	}
	c.pkg.OpAssigns[s] = e
	return e
}

// hasCall reports whether e calls a function.
func hasCall(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		_, isCall := n.(*ast.CallExpr)
		found = found || isCall
		return !found
	})
	return found
}
