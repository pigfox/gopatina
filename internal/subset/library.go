package subset

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/pigfox/gopatina/internal/load"
)

// Library reports whether pkg is translated as a library crate (RFC 0014):
// any package other than main.
func Library(pkg *load.Package) bool { return pkg.Name != "main" }

// checkLibrary reports what RFC 0014 refuses in a library: init functions
// and package-level variables a library has no start-up to initialize, and
// an API that would expose GoPatina's own types.
func (c *checker) checkLibrary() {
	info := c.pkg.Info
	kinds := Globals(c.pkg)
	for _, f := range c.pkg.Files {
		root := c.pkg.PkgOf[f] == c.pkg.Types
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name == "init" {
					c.unsupported(d.Name, "an init function in a library (a library has no start-up to run it)")
					continue
				}
				if root && d.Name.IsExported() {
					c.apiFunc(d)
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.ValueSpec:
						if d.Tok != token.VAR {
							continue
						}
						for _, name := range s.Names {
							v, ok := info.Defs[name].(*types.Var)
							if !ok || name.Name == "_" {
								continue
							}
							c.libraryVar(name, v, kinds[v], root)
						}
					case *ast.TypeSpec:
						if root && s.Name.IsExported() {
							c.apiStruct(s)
						}
					}
				}
			}
		}
	}
}

// libraryVar checks a package-level variable of a library: read-only,
// initialized without calling the program, which a lazy static does on its
// first use; and, exported from the library, only a sentinel error.
func (c *checker) libraryVar(name *ast.Ident, v *types.Var, kind GlobalKind, root bool) {
	switch {
	case kind != GlobalStatic:
		c.unsupported(name, "a package-level variable that a library changes (a library has no start-up to initialize it)")
		return
	case root && name.IsExported() && !isError(v.Type()):
		c.unsupported(name, "an exported package-level variable in a library, other than a sentinel error")
		return
	}
	for _, ini := range c.pkg.Info.InitOrder {
		if ini.Lhs[0] == v && len(HoistableCalls(c.pkg, ini.Rhs)) > 0 {
			c.unsupported(name, "a package-level variable of a library initialized by a call of the program (a library has no start-up to make it)")
		}
	}
}

// apiFunc checks an exported function or method of a library's API.
func (c *checker) apiFunc(d *ast.FuncDecl) {
	fn := c.pkg.Info.Defs[d.Name].(*types.Func)
	sig := fn.Signature()
	if r := sig.Recv(); r != nil {
		if !recvExported(r.Type()) {
			return // a method of an unexported type is not in the API
		}
		if what := c.apiTypeProblem(r.Type()); what != "" {
			c.unsupported(d.Name, what+" as the receiver of an exported method of a library")
			return
		}
	}
	for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
		for i := 0; i < tup.Len(); i++ {
			t := tup.At(i).Type()
			if isError(t) && tup == sig.Params() {
				// Only a result's error crosses the API, as Result's Error.
				c.unsupported(d.Name, "an error parameter in the API of a library")
				return
			}
			if what := c.apiTypeProblem(t); what != "" {
				c.unsupported(d.Name, what+" in the API of a library")
				return
			}
		}
	}
}

// apiStruct checks the exported fields of an exported struct type of a
// library's API.
func (c *checker) apiStruct(s *ast.TypeSpec) {
	st, ok := c.pkg.Info.TypeOf(s.Type).Underlying().(*types.Struct)
	if !ok {
		return
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		what := c.apiTypeProblem(f.Type())
		if isError(f.Type()) {
			what = "an error value"
		}
		if what != "" {
			c.unsupported(s.Name, what+" in an exported field of a library's exported struct")
			return
		}
	}
}

// apiTypeProblem says why a value of type t cannot cross a library's API
// (RFC 0014), or returns "".
func (c *checker) apiTypeProblem(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return c.apiTypeProblem(u.Elem())
	case *types.Pointer:
		if Ptrs(c.pkg).HeapType(t) {
			return "a pointer that escapes"
		}
		return c.apiTypeProblem(u.Elem())
	}
	if IsIface(t) && !isError(t) {
		return "an interface type"
	}
	return ""
}

// recvExported reports whether a method's receiver type is exported.
func recvExported(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	return types.Unalias(t).(*types.Named).Obj().Exported()
}
