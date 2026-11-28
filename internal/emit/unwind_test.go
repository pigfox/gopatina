package emit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// panicAbort matches a Cargo profile setting that makes a panic abort.
var panicAbort = regexp.MustCompile(`(?m)^\s*panic\s*=\s*"abort"`)

// TestCratesUnwind checks that no profile of an emitted crate makes a panic
// abort: rt::Defers runs deferred calls as a panic unwinds, and
// rt::recovering stops a recovered one with catch_unwind (RFC 0020,
// condition 2), so every crate the emitter writes, the goldens and the
// examples, keeps Rust's default, unwind.
func TestCratesUnwind(t *testing.T) {
	var tomls []string
	for _, pattern := range []string{"testdata/golden/*/rust/Cargo.toml", "testdata/golden-lib/*/rust/Cargo.toml", "examples/*/rust/Cargo.toml"} {
		found, err := filepath.Glob(filepath.Join(root(), pattern))
		if err != nil {
			t.Fatal(err)
		}
		tomls = append(tomls, found...)
	}
	if len(tomls) < 10 {
		t.Fatalf("found %d crates; the globs are wrong", len(tomls))
	}
	for _, path := range tomls {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if loc := panicAbort.FindIndex(b); loc != nil {
			line := strings.Count(string(b[:loc[0]]), "\n") + 1
			t.Errorf("%s:%d sets panic = \"abort\"; emitted crates must unwind (RFC 0020)", path, line)
		}
	}
	if !panicAbort.MatchString("[profile.release]\npanic = \"abort\"\n") {
		t.Error("panicAbort does not match the setting it guards against")
	}
}
