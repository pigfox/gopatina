package patina

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// goPanicLine translates Rust's standard runtime panic text into the first
// line Go prints for the same operation. GoPatina's emitted program prints
// Rust's text; normalising it in the test restores the check on panic
// behaviour without rewriting a user's own panic("...") payload at run time.
func goPanicLine(s string) string {
	line := stderrLine(s)
	msg, ok := strings.CutPrefix(line, "panic: ")
	if !ok {
		return line
	}
	switch msg {
	case "attempt to divide by zero", "attempt to calculate the remainder with a divisor of zero":
		return "panic: runtime error: integer divide by zero"
	}
	const idxPrefix = "index out of bounds: the len is "
	if !strings.HasPrefix(msg, idxPrefix) {
		return line
	}
	parts := strings.Split(strings.TrimPrefix(msg, idxPrefix), " but the index is ")
	if len(parts) != 2 {
		return line
	}
	length, errLen := strconv.ParseUint(parts[0], 10, 64)
	index, errIdx := strconv.ParseUint(parts[1], 10, 64)
	if errLen != nil || errIdx != nil {
		return line
	}
	if signed := int64(index); signed < 0 {
		return fmt.Sprintf("panic: runtime error: index out of range [%d]", signed)
	}
	return fmt.Sprintf("panic: runtime error: index out of range [%d] with length %d", index, length)
}

func TestGoPanicLine(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"panic: attempt to divide by zero\nstack", "panic: runtime error: integer divide by zero"},
		{"panic: attempt to calculate the remainder with a divisor of zero\n", "panic: runtime error: integer divide by zero"},
		{"panic: index out of bounds: the len is 3 but the index is 5\n", "panic: runtime error: index out of range [5] with length 3"},
		{"panic: index out of bounds: the len is 3 but the index is 18446744073709551615\n", "panic: runtime error: index out of range [-1]"},
		{"panic: index out of bounds: the len is 3 but the index is x\n", "panic: index out of bounds: the len is 3 but the index is x"},
		{"panic: index out of bounds: the len is 3\n", "panic: index out of bounds: the len is 3"},
		{"panic: too big: 4\n", "panic: too big: 4"},
		{"hello\n", "hello"},
	} {
		if got := goPanicLine(tt.in); got != tt.want {
			t.Errorf("goPanicLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
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
			gotPanic := goPanicLine(got.stderr)
			if got.stdout != want.stdout || got.code != want.code || gotPanic != stderrLine(want.stderr) {
				t.Errorf("Go and patina differ:\ngo     (exit %d):\n%sstderr: %q\npatina (exit %d):\n%sstderr: %q",
					want.code, want.stdout, stderrLine(want.stderr),
					got.code, got.stdout, gotPanic)
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
