package subset

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/pigfox/gopatina/internal/load"
)

// A GlobalKind is how a package-level variable is held in Rust (RFC 0010).
type GlobalKind int

const (
	// GlobalStatic is a variable nothing changes after initialization: a
	// static LazyLock, forced in Go's initialization order.
	GlobalStatic GlobalKind = iota
	// GlobalCell is a changed variable of a copied kind: a thread-local
	// Cell.
	GlobalCell
	// GlobalString is a changed string: a thread-local RefCell<String>.
	GlobalString
	// GlobalChangedShared is a changed variable of another kind (a slice,
	// a struct holding one), which patina mode does not support yet.
	GlobalChangedShared
)

// Global returns the package-level variable obj is, or nil.
func Global(obj types.Object) *types.Var {
	v, ok := obj.(*types.Var)
	if !ok || v.IsField() || v.Pkg() == nil || v.Parent() != v.Pkg().Scope() {
		return nil
	}
	return v
}

// Globals classifies every package-level variable of the program in pkg by
// what the program does with it after initialization.
func Globals(pkg *load.Package) map[*types.Var]GlobalKind {
	changed := map[*types.Var]bool{}
	root := func(e ast.Expr) *types.Var {
		for {
			switch x := ast.Unparen(e).(type) {
			case *ast.SelectorExpr:
				if g := Global(pkg.Info.Uses[x.Sel]); g != nil {
					return g // pkg.V
				}
				e = x.X
			case *ast.IndexExpr:
				e = x.X
			case *ast.StarExpr:
				e = x.X
			case *ast.Ident:
				return Global(pkg.Info.Uses[x])
			default:
				return nil
			}
		}
	}
	for _, f := range pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if n.Tok != token.DEFINE {
					for _, l := range n.Lhs {
						if g := root(l); g != nil {
							changed[g] = true
						}
					}
				}
			case *ast.IncDecStmt:
				if g := root(n.X); g != nil {
					changed[g] = true
				}
			case *ast.UnaryExpr:
				if n.Op == token.AND {
					if g := root(n.X); g != nil {
						changed[g] = true
					}
				}
			case *ast.SelectorExpr:
				// A pointer-receiver method called on the variable.
				if sel := pkg.Info.Selections[n]; sel != nil && sel.Kind() == types.MethodVal {
					if _, ptr := sel.Obj().(*types.Func).Signature().Recv().Type().(*types.Pointer); ptr && !isPointer(pkg.Info.Types[n.X].Type) {
						if g := root(n.X); g != nil {
							changed[g] = true
						}
					}
				}
			}
			return true
		})
	}
	kinds := map[*types.Var]GlobalKind{}
	for _, p := range pkg.Pkgs {
		scope := p.Scope()
		for _, name := range scope.Names() {
			v, ok := scope.Lookup(name).(*types.Var)
			if !ok {
				continue
			}
			switch {
			case !changed[v]:
				kinds[v] = GlobalStatic
			case copyKind(v.Type()):
				kinds[v] = GlobalCell
			case isString(v.Type()):
				kinds[v] = GlobalString
			default:
				kinds[v] = GlobalChangedShared
			}
		}
	}
	return kinds
}

// copyKind reports whether values of t are copied in Rust (borrow.Copy):
// numbers, booleans, and structs of them.
func copyKind(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Info()&types.IsString == 0
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if !copyKind(u.Field(i).Type()) {
				return false
			}
		}
		return true
	}
	return false
}

// globalSpec checks a package-level var spec (RFC 0010).
func (c *checker) globalSpec(s *ast.ValueSpec) {
	if len(s.Values) == 1 && len(s.Names) > 1 {
		c.unsupported(s, "a package-level variable spec with a multi-value initializer")
		return
	}
	for _, n := range s.Names {
		v := c.pkg.Info.Defs[n].(*types.Var) // _ too: go/types defines it
		switch t := v.Type(); {
		case isWriter(t):
			c.unsupported(n, "a package-level variable of type io.Writer")
		case holdsIface(t) && !IsIface(t):
			// An interface value itself is a thread-local (RFC 0012) until
			// goroutines choose the shareable form (RFC 0002); inside a
			// struct or slice it is not supported yet.
			c.unsupported(n, "a package-level variable holding an interface value in a struct or slice")
		case !isPointer(t) && holdsPointer(t, map[types.Type]bool{}):
			// Heap pointers are reference-counted cells, which a static
			// cannot hold (RFC 0013); a pointer variable itself is
			// rejected by typ.
			c.unsupported(n, "a package-level variable holding pointers in a struct or slice")
		default:
			c.typ(n, t, false)
		}
	}
	c.exprs(s.Values)
}

// checkGlobals checks what the program does with its package-level
// variables (RFC 0010): a changed slice, or a struct holding one, is not
// supported yet; nor is a variable's address; a slice nothing changes may
// only be read; and a loop condition may not both read a changed variable
// and call a function of the program, since Go calls the function first.
func (c *checker) checkGlobals() {
	kinds := Globals(c.pkg)
	for _, f := range c.pkg.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, s := range gd.Specs {
				for _, n := range s.(*ast.ValueSpec).Names {
					if v, ok := c.pkg.Info.Defs[n].(*types.Var); ok && kinds[v] == GlobalChangedShared {
						c.unsupported(n, fmt.Sprintf("a package-level variable of type %s that the program changes", v.Type()))
					}
				}
			}
		}
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			var parent ast.Node
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, n)
			switch n := n.(type) {
			case *ast.UnaryExpr:
				if n.Op == token.AND && c.globalAt(n.X) != nil {
					c.unsupported(n, "taking the address of a package-level variable")
				}
			case *ast.SelectorExpr:
				if sel := c.pkg.Info.Selections[n]; sel != nil && sel.Kind() == types.MethodVal {
					if _, ptr := sel.Obj().(*types.Func).Signature().Recv().Type().(*types.Pointer); ptr && c.globalAt(n.X) != nil {
						c.unsupported(n, "calling a pointer-receiver method on a package-level variable")
					}
				}
			case *ast.ForStmt:
				c.conditions(kinds, n.Cond, n.Post)
			case *ast.IfStmt:
				c.conditions(kinds, n.Cond)
			case *ast.SwitchStmt:
				c.conditions(kinds, n.Tag)
			case *ast.CaseClause:
				for _, e := range n.List {
					c.conditions(kinds, e)
				}
			}
			if g := c.globalUse(n); g != nil && kinds[g] == GlobalStatic && !copyKind(g.Type()) && !isString(g.Type()) && !IsIface(g.Type()) && !c.readUse(n, parent) {
				c.unsupported(n, fmt.Sprintf("using the package-level variable %s this way (a copy of a slice shares its elements)", g.Name()))
			}
			return true
		})
	}
}

// conditions reports each condition (of an if, a switch, a case, a loop
// or its post statement) that reads a changed package-level variable and
// calls a function of the program: Go calls the function first, which
// patina mode writes for simple statements only (it would have to move the
// call out of the condition, which Go may not evaluate).
func (c *checker) conditions(kinds map[*types.Var]GlobalKind, ns ...ast.Node) {
	for _, n := range ns {
		if n != nil && c.ReadsChangedGlobal(n, kinds) && c.hasHoistable(n) {
			c.unsupported(n, "a condition that reads a changed package-level variable and calls a function of the program")
		}
	}
}

// globalUse returns the variable n names, when n is a use of a
// package-level variable: v, or pkg.V.
func (c *checker) globalUse(n ast.Node) *types.Var {
	switch n := n.(type) {
	case *ast.Ident:
		if v := Global(c.pkg.Info.Uses[n]); v != nil {
			return v
		}
	case *ast.SelectorExpr:
		if id, ok := n.X.(*ast.Ident); ok {
			if _, pkg := c.pkg.Info.Uses[id].(*types.PkgName); pkg {
				return Global(c.pkg.Info.Uses[n.Sel])
			}
		}
	}
	return nil
}

// globalAt returns the package-level variable that e is, or is a field or
// element of.
func (c *checker) globalAt(e ast.Expr) *types.Var {
	for {
		e = ast.Unparen(e)
		if g := c.globalUse(e); g != nil {
			return g
		}
		switch x := e.(type) {
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// readUse reports whether use n, with parent parent, only reads a slice
// variable: indexes it, takes its length, ranges over it, reads a field,
// prints it, or passes it to a function of the program (which the borrow
// checker requires to only read it).
func (c *checker) readUse(n, parent ast.Node) bool {
	if sel, ok := parent.(*ast.SelectorExpr); ok && sel.Sel == n {
		return true // the V of pkg.V: its parent's use is checked
	}
	switch p := parent.(type) {
	case *ast.IndexExpr:
		return p.X == n
	case *ast.RangeStmt:
		return p.X == n
	case *ast.SelectorExpr:
		return p.X == n
	case *ast.CallExpr:
		if id, ok := ast.Unparen(p.Fun).(*ast.Ident); ok {
			if b, ok := c.pkg.Info.Uses[id].(*types.Builtin); ok {
				return b.Name() == "len"
			}
		}
		return !c.pkg.Info.Types[p.Fun].IsType()
	}
	return false
}

// ReadsChangedGlobal reports whether n reads a package-level variable that
// the program changes (a Cell or a String, RFC 0010).
func (c *checker) ReadsChangedGlobal(n ast.Node, kinds map[*types.Var]GlobalKind) bool {
	return ReadsChangedGlobal(c.pkg, n, kinds)
}

// ReadsChangedGlobal reports whether n reads a package-level variable of
// kind GlobalCell or GlobalString.
func ReadsChangedGlobal(pkg *load.Package, n ast.Node, kinds map[*types.Var]GlobalKind) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if _, ok := x.(*ast.FuncLit); ok {
			return false // runs when it is called, not here (RFC 0018)
		}
		if id, ok := x.(*ast.Ident); ok {
			if g := Global(pkg.Info.Uses[id]); g != nil && (kinds[g] == GlobalCell || kinds[g] == GlobalString) {
				found = true
			}
		}
		return !found
	})
	return found
}

func (c *checker) hasHoistable(n ast.Node) bool { return len(HoistableCalls(c.pkg, n)) > 0 }

// HoistableCalls returns the calls in n that Go makes before it reads the
// variables of the same expression (gc evaluates every call of an
// expression first, in order, measured): calls of functions and methods of
// the program, innermost first, left to right, outside the right operand of
// && and ||, which Go evaluates only when needed, and outside function
// literals.
func HoistableCalls(pkg *load.Package, n ast.Node) []*ast.CallExpr {
	var out []*ast.CallExpr
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		ast.Inspect(n, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.FuncLit:
				return false
			case *ast.BinaryExpr:
				if x.Op == token.LAND || x.Op == token.LOR {
					walk(x.X)
					return false
				}
			case *ast.CallExpr:
				for _, a := range x.Args {
					walk(a)
				}
				if sel, ok := ast.Unparen(x.Fun).(*ast.SelectorExpr); ok {
					walk(sel.X)
				}
				if programCall(pkg, x) {
					out = append(out, x)
				}
				return false
			}
			return true
		})
	}
	walk(n)
	return out
}

// programCall reports whether call calls a function or method translated
// from the program's source.
func programCall(pkg *load.Package, call *ast.CallExpr) bool {
	var id *ast.Ident
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return false
	}
	fn, ok := pkg.Info.Uses[id].(*types.Func)
	return ok && fn.Pkg() != nil && pkg.Local[fn.Pkg()] && !load.IsIntrinsic(fn)
}

// holdsPointer reports whether a value of type t holds a pointer: in a
// slice or array element or a struct field, at any depth.
func holdsPointer(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		return true
	case *types.Slice:
		return holdsPointer(u.Elem(), seen)
	case *types.Array:
		return holdsPointer(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if holdsPointer(u.Field(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}
