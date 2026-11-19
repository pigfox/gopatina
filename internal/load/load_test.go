package load

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDirSelectsPatinaFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"go.mod":         "module example.com/app\n\ngo 1.25\n",
		"main.go":        "package main\n\nimport \"fmt\"\n\nfunc main() { hello(); fmt.Println() }\n",
		"patina.go":      "//go:build patina\n\npackage main\n\nfunc hello() {}\n",
		"plain.go":       "//go:build !patina\n\npackage main\n\nfunc hello() { panic(\"plain\") }\n",
		"main_test.go":   "package main\n",
		"sub/ignored.go": "package sub\n",
	})
	pkg, err := Dir(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Name != "main" || pkg.GoVersion != "go1.25" || len(pkg.Files) != 2 {
		t.Errorf("got package %q, go %q, %d files", pkg.Name, pkg.GoVersion, len(pkg.Files))
	}
	if got := filepath.Base(pkg.Fset.Position(pkg.Files[1].Pos()).Filename); got != "patina.go" {
		t.Errorf("second file = %s, want patina.go", got)
	}
}

func TestDirErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Dir(dir, dir); err == nil || !strings.Contains(err.Error(), "no Go files in .") {
		t.Errorf("empty dir: %v", err)
	}
	if _, err := Dir(filepath.Join(dir, "missing"), dir); err == nil {
		t.Error("missing dir: no error")
	}

	syntax := t.TempDir()
	write(t, syntax, map[string]string{"main.go": "package main\nfunc main() {\n"})
	if _, err := Dir(syntax, syntax); err == nil || !strings.Contains(err.Error(), "./main.go:") {
		t.Errorf("syntax error: %v", err)
	}

	typeErr := t.TempDir()
	write(t, typeErr, map[string]string{"main.go": "package main\nfunc main() { var x int = \"s\" }\n"})
	if _, err := Dir(typeErr, typeErr); err == nil || !strings.Contains(err.Error(), "cannot use") {
		t.Errorf("type error: %v", err)
	}

	cgo := t.TempDir()
	write(t, cgo, map[string]string{"main.go": "package main\nimport \"C\"\nfunc main() {}\n"})
	if _, err := Dir(cgo, cgo); err == nil || !strings.Contains(err.Error(), "cgo") {
		t.Errorf("cgo: %v", err)
	}

	unreadable := t.TempDir()
	write(t, unreadable, map[string]string{"main.go": "package main\nfunc main() {}\n"})
	os.Chmod(filepath.Join(unreadable, "main.go"), 0)
	if _, err := os.ReadFile(filepath.Join(unreadable, "main.go")); err == nil {
		t.Log("running as root: the file stays readable")
	} else if _, err := Dir(unreadable, unreadable); err == nil {
		t.Error("unreadable file: no error")
	}

	badMod := t.TempDir()
	write(t, badMod, map[string]string{"main.go": "package main\nfunc main() {}\n"})
	os.Mkdir(filepath.Join(badMod, "go.mod"), 0o755)
	if _, err := Dir(badMod, badMod); err == nil {
		t.Error("go.mod that is a directory: no error")
	}
}

func TestGoDirective(t *testing.T) {
	if v := goDirective([]byte("module x\n\ngo 1.26.1\n")); v != "go1.26.1" {
		t.Errorf("goDirective = %q", v)
	}
	if v := goDirective([]byte("module x\n")); v != "" {
		t.Errorf("goDirective = %q", v)
	}
}

func TestNoModule(t *testing.T) {
	// The root directory has no go.mod above it.
	if v, err := moduleGoVersion(string(filepath.Separator)); err != nil || v != "" {
		t.Errorf("moduleGoVersion(/) = %q, %v", v, err)
	}
}

func TestSource(t *testing.T) {
	pkg, err := Source("/x", "main.go", "package main\nfunc main() {}\n", "go1.25")
	if err != nil || pkg.Name != "main" {
		t.Fatalf("Source: %v", err)
	}
}

// The type checker must use the target's int width, not the host's: a
// suffix bound that fits an amd64 int can overflow a 386 int before the
// subset or borrow checker ever sees it.
func TestTargetSizes(t *testing.T) {
	const src = `package main
func read([]int) {}
func main() { s := []int{1}; read(s[2147483648:]) }`
	for _, arch := range []string{"386", "amd64", "", "no-such-arch"} {
		t.Run(arch, func(t *testing.T) {
			t.Setenv("GOARCH", arch)
			sizes := targetSizes()
			if sizes == nil || sizes.Sizeof(types.Typ[types.Int]) == 0 {
				t.Fatalf("targetSizes(%q) = %v", arch, sizes)
			}
			_, err := Source("/x", "main.go", src, "go1.25")
			wantErr := sizes.Sizeof(types.Typ[types.Int]) < 8
			if gotErr := err != nil; gotErr != wantErr {
				t.Fatalf("GOARCH=%q: load error = %v, want error %t", arch, err, wantErr)
			}
		})
	}
}

func TestSliceSuffixTypes(t *testing.T) {
	const src = `package main
func read([]int) {}
func main() { s := []int{1}; const start byte = 1; read(s[start:]) }`
	pkg, err := Source("/x", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for n := range ast.Preorder(pkg.Files[0]) {
		if s, ok := n.(*ast.SliceExpr); ok {
			found = true
			if s.High != nil || !types.Identical(pkg.Info.TypeOf(s.X), types.NewSlice(types.Typ[types.Int])) ||
				!types.Identical(pkg.Info.TypeOf(s.Low), types.Typ[types.Uint8]) ||
				!constant.Compare(pkg.Info.Types[s.Low].Value, token.EQL, constant.MakeInt64(1)) {
				t.Fatalf("unexpected suffix type information: %+v", s)
			}
			break
		}
	}
	if !found {
		t.Fatal("loader did not retain the suffix expression")
	}
	bad := strings.Replace(src, "s[start:]", "s[-1:]", 1)
	if _, err := Source("/x", "main.go", bad, "go1.25"); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("negative constant bound: %v", err)
	}
}

func TestReadAll(t *testing.T) {
	if _, _, err := sources(t.TempDir(), []string{"missing.go"}); err == nil {
		t.Error("sources of a missing file succeeded")
	}
}

func TestRelativeDir(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"app/main.go": "package main\nfunc main() {}\n"})
	if _, err := Dir("app", dir); err != nil {
		t.Error(err)
	}
}
