package emit

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

var update = flag.Bool("update", false, "rewrite the golden crates under testdata/golden/*/rust")

// goldenTarget is the target every golden crate is emitted for, whatever
// machine the tests run on.
const goldenTarget = "x86_64-unknown-linux-gnu"

func root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// programs returns the test programs under testdata/<kind>.
func programs(t testing.TB, kind string) []string {
	base := filepath.Join(root(), "testdata", kind)
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(base, e.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs
}

// emitDir runs the whole pipeline on the package in dir.
func emitDir(t testing.TB, dir string) *Crate {
	t.Helper()
	pkg, err := load.Dir(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if ds := subset.Check(pkg); len(ds) > 0 {
		t.Fatalf("subset: %v", ds)
	}
	res, ds := borrow.Check(pkg)
	if len(ds) > 0 {
		t.Fatalf("borrow: %v", ds)
	}
	return Emit(pkg, res, Options{Target: goldenTarget, Version: "devel"})
}

// TestGolden checks that every program under testdata/golden emits exactly
// the crate in its rust directory. Run go test ./internal/emit -update to
// rewrite them after a deliberate change, and review the diff.
func TestGolden(t *testing.T) {
	for _, dir := range programs(t, "golden") {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			c := emitDir(t, dir)
			want := filepath.Join(dir, "rust")
			if *update {
				os.RemoveAll(want)
				if err := c.Write(want); err != nil {
					t.Fatal(err)
				}
			}
			files := append(c.Files, File{ManifestName, c.Manifest()})
			for _, f := range files {
				got, err := os.ReadFile(filepath.Join(want, filepath.FromSlash(f.Path)))
				if err != nil {
					t.Fatalf("%v (run go test ./internal/emit -update)", err)
				}
				if !bytes.Equal(got, f.Data) {
					t.Errorf("%s differs from the golden file; run go test ./internal/emit -update and review the diff\n--- got:\n%s", f.Path, f.Data)
				}
			}
		})
	}
}

// TestDeterministic emits every golden program twice, in two different
// directories, and requires identical manifests (docs/REPRODUCIBLE.md).
func TestDeterministic(t *testing.T) {
	for _, dir := range programs(t, "golden") {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "a-longer-directory")
			for _, out := range []string{a, b} {
				if err := emitDir(t, dir).Write(out); err != nil {
					t.Fatal(err)
				}
			}
			ma, _ := os.ReadFile(filepath.Join(a, ManifestName))
			mb, _ := os.ReadFile(filepath.Join(b, ManifestName))
			if len(ma) == 0 || !bytes.Equal(ma, mb) {
				t.Errorf("manifests differ:\n%s\n%s", ma, mb)
			}
		})
	}
}

// TestExamples checks the examples documented in EXAMPLES.md: they are the
// test programs testdata/golden/hello and testdata/reject/ownership, and
// examples/hello/rust is exactly the crate gopatina emits for hello.
func TestExamples(t *testing.T) {
	for example, copy := range map[string]string{
		"hello":     filepath.Join("golden", "hello"),
		"ownership": filepath.Join("reject", "ownership"),
	} {
		a, err := os.ReadFile(filepath.Join(root(), "examples", example, "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(root(), "testdata", copy, "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("examples/%s/main.go differs from testdata/%s/main.go; keep them the same", example, copy)
		}
	}
	c := emitDir(t, filepath.Join(root(), "examples", "hello"))
	crate := filepath.Join(root(), "examples", "hello", "rust")
	if *update {
		os.RemoveAll(crate)
		if err := c.Write(crate); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range append(c.Files, File{ManifestName, c.Manifest()}) {
		got, err := os.ReadFile(filepath.Join(crate, filepath.FromSlash(f.Path)))
		if err != nil || !bytes.Equal(got, f.Data) {
			t.Errorf("examples/hello/rust/%s is not what gopatina emits (run go test ./internal/emit -update): %v", f.Path, err)
		}
	}
}
