package emit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReferenceCloseness measures, for each golden program with a
// reference.rs (the Rust a person writes for it), how close GoPatina's
// Rust reads to that reference, and checks the measure against the
// program's closeness.txt. Run go test ./internal/emit -update to rewrite
// it after a deliberate change. The measure is over lines, ignoring
// indentation, blank lines and comments: the lines the two have in common,
// in order (their longest common subsequence), against all of both.
func TestReferenceCloseness(t *testing.T) {
	refs, err := filepath.Glob(filepath.Join(root(), "testdata", "golden", "*", "reference.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) == 0 {
		t.Fatal("no golden program has a reference.rs")
	}
	for _, ref := range refs {
		dir := filepath.Dir(ref)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			want, err := os.ReadFile(ref)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "rust", "src", "main.rs"))
			if err != nil {
				t.Fatal(err)
			}
			report := closeness(string(got), string(want))
			path := filepath.Join(dir, "closeness.txt")
			if *update {
				if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			old, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(old) != report {
				t.Errorf("%s is not the measure (run go test ./internal/emit -update):\n%s", path, report)
			}
		})
	}
}

// closeness reports how close generated Rust reads to reference Rust.
func closeness(generated, reference string) string {
	g, r := codeLines(generated), codeLines(reference)
	common := lcs(g, r)
	ratio := 0.0
	if len(g)+len(r) > 0 {
		ratio = 2 * float64(common) / float64(len(g)+len(r))
	}
	return fmt.Sprintf(`# How close GoPatina's Rust (rust/src/main.rs) reads to reference.rs,
# the Rust a person writes for this program. Lines are compared without
# their indentation; blank lines and comments are left out. Written by
# go test ./internal/emit -update (TestReferenceCloseness).
reference lines: %d
generated lines: %d
lines in common, in order: %d
closeness (2 x common / both): %.2f
`, len(r), len(g), common, ratio)
}

// codeLines returns the lines of Rust source src that are code, without
// their indentation.
func codeLines(src string) []string {
	var out []string
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "//") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// lcs returns the length of the longest common subsequence of a and b.
func lcs(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := range a {
		for j := range b {
			switch {
			case a[i] == b[j]:
				cur[j+1] = prev[j] + 1
			case prev[j+1] >= cur[j]:
				cur[j+1] = prev[j+1]
			default:
				cur[j+1] = cur[j]
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
