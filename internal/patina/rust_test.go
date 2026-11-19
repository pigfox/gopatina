package patina

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The tests in this file need Go and a Rust toolchain (cargo and rustfmt),
// and build every program under testdata/golden twice, so they run only
// when GOPATINA_RUST_TESTS=1. CI sets it in the toolchains and
// reproducible workflows (RFC 0003).

func needRust(t *testing.T) {
	t.Helper()
	if os.Getenv("GOPATINA_RUST_TESTS") != "1" {
		t.Skip("set GOPATINA_RUST_TESTS=1 to build and run the Rust crates")
	}
}

// goldenDirs returns every golden program, and the hello example of
// EXAMPLES.md.
func goldenDirs(t *testing.T) []string {
	dirs, err := filepath.Glob(filepath.Join(root(), "testdata", "golden", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no golden programs: %v", err)
	}
	return append(dirs, filepath.Join(root(), "examples", "hello"))
}

// realSession uses the cargo on PATH.
func realSession(t *testing.T) *Session {
	cache := t.TempDir()
	return &Session{
		Cwd: root(), Stdout: os.Stdout, Stderr: os.Stderr, Version: "devel",
		Getenv: func(k string) string {
			if k == "GOPATINA_CACHE" {
				return cache
			}
			return os.Getenv(k)
		},
	}
}

type result struct {
	stdout string
	code   int
	stderr string
}

// stderrLine reduces a program's stderr to the line tests compare. Go
// prints a stack trace after a panic; GoPatina's Rust runtime prints only
// the panic message. The first line carries the contract both share.
func stderrLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimRight(line, " \t\r")
}

func runBinary(t *testing.T, bin string) result {
	t.Helper()
	var out, stderr bytes.Buffer
	cmd := exec.Command(bin)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return result{stdout: out.String(), stderr: stderr.String()}
	case errors.As(err, &exitErr):
		return result{stdout: out.String(), code: exitErr.ExitCode(), stderr: stderr.String()}
	}
	t.Fatalf("running %s: %v", bin, err)
	return result{}
}

// TestDifferential builds every golden program with go build and with
// gopatina build -patina, runs both, and requires the same standard output,
// first stderr line and exit status.
func TestDifferential(t *testing.T) {
	needRust(t)
	s := realSession(t)
	for _, dir := range goldenDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			tmp := t.TempDir()
			goBin := filepath.Join(tmp, "go-bin")
			build := exec.Command("go", "build", "-o", goBin, ".")
			build.Dir = dir
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go build: %v\n%s", err, out)
			}
			patinaBin := filepath.Join(tmp, "patina-bin")
			if err := s.Build(dir, patinaBin); err != nil {
				t.Fatal(err)
			}
			want, got := runBinary(t, goBin), runBinary(t, patinaBin)
			if got.stdout != want.stdout || got.code != want.code || stderrLine(got.stderr) != stderrLine(want.stderr) {
				t.Errorf("Go and patina differ:\ngo     (exit %d):\n%sstderr: %q\npatina (exit %d):\n%sstderr: %q",
					want.code, want.stdout, stderrLine(want.stderr),
					got.code, got.stdout, stderrLine(got.stderr))
			}
		})
	}
}

// pinnedEnv is the environment without RUSTUP_TOOLCHAIN, so cargo and
// rustfmt use the toolchain each crate pins.
func pinnedEnv() []string {
	var env []string
	for _, kv := range cargoEnv(os.Environ()) {
		if !strings.HasPrefix(kv, "RUSTUP_TOOLCHAIN=") {
			env = append(env, kv)
		}
	}
	return append(env, "CARGO_TERM_COLOR=never")
}

// TestGoldenCratesAreClean builds each committed golden crate with plain
// cargo and the pinned toolchain, requiring no warnings, and checks that
// it is rustfmt-clean with the pinned rustfmt.
func TestGoldenCratesAreClean(t *testing.T) {
	needRust(t)
	for _, dir := range goldenDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			crate := filepath.Join(dir, "rust")
			target := t.TempDir()
			cmd := exec.Command("cargo", "build", "--release", "--locked", "--target-dir", target)
			cmd.Dir = crate
			cmd.Env = pinnedEnv()
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("cargo build: %v\n%s", err, out)
			}
			if strings.Contains(string(out), "warning") {
				t.Errorf("cargo build printed warnings:\n%s", out)
			}
			srcs, _ := filepath.Glob(filepath.Join(crate, "src", "*.rs"))
			fmtCmd := exec.Command("rustfmt", append([]string{"--check", "--edition", "2024"}, srcs...)...)
			fmtCmd.Dir = crate
			fmtCmd.Env = pinnedEnv()
			if out, err := fmtCmd.CombinedOutput(); err != nil {
				t.Errorf("rustfmt --check: %v\n%s", err, out)
			}
		})
	}
}

// TestVerifyGolden runs gopatina verify on every golden program: emission
// is deterministic, and the -patina binary is byte-identical to a plain
// cargo build of the emitted crate in another directory.
func TestVerifyGolden(t *testing.T) {
	needRust(t)
	for _, dir := range goldenDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			s := realSession(t)
			var out bytes.Buffer
			s.Stdout = &out
			if err := s.Verify(dir); err != nil {
				t.Fatalf("%v\n%s", err, out.String())
			}
		})
	}
}
