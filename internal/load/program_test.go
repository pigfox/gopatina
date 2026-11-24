package load

import (
	"errors"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree writes files (path relative to dir: contents) under dir.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProgram(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"go.mod":  "module example.com/p\n\ngo 1.25\n",
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/p/a\"\n\t\"example.com/p/c\"\n)\n\nfunc main() { fmt.Println(a.F(), c.K) }\n",
		"a/a.go":  "package a\n\nimport (\n\t\"math/bits\"\n\n\t\"example.com/p/z\"\n)\n\ntype T int\n\nfunc (t T) String() string { return \"t\" }\n\nfunc (t *T) Error() string { return \"e\" }\n\nfunc (t T) Other() int { return 1 }\n\nvar V = 1\n\nconst C, D = 1, 2\n\nfunc F() T {\n\tx := T(z.G() + bits.OnesCount(1))\n\t_ = x.Other()\n\treturn x\n}\n\nfunc Unused() {}\n\nfunc init() {}\n",
		"c/c.go":  "package c\n\nimport \"example.com/p/z\"\n\nconst K = z.N\n",
		"z/z.go":  "package z\n\nimport \"fmt\"\n\nconst N = 3\n\nfunc G() int { return len(fmt.Sprint(2)) }\n\nvar Acts = G()\n",
	})
	pkg, err := Dir(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, p := range pkg.Pkgs {
		paths = append(paths, p.Path())
	}
	// Go's order (the Go 1.21 rule, checked against go run): z, then c
	// (it only needs z), then math/bits, then a.
	if got := strings.Join(paths, " "); got != "example.com/p/z example.com/p/c math/bits example.com/p/a main" {
		t.Errorf("initialization order %s", got)
	}
	var decls []string
	for _, f := range pkg.Files {
		if len(f.Imports) > 0 && pkg.PkgOf[f].Path() != "main" {
			t.Errorf("%s keeps its imports", pkg.PkgOf[f].Path())
		}
		for _, d := range f.Decls {
			// Relative to the directory go list reports, which resolves
			// symbolic links (macOS's temporary directories are one).
			pos := pkg.Fset.Position(d.Pos())
			rel, _ := filepath.Rel(pkg.Dir, pos.Filename)
			decls = append(decls, fmt.Sprintf("%s:%d:%d", rel, pos.Line, pos.Column))
		}
	}
	// math/bits keeps nothing (OnesCount is an intrinsic); z keeps N and G,
	// and Acts, which nothing uses but whose initializer calls G;
	// a keeps T, String and Error (fmt may call them), Other (F calls it),
	// F and init, not V, C, D or Unused; c keeps K.
	got := strings.Join(decls, " ")
	want := "z/z.go:5:1 z/z.go:7:1 z/z.go:9:1 c/c.go:5:1 a/a.go:9:1 a/a.go:11:1 a/a.go:13:1 a/a.go:15:1 a/a.go:21:1 a/a.go:29:1 main.go:3:1 main.go:10:1"
	if got != want {
		t.Errorf("declarations %s, want %s", got, want)
	}
	if !pkg.Local[pkg.Types] || len(pkg.Local) != 5 {
		t.Errorf("Local = %v", pkg.Local)
	}
}

func TestGoList(t *testing.T) {
	if _, err := goList(t.TempDir()); err == nil || !strings.Contains(err.Error(), "go list") {
		t.Errorf("go list in an empty directory: %v", err)
	}
}

// fakeList replaces go list with out for the test.
func fakeList(t *testing.T, out string, err error) {
	old := goList
	t.Cleanup(func() { goList = old })
	goList = func(string) ([]byte, error) { return []byte(out), err }
}

func TestProgramErrors(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"main.go":    "package main\n\nimport \"x\"\n\nfunc main() { x.F() }\n",
		"x/x.go":     "package x\n\nfunc F() {}\n",
		"bad/bad.go": "package bad\n\nfunc F( {}\n",
		"typ/typ.go": "package typ\n\nfunc F() { undefined() }\n",
	})
	main := `{"ImportPath":"m","Dir":"` + dir + `","Name":"main","GoFiles":["main.go"],"Imports":["x","fmt","unsafe","missing"],"ImportMap":{"x":"vendor/x"},"Module":{"GoVersion":"1.25"}}`
	x := func(sub, file string) string {
		return `{"ImportPath":"vendor/x","Dir":"` + filepath.Join(dir, sub) + `","Name":"x","GoFiles":["` + file + `"]}`
	}
	for _, tc := range []struct {
		name, out string
		err       error
		want      string
	}{
		{"list fails", "", errors.New("go list: boom"), "go list: boom"},
		{"bad json", "{", nil, "go list: unexpected EOF"},
		{"listed error", `{"ImportPath":"m","Error":{"Err":"no such package"}}`, nil, "no such package"},
		{"missing file", x("x", "gone.go") + main, nil, "gone.go"},
		{"syntax error", x("bad", "bad.go") + main, nil, "expected"},
		{"type error", x("typ", "typ.go") + main, nil, "undefined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeList(t, tc.out, tc.err)
			if _, err := program(dir, dir); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v, want %q", err, tc.want)
			}
		})
	}
	t.Run("vendored", func(t *testing.T) {
		fakeList(t, x("x", "x.go")+main, nil)
		pkg, err := program(dir, dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(pkg.Pkgs) != 2 || pkg.Pkgs[0].Path() != "vendor/x" || pkg.GoVersion != "go1.25" {
			t.Errorf("packages %v, version %q", pkg.Pkgs, pkg.GoVersion)
		}
	})
}

func TestChain(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"go.mod":  "module example.com/p\n\ngo 1.25\n",
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/p/a\"\n)\n\ntype W struct{ U a.U }\n\nfunc main() { fmt.Println(a.F(), W{}, a.K) }\n",
		"a/a.go":  "package a\n\ntype T int\n\nfunc (t T) String() string { return g() }\n\nfunc g() string { return \"t\" }\n\ntype U struct{}\n\nconst K = 1\n\nfunc F() T { return 1 }\n\nfunc init() {}\n",
	})
	pkg, err := Dir(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	chains := map[string]string{}
	for _, f := range pkg.Files {
		if pkg.PkgOf[f] == pkg.Types {
			continue
		}
		for _, d := range f.Decls {
			var n ast.Node = d
			if gd, ok := d.(*ast.GenDecl); ok {
				n = gd.Specs[0]
			}
			chains[declName(pkg, n)] = Chain(pkg, n)
		}
	}
	for name, want := range map[string]string{
		"a.F":        "main.main -> a.F",
		"a.T":        "main.main -> a.F -> a.T",
		"a.T.String": "main.main -> a.F -> a.T -> a.T.String",
		"a.g":        "main.main -> a.F -> a.T -> a.T.String -> a.g",
		"a.U":        "main -> a.U",
		"a.K":        "main.main -> a.K",
		"a.init":     "",
	} {
		if got := chains[name]; got != want {
			t.Errorf("Chain(%s) = %q, want %q", name, got, want)
		}
	}
}

func TestProvided(t *testing.T) {
	for path, want := range map[string]bool{
		"fmt": true, "sync": true, "internal/runtime/atomic": true, "runtime/debug": true,
		"internal/bytealg": true, "strconv": false, "unicode/utf8": false, "example.com/m": false,
	} {
		if Provided(path) != want {
			t.Errorf("Provided(%q) = %v", path, !want)
		}
	}
}

// TestProgramInterfaceMethods checks that a reached type keeps the methods
// a call through an interface may reach (RFC 0012): those named like a
// method of an interface the program declares, and no others.
func TestProgramInterfaceMethods(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"go.mod":  "module example.com/p\n\ngo 1.25\n",
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/p/a\"\n)\n\ntype I interface{ Area() int }\n\nfunc main() {\n\tvar i I = a.T(2)\n\tfmt.Println(i.Area())\n}\n",
		"a/a.go":  "package a\n\ntype T int\n\nfunc (t T) Area() int { return int(t) }\n\nfunc (t T) Other() int { return 1 }\n",
	})
	pkg, err := Dir(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, f := range pkg.Files {
		if pkg.PkgOf[f].Path() != "example.com/p/a" {
			continue
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				kept = append(kept, fd.Name.Name)
			}
		}
	}
	if got := strings.Join(kept, " "); got != "Area" {
		t.Errorf("a keeps methods %q, want Area", got)
	}
}
