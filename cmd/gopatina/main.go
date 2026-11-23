// Command gopatina is the GoPatina command-line tool.
//
// Without a patina flag, every subcommand is passed unchanged to the go
// command, so gopatina behaves exactly like the installed Go toolchain:
//
//	gopatina build ./...
//	gopatina test ./...
//
// The patina flags, given directly after the build or run subcommand,
// select the patina modes described in docs/rfcs/0001-gopatina.md:
//
//	gopatina build -emit-rust -o crate ./cmd/app   readable Rust source (a cargo crate)
//	gopatina build -patina -o app ./cmd/app        native binary built by rustc, no GC
//	gopatina run -patina ./cmd/app                 build and run it
//	gopatina build -patina=check ./cmd/app         patina checks, then the Go compiler
//	gopatina build -patina -explain ./cmd/app      also report every automatic fix
//	gopatina verify ./cmd/app                      check the build is reproducible
//
// Phase 1 accepts a defined subset of Go; see docs/patina-subset.md.
//
// The go command is found on PATH, or taken from the GOPATINA_GO environment
// variable when it is set; cargo likewise, or GOPATINA_CARGO.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/pigfox/gopatina/internal/patina"
)

// version is the gopatina version. Release builds set it with
// -ldflags "-X main.version=v0.0.1".
var version = "devel"

// exit is os.Exit, replaceable in tests.
var exit = os.Exit

// Exit statuses of gopatina itself. When the go command or a program built
// in patina mode runs, its exit status is returned unchanged.
const (
	exitOK       = 0
	exitFailure  = 1 // a build failed, or the go command could not be started
	exitUsage    = 2 // bad usage
	envGoCommand = "GOPATINA_GO"
)

// A mode is a build mode selected by a patina flag.
type mode struct {
	flag string // the flag as documented, e.g. "-patina"
}

var (
	patinaMode   = mode{"-patina"}
	checkMode    = mode{"-patina=check"}
	emitRustMode = mode{"-emit-rust"}
)

// modeFlags maps every accepted spelling of a patina flag to its mode.
var modeFlags = map[string]mode{
	"-patina":        patinaMode,
	"--patina":       patinaMode,
	"-patina=check":  checkMode,
	"--patina=check": checkMode,
	"-emit-rust":     emitRustMode,
	"--emit-rust":    emitRustMode,
}

const usage = `GoPatina: Go on the surface, Rust underneath.

Usage:

	gopatina <command> [arguments]

Without a patina flag, every command is passed to the go command unchanged,
so gopatina behaves exactly like the installed Go toolchain.

Patina flags (directly after build or run), for the subset of Go described
in docs/patina-subset.md:

	-emit-rust      translate the package to a readable Rust crate and stop
	                (build only; -o names the crate directory, default ./rust
	                in the package directory)
	-patina         compile the package to a native binary that behaves like
	                Rust under the hood: no garbage collector, ownership and
	                borrowing, Rust-grade memory safety (needs a Rust toolchain)
	-patina=check   run the patina checks, then build with the Go compiler
	                (the binary has Go's garbage collector; no Rust needed)
	-explain        also report every decision the borrow checker made on its
	                own: each move, copy and borrow
	-o path         the output file (or crate directory with -emit-rust)
	-crate-name n   the crate's name (default: for a library, the last element
	                of its import path, with - as _; RFC 0014)
	-version vX.Y.Z the crate's version (default 0.0.0)

Commands handled by gopatina itself:

	verify [pkg]    check that emitting the package is deterministic and that
	                the -patina binary is byte-identical to a plain
	                cargo build --release --locked of the emitted crate
	version         print the gopatina version
	help            print this help

The design is described in RFC 0001:
https://github.com/pigfox/gopatina/blob/main/docs/rfcs/0001-gopatina.md
`

func main() {
	exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes gopatina with args (without the program name) and returns
// the process exit status.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	switch cmd := args[0]; cmd {
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "version":
		fmt.Fprintf(stdout, "gopatina version %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		return exitOK
	case "verify":
		return verify(args[1:], stdout, stderr)
	case "build", "run":
		inv, ok, code := parse(cmd, args[1:], stderr)
		if !ok {
			return goTool(args, stdin, stdout, stderr)
		}
		if code >= 0 {
			return code
		}
		return inv.exec(stdin, stdout, stderr)
	default:
		if m, ok := modeFlags[firstArg(args[1:])]; ok {
			fmt.Fprintf(stderr, "gopatina %s: %s is supported only with build and run\n", cmd, m.flag)
			return exitUsage
		}
		return goTool(args, stdin, stdout, stderr)
	}
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// An invocation is a build or run in a patina mode.
type invocation struct {
	cmd     string // "build" or "run"
	mode    mode
	explain bool
	out     string
	pkg     string
	args    []string // the program's arguments (run)
	// The crate's name and version (-crate-name, -version; RFC 0014).
	crateName, crateVersion string
}

var (
	crateNameRE    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	crateVersionRE = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)
)

// parse reads the patina flags of build or run. ok is false when there is
// no patina flag, so the command is the go command's. code is an exit
// status when the arguments are wrong, and -1 otherwise.
func parse(cmd string, args []string, stderr io.Writer) (inv invocation, ok bool, code int) {
	inv = invocation{cmd: cmd, pkg: "."}
	var modes []mode
	var unknown string
	i := 0
loop:
	for ; i < len(args); i++ {
		a := args[i]
		switch m, isMode := modeFlags[a]; {
		case isMode:
			modes = append(modes, m)
		case a == "-explain" || a == "--explain":
			inv.explain = true
		case (a == "-o" || a == "--o") && i+1 < len(args):
			inv.out = args[i+1]
			i++
		case strings.HasPrefix(a, "-o=") || strings.HasPrefix(a, "--o="):
			_, inv.out, _ = strings.Cut(a, "=")
		case (a == "-crate-name" || a == "-version") && i+1 < len(args):
			if !inv.crateFlag(cmd, a, args[i+1], stderr) {
				return inv, true, exitUsage
			}
			i++
		case strings.HasPrefix(a, "-crate-name=") || strings.HasPrefix(a, "-version="):
			name, value, _ := strings.Cut(a, "=")
			if !inv.crateFlag(cmd, name, value, stderr) {
				return inv, true, exitUsage
			}
		case strings.HasPrefix(a, "-patina=") || strings.HasPrefix(a, "--patina="):
			fmt.Fprintf(stderr, "gopatina %s: unknown patina mode %s (want -patina or -patina=check)\n", cmd, a)
			return inv, true, exitUsage
		case strings.HasPrefix(a, "-"):
			if unknown == "" {
				unknown = a
			}
			if len(modes) == 0 {
				break loop
			}
		default:
			break loop
		}
	}
	if len(modes) == 0 {
		return inv, false, -1
	}
	for _, m := range modes[1:] {
		if m != modes[0] {
			fmt.Fprintf(stderr, "gopatina %s: %s and %s cannot be used together\n", cmd, modes[0].flag, m.flag)
			return inv, true, exitUsage
		}
	}
	inv.mode = modes[0]
	if unknown != "" {
		fmt.Fprintf(stderr, "gopatina %s %s: flag %s is not supported in patina mode yet\n", cmd, inv.mode.flag, unknown)
		return inv, true, exitUsage
	}
	if inv.mode == emitRustMode && cmd == "run" {
		fmt.Fprintf(stderr, "gopatina run: -emit-rust is supported only with build (it writes a crate and stops)\n")
		return inv, true, exitUsage
	}
	rest := args[i:]
	if len(rest) > 0 {
		inv.pkg, inv.args = rest[0], rest[1:]
	}
	if cmd == "build" && len(inv.args) > 0 {
		fmt.Fprintf(stderr, "gopatina build %s: one package directory at a time in patina mode (got %s)\n", inv.mode.flag, strings.Join(rest, " "))
		return inv, true, exitUsage
	}
	return inv, true, -1
}

// crateFlag sets -crate-name or -version, or reports a bad value.
func (inv *invocation) crateFlag(cmd, flag, value string, stderr io.Writer) bool {
	if flag == "-crate-name" {
		if !crateNameRE.MatchString(value) {
			fmt.Fprintf(stderr, "gopatina %s: -crate-name %q is not a crate name (lowercase letters, digits and _, starting with a letter)\n", cmd, value)
			return false
		}
		inv.crateName = value
		return true
	}
	if !crateVersionRE.MatchString(value) {
		fmt.Fprintf(stderr, "gopatina %s: -version %q is not a version vX.Y.Z\n", cmd, value)
		return false
	}
	inv.crateVersion = value
	return true
}

func newSession(stdout, stderr io.Writer, explain bool) *patina.Session {
	cwd, _ := os.Getwd()
	return &patina.Session{Cwd: cwd, Stdout: stdout, Stderr: stderr, Explain: explain, Version: version, Getenv: os.Getenv}
}

// exec runs the invocation.
func (inv invocation) exec(stdin io.Reader, stdout, stderr io.Writer) int {
	s := newSession(stdout, stderr, inv.explain)
	s.CrateName, s.CrateVersion = inv.crateName, inv.crateVersion
	switch {
	case inv.mode == emitRustMode:
		out := inv.out
		if out == "" {
			out = filepath.Join(inv.pkg, "rust")
		}
		return status(stderr, s.EmitTo(inv.pkg, out))
	case inv.mode == checkMode:
		if _, _, err := s.Check(inv.pkg); err != nil {
			return status(stderr, err)
		}
		fmt.Fprintf(stderr, "gopatina: patina checks passed; %s with the Go compiler (garbage collected, not Rust)\n", map[string]string{"build": "building", "run": "running"}[inv.cmd])
		goArgs := []string{inv.cmd, "-tags", "patina"}
		if inv.out != "" {
			goArgs = append(goArgs, "-o", inv.out)
		}
		goArgs = append(append(goArgs, inv.pkg), inv.args...)
		return goTool(goArgs, stdin, stdout, stderr)
	case inv.cmd == "build":
		return status(stderr, s.Build(inv.pkg, inv.out))
	}
	// run -patina: build into a temporary directory and run the binary.
	tmp, err := os.MkdirTemp("", "gopatina-run-")
	if err != nil {
		return status(stderr, err)
	}
	defer os.RemoveAll(tmp)
	bin := filepath.Join(tmp, "program")
	if err := s.Build(inv.pkg, bin); err != nil {
		return status(stderr, err)
	}
	return runCommand("the program", bin, inv.args, stdin, stdout, stderr)
}

// verify runs gopatina verify [-explain] [pkg].
func verify(args []string, stdout, stderr io.Writer) int {
	explain := false
	if len(args) > 0 && (args[0] == "-explain" || args[0] == "--explain") {
		explain, args = true, args[1:]
	}
	pkg := "."
	switch len(args) {
	case 0:
	case 1:
		pkg = args[0]
	default:
		fmt.Fprintf(stderr, "usage: gopatina verify [-explain] [package directory]\n")
		return exitUsage
	}
	return status(stderr, newSession(stdout, stderr, explain).Verify(pkg))
}

// status prints err, if it is not already printed, and returns the exit
// status for it.
func status(stderr io.Writer, err error) int {
	switch {
	case err == nil:
		return exitOK
	case !errors.Is(err, patina.ErrFailed):
		fmt.Fprintf(stderr, "gopatina: %v\n", err)
	}
	return exitFailure
}

// goTool runs the go command with args, connected to the given streams and
// the current environment, and returns its exit status.
func goTool(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	name := os.Getenv(envGoCommand)
	if name == "" {
		name = "go"
	}
	return runCommand("the go command", name, args, stdin, stdout, stderr)
}

// runCommand runs name with args and returns its exit status. what names
// the command in an error message.
func runCommand(what, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr

	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &exitErr):
		return exitErr.ExitCode()
	default:
		fmt.Fprintf(stderr, "gopatina: cannot run %s: %v\n", what, err)
		return exitFailure
	}
}
