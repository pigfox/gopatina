package borrow

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
)

func suffixArg(call *ast.CallExpr) *ast.SliceExpr {
	if len(call.Args) != 1 {
		return nil
	}
	s, _ := ast.Unparen(call.Args[0]).(*ast.SliceExpr)
	return s
}

func (a *analyzer) checkSuffixes() {
	for call, fn := range a.suffixes {
		s := suffixArg(call)
		if !a.suffixReader(fn, 0, map[*Param]bool{}) {
			a.report(s.Pos(), a.errorf(s.Pos(), "%s cannot borrow this slice suffix: its slice parameter is not proven read-only", fn.Obj.Name()).With(
				"a suffix may only be read or forwarded to another proven reader; pass the base and the offset separately otherwise"))
			continue
		}
		a.res.Suffixes[call] = s
		a.fix(s.Pos(), fmt.Sprintf("borrowed %s as a read-only slice suffix: %s only reads it", exprString(s), fn.Obj.Name()))
	}
}

// suffixReader checks the fixed-point borrow mode and audits uses of a
// parameter. Follow all forwarding edges, including multiple parameters
// of one function; a cycle is safe only if every reachable use is safe.
func (a *analyzer) suffixReader(fn *Func, index int, seen map[*Param]bool) bool {
	p := fn.Params[index]
	if seen[p] {
		return true
	}
	if p.Mode != Ref {
		return false
	}
	seen[p] = true
	valid := true
	ast.PreorderStack(fn.Decl.Body, nil, func(n ast.Node, stack []ast.Node) bool {
		if !valid {
			return false
		}
		id, ok := n.(*ast.Ident)
		if ok && a.pkg.Info.Uses[id] == p.Var {
			valid = a.suffixUse(id, stack, seen)
		}
		return valid
	})
	return valid
}

// usage removes enclosing parentheses and returns the immediate context.
func usage(e ast.Expr, stack []ast.Node) (ast.Expr, ast.Node, []ast.Node) {
	for i := len(stack) - 1; ; i-- {
		if p, ok := stack[i].(*ast.ParenExpr); ok {
			e = p
			continue
		}
		return e, stack[i], stack[:i]
	}
}

func (a *analyzer) suffixUse(id *ast.Ident, stack []ast.Node, seen map[*Param]bool) bool {
	e, parent, rest := usage(id, stack)
	switch parent := parent.(type) {
	case *ast.IndexExpr:
		return parent.X == e // writes were rejected by the borrow mode
	case *ast.RangeStmt:
		return parent.X == e
	case *ast.SliceExpr:
		// Only a suffix supplied to a direct call survives the subset, and
		// this []int identifier cannot be its low bound.
		arg, use, _ := usage(parent, rest)
		return a.suffixCall(use.(*ast.CallExpr), arg, seen)
	default:
		call, ok := parent.(*ast.CallExpr)
		return ok && a.suffixCall(call, e, seen)
	}
}

func (a *analyzer) suffixCall(call *ast.CallExpr, arg ast.Expr, seen map[*Param]bool) bool {
	info := a.pkg.Info
	fun := ast.Unparen(call.Fun)
	// A slice passed through a conversion is not a permitted use.
	if info.Types[fun].IsType() {
		return false
	}
	if f, ok := fun.(*ast.Ident); ok {
		if builtin, ok := info.Uses[f].(*types.Builtin); ok {
			return builtin.Name() == "len"
		}
		fn := a.funcs[info.Uses[f].(*types.Func)]
		return a.suffixReader(fn, slices.Index(call.Args, arg), seen)
	}
	sel := fun.(*ast.SelectorExpr)
	return info.Uses[sel.Sel].(*types.Func).Pkg().Path() == "fmt"
}
