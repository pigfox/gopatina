package patina

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/fma"
)

var updateSites = flag.Bool("update-fma-sites", false, "rewrite testdata/golden/*/fma-sites.txt")

// fusingTargets are the targets on which gc fuses float multiply-adds.
var fusingTargets = []fma.Target{
	{GOOS: "linux", GOARCH: "arm64"},
	{GOOS: "linux", GOARCH: "amd64", GOAMD64: "v3"},
}

// TestFMASites records, for each golden program whose own code gc fuses,
// the sites gc reports per fusing target, with the Go version: a change
// of gc's fusion, or of the program, shows in fma-sites.txt. With another
// Go version than the file's, the sites are only logged: gc's choices are
// that version's.
func TestFMASites(t *testing.T) {
	goCmd, err := exec.LookPath(goCommand)
	if err != nil {
		t.Skip(err)
	}
	version, err := exec.Command(goCmd, "env", "GOVERSION").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range goldenDirs(t) {
		if !mentionsFloat(dir) {
			continue // gc fuses only float operations
		}
		var b strings.Builder
		for _, tg := range fusingTargets {
			tg.GoVersion = strings.TrimSpace(string(version))
			sites, err := fma.Find(goCmd, os.Environ(), dir, tg)
			if err != nil {
				t.Fatalf("%s: %v", dir, err)
			}
			var own []string
			for _, s := range sites.List {
				if rel, err := filepath.Rel(dir, s.Pos.File); err == nil && !strings.HasPrefix(rel, "..") {
					own = append(own, strings.ReplaceAll(s.String(), dir+string(filepath.Separator), ""))
				}
			}
			if len(own) > 0 {
				b.WriteString("# " + tg.String() + "\n" + strings.Join(own, "\n") + "\n")
			}
		}
		file := filepath.Join(dir, "fma-sites.txt")
		got := b.String()
		if *updateSites {
			if got == "" {
				os.Remove(file)
			} else if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, _ := os.ReadFile(file)
		switch {
		case got == string(want):
		case !strings.Contains(string(want), "# "+strings.TrimSpace(string(version))+" "):
			t.Logf("%s: gc %s fuses differently from the gc fma-sites.txt records:\n%s", filepath.Base(dir), strings.TrimSpace(string(version)), got)
		default:
			t.Errorf("%s: fma-sites.txt differs (go test ./internal/patina -run TestFMASites -update-fma-sites):\n--- got:\n%s--- want:\n%s", filepath.Base(dir), got, want)
		}
	}
}

// mentionsFloat reports whether a Go file in dir names a float type.
func mentionsFloat(dir string) bool {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	more, _ := filepath.Glob(filepath.Join(dir, "*", "*.go"))
	for _, f := range append(files, more...) {
		if src, _ := os.ReadFile(f); strings.Contains(string(src), "float") {
			return true
		}
	}
	return false
}
