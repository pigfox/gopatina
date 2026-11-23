package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// script writes an executable shell script.
func script(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakePatina is a gopatina whose -emit-rust makes a crate directory (or
// fails, with emitFail), and whose -patina build is tool's.
func fakePatina(t *testing.T, tool string, emitFail bool) string {
	fail := ""
	if emitFail {
		fail = "echo emit broke; exit 1"
	}
	return script(t, "gopatina", `case " $* " in *" -emit-rust "*)
  while [ $# -gt 0 ]; do if [ "$1" = -o ]; then out="$2"; fi; shift; done
  mkdir -p "$out"
  `+fail+`
  exit 0;;
esac
exec '`+tool+`' "$@"
`)
}

// fakeCargo is a cargo whose clippy runs clippy (a shell snippet), and
// whose build runs build, then writes a check binary printing out.
func fakeCargo(t *testing.T, clippy, build, out string) string {
	return script(t, "cargo", `case "$1" in
clippy) `+clippy+` ;;
build) `+build+`
  mkdir -p "$CARGO_TARGET_DIR/debug"
  printf '#!/bin/sh\nprintf "`+out+`"\n' > "$CARGO_TARGET_DIR/debug/check"
  chmod +x "$CARGO_TARGET_DIR/debug/check" ;;
esac
`)
}

// splitGo sends the library check's go build (of ./check) to check, and
// the batches' builds to batch.
func splitGo(t *testing.T, batch, check string) string {
	return script(t, "go", `case "$*" in *./check*) exec '`+check+`' "$@" ;; *) exec '`+batch+`' "$@" ;; esac
`)
}

func report_(t *testing.T, out string) report {
	t.Helper()
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return r
}

// TestChecks runs the lint and library checks on every batch, clean and
// failing.
func TestChecks(t *testing.T) {
	g := fakeTool(t, "go", false, "x", "true")
	p := fakeTool(t, "gopatina", false, "x", "true")
	ok := fakeCargo(t, "exit 0", "", "")
	for _, tc := range []struct {
		name                         string
		patina, cargo                string
		lintFailed, libFailed, lints int
		kept                         string
	}{
		{"clean", fakePatina(t, p, false), ok, 0, 0, 2, ""},
		{"lint fails", fakePatina(t, p, false), fakeCargo(t, "echo 'warning: x'; exit 101", "", ""), 2, 0, 2, "seed1-b0/clippy.txt"},
		{"library differs", fakePatina(t, p, false), fakeCargo(t, "exit 0", "", "x"), 0, 2, 2, "seed1-lib0/rust.out"},
		{"library does not build", fakePatina(t, p, false), fakeCargo(t, "exit 0", "echo nope; exit 101", ""), 0, 2, 2, "seed1-lib0/cargo-build.txt"},
		{"emit fails", fakePatina(t, p, true), ok, 2, 2, 2, "seed1-lib0/emit.txt"},
		{"batch does not build", fakePatina(t, fakeTool(t, "gopatina", true, "", ""), false), ok, 0, 0, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fail := t.TempDir()
			code, out, errs := difftestWith("-programs", "4", "-per", "2", "-lint", "1", "-libraries", "1",
				"-go", g, "-gopatina", tc.patina, "-cargo", tc.cargo, "-failures", fail)
			if code != 0 && code != 3 {
				t.Fatalf("code %d: %s", code, errs)
			}
			r := report_(t, out)
			if r.LintChecks != tc.lints || r.LintFailed != tc.lintFailed || r.LibraryChecks != 2 || r.LibraryFailed != tc.libFailed {
				t.Errorf("report %s", out)
			}
			if tc.kept != "" {
				if _, err := os.Stat(filepath.Join(fail, tc.kept)); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

// TestCheckErrors checks the failures that are not GoPatina defects: a
// machine without clippy or cargo, and a generated library check the Go
// toolchain does not build or run cleanly.
func TestCheckErrors(t *testing.T) {
	g := fakeTool(t, "go", false, "x", "true")
	p := fakePatina(t, fakeTool(t, "gopatina", false, "x", "true"), false)
	ok := fakeCargo(t, "exit 0", "", "")
	missing := filepath.Join(t.TempDir(), "no-cargo")
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no clippy", []string{"-lint", "1", "-cargo", fakeCargo(t, "echo \"error: 'cargo-clippy' is not installed for the toolchain '1.93.0'\"; exit 1", "", "")}, "clippy is not installed"},
		{"no cargo to lint", []string{"-lint", "1", "-cargo", missing}, "could not run"},
		{"no cargo for libraries", []string{"-libraries", "1", "-cargo", missing}, "could not run"},
		{"check does not build", []string{"-libraries", "1", "-cargo", ok, "-go", splitGo(t, g, fakeTool(t, "gocheck", true, "", ""))}, "go build"},
		{"check fails", []string{"-libraries", "1", "-cargo", ok, "-go", splitGo(t, g, fakeTool(t, "gocheck", false, "x", "exit 3"))}, "the Go check exited 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-programs", "2", "-per", "2", "-lint", "0", "-libraries", "0", "-go", g, "-gopatina", p}, tc.args...)
			code, _, errs := difftestWith(args...)
			if code != 1 || !strings.Contains(errs, tc.want) {
				t.Errorf("code %d, stderr %s", code, errs)
			}
		})
	}
}
