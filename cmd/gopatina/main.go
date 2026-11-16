// Command gopatina is the GoPatina command-line tool.
//
// Without a patina flag, every subcommand is passed unchanged to the go
// command, so gopatina behaves exactly like the installed Go toolchain:
//
//	gopatina build ./...
//	gopatina test ./...
//
// Two flags, recognised only directly after the build or run subcommand,
// select the patina modes described in docs/rfcs/0001-gopatina.md:
//
//	gopatina build -patina ./cmd/app     native binary, no garbage collector
//	gopatina build -emit-rust ./cmd/app  readable Rust source (a cargo crate)
//
// Neither patina mode is implemented yet; both report so and exit with
// status 2.
//
// The go command is found on PATH, or taken from the GOPATINA_GO environment
// variable when it is set.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// version is the gopatina version. Release builds set it with
// -ldflags "-X main.version=v0.0.1".
var version = "devel"

// exit is os.Exit, replaceable in tests.
var exit = os.Exit

// Exit statuses of gopatina itself. When the go command runs, its exit
// status is returned unchanged.
const (
	exitOK       = 0
	exitFailure  = 1 // the go command could not be started
	exitUsage    = 2 // bad usage, or a mode that is not implemented yet
	envGoCommand = "GOPATINA_GO"
)

// mode is a build mode selected by a patina flag.
type mode struct {
	flag string // the flag as documented, e.g. "-patina"
	name string // what the mode produces, for messages
}

var (
	patinaMode   = mode{flag: "-patina", name: "patina native backend"}
	emitRustMode = mode{flag: "-emit-rust", name: "Rust source emitter"}
)

// modeFlags maps every accepted spelling of a patina flag to its mode.
var modeFlags = map[string]mode{
	"-patina":     patinaMode,
	"--patina":    patinaMode,
	"-emit-rust":  emitRustMode,
	"--emit-rust": emitRustMode,
}

const usage = `GoPatina: Go on the surface, Rust underneath.

Usage:

	gopatina <command> [arguments]

Without a patina flag, every command is passed to the go command unchanged,
so gopatina behaves exactly like the installed Go toolchain.

Patina flags (directly after build or run; not implemented yet):

	-patina      compile a defined subset of Go to a native binary that
	             behaves like Rust under the hood: no garbage collector,
	             ownership and borrowing, Rust-grade memory safety
	-emit-rust   translate the same subset to readable Rust source
	             (a cargo crate) that you can keep, edit and build with cargo

Commands handled by gopatina itself:

	version      print the gopatina version
	help         print this help

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
	default:
		modes := leadingModes(args[1:])
		if len(modes) == 0 {
			return goTool(args, stdin, stdout, stderr)
		}
		return patina(cmd, modes, stderr)
	}
}

// leadingModes returns the modes selected by the patina flags at the start
// of args, in order. Flags later in args are left to the go command.
func leadingModes(args []string) []mode {
	var modes []mode
	for _, arg := range args {
		m, ok := modeFlags[arg]
		if !ok {
			break
		}
		modes = append(modes, m)
	}
	return modes
}

// patina handles a command that carries at least one patina flag.
func patina(cmd string, modes []mode, stderr io.Writer) int {
	if cmd != "build" && cmd != "run" {
		fmt.Fprintf(stderr, "gopatina %s: %s is supported only with build and run\n", cmd, modes[0].flag)
		return exitUsage
	}
	for _, m := range modes[1:] {
		if m != modes[0] {
			fmt.Fprintf(stderr, "gopatina %s: %s and %s cannot be used together\n", cmd, modes[0].flag, m.flag)
			return exitUsage
		}
	}
	fmt.Fprintf(stderr, "gopatina %s %s: %s not implemented yet (see RFC 0001)\n", cmd, modes[0].flag, modes[0].name)
	return exitUsage
}

// goTool runs the go command with args, connected to the given streams and
// the current environment, and returns its exit status.
func goTool(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	name := os.Getenv(envGoCommand)
	if name == "" {
		name = "go"
	}
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
		fmt.Fprintf(stderr, "gopatina: cannot run the go command: %v\n", err)
		return exitFailure
	}
}
