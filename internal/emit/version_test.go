package emit

import (
	"testing"

	"github.com/pigfox/gopatina/internal/fma"
	"github.com/pigfox/gopatina/internal/load"
)

func TestVersionLess(t *testing.T) {
	for _, c := range []struct {
		v, w string
		want bool
	}{
		{"go1.26.3", "go1.27", true},
		{"go1.27.0", "go1.27", false},
		{"go1.27.4", "go1.27", false},
		{"go1.28rc1", "go1.27", false},
		{"go1.9", "go1.27", true},
	} {
		if got := versionLess(c.v, c.w); got != c.want {
			t.Errorf("versionLess(%q, %q) = %t", c.v, c.w, got)
		}
	}
}

// panic(nil) prints a runtime error from Go 1.27 on, and plain text before.
func TestPanicNilText(t *testing.T) {
	for version, want := range map[string]string{
		"go1.26.9": "panic called with nil argument",
		"go1.27.4": "runtime error: panic called with nil argument",
		"":         "runtime error: panic called with nil argument",
	} {
		g := &gen{pkg: &load.Package{Target: fma.Target{GoVersion: version}}}
		if got := g.panicNilText(); got != want {
			t.Errorf("%q: %q, want %q", version, got, want)
		}
	}
}
