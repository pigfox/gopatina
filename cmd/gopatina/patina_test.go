package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// When fakeCargoEnv is set, the test binary stands in for cargo: it copies
// itself to where cargo would put the crate's binary, so the "built"
// program is this test binary, which then runs as fakeProgram.
const (
	fakeCargoEnv   = "GOPATINA_TEST_FAKE_CARGO"
	fakeProgramEnv = "GOPATINA_TEST_FAKE_PROGRAM"
)

func init() {
	mode, cargo := os.LookupEnv(fakeCargoEnv)
	if cargo && strings.Join(os.Args[1:], " ") == "build --release --locked" {
		os.Exit(fakeCargo(mode))
	}
	if os.Getenv(fakeProgramEnv) != "" {
		fmt.Printf("program args=%q\n", os.Args[1:])
		os.Exit(7)
	}
}

func fakeCargo(mode string) int {
	if mode == "fail" {
		fmt.Fprintln(os.Stderr, "fake rustc error")
		return 101
	}
	cfg, _ := os.ReadFile(".cargo/config.toml")
	toml, _ := os.ReadFile("Cargo.toml")
	_, rest, _ := strings.Cut(string(cfg), `target = "`)
	target, _, _ := strings.Cut(rest, `"`)
	_, rest, _ = strings.Cut(string(toml), `name = "`)
	name, _, _ := strings.Cut(rest, `"`)
	exe, _ := os.Executable()
	data, _ := os.ReadFile(exe)
	dir := filepath.Join("target", target, "release")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, name), data, 0o755)
	return 0
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func golden(name string) string { return filepath.Join(repoRoot(), "testdata", "golden", name) }

// useFakeCargo makes the patina modes run this test binary as cargo.
func useFakeCargo(t *testing.T, mode string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake cargo writes a Unix binary name")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPATINA_CARGO", exe)
	t.Setenv(fakeCargoEnv, mode)
	t.Setenv("GOPATINA_CACHE", t.TempDir())
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "amd64")
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"build", "-patina=fast", "."}, "gopatina build: unknown patina mode -patina=fast (want -patina or -patina=check)\n"},
		{[]string{"build", "-patina", "-patina=check", "."}, "gopatina build: -patina and -patina=check cannot be used together\n"},
		{[]string{"build", "-patina", "-race", "."}, "gopatina build -patina: flag -race is not supported in patina mode yet\n"},
		{[]string{"run", "-emit-rust", "."}, "gopatina run: -emit-rust is supported only with build (it writes a crate and stops)\n"},
		{[]string{"build", "-patina", "a", "b"}, "gopatina build -patina: one package directory at a time in patina mode (got a b)\n"},
		{[]string{"build", "-emit-rust", "-crate-name", "Bad", "."}, "gopatina build: -crate-name \"Bad\" is not a crate name (lowercase letters, digits and _, starting with a letter)\n"},
		{[]string{"build", "-emit-rust", "-crate-name=9x", "."}, "gopatina build: -crate-name \"9x\" is not a crate name (lowercase letters, digits and _, starting with a letter)\n"},
		{[]string{"build", "-emit-rust", "-version", "1.2", "."}, "gopatina build: -version \"1.2\" is not a version vX.Y.Z\n"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			useFakeGo(t, 0) // must not be run
			r := runCLI(t, "", tt.args...)
			if r.code != exitUsage || r.stderr != tt.want || r.stdout != "" {
				t.Errorf("got %+v, want exit %d and stderr %q", r, exitUsage, tt.want)
			}
		})
	}
}

// -static emits the crate for Rust's musl target, linked by rust-lld.
func TestEmitRustStatic(t *testing.T) {
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "arm64")
	out := filepath.Join(t.TempDir(), "crate")
	r := runCLI(t, "", "build", "-emit-rust", "-static", "-o", out, golden("hello"))
	if r.code != exitOK {
		t.Fatalf("got %+v", r)
	}
	cfg, err := os.ReadFile(filepath.Join(out, ".cargo", "config.toml"))
	if err != nil || !strings.Contains(string(cfg), `target = "aarch64-unknown-linux-musl"`) || !strings.Contains(string(cfg), `linker = "rust-lld"`) {
		t.Errorf(".cargo/config.toml = %q, %v", cfg, err)
	}
	t.Setenv("GOOS", "windows")
	r = runCLI(t, "", "build", "--static", "-emit-rust", "-o", filepath.Join(t.TempDir(), "w"), golden("hello"))
	if r.code == exitOK || !strings.Contains(r.stderr, "has no static target") {
		t.Errorf("-static on windows: %+v", r)
	}
}

func TestEmitRust(t *testing.T) {
	out := filepath.Join(t.TempDir(), "crate")
	r := runCLI(t, "", "build", "-emit-rust", "-explain", "-o", out, golden("hello"))
	if r.code != exitOK {
		t.Fatalf("got %+v", r)
	}
	if !strings.Contains(r.stderr, "copied names[0] into first") || !strings.Contains(r.stderr, "wrote crate") {
		t.Errorf("stderr = %q", r.stderr)
	}
	want, _ := os.ReadFile(filepath.Join(golden("hello"), "rust", "src", "main.rs"))
	got, err := os.ReadFile(filepath.Join(out, "src", "main.rs"))
	if err != nil || string(got) != string(want) {
		t.Errorf("src/main.rs differs from the golden crate: %v", err)
	}
	// The default directory is the package's rust directory, here the
	// committed golden crate, which gopatina wrote and may write again.
	r = runCLI(t, "", "build", "--emit-rust", "-o="+out, golden("hello"))
	if r.code != exitOK {
		t.Errorf("got %+v", r)
	}
}

func TestEmitRustDefaultDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	r := runCLI(t, "", "build", "-emit-rust", dir)
	if r.code != exitOK {
		t.Fatalf("got %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "rust", "Cargo.toml")); err != nil {
		t.Error(err)
	}
}

func TestRejectedProgram(t *testing.T) {
	r := runCLI(t, "", "build", "-patina", filepath.Join(repoRoot(), "testdata", "reject", "ownership"))
	if r.code != exitFailure || !strings.Contains(r.stderr, "m.Body is changed after m was handed to out.Queue on line 32") {
		t.Errorf("got %+v", r)
	}
	if strings.Contains(r.stderr, "gopatina:") {
		t.Errorf("the diagnostics are printed once, without a gopatina: prefix:\n%s", r.stderr)
	}
}

func TestBuildPatina(t *testing.T) {
	useFakeCargo(t, "")
	out := filepath.Join(t.TempDir(), "hello")
	if r := runCLI(t, "", "build", "-patina", "-o", out, golden("hello")); r.code != exitOK {
		t.Fatalf("got %+v", r)
	}
	if _, err := os.Stat(out); err != nil {
		t.Error(err)
	}
}

func TestBuildPatinaFails(t *testing.T) {
	useFakeCargo(t, "fail")
	r := runCLI(t, "", "build", "-patina", "-o", filepath.Join(t.TempDir(), "x"), golden("hello"))
	if r.code != exitFailure || !strings.Contains(r.stderr, "gopatina: cargo build failed") {
		t.Errorf("got %+v", r)
	}
}

func TestRunPatina(t *testing.T) {
	useFakeCargo(t, "")
	t.Setenv(fakeProgramEnv, "1")
	// The fake program is this test binary; fakeProgramEnv makes it print
	// its arguments and exit with status 7, which run returns.
	r := runCLI(t, "", "run", "-patina", golden("hello"), "a", "b")
	if r.code != 7 || r.stdout != "program args=[\"a\" \"b\"]\n" {
		t.Errorf("got %+v", r)
	}
}

func TestRunPatinaFails(t *testing.T) {
	useFakeCargo(t, "fail")
	if r := runCLI(t, "", "run", "-patina", golden("hello")); r.code != exitFailure {
		t.Errorf("got %+v", r)
	}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if r := runCLI(t, "", "run", "-patina", golden("hello")); r.code != exitFailure {
		t.Errorf("without a temporary directory: got %+v", r)
	}
}

func TestPatinaCheck(t *testing.T) {
	tests := []struct {
		args   []string
		goArgs []string
	}{
		{[]string{"build", "-patina=check", "-o", "out", golden("hello")}, []string{"build", "-tags", "patina", "-o", "out", golden("hello")}},
		{[]string{"run", "--patina=check", golden("hello"), "x"}, []string{"run", "-tags", "patina", golden("hello"), "x"}},
	}
	for _, tt := range tests {
		useFakeGo(t, 0)
		r := runCLI(t, "", tt.args...)
		if r.code != exitOK || !strings.HasPrefix(r.stdout, fmt.Sprintf("args=%q", tt.goArgs)) {
			t.Errorf("got %+v, want the go command to run with %q", r, tt.goArgs)
		}
		if !strings.Contains(r.stderr, "patina checks passed") || !strings.Contains(r.stderr, "with the Go compiler (garbage collected, not Rust)") {
			t.Errorf("stderr = %q", r.stderr)
		}
	}
	useFakeGo(t, 0)
	r := runCLI(t, "", "build", "-patina=check", filepath.Join(repoRoot(), "testdata", "reject", "ownership"))
	if r.code != exitFailure || strings.Contains(r.stdout, "args=") {
		t.Errorf("a rejected program must not reach the Go compiler: %+v", r)
	}
}

func TestVerify(t *testing.T) {
	useFakeCargo(t, "")
	r := runCLI(t, "", "verify", "-explain", golden("hello"))
	if r.code != exitOK || !strings.Contains(r.stdout, "verified:") {
		t.Errorf("got %+v", r)
	}
	cwd, _ := os.Getwd()
	t.Chdir(golden("hello"))
	if r := runCLI(t, "", "verify"); r.code != exitOK {
		t.Errorf("verify in the package directory: %+v", r)
	}
	t.Chdir(cwd)
	if r := runCLI(t, "", "verify", "a", "b"); r.code != exitUsage {
		t.Errorf("got %+v", r)
	}
}

func TestProgramThatCannotRun(t *testing.T) {
	r := result{}
	var stdout, stderr strings.Builder
	r.code = runCommand("the program", filepath.Join(t.TempDir(), "missing"), nil, nil, &stdout, &stderr)
	if r.code != exitFailure || !strings.HasPrefix(stderr.String(), "gopatina: cannot run the program: ") {
		t.Errorf("exit %d, stderr %q", r.code, stderr.String())
	}
}

// TestEmitLibrary emits a library crate with -crate-name and -version, in
// both spellings (RFC 0014).
func TestEmitLibrary(t *testing.T) {
	lib := filepath.Join(repoRoot(), "testdata", "golden-lib", "wordstat")
	for _, args := range [][]string{
		{"-crate-name", "words", "-version", "v1.2.3"},
		{"-crate-name=words", "-version=1.2.3", "-api-check=" + filepath.Join(lib, "rust", "api.txt")},
		{"-crate-name", "words", "-version", "v1.2.3", "-api-check", filepath.Join(lib, "rust", "api.txt")},
	} {
		out := filepath.Join(t.TempDir(), "crate")
		r := runCLI(t, "", append(append([]string{"build", "-emit-rust"}, args...), "-o", out, lib)...)
		if r.code != exitOK {
			t.Fatalf("%v: got %+v", args, r)
		}
		toml, _ := os.ReadFile(filepath.Join(out, "Cargo.toml"))
		if !strings.Contains(string(toml), "name = \"words\"") || !strings.Contains(string(toml), "version = \"1.2.3\"") {
			t.Errorf("%v: Cargo.toml:\n%s", args, toml)
		}
		if _, err := os.Stat(filepath.Join(out, "api.txt")); err != nil {
			t.Error(err)
		}
	}
}
