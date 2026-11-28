package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// Maps (RFC 0019, step 1): a local variable of type map[K]V, made by make
// or a literal, with K a string, an integer or a bool and V a number, a
// bool or a string. It is a Rust rt::Map, a HashMap with a fixed hasher,
// owned by that variable: the variable is only indexed, read or written,
// passed to delete or len, or printed, so it is never copied.

// isMap reports whether t is a map type.
func isMap(t types.Type) bool {
	_, ok := t.Underlying().(*types.Map)
	return ok
}

// mapTypeProblem names what of map type t step 1 does not take, or is "".
func mapTypeProblem(t *types.Map) string {
	k, ok := t.Key().Underlying().(*types.Basic)
	switch {
	case !ok || k.Info()&(types.IsString|types.IsInteger|types.IsBoolean) == 0:
		// RFC 0019 decision 4: float keys are refused (Go's NaN is never
		// equal to itself).
		return fmt.Sprintf("a map with keys of type %s (step 1 of RFC 0019 takes strings, integers and bools)", t.Key())
	case !isCopyBasic(t.Elem()) && !isString(t.Elem()) && !copyStruct(t.Elem()):
		return fmt.Sprintf("a map with values of type %s (RFC 0019 takes numbers, bools, strings and structs of numbers and bools)", t.Elem())
	}
	return ""
}

// copyStruct reports whether t is a named struct of numbers and bools (or
// of such structs): a Copy value, which a map hands out as a copy.
func copyStruct(t types.Type) bool {
	_, named := types.Unalias(t).(*types.Named)
	_, isStruct := t.Underlying().(*types.Struct)
	return named && isStruct && copyKind(t)
}

// globalMap checks package-level variable v, of map type t, declared by
// spec s (RFC 0019, step 2): made by a map literal, then only read, which
// checkGlobals checks (a changed one is refused there).
func (c *checker) globalMap(s *ast.ValueSpec, n *ast.Ident, t *types.Map) {
	if what := mapTypeProblem(t); what != "" {
		c.unsupported(n, what)
		return
	}
	if len(s.Values) != len(s.Names) {
		c.unsupported(n, fmt.Sprintf("a package-level map variable %s not made by a map literal where it is declared (a nil map; step 2 of RFC 0019)", n.Name))
		return
	}
	for i, name := range s.Names {
		if name != n {
			continue
		}
		lit, ok := ast.Unparen(s.Values[i]).(*ast.CompositeLit)
		if !ok {
			c.unsupported(n, fmt.Sprintf("a package-level map variable %s not made by a map literal where it is declared (step 2 of RFC 0019)", n.Name))
			return
		}
		if c.mapInits == nil {
			c.mapInits = map[ast.Expr]bool{}
		}
		c.mapInits[lit] = true
	}
}

// localMap checks local variable v, of map type t, defined at n: made by
// make or a map literal, then used only as step 1 takes.
func (c *checker) localMap(n ast.Node, v *types.Var, t *types.Map) {
	if what := mapTypeProblem(t); what != "" {
		c.unsupported(n, what)
		return
	}
	if !c.mapMade[v] {
		c.unsupported(n, fmt.Sprintf("a map variable %s not made by make or a map literal where it is declared (a nil map; step 1 of RFC 0019)", v.Name()))
		return
	}
	if what := mapUseProblem(c.pkg.Info, v, c.body); what != "" {
		c.unsupported(n, what)
	}
}

// mapMakes records the local map variables of body that are made where
// they are declared: m := make(map[K]V), m := map[K]V{...} or var m = ....
func mapMakes(info *types.Info, body *ast.BlockStmt) (made map[*types.Var]bool, inits map[ast.Expr]bool) {
	made, inits = map[*types.Var]bool{}, map[ast.Expr]bool{}
	mark := func(lhs []*ast.Ident, rhs []ast.Expr) {
		if len(lhs) != len(rhs) {
			return
		}
		for i, id := range lhs {
			v, _ := info.Defs[id].(*types.Var)
			if v != nil && isMap(v.Type()) && makesMap(info, rhs[i]) {
				made[v] = true
				inits[ast.Unparen(rhs[i])] = true
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				var ids []*ast.Ident
				for _, l := range n.Lhs {
					ids = append(ids, l.(*ast.Ident))
				}
				mark(ids, n.Rhs)
			}
		case *ast.ValueSpec:
			mark(n.Names, n.Values)
		}
		return true
	})
	return made, inits
}

// mapInit checks that e, a map literal or make of a map, initializes a
// local map variable, whose type the Rust names.
func (c *checker) mapInit(e ast.Expr) bool {
	if !c.mapInits[e] {
		c.unsupported(e, "a map literal or make of a map other than the value of a local map variable where it is declared (step 1 of RFC 0019)")
		return false
	}
	return true
}

// makesMap reports whether e is make(map...) or a map literal.
func makesMap(info *types.Info, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.CompositeLit:
		return true
	case *ast.CallExpr:
		id, ok := ast.Unparen(e.Fun).(*ast.Ident)
		if !ok {
			return false
		}
		b, ok := info.Uses[id].(*types.Builtin)
		return ok && b.Name() == "make"
	}
	return false
}

// mapUseProblem checks each use of map variable v in body: the map of an
// index expression, or the map of delete or len, or an argument of a
// print. Anything else would copy it, which one owner does not allow.
func mapUseProblem(info *types.Info, v *types.Var, body *ast.BlockStmt) string {
	ok := map[*ast.Ident]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IndexExpr:
			if id, isID := ast.Unparen(n.X).(*ast.Ident); isID {
				ok[id] = true
			}
		case *ast.CallExpr:
			if mapCallArg(info, n) {
				for _, a := range n.Args {
					if id, isID := ast.Unparen(a).(*ast.Ident); isID {
						ok[id] = true
					}
				}
			}
		}
		return true
	})
	problem := ""
	ast.Inspect(body, func(n ast.Node) bool {
		if id, isID := n.(*ast.Ident); isID && info.Uses[id] == v && !ok[id] {
			problem = fmt.Sprintf("using the map %s other than by indexing it, passing it to delete or len, or printing it (it would be copied; step 1 of RFC 0019)", v.Name())
		}
		return problem == ""
	})
	return problem
}

// mapCallArg reports whether a map may be an argument of call: delete,
// len, or a print of fmt.
func mapCallArg(info *types.Info, call *ast.CallExpr) bool {
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		b, ok := info.Uses[f].(*types.Builtin)
		return ok && (b.Name() == "delete" || b.Name() == "len")
	case *ast.SelectorExpr:
		id, ok := f.X.(*ast.Ident)
		if !ok {
			return false
		}
		pn, ok := info.Uses[id].(*types.PkgName)
		if !ok || pn.Imported().Path() != "fmt" {
			return false
		}
		switch f.Sel.Name {
		case "Print", "Println", "Printf", "Sprint", "Sprintln", "Sprintf":
			return true
		}
	}
	return false
}

// mapUpdateProblem checks m[k] op= v or m[k]++ of a map element: step 1
// takes +=, -=, *=, ++ and -- of a number, which Rust's entry API writes
// as Go computes them.
func mapUpdateProblem(info *types.Info, target ast.Expr, tok token.Token) string {
	ix, ok := ast.Unparen(target).(*ast.IndexExpr)
	if !ok || !isMap(info.Types[ix.X].Type) {
		return ""
	}
	switch tok {
	case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.INC, token.DEC:
		if b, ok := info.Types[target].Type.Underlying().(*types.Basic); ok && b.Info()&types.IsNumeric != 0 && b.Info()&types.IsComplex == 0 {
			return ""
		}
	}
	return fmt.Sprintf("%s on an element of a map other than +=, -=, *=, ++ or -- of a number (step 1 of RFC 0019)", tok)
}
