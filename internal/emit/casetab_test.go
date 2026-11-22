package emit

import (
	"testing"

	"github.com/pigfox/gopatina/internal/emit/internal/casetab"
)

// TestCaseTables checks the committed case tables against the running
// toolchain's package unicode when it has the same Unicode version.
func TestCaseTables(t *testing.T) {
	if casetab.Version() != UnicodeVersion {
		t.Skipf("toolchain Unicode %s, tables %s", casetab.Version(), UnicodeVersion)
	}
	if casetab.Rust() != caseTables {
		t.Error("casetables.go is stale: go generate ./internal/emit")
	}
}
