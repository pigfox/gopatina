package casetab

import (
	"strings"
	"testing"
	"unicode"
)

func TestRust(t *testing.T) {
	s := Rust()
	for _, want := range []string{"const CASE", "const FOLD", "(0x41, 0x5A, 0, 32)", ", UL, UL)", "(0x6B, 0x212A)"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if Version() != unicode.Version {
		t.Error("Version")
	}
}
