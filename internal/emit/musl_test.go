package emit

import (
	"strings"
	"testing"
)

// A musl crate is linked by rust-lld with Rust's own musl runtime objects,
// so the host's C toolchain is not involved (RFC 0001); other targets keep
// the plain config.
func TestCargoConfigMusl(t *testing.T) {
	for _, target := range []string{"x86_64-unknown-linux-musl", "aarch64-unknown-linux-musl"} {
		got := cargoConfig(target)
		for _, want := range []string{
			"[build]\ntarget = \"" + target + "\"\n",
			"[target." + target + "]\nlinker = \"rust-lld\"\nrustflags = [\"-C\", \"link-self-contained=yes\"]\n",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("cargoConfig(%q) lacks %q:\n%s", target, want, got)
			}
		}
	}
	if got := cargoConfig("x86_64-unknown-linux-gnu"); strings.Contains(got, "rust-lld") || strings.Contains(got, "[target.") {
		t.Errorf("a gnu crate's config names a linker:\n%s", got)
	}
}
