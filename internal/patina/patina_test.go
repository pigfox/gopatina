package patina

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/emit"
)

// When fakeCargoEnv is set, the test binary stands in for cargo: it checks
// its arguments, and writes a "binary" made from the crate's sources where
// cargo would put it. The value "fail" makes it fail, "nobin" makes it
// succeed without writing the binary.
const fakeCargoEnv = "GOPATINA_TEST_FAKE_CARGO"

func TestMain(m *testing.M) {
	if mode, ok := os.LookupEnv(fakeCargoEnv); ok {
		os.Exit(fakeCargo(mode))
	}
	os.Exit(m.Run())
}

func fakeCargo(mode string) int {
	if strings.Join(os.Args[1:], " ") != strings.Join(CargoArgs, " ") {
		fmt.Fprintf(os.Stderr, "unexpected arguments %q\n", os.Args[1:])
		return 3
	}
	if os.Getenv("RUSTFLAGS") != "" {
		fmt.Fprintln(os.Stderr, "RUSTFLAGS must not reach cargo")
		return 3
	}
	if mode == "fail" {
		fmt.Fprintln(os.Stderr, "error[E0000]: fake rustc error")
		return 101
	}
	if mode == "nobin" {
		return 0
	}
	if wd, _ := os.Getwd(); mode == "plainfail" && strings.Contains(wd, "plain-cargo-build") {
		return 101
	}
	cfg, _ := os.ReadFile(".cargo/config.toml")
	toml, _ := os.ReadFile("Cargo.toml")
	src, _ := os.ReadFile("src/main.rs")
	_, rest, _ := strings.Cut(string(cfg), `target = "`)
	target, _, _ := strings.Cut(rest, `"`)
	_, rest, _ = strings.Cut(string(toml), `name = "`)
	name, _, _ := strings.Cut(rest, `"`)
	dir := filepath.Join("target", target, "release")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, name), append([]byte("fake binary\n"), src...), 0o755)
	return 0
}

// session returns a session whose cargo is the fake, and its output.
func session(t *testing.T, mode string) (*Session, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeCargoEnv, mode)
	t.Setenv("RUSTFLAGS", "-C opt-level=0")
	env := map[string]string{
		EnvCargo:         exe,
		"GOPATINA_CACHE": t.TempDir(),
		"GOOS":           "linux",
		"GOARCH":         "amd64",
	}
	var stdout, stderr bytes.Buffer
	s := &Session{
		Cwd: root(), Stdout: &stdout, Stderr: &stderr, Version: "test",
		Getenv: func(k string) string { return env[k] },
	}
	return s, &stdout, &stderr
}

func root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func golden(name string) string { return filepath.Join(root(), "testdata", "golden", name) }
func reject(name string) string { return filepath.Join(root(), "testdata", "reject", name) }

func TestCheckAndExplain(t *testing.T) {
	s, _, stderr := session(t, "")
	s.Explain = true
	if _, _, err := s.Check(golden("hello")); err != nil {
		t.Fatal(err)
	}
	want := "./testdata/golden/hello/main.go:30:11: copied names[0] into first"
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("-explain output does not contain %q:\n%s", want, stderr)
	}
}

// TestEmitExplainNotes checks that -explain also prints the emitter's
// decisions: loop specialization (RFC 0006).
func TestEmitExplainNotes(t *testing.T) {
	s, _, stderr := session(t, "")
	s.Explain = true
	if _, err := s.Emit(golden("ranges_sieve")); err != nil {
		t.Fatal(err)
	}
	want := "./testdata/golden/ranges_sieve/main.go:10:2: loop specialized for n <= 3037000500: usize for i, j"
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("-explain output does not contain %q:\n%s", want, stderr)
	}
}

func TestCheckErrors(t *testing.T) {
	tests := []struct{ dir, want string }{
		{reject("ownership"), "./testdata/reject/ownership/main.go:33:2: m.Body is changed after m was handed to out.Queue on line 32"},
		{reject("switch_misc"), "is not supported in patina mode yet"},
	}
	for _, tt := range tests {
		s, _, stderr := session(t, "")
		if _, _, err := s.Check(tt.dir); !errors.Is(err, ErrFailed) {
			t.Errorf("%s: err = %v, want ErrFailed", tt.dir, err)
		}
		if !strings.Contains(stderr.String(), tt.want) {
			t.Errorf("%s: stderr does not contain %q:\n%s", tt.dir, tt.want, stderr)
		}
	}
}

func TestCheckLoadErrors(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() { x }\n"), 0o644)
	s, _, stderr := session(t, "")
	if _, _, err := s.Check(dir); !errors.Is(err, ErrFailed) {
		t.Errorf("err = %v, want ErrFailed", err)
	}
	if !strings.Contains(stderr.String(), "undefined: x") {
		t.Errorf("stderr = %q", stderr)
	}
	if _, _, err := s.Check(filepath.Join(dir, "missing")); err == nil || errors.Is(err, ErrFailed) {
		t.Errorf("missing directory: err = %v", err)
	}
	if _, err := s.Emit(filepath.Join(dir, "missing")); err == nil {
		t.Error("Emit of a missing directory succeeded")
	}
}

func TestTarget(t *testing.T) {
	s, _, _ := session(t, "")
	env := map[string]string{"GOOS": "darwin", "GOARCH": "arm64"}
	s.Getenv = func(k string) string { return env[k] }
	if got, err := s.Target(); err != nil || got != "aarch64-apple-darwin" {
		t.Errorf("Target() = %q, %v", got, err)
	}
	env = map[string]string{"GOOS": "plan9", "GOARCH": "386"}
	if _, err := s.Target(); err == nil || !strings.Contains(err.Error(), "plan9/386 has no Rust target") {
		t.Errorf("Target() error = %v", err)
	}
	if _, err := s.Emit(golden("hello")); err == nil {
		t.Error("Emit with an unknown target succeeded")
	}
	env = map[string]string{}
	if _, err := s.Target(); err != nil && !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("Target() for the host: %v", err)
	}
}

func TestCargoEnv(t *testing.T) {
	got := cargoEnv([]string{"PATH=/bin", "RUSTFLAGS=-g", "CARGO_TARGET_DIR=/t", "CARGO_PROFILE_RELEASE_LTO=true", "HOME=/h"})
	if strings.Join(got, " ") != "PATH=/bin HOME=/h" {
		t.Errorf("cargoEnv = %q", got)
	}
}

func TestEmitTo(t *testing.T) {
	s, _, stderr := session(t, "")
	out := filepath.Join(t.TempDir(), "crate")
	if err := s.EmitTo(golden("hello"), out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "wrote crate") {
		t.Errorf("stderr = %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(out, emit.ManifestName)); err != nil {
		t.Error(err)
	}
	if err := s.EmitTo(reject("ownership"), out); !errors.Is(err, ErrFailed) {
		t.Errorf("err = %v", err)
	}
	// A directory gopatina did not write is refused.
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, "keep.txt"), nil, 0o644)
	if err := s.EmitTo(golden("hello"), other); err == nil {
		t.Error("EmitTo overwrote a directory it did not write")
	}
}

func TestBuild(t *testing.T) {
	s, _, _ := session(t, "")
	out := filepath.Join(t.TempDir(), "hello")
	if err := s.Build(golden("hello"), out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || !bytes.HasPrefix(data, []byte("fake binary\n")) {
		t.Errorf("binary = %q, %v", data, err)
	}
	// Building again reuses the cached crate.
	if err := s.Build(golden("hello"), out); err != nil {
		t.Fatal(err)
	}
	// With no -o, the binary is named after the package, in the current
	// directory.
	s.Cwd = t.TempDir()
	if err := s.Build(golden("hello"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Cwd, "hello")); err != nil {
		t.Error(err)
	}
}

func TestBuildErrors(t *testing.T) {
	s, _, _ := session(t, "fail")
	err := s.Build(golden("hello"), filepath.Join(t.TempDir(), "x"))
	if err == nil || !strings.Contains(err.Error(), "fake rustc error") || !strings.Contains(err.Error(), "GoPatina bug") {
		t.Errorf("err = %v", err)
	}
	s, _, _ = session(t, "nobin")
	if err := s.Build(golden("hello"), filepath.Join(t.TempDir(), "x")); err == nil || !strings.Contains(err.Error(), "did not produce") {
		t.Errorf("err = %v", err)
	}
	s, _, _ = session(t, "")
	env := map[string]string{EnvCargo: filepath.Join(t.TempDir(), "no-cargo"), "GOOS": "linux", "GOARCH": "amd64", "GOPATINA_CACHE": t.TempDir()}
	s.Getenv = func(k string) string { return env[k] }
	if err := s.Build(golden("hello"), filepath.Join(t.TempDir(), "x")); err == nil || !strings.Contains(err.Error(), "-patina=check") {
		t.Errorf("err = %v", err)
	}
	if err := s.Build(reject("ownership"), ""); !errors.Is(err, ErrFailed) {
		t.Errorf("err = %v", err)
	}
	// The cache directory cannot be written.
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o644)
	env["GOPATINA_CACHE"] = file
	if err := s.Build(golden("hello"), ""); err == nil {
		t.Error("Build into an unwritable cache succeeded")
	}
	// The output cannot be written.
	s, _, _ = session(t, "")
	if err := s.Build(golden("hello"), filepath.Join(file, "x")); err == nil {
		t.Error("Build to an unwritable output succeeded")
	}
}

func TestCacheDir(t *testing.T) {
	s, _, _ := session(t, "")
	c, err := s.Emit(golden("hello"))
	if err != nil {
		t.Fatal(err)
	}
	s.Getenv = func(string) string { return "" }
	t.Setenv("HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("LocalAppData", "")
	if _, err := s.CacheDir(c); err == nil {
		t.Skip("this platform has a cache directory without HOME")
	}
	if err := s.Build(golden("hello"), ""); err == nil {
		t.Error("Build without a cache directory succeeded")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if d, err := s.CacheDir(c); err != nil || !strings.Contains(d, filepath.Join("gopatina", "crates", "hello-")) {
		t.Errorf("CacheDir = %q, %v", d, err)
	}
}

func TestVerify(t *testing.T) {
	s, stdout, _ := session(t, "")
	if err := s.Verify(golden("hello")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"emission:  deterministic", "toolchain: Rust " + emit.Toolchain, "verified:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestVerifyErrors(t *testing.T) {
	s, _, _ := session(t, "")
	if err := s.Verify(reject("ownership")); !errors.Is(err, ErrFailed) {
		t.Errorf("err = %v", err)
	}
	s, _, _ = session(t, "fail")
	if err := s.Verify(golden("hello")); err == nil {
		t.Error("Verify succeeded with a failing cargo")
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	s, _, _ = session(t, "")
	if err := s.Verify(golden("hello")); err == nil {
		t.Error("Verify succeeded without a temporary directory")
	}
}

func TestVerifyFailures(t *testing.T) {
	s, _, _ := session(t, "plainfail")
	if err := s.Verify(golden("hello")); err == nil {
		t.Error("Verify succeeded with a failing plain build")
	}
	s, _, _ = session(t, "")
	old := writeCrate
	t.Cleanup(func() { writeCrate = old })
	writeCrate = func(*emit.Crate, string) error { return errors.New("disk full") }
	if err := s.Verify(golden("hello")); err == nil || err.Error() != "disk full" {
		t.Errorf("err = %v", err)
	}
}

func TestCompare(t *testing.T) {
	var out bytes.Buffer
	s := &Session{Stdout: &out}
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(a, []byte("x"), 0o644)
	os.WriteFile(b, []byte("y"), 0o644)
	if err := s.compare(a, b); err == nil || !strings.Contains(err.Error(), "differ") {
		t.Errorf("err = %v", err)
	}
	if err := s.compare(a, filepath.Join(dir, "missing")); err == nil {
		t.Error("compare with a missing file succeeded")
	}
}

func TestSameManifests(t *testing.T) {
	s := &Session{}
	a := &emit.Crate{Files: []emit.File{{Path: "a", Data: []byte("1")}}}
	b := &emit.Crate{Files: []emit.File{{Path: "a", Data: []byte("2")}}}
	if err := s.sameEmission(a, b); err == nil || !strings.Contains(err.Error(), "not deterministic") {
		t.Errorf("err = %v", err)
	}
}

func TestSums(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	os.WriteFile(a, []byte("x"), 0o644)
	if _, _, err := sums(filepath.Join(dir, "missing"), a); err == nil {
		t.Error("sums of a missing file succeeded")
	}
	if x, y, err := sums(a, a); err != nil || x != y {
		t.Errorf("sums = %q, %q, %v", x, y, err)
	}
}

func TestExeSuffix(t *testing.T) {
	if exeSuffix("x86_64-pc-windows-msvc") != ".exe" || exeSuffix("x86_64-unknown-linux-gnu") != "" {
		t.Error("exeSuffix")
	}
}

func TestCopyFile(t *testing.T) {
	if err := copyFile(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "x")); err == nil {
		t.Error("copyFile of a missing file succeeded")
	}
}

func TestCargoOnPath(t *testing.T) {
	s, _, _ := session(t, "")
	env := map[string]string{"GOOS": "linux", "GOARCH": "amd64", "GOPATINA_CACHE": t.TempDir()}
	s.Getenv = func(k string) string { return env[k] }
	t.Setenv("PATH", t.TempDir())
	err := s.Build(golden("hello"), filepath.Join(t.TempDir(), "x"))
	if err == nil || !strings.Contains(err.Error(), "cargo was not found") {
		t.Errorf("err = %v", err)
	}
}

// TestBuildLibrary checks that -patina refuses a library, which has no
// binary, and names -emit-rust (RFC 0014).
func TestBuildLibrary(t *testing.T) {
	s, _, _ := session(t, "")
	err := s.Build(filepath.Join(root(), "testdata", "golden-lib", "wordstat"), filepath.Join(t.TempDir(), "bin"))
	if err == nil || err.Error() != "wordstat is a library, which has no binary: gopatina build -emit-rust writes its crate (RFC 0014)" {
		t.Errorf("Build of a library: %v", err)
	}
}

// TestAPICheck checks -api-check from a session: refused for a program, a
// missing earlier api.txt, a breaking change without a new version, and a
// clean check (RFC 0014).
func TestAPICheck(t *testing.T) {
	lib := filepath.Join(root(), "testdata", "golden-lib", "wordstat")
	committed := filepath.Join(lib, "rust", "api.txt")
	s, _, stderr := session(t, "")
	s.APICheck = committed
	if err := s.EmitTo(lib, filepath.Join(t.TempDir(), "same")); err != nil {
		t.Errorf("the committed API: %v", err)
	}
	if err := s.EmitTo(golden("hello"), filepath.Join(t.TempDir(), "prog")); err == nil || !strings.Contains(err.Error(), "applies to a library") {
		t.Errorf("a program: %v", err)
	}
	s.APICheck = filepath.Join(t.TempDir(), "missing.txt")
	if err := s.EmitTo(lib, filepath.Join(t.TempDir(), "x")); err == nil {
		t.Error("a missing api.txt")
	}
	data, _ := os.ReadFile(committed)
	old := filepath.Join(t.TempDir(), "api.txt")
	os.WriteFile(old, append(data, []byte("pub fn removed_later() -> i64\n")...), 0o644)
	s.APICheck = old
	err := s.EmitTo(lib, filepath.Join(t.TempDir(), "y"))
	if err == nil || !strings.Contains(err.Error(), "removes or changes 1 item(s)") || !strings.Contains(stderr.String(), "api: removed or changed: pub fn removed_later() -> i64") {
		t.Errorf("a removed item: %v\n%s", err, stderr.String())
	}
	s.CrateVersion = "v1.0.0"
	if err := s.EmitTo(lib, filepath.Join(t.TempDir(), "z")); err != nil {
		t.Errorf("a removed item in a new version: %v", err)
	}
	header, _, _ := strings.Cut(string(data), "\npub ")
	os.WriteFile(old, []byte(header+"\n"), 0o644) // an earlier API with no items
	stderr.Reset()
	if err := s.EmitTo(lib, filepath.Join(t.TempDir(), "w")); err != nil || !strings.Contains(stderr.String(), "api: added: pub fn top(") {
		t.Errorf("added items: %v\n%s", err, stderr.String())
	}
}
