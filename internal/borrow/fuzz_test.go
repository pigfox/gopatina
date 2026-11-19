package borrow

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

// FuzzBorrow runs the whole front end on arbitrary Go source. The checker
// must report diagnostics or accept a program; it must never panic.
func FuzzBorrow(f *testing.F) {
	seedPrograms(f)
	f.Fuzz(func(t *testing.T, src string) {
		pkg, err := load.Source("/fuzz", "main.go", src, "go1.25")
		if err != nil {
			return
		}
		if ds := subset.Check(pkg); len(ds) > 0 {
			return
		}
		_, _ = Check(pkg)
	})
}

// seedPrograms adds every fixture under testdata/golden and testdata/reject
// to a fuzz target's corpus.
func seedPrograms(f *testing.F) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	for _, kind := range []string{"golden", "reject"} {
		dir := filepath.Join(root, "testdata", kind)
		entries, err := os.ReadDir(dir)
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			src, err := os.ReadFile(filepath.Join(dir, e.Name(), "main.go"))
			if err != nil {
				f.Fatal(err)
			}
			f.Add(string(src))
		}
	}
}
