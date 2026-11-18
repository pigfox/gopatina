package patina

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Check Go semantics for slices and for patterns Patina rejects.
// Compare the accepted cases against Rust as well when it is available.
func TestSliceGoSemantics(t *testing.T) {
	s := realSession(t)
	tests := []struct {
		name       string
		corpus     string
		stdout     string
		panicLine  string
		exitStatus int
	}{
		{name: "slice_suffix", corpus: "golden", stdout: "[20 30] 2\n[] 0\n[99 30] 2\n[10 99 30]\n[] 0\n[] 0\n4\n"},
		{name: "slice_suffix_high", corpus: "golden", stdout: "before ", exitStatus: 2, panicLine: "panic: runtime error: slice bounds out of range [3:2]"},
		{name: "slice_suffix_negative", corpus: "golden", stdout: "before\n", exitStatus: 2, panicLine: "panic: runtime error: slice bounds out of range [-1:]"},
		{name: "slice_explicit_high", stdout: "[7 0 0] 3\n"},
		{name: "slice_suffix_write", stdout: "[10 99 30]\n"},
		{name: "slice_suffix_effects", stdout: "low\n[20]\n"},
		{name: "range_slice_element_pointer", corpus: "golden", stdout: "1\n9\n9\n0 9\n1 9\n2 9\n"},
		{name: "range_slice_replaced", corpus: "reject", stdout: "123\n012\n012\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			bin := filepath.Join(tmp, "go-bin")
			corpus := tt.corpus
			if corpus == "" {
				corpus = "go_semantics"
			}
			dir := filepath.Join(root(), "testdata", corpus, tt.name)
			build := exec.Command("go", "build", "-o", bin, ".")
			build.Dir = dir
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go build: %v\n%s", err, out)
			}
			check := func(t *testing.T, mode string, got result) {
				t.Helper()
				// Compare panic text but not the Go toolchain's stack trace.
				line, _, _ := strings.Cut(got.stderr, "\n")
				if got.stdout != tt.stdout || got.code != tt.exitStatus || line != tt.panicLine {
					t.Errorf("%s (stdout, exit, stderr first line) = (%q, %d, %q), want (%q, %d, %q)",
						mode, got.stdout, got.code, line, tt.stdout, tt.exitStatus, tt.panicLine)
				}
			}
			check(t, "Go", runBinary(t, bin))
			if corpus == "golden" {
				t.Run("patina", func(t *testing.T) {
					needRust(t)
					bin := filepath.Join(tmp, "patina-bin")
					if err := s.Build(dir, bin); err != nil {
						t.Fatal(err)
					}
					check(t, "patina", runBinary(t, bin))
				})
			}
		})
	}
}
