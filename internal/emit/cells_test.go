package emit

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// sharedCell matches what Rust written by hand for a Go program would not
// hold: a reference count or a runtime cell.
var sharedCell = regexp.MustCompile(`\bRc\b|\bRefCell\b|\brt::Ptr\b`)

// TestNoSharedCells checks the committed crates (each equal to what
// gopatina emits, which TestGolden and TestExamples check) against
// testdata/shared-cells.txt: a crate not listed there must hold no Rc,
// RefCell or rt::Ptr, and a listed crate must still hold one, so the list
// is exact and only shrinks (RFC 0017).
func TestNoSharedCells(t *testing.T) {
	listed := sharedCellList(t)
	var crates []string
	for _, pattern := range []string{"testdata/golden/*/rust", "testdata/golden-lib/*/rust", "examples/*/rust"} {
		dirs, err := filepath.Glob(filepath.Join(root(), pattern))
		if err != nil {
			t.Fatal(err)
		}
		crates = append(crates, dirs...)
	}
	sort.Strings(crates)
	found := map[string]bool{}
	for _, crate := range crates {
		rel, err := filepath.Rel(root(), filepath.Dir(crate))
		if err != nil {
			t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		has, where := cratesCell(t, crate)
		found[rel] = true
		switch {
		case has && listed[rel] == "":
			t.Errorf("%s holds a shared cell (%s), and is not in testdata/shared-cells.txt: the generated Rust must hold no Rc, RefCell or rt::Ptr", rel, where)
		case !has && listed[rel] != "":
			t.Errorf("%s holds no shared cell any more: remove it from testdata/shared-cells.txt", rel)
		}
	}
	for rel := range listed {
		if !found[rel] {
			t.Errorf("testdata/shared-cells.txt lists %s, which is not a crate", rel)
		}
	}
}

// cratesCell reports whether a crate's sources hold a shared cell, and the
// first line that does.
func cratesCell(t *testing.T, crate string) (bool, string) {
	srcs, err := filepath.Glob(filepath.Join(crate, "src", "*.rs"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(srcs)
	for _, src := range srcs {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if code, _, _ := strings.Cut(line, "//"); sharedCell.MatchString(code) {
				return true, filepath.Base(src) + ":" + strconv.Itoa(i+1)
			}
		}
	}
	return false, ""
}

// sharedCellList reads testdata/shared-cells.txt: crate directory to why.
func sharedCellList(t *testing.T) map[string]string {
	f, err := os.Open(filepath.Join(root(), "testdata", "shared-cells.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dir, why, ok := strings.Cut(line, "\t")
		if !ok || strings.TrimSpace(why) == "" {
			t.Fatalf("testdata/shared-cells.txt: %q has no reason after a tab", line)
		}
		out[dir] = why
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
