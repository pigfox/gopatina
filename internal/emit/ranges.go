package emit

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"math/big"
	"sort"
	"strings"
)

// Proven integer ranges and loop specialization (RFC 0006).
//
// For a loop nest (an outermost for statement and everything in it), the
// emitter looks for induction variables whose values it can prove lie in
// [0, limit] and whose arithmetic cannot wrap, so that they can be Rust
// usize values, as a Rust programmer would write them, and indexes need no
// conversion. When the proof needs a bound on one loop-invariant variable,
// such as n <= 3037000499, the nest is written twice: the fast copy under
// that condition and the general copy, unchanged, otherwise.

// maxFastLines is the most lines a fast copy may have (RFC 0006, decided by
// the project lead on 2026-11-21: "Approve RFC 0006 with 60 and 1.").
const maxFastLines = 60

// A Note is a decision the emitter made, reported by -explain.
type Note struct {
	Pos  token.Position
	Text string
}

// nestPlan is how one loop nest is specialized.
type nestPlan struct {
	uvars   map[*types.Var]bool   // induction variables written as usize
	shadows map[*types.Var]string // loop bounds, as usize copies
	nonNeg  map[*types.Var]bool   // bounds proven >= 0
	order   []*types.Var          // the shadows, in source order
	cond    *types.Var            // the side condition's variable, or nil
	condMax *big.Int              // the side condition is cond <= condMax
	// raw holds every expression the fast copy writes as a usize: each
	// was checked not to exceed the limit, so it cannot wrap.
	raw map[ast.Expr]bool
}

// loopShape is one loop of a nest whose variable may become a usize.
type loopShape struct {
	v       *types.Var
	start   ast.Expr // the initial value
	bound   ast.Expr // the loop runs while v < bound (v <= bound if incl)
	incl    bool
	step    ast.Expr // the increment of a monotone loop; nil for v++
	counted bool     // a counted loop (for v in a..b); else a while loop
}

var maxInt64 = big.NewInt(math.MaxInt64)

// minCond is the smallest bound a side condition may allow: a fast copy for
// fewer iterations than that is not worth its code.
var minCond = big.NewInt(1 << 20)

// isGoInt reports whether t is Go's int, which GoPatina writes as i64.
func isGoInt(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.Int
}

// planNest returns the specialization of the nest at s, or nil.
func (g *gen) planNest(s *ast.ForStmt) *nestPlan {
	shapes := map[*types.Var]*loopShape{}
	var order []*types.Var
	ast.Inspect(s, func(n ast.Node) bool {
		if f, ok := n.(*ast.ForStmt); ok {
			if sh := g.loopShape(f); sh != nil {
				shapes[sh.v] = sh
				order = append(order, sh.v)
			}
		}
		return true
	})
	p := &nestPlan{uvars: map[*types.Var]bool{}, shadows: map[*types.Var]string{}, nonNeg: map[*types.Var]bool{}}
	for v := range shapes {
		p.uvars[v] = true
	}
	// Drop variables whose start, bound or step is not usize-shaped, until
	// nothing changes.
	for changed := true; changed; {
		changed = false
		for _, v := range order {
			sh := shapes[v]
			if !p.uvars[v] {
				continue
			}
			if !g.isUexpr(p, sh.start) || !g.boundOK(p, sh, s) || sh.step != nil && !g.isUexpr(p, sh.step) {
				delete(p.uvars, v)
				changed = true
			}
		}
	}
	// It is worth it only if some index becomes a usize.
	var raw []ast.Expr
	ast.Inspect(s, func(n ast.Node) bool {
		if ix, ok := n.(*ast.IndexExpr); ok && g.isUexpr(p, ix.Index) && g.mentionsUvar(p, ix.Index) {
			raw = append(raw, ix.Index)
		}
		return true
	})
	if len(raw) == 0 {
		return nil
	}
	// Comparisons of usize expressions are written as usize too, and so are
	// the loops' starts, steps and bounds.
	ast.Inspect(s, func(n ast.Node) bool {
		if b, ok := n.(*ast.BinaryExpr); ok && binOps[b.Op].prec == precCmp && isGoInt(g.typeOf(b.X)) {
			for _, x := range []ast.Expr{b.X, b.Y} {
				if g.isUexpr(p, x) {
					raw = append(raw, x)
				}
			}
		}
		return true
	})
	for _, v := range order {
		if sh := shapes[v]; p.uvars[v] {
			raw = append(raw, sh.start)
			if sh.step != nil {
				raw = append(raw, sh.step)
			}
			if g.isUexpr(p, sh.bound) {
				raw = append(raw, sh.bound)
			}
		}
	}
	// The bounds that are variables, with what is known of them.
	known := map[*types.Var]*big.Int{}
	var bounds []*types.Var
	for _, v := range order {
		if !p.uvars[v] {
			continue
		}
		if b := g.boundVar(p, shapes[v].bound); b != nil && known[b] == nil {
			nonNeg, max := g.makeFact(s, b)
			if max == nil {
				max = maxInt64
			}
			known[b] = max
			p.nonNeg[b] = nonNeg
			bounds = append(bounds, b)
		}
	}
	limit := maxInt64
	if g.pkg.Sizes.Sizeof(types.Typ[types.Uintptr]) == 4 {
		limit = big.NewInt(math.MaxUint32)
	}
	fits := func(vals map[*types.Var]*big.Int) bool {
		hi := func(e ast.Expr) *big.Int { return g.hiOf(p, shapes, vals, e) }
		ok := true
		check := func(e ast.Expr) {
			ast.Inspect(e, func(n ast.Node) bool {
				if x, isExpr := n.(ast.Expr); isExpr && ok && hi(x).Cmp(limit) > 0 {
					ok = false
				}
				return ok
			})
		}
		for _, e := range raw {
			check(e)
		}
		for _, v := range order {
			if !p.uvars[v] {
				continue
			}
			sh := shapes[v]
			hv := g.hiVar(p, shapes, vals, v)
			if hv.Cmp(limit) > 0 {
				return false
			}
			if sh.step != nil && new(big.Int).Add(hv, hi(sh.step)).Cmp(limit) > 0 {
				return false
			}
		}
		return ok
	}
	if !fits(known) {
		// One variable may get a side condition: the largest bound that
		// fits, found by bisection (fits only gets harder as it grows).
		for _, b := range bounds {
			vals := map[*types.Var]*big.Int{}
			for k, v := range known {
				vals[k] = v
			}
			lo, hi := big.NewInt(0), new(big.Int).Set(known[b])
			vals[b] = lo
			if !fits(vals) {
				continue
			}
			for lo.Cmp(hi) < 0 {
				mid := new(big.Int).Add(lo, hi)
				mid.Add(mid, big.NewInt(1)).Rsh(mid, 1)
				vals[b] = mid
				if fits(vals) {
					lo = mid
				} else {
					hi = mid.Sub(mid, big.NewInt(1))
				}
			}
			if lo.Cmp(minCond) >= 0 {
				p.cond, p.condMax = b, lo
				break
			}
		}
		if p.cond == nil {
			g.notes = append(g.notes, Note{Pos: g.pkg.Fset.Position(s.Pos()), Text: "loop not specialized: no single condition proves that its index arithmetic cannot wrap"})
			return nil
		}
	}
	for _, b := range bounds {
		p.order = append(p.order, b)
	}
	sort.Slice(p.order, func(i, j int) bool { return p.order[i].Pos() < p.order[j].Pos() })
	p.raw = map[ast.Expr]bool{}
	for _, e := range raw {
		p.raw[e] = true
	}
	return p
}

// loopShape returns the shape of f if its variable could become a usize.
func (g *gen) loopShape(f *ast.ForStmt) *loopShape {
	init, ok := f.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 1 || len(init.Rhs) != 1 {
		return nil
	}
	v := g.localVar(init.Lhs[0].(*ast.Ident))
	cond, ok := ast.Unparen(f.Cond).(*ast.BinaryExpr)
	if v == nil || !isGoInt(v.Type()) || g.addressTaken(v) || !ok || !isIdent(cond.X, v, g) {
		return nil
	}
	sh := &loopShape{v: v, start: init.Rhs[0], bound: cond.Y, incl: cond.Op == token.LEQ}
	if g.res.Counted[f] {
		sh.counted = true
		return sh
	}
	if cond.Op != token.LSS {
		return nil
	}
	switch post := f.Post.(type) {
	case *ast.IncDecStmt:
		if post.Tok != token.INC || !isIdent(post.X, v, g) {
			return nil
		}
	case *ast.AssignStmt:
		if post.Tok != token.ADD_ASSIGN || !isIdent(post.Lhs[0], v, g) {
			return nil
		}
		sh.step = post.Rhs[0]
	default:
		return nil
	}
	if g.writes(f.Body, v) {
		return nil
	}
	return sh
}

func isIdent(e ast.Expr, v *types.Var, g *gen) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && g.localVar(id) == v
}

// writes reports whether n assigns or increments v.
func (g *gen) writes(n ast.Node, v *types.Var) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for _, l := range s.Lhs {
				found = found || isIdent(l, v, g)
			}
		case *ast.IncDecStmt:
			found = found || isIdent(s.X, v, g)
		}
		return !found
	})
	return found
}

// addressTaken reports whether &v appears in the current function.
func (g *gen) addressTaken(v *types.Var) bool {
	found := false
	ast.Inspect(g.fnBody, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND && isIdent(u.X, v, g) {
			found = true
		}
		return !found
	})
	return found
}

// isUexpr reports whether e can be written as a usize: non-negative integer
// constants and usize variables, combined with + and *.
func (g *gen) isUexpr(p *nestPlan, e ast.Expr) bool {
	if tv := g.info.Types[e]; tv.Value != nil {
		return tv.Value.Kind() == constant.Int && constant.Sign(tv.Value) >= 0 && constant.Compare(tv.Value, token.LEQ, constant.MakeInt64(math.MaxInt64))
	}
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		v := g.localVar(x)
		return v != nil && p.uvars[v]
	case *ast.BinaryExpr:
		return (x.Op == token.ADD || x.Op == token.MUL) && g.isUexpr(p, x.X) && g.isUexpr(p, x.Y)
	}
	return false
}

// mentions reports whether e reads v.
func mentions(g *gen, e ast.Expr, v *types.Var) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && g.localVar(id) == v {
			found = true
		}
		return !found
	})
	return found
}

func (g *gen) mentionsUvar(p *nestPlan, e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && p.uvars[g.localVar(id)] {
			found = true
		}
		return !found
	})
	return found
}

// boundVar returns the variable a loop bound is, if it is a loop-invariant
// int variable that is not itself a usize variable.
func (g *gen) boundVar(p *nestPlan, e ast.Expr) *types.Var {
	if g.info.Types[e].Value != nil {
		return nil
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	v := g.localVar(id)
	if v == nil || p.uvars[v] || !isGoInt(v.Type()) || g.res.Mut[v] || g.addressTaken(v) {
		return nil
	}
	return v
}

// boundOK reports whether sh's bound can be compared as a usize.
func (g *gen) boundOK(p *nestPlan, sh *loopShape, s *ast.ForStmt) bool {
	if mentions(g, sh.bound, sh.v) {
		return false // the bound must come from outside the loop
	}
	if g.isUexpr(p, sh.bound) {
		return true
	}
	b := g.boundVar(p, sh.bound)
	if b == nil {
		return false
	}
	if sh.incl {
		// b.max(0) is exact for v < b with v >= 0, not for v <= b.
		nonNeg, _ := g.makeFact(s, b)
		return nonNeg
	}
	return true
}

// makeFact reports what make([]T, v) proves about v: if a statement of the
// function body, before the nest at s, is x := make([]T, v) (or x = ...),
// then once it has run, v >= 0 and v * size(T) <= Go's maxAlloc; GoPatina's
// makeslice check panics otherwise, as Go does. v never changes (boundVar).
func (g *gen) makeFact(s *ast.ForStmt, v *types.Var) (bool, *big.Int) {
	maxAlloc := new(big.Int).Lsh(big.NewInt(1), 48)
	if g.pkg.Sizes.Sizeof(types.Typ[types.Uintptr]) == 4 {
		maxAlloc = big.NewInt(math.MaxUint32)
	}
	for _, st := range g.fnBody.List {
		if st.Pos() >= s.Pos() {
			break
		}
		a, ok := st.(*ast.AssignStmt)
		if !ok || len(a.Rhs) != 1 {
			continue
		}
		c, ok := ast.Unparen(a.Rhs[0]).(*ast.CallExpr)
		if !ok || len(c.Args) != 2 {
			continue
		}
		if fn, ok := c.Fun.(*ast.Ident); !ok || fn.Name != "make" || g.info.Uses[fn] != types.Universe.Lookup("make") {
			continue
		}
		if !isIdent(c.Args[1], v, g) {
			continue
		}
		size := g.pkg.Sizes.Sizeof(g.info.Types[c.Args[0]].Type.Underlying().(*types.Slice).Elem())
		if size == 0 {
			return true, nil
		}
		return true, new(big.Int).Div(maxAlloc, big.NewInt(size))
	}
	return false, nil
}

// hiOf returns the largest value e can have in the nest; vals holds the
// largest value of each bound variable. e is a usize expression or a bound
// (planNest checked both), and a bound only reads variables of enclosing
// loops, so the recursion ends.
func (g *gen) hiOf(p *nestPlan, shapes map[*types.Var]*loopShape, vals map[*types.Var]*big.Int, e ast.Expr) *big.Int {
	if tv := g.info.Types[e]; tv.Value != nil {
		n, _ := new(big.Int).SetString(tv.Value.ExactString(), 10)
		return n
	}
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		v := g.localVar(x)
		if p.uvars[v] {
			return g.hiVar(p, shapes, vals, v)
		}
		return vals[v]
	default:
		b := x.(*ast.BinaryExpr)
		l, r := g.hiOf(p, shapes, vals, b.X), g.hiOf(p, shapes, vals, b.Y)
		if b.Op == token.ADD {
			return new(big.Int).Add(l, r)
		}
		return new(big.Int).Mul(l, r)
	}
}

// hiVar returns the largest value usize variable v has inside its loop.
func (g *gen) hiVar(p *nestPlan, shapes map[*types.Var]*loopShape, vals map[*types.Var]*big.Int, v *types.Var) *big.Int {
	sh := shapes[v]
	b := g.hiOf(p, shapes, vals, sh.bound)
	if sh.incl {
		return b
	}
	return new(big.Int).Sub(b, big.NewInt(1))
}

// specialize writes the nest at s with its fast copy, if it fits.
func (g *gen) specialize(s *ast.ForStmt, p *nestPlan) bool {
	var names []string
	for _, v := range sortedVars(p.uvars) {
		names = append(names, g.varName(v))
	}
	pos := g.pkg.Fset.Position(s.Pos())
	var b strings.Builder
	out, ind, last := g.out, g.ind, g.lastLine
	used := make(map[*ast.CommentGroup]bool, len(g.usedComments))
	for k, v := range g.usedComments {
		used[k] = v
	}
	g.out = &b
	g.fast = p
	if p.cond != nil || len(p.order) > 0 {
		g.ind++
	}
	quiet := g.quiet
	g.quiet = p.cond != nil // the general copy keeps the comments
	for _, v := range p.order {
		name := g.lnames.name(v.Name()+"_usize", snake)
		p.shadows[v] = name
		if p.nonNeg[v] {
			g.line("let " + name + " = " + g.varName(v) + " as usize;")
		} else {
			g.line("let " + name + " = " + g.varName(v) + ".max(0) as usize;")
		}
	}
	g.forBody(s)
	g.fast, g.quiet = nil, quiet
	g.out, g.ind, g.lastLine = out, ind, last
	fast := b.String()
	lines := strings.Count(fast, "\n")
	if lines > maxFastLines {
		g.usedComments = used
		g.notes = append(g.notes, Note{Pos: pos, Text: "loop not specialized: its fast copy would be " + itoa(lines) + " lines, over the limit of " + itoa(maxFastLines)})
		return false
	}
	g.specialized = true
	what := "usize for " + strings.Join(names, ", ") + " (proven non-negative; the arithmetic cannot wrap)"
	switch {
	case p.cond != nil:
		cond := g.varName(p.cond) + " <= " + p.condMax.String()
		g.notes = append(g.notes, Note{Pos: pos, Text: "loop specialized for " + cond + ": " + what})
		g.notes = append(g.notes, Note{Pos: pos, Text: "loop kept general otherwise: indexes keep Go's checks"})
		g.line("if " + cond + " {")
		g.out.WriteString(fast)
		g.line("} else {")
		g.ind++
		g.noPlan = true
		g.forBody(s)
		g.noPlan = false
		g.ind--
		g.line("}")
	case len(p.order) > 0:
		g.notes = append(g.notes, Note{Pos: pos, Text: "loop specialized: " + what})
		g.line("{")
		g.out.WriteString(fast)
		g.line("}")
	default:
		g.notes = append(g.notes, Note{Pos: pos, Text: "loop specialized: " + what})
		g.out.WriteString(fast)
	}
	return true
}

func sortedVars(m map[*types.Var]bool) []*types.Var {
	var vs []*types.Var
	for v := range m {
		vs = append(vs, v)
	}
	sort.Slice(vs, func(i, j int) bool { return vs[i].Pos() < vs[j].Pos() })
	return vs
}

// uNode renders a usize expression (isUexpr).
func (g *gen) uNode(e ast.Expr) node {
	if tv := g.info.Types[e]; tv.Value != nil {
		return text(tv.Value.ExactString())
	}
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return text(g.varName(g.localVar(x)))
	default:
		b := x.(*ast.BinaryExpr)
		op := binOps[b.Op]
		return &binary{op: op.op, l: g.uNode(b.X), r: g.uNode(b.Y), p: op.prec}
	}
}

// uTyped renders a usize expression whose type Rust must know on its own:
// a constant gets the usize suffix.
func (g *gen) uTyped(e ast.Expr) node {
	if g.info.Types[e].Value != nil {
		return text(g.info.Types[e].Value.ExactString() + "_usize")
	}
	return g.uNode(e)
}

// uOperand renders a comparison operand in the fast copy: a usize
// expression, or a bound through its usize copy. shadow reports the
// latter, and exact whether the copy equals the bound (it is clamped at 0).
func (g *gen) uOperand(e ast.Expr) (n node, ok, exact bool) {
	if g.fast.raw[e] {
		return g.uNode(e), true, true
	}
	if id, isID := ast.Unparen(e).(*ast.Ident); isID {
		if v := g.localVar(id); v != nil && g.fast.shadows[v] != "" {
			return text(g.fast.shadows[v]), true, g.fast.nonNeg[v]
		}
	}
	return nil, false, false
}

// fastCompare renders x op y on usize values in the fast copy, when that is
// exact: both sides are usize expressions or bounds proven >= 0, or a
// clamped bound b is used as x < b or b > x with x >= 0.
func (g *gen) fastCompare(e *ast.BinaryExpr) (node, bool) {
	if g.fast == nil || !isGoInt(g.typeOf(e.X)) {
		return nil, false
	}
	l, lok, lexact := g.uOperand(e.X)
	r, rok, rexact := g.uOperand(e.Y)
	if !lok || !rok {
		return nil, false
	}
	if !lexact && !(e.Op == token.GTR && rexact) || !rexact && !(e.Op == token.LSS && lexact) {
		return nil, false
	}
	op := binOps[e.Op]
	return &binary{op: op.op, l: l, r: r, p: op.prec, nonAssoc: true}, true
}

// fastVar reports whether v is a usize variable of the fast copy.
func (g *gen) fastVar(v *types.Var) bool {
	return g.fast != nil && g.fast.uvars[v]
}

// usizeAsI64 renders usize variable v where Go uses its int value.
func (g *gen) usizeAsI64(v *types.Var) node {
	return atom{g.varName(v) + " as i64", precCast}
}
