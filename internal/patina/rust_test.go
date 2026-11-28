package patina

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// libraryDirs returns the golden library packages (RFC 0014).
func libraryDirs(t *testing.T) []string {
	dirs, err := filepath.Glob(filepath.Join(root(), "testdata", "golden-lib", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no golden libraries: %v", err)
	}
	return dirs
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
// the panic message, in Go's wording. The first line is the shared contract.
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
	target, err := s.FMATarget()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range goldenDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			tmp := t.TempDir()
			goBin := filepath.Join(tmp, "go-bin")
			// Go's reference build: optimization and inlining off, the
			// closest gc comes to the specification. Optimized gc builds have
			// printed wrong results (testdata/golden/gc_*). Where gc fuses
			// float multiply-adds, GoPatina matches the default build, which
			// fuses across statements and -N -l does not (internal/fma); the
			// gc_ programs keep -N -l, the right result.
			flags := []string{"build", "-gcflags=all=-N -l", "-o", goBin, "."}
			if target.Fuses() && !strings.HasPrefix(filepath.Base(dir), "gc_") {
				flags = []string{"build", "-o", goBin, "."}
			}
			build := exec.Command("go", flags...)
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
			if want.stderr == "" && got.stderr != "" {
				// A recovered panic prints nothing (RFC 0020): Rust's
				// default hook would print before catch_unwind stops it.
				t.Errorf("Go prints nothing on standard error, patina prints:\n%s", got.stderr)
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
// cargo and the pinned toolchain, requiring no warnings, runs clippy with
// warnings denied, and checks that it is rustfmt-clean with the pinned
// rustfmt. The explicit allow list in emitted crates covers only patterns
// a faithful Go translation cannot avoid.
func TestGoldenCratesAreClean(t *testing.T) {
	needRust(t)
	for _, dir := range append(goldenDirs(t), libraryDirs(t)...) {
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
			clippy := exec.Command("cargo", "clippy", "--release", "--locked", "--target-dir", target, "--", "-D", "warnings")
			clippy.Dir = crate
			clippy.Env = pinnedEnv()
			if out, err := clippy.CombinedOutput(); err != nil {
				t.Errorf("cargo clippy -D warnings: %v\n%s", err, out)
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

// crateNameRE finds a crate's name in its Cargo.toml.
var crateNameRE = regexp.MustCompile(`(?m)^name = "([a-z0-9_]+)"`)

// TestLibraries checks each golden library crate (RFC 0014) from the
// outside, as a Rust project uses it: a cargo workspace with the committed
// crate and check/main.rs, a hand-written Rust program calling it, built,
// linted and run with plain cargo, must print what check/main.go prints
// calling the Go package (a differential test).
func TestLibraries(t *testing.T) {
	needRust(t)
	for _, dir := range libraryDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			tmp := t.TempDir()
			ws := filepath.Join(tmp, "ws")
			if err := os.CopyFS(filepath.Join(ws, "lib"), os.DirFS(filepath.Join(dir, "rust"))); err != nil {
				t.Fatal(err)
			}
			manifest, _ := os.ReadFile(filepath.Join(dir, "rust", "Cargo.toml"))
			name := string(crateNameRE.FindSubmatch(manifest)[1])
			src, err := os.ReadFile(filepath.Join(dir, "check", "main.rs"))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"Cargo.toml": "[workspace]\nmembers = [\"lib\", \"check\"]\nresolver = \"3\"\n\n" +
					"[profile.dev.package." + name + "]\noverflow-checks = false\n",
				"check/Cargo.toml": "[package]\nname = \"check\"\nversion = \"0.1.0\"\nedition = \"2024\"\n\n" +
					"[dependencies]\n" + name + " = { path = \"../lib\" }\n",
				"check/src/main.rs": string(src),
			}
			for f, data := range files {
				path := filepath.Join(ws, filepath.FromSlash(f))
				os.MkdirAll(filepath.Dir(path), 0o755)
				if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cargo := func(args ...string) {
				cmd := exec.Command("cargo", args...)
				cmd.Dir = ws
				cmd.Env = append(cargoEnv(os.Environ()), "CARGO_TERM_COLOR=never")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("cargo %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
			cargo("clippy", "--quiet", "--workspace", "--", "-D", "warnings")
			cargo("build", "--quiet", "--workspace")
			goBin := filepath.Join(tmp, "go-check")
			build := exec.Command("go", "build", "-gcflags=all=-N -l", "-o", goBin, "./check")
			build.Dir = dir
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go build: %v\n%s", err, out)
			}
			want, got := runBinary(t, goBin), runBinary(t, filepath.Join(ws, "target", "debug", "check"))
			if got.stdout != want.stdout || got.code != want.code {
				t.Errorf("Go and the Rust crate differ:\ngo (exit %d):\n%srust (exit %d):\n%s", want.code, want.stdout, got.code, got.stdout)
			}
		})
	}
}

// TestReferences builds each golden program's reference.rs, the Rust a
// person writes for it (internal/emit's TestReferenceCloseness measures
// GoPatina's Rust against it), with rustc and warnings denied, checks that
// clippy and rustfmt are clean on it, and requires it to print what the Go
// program prints: a reference that drifts from the program measures
// nothing.
func TestReferences(t *testing.T) {
	needRust(t)
	refs, err := filepath.Glob(filepath.Join(root(), "testdata", "golden", "*", "reference.rs"))
	if err != nil || len(refs) == 0 {
		t.Fatalf("no references: %v", err)
	}
	for _, ref := range refs {
		dir := filepath.Dir(ref)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			tmp := t.TempDir()
			goBin := filepath.Join(tmp, "go-bin")
			build := exec.Command("go", "build", "-o", goBin, ".")
			build.Dir = dir
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go build: %v\n%s", err, out)
			}
			refBin := filepath.Join(tmp, "ref-bin")
			rustc := exec.Command("rustc", "--edition", "2024", "-O", "-D", "warnings", "-o", refBin, ref)
			if out, err := rustc.CombinedOutput(); err != nil {
				t.Fatalf("rustc: %v\n%s", err, out)
			}
			clippy := exec.Command("clippy-driver", "--edition", "2024", "-D", "warnings", "--emit=metadata", "--out-dir", tmp, ref)
			if out, err := clippy.CombinedOutput(); err != nil {
				t.Errorf("clippy: %v\n%s", err, out)
			}
			if out, err := exec.Command("rustfmt", "--check", "--edition", "2024", ref).CombinedOutput(); err != nil {
				t.Errorf("rustfmt --check: %v\n%s", err, out)
			}
			want, got := runBinary(t, goBin), runBinary(t, refBin)
			if got.stdout != want.stdout || got.code != want.code {
				t.Errorf("Go and the reference differ:\ngo  (exit %d):\n%sref (exit %d):\n%s", want.code, want.stdout, got.code, got.stdout)
			}
		})
	}
}
