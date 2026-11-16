package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// When fakeGoEnv is set, the test binary acts as a stand-in for the go
// command: it prints its arguments and a line read from stdin, writes a
// marker to stderr, and exits with the status in fakeGoEnv.
const fakeGoEnv = "GOPATINA_TEST_FAKE_GO_EXIT"

func TestMain(m *testing.M) {
	if code, ok := os.LookupEnv(fakeGoEnv); ok {
		os.Exit(fakeGo(code, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func fakeGo(code string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	in, _ := io.ReadAll(stdin)
	fmt.Fprintf(stdout, "args=%q stdin=%q\n", args, in)
	fmt.Fprintln(stderr, "fake go stderr")
	n, _ := strconv.Atoi(code)
	return n
}

// useFakeGo makes goTool run this test binary as the go command, exiting
// with status code.
func useFakeGo(t *testing.T, code int) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(envGoCommand, exe)
	t.Setenv(fakeGoEnv, strconv.Itoa(code))
}

type result struct {
	code           int
	stdout, stderr string
}

func runCLI(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(stdin), &stdout, &stderr)
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestNoArgumentsPrintsUsageAndFails(t *testing.T) {
	r := runCLI(t, "")
	if r.code != exitUsage {
		t.Errorf("exit = %d, want %d", r.code, exitUsage)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if !strings.HasPrefix(r.stderr, "GoPatina: Go on the surface, Rust underneath.") {
		t.Errorf("stderr does not start with the usage text:\n%s", r.stderr)
	}
}

func TestHelp(t *testing.T) {
	for _, arg := range []string{"help", "-h", "-help", "--help"} {
		t.Run(arg, func(t *testing.T) {
			r := runCLI(t, "", arg)
			if r.code != exitOK {
				t.Errorf("exit = %d, want %d", r.code, exitOK)
			}
			if r.stdout != usage {
				t.Errorf("stdout is not the usage text:\n%s", r.stdout)
			}
			for _, want := range []string{"-patina", "-emit-rust", "RFC 0001"} {
				if !strings.Contains(r.stdout, want) {
					t.Errorf("usage does not mention %q", want)
				}
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v9.9.9-test"

	r := runCLI(t, "", "version")
	want := fmt.Sprintf("gopatina version v9.9.9-test %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if r.code != exitOK || r.stdout != want || r.stderr != "" {
		t.Errorf("got %+v, want exit 0 and stdout %q", r, want)
	}
}

func TestPatinaModesAreNotImplemented(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"build", "-patina", "./..."}, "gopatina build -patina: patina native backend not implemented yet (see RFC 0001)\n"},
		{[]string{"build", "--patina"}, "gopatina build -patina: patina native backend not implemented yet (see RFC 0001)\n"},
		{[]string{"run", "-patina", "."}, "gopatina run -patina: patina native backend not implemented yet (see RFC 0001)\n"},
		{[]string{"build", "-emit-rust", "./cmd/app"}, "gopatina build -emit-rust: Rust source emitter not implemented yet (see RFC 0001)\n"},
		{[]string{"run", "--emit-rust"}, "gopatina run -emit-rust: Rust source emitter not implemented yet (see RFC 0001)\n"},
		{[]string{"build", "-patina", "--patina"}, "gopatina build -patina: patina native backend not implemented yet (see RFC 0001)\n"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			useFakeGo(t, 0) // must not be run
			r := runCLI(t, "", tt.args...)
			if r.code != exitUsage {
				t.Errorf("exit = %d, want %d", r.code, exitUsage)
			}
			if r.stderr != tt.want {
				t.Errorf("stderr = %q, want %q", r.stderr, tt.want)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty (the go command must not run)", r.stdout)
			}
		})
	}
}

func TestPatinaAndEmitRustAreExclusive(t *testing.T) {
	r := runCLI(t, "", "build", "-patina", "-emit-rust", ".")
	want := "gopatina build: -patina and -emit-rust cannot be used together\n"
	if r.code != exitUsage || r.stderr != want {
		t.Errorf("got %+v, want exit %d and stderr %q", r, exitUsage, want)
	}
}

func TestPatinaFlagOnlyWithBuildAndRun(t *testing.T) {
	r := runCLI(t, "", "test", "-emit-rust", "./...")
	want := "gopatina test: -emit-rust is supported only with build and run\n"
	if r.code != exitUsage || r.stderr != want {
		t.Errorf("got %+v, want exit %d and stderr %q", r, exitUsage, want)
	}
}

func TestDefaultModePassesEverythingToGo(t *testing.T) {
	tests := [][]string{
		{"build", "-o", "out", "./..."},
		{"run", "."},
		{"test", "-race", "./..."},
		{"env", "GOOS"},
		// A patina flag that is not directly after the subcommand is the go
		// command's to interpret (here, it would reject it).
		{"build", "-v", "-patina"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			useFakeGo(t, 0)
			r := runCLI(t, "from stdin", args...)
			if r.code != exitOK {
				t.Errorf("exit = %d, want %d (stderr %q)", r.code, exitOK, r.stderr)
			}
			want := fmt.Sprintf("args=%q stdin=%q\n", args, "from stdin")
			if r.stdout != want {
				t.Errorf("stdout = %q, want %q", r.stdout, want)
			}
			if r.stderr != "fake go stderr\n" {
				t.Errorf("stderr = %q, want the go command's stderr", r.stderr)
			}
		})
	}
}

func TestDefaultModeReturnsGoExitStatus(t *testing.T) {
	useFakeGo(t, 3)
	if r := runCLI(t, "", "vet", "./..."); r.code != 3 {
		t.Errorf("exit = %d, want 3", r.code)
	}
}

func TestDefaultModeUsesGoOnPath(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := "go"
	if runtime.GOOS == "windows" {
		name = "go.exe"
	}
	if err := os.Symlink(exe, filepath.Join(dir, name)); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	t.Setenv("PATH", dir)
	t.Setenv(envGoCommand, "")
	t.Setenv(fakeGoEnv, "0")

	r := runCLI(t, "", "env")
	if r.code != exitOK || r.stdout != fmt.Sprintf("args=%q stdin=%q\n", []string{"env"}, "") {
		t.Errorf("got %+v, want the go command from PATH to run", r)
	}
}

func TestMissingGoCommand(t *testing.T) {
	t.Setenv(envGoCommand, filepath.Join(t.TempDir(), "no-such-go"))
	r := runCLI(t, "", "build", ".")
	if r.code != exitFailure {
		t.Errorf("exit = %d, want %d", r.code, exitFailure)
	}
	if !strings.HasPrefix(r.stderr, "gopatina: cannot run the go command: ") {
		t.Errorf("stderr = %q", r.stderr)
	}
}

func TestMainExitsWithRunStatus(t *testing.T) {
	oldExit, oldArgs, oldStdout := exit, os.Args, os.Stdout
	t.Cleanup(func() { exit, os.Args, os.Stdout = oldExit, oldArgs, oldStdout })

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { devNull.Close() })
	os.Stdout = devNull

	got := -1
	exit = func(code int) { got = code }
	os.Args = []string{"gopatina", "version"}
	main()
	if got != exitOK {
		t.Errorf("main exited with %d, want %d", got, exitOK)
	}
}
