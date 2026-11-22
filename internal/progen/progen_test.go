package progen

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/emit"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

// TestBatchesAreAccepted checks that generated batches are valid Go inside
// the patina subset: they type-check, pass the subset and borrow checks,
// and emit a crate. Building and running them is cmd/gopatina-difftest's
// job.
func TestBatchesAreAccepted(t *testing.T) {
	for seed := range uint64(64) {
		src := Batch(seed, int(seed)*4, 3)
		dir := t.TempDir()
		for name, data := range map[string]string{
			"go.mod":     "module " + Module + "\n\ngo 1.25\n",
			"main.go":    src,
			"lib/lib.go": Lib(seed, int(seed)*4, 3),
		} {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		pkg, err := load.Dir(dir, dir)
		if err != nil {
			t.Fatalf("seed %d: %v\n%s", seed, err, src)
		}
		if ds := subset.Check(pkg); len(ds) > 0 {
			t.Fatalf("seed %d: subset: %v\n%s", seed, ds, src)
		}
		res, ds := borrow.Check(pkg)
		if len(ds) > 0 {
			t.Fatalf("seed %d: borrow: %v\n%s", seed, ds, src)
		}
		if c := emit.Emit(pkg, res, emit.Options{Target: "x86_64-unknown-linux-gnu", Version: "devel"}); c == nil {
			t.Fatalf("seed %d: no crate", seed)
		}
	}
}

func TestBatchIsDeterministic(t *testing.T) {
	a, b := Batch(7, 3, 5), Batch(7, 3, 5)
	if a != b {
		t.Fatal("same seed, different source")
	}
	if a == Batch(8, 3, 5) {
		t.Error("different seeds, same source")
	}
	for k := 3; k < 8; k++ {
		for _, want := range []string{"func p" + strconv.Itoa(k) + "()", "func h" + strconv.Itoa(k) + "(", `fmt.Println("#` + strconv.Itoa(k) + `")`} {
			if !strings.Contains(a, want) {
				t.Errorf("missing %q", want)
			}
		}
	}
}
