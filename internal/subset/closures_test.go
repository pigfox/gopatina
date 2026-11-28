package subset

import (
	"go/ast"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
)

// A closure's body runs when it is called: the calls and the reads of
// package variables inside it are not the statement's own. The checker
// keeps both out of closures in step 1 of RFC 0018, so this is checked on
// a source it would refuse.
func TestClosureBodyIsNotTheStatement(t *testing.T) {
	src := "package main\n\nvar n int\n\nfunc f() int { n++; return n }\n\nfunc main() {\n\tg := func() int { return f() + n }\n\t_ = g\n}\n"
	pkg, err := load.Source("/p", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	var stmt ast.Stmt
	for _, d := range pkg.Files[0].Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "main" {
			stmt = fd.Body.List[0]
		}
	}
	if calls := HoistableCalls(pkg, stmt); len(calls) != 0 {
		t.Errorf("HoistableCalls found %d calls inside the closure", len(calls))
	}
	if ReadsChangedGlobal(pkg, stmt, Globals(pkg)) {
		t.Error("ReadsChangedGlobal counted a read inside the closure")
	}
}
