// Package patina runs GoPatina's pipeline for the patina modes: load the Go
// package, check the subset (stage 1), run the borrow checker (stage 3),
// emit the Rust crate (stage 4), and build it with cargo.
package patina

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/diag"
	"github.com/pigfox/gopatina/internal/emit"
	"github.com/pigfox/gopatina/internal/fma"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

// EnvCargo names the cargo command; by default cargo on PATH.
const EnvCargo = "GOPATINA_CARGO"

// Session holds what every step needs.
type Session struct {
	Cwd     string
	Stdout  io.Writer
	Stderr  io.Writer
	Explain bool   // report every automatic decision of the borrow checker
	Version string // the GoPatina version, recorded in Cargo.toml
	// CrateName and CrateVersion set the emitted crate's name and version
	// (-crate-name, -version; RFC 0014); empty for the defaults.
	CrateName, CrateVersion string
	// APICheck is an earlier api.txt a library's new API is checked
	// against (-api-check; RFC 0014), or "".
	APICheck string
	// Static builds a fully static binary on Linux (-static): Rust's musl
	// target, linked by rust-lld with Rust's own musl runtime (RFC 0001).
	Static bool
	// Getenv reads the environment (os.Getenv, replaceable in tests).
	Getenv func(string) string
}

// ErrFailed is returned when diagnostics were printed.
var ErrFailed = errors.New("patina checks failed")

// Check loads the package in dir and runs the subset checker and the
// borrow checker. Errors are printed to s.Stderr in Go style and ErrFailed
// is returned; with s.Explain, every automatic decision is printed too.
func (s *Session) Check(dir string) (*load.Package, *borrow.Result, error) {
	pkg, err := load.Dir(dir, s.Cwd)
	if err != nil {
		var de *diag.Error
		if errors.As(err, &de) {
			fmt.Fprintln(s.Stderr, de.Error())
			return nil, nil, ErrFailed
		}
		return nil, nil, err
	}
	s.findFused(pkg)
	if ds := subset.Check(pkg); len(ds) > 0 {
		s.printDiags(ds)
		return nil, nil, ErrFailed
	}
	res, ds := borrow.Check(pkg)
	if s.Explain {
		for _, f := range res.Fixes {
			fmt.Fprintf(s.Stderr, "%s:%d:%d: %s\n", diag.RelPath(s.Cwd, f.Pos.Filename), f.Pos.Line, f.Pos.Column, f.Text)
		}
	}
	if len(ds) > 0 {
		s.printDiags(ds)
		return nil, nil, ErrFailed
	}
	return pkg, res, nil
}

// findFused asks gc where it fuses float multiply-adds in pkg for the
// target (package fma), when the target fuses at all: pkg.Fused, or
// pkg.FusedErr when gc cannot say, and the subset checker then refuses
// what gc may fuse.
func (s *Session) findFused(pkg *load.Package) {
	t, err := s.FMATarget()
	if err != nil {
		pkg.FusedErr = err
		return
	}
	pkg.Target = t
	if !t.Fuses() {
		return
	}
	pkg.Fused, pkg.FusedErr = findFused(goCommand, os.Environ(), pkg.Dir, t)
}

// findFused is fma.Find; tests replace it.
var findFused = fma.Find

// goCommand is the go command gopatina runs.
const goCommand = "go"

// FMATarget returns what gc's float multiply-add fusion depends on for the
// build: the Go version, GOOS and GOARCH (as Target), and, on amd64,
// GOAMD64 from the environment or, when unset, the go command's default.
func (s *Session) FMATarget() (fma.Target, error) {
	goos, goarch := s.Getenv("GOOS"), s.Getenv("GOARCH")
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	cmd := exec.Command(goCommand, "env", "GOVERSION", "GOAMD64")
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)
	out, err := goEnv(cmd)
	if err != nil {
		return fma.Target{}, fmt.Errorf("go env: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	t := fma.Target{GoVersion: lines[0], GOOS: goos, GOARCH: goarch}
	if goarch == "amd64" {
		t.GOAMD64 = s.Getenv("GOAMD64")
		if t.GOAMD64 == "" && len(lines) > 1 {
			t.GOAMD64 = strings.TrimSpace(lines[1])
		}
	}
	return t, nil
}

// goEnv runs go env; tests replace it.
var goEnv = (*exec.Cmd).Output

func (s *Session) printDiags(ds []diag.Diagnostic) {
	fmt.Fprintln(s.Stderr, (&diag.Error{Dir: s.Cwd, Diags: ds}).Error())
}

// Emit checks the package in dir and translates it to a crate.
func (s *Session) Emit(dir string) (*emit.Crate, error) {
	pkg, res, err := s.Check(dir)
	if err != nil {
		return nil, err
	}
	target, err := s.Target()
	if err != nil {
		return nil, err
	}
	c := emit.Emit(pkg, res, emit.Options{Target: target, Version: s.Version, CrateName: s.CrateName, CrateVersion: s.CrateVersion})
	if s.Explain {
		for _, n := range c.Notes {
			fmt.Fprintf(s.Stderr, "%s:%d:%d: %s\n", diag.RelPath(s.Cwd, n.Pos.Filename), n.Pos.Line, n.Pos.Column, n.Text)
		}
	}
	return c, nil
}

// targets maps GOOS/GOARCH to the Rust target patina binaries are built
// for by default, each platform's default C runtime. staticTargets are
// -static's: Rust's musl targets, fully static. They are not the default
// yet, as RFC 0001 plans: musl's allocator is slower than glibc's
// (bench/results/2026-11-27-allocators.md).
var staticTargets = map[string]string{
	"linux/amd64": "x86_64-unknown-linux-musl",
	"linux/arm64": "aarch64-unknown-linux-musl",
}

var targets = map[string]string{
	"linux/amd64":   "x86_64-unknown-linux-gnu",
	"linux/arm64":   "aarch64-unknown-linux-gnu",
	"darwin/amd64":  "x86_64-apple-darwin",
	"darwin/arm64":  "aarch64-apple-darwin",
	"windows/amd64": "x86_64-pc-windows-msvc",
	"windows/arm64": "aarch64-pc-windows-msvc",
}

// Target returns the Rust target triple for GOOS and GOARCH, from the
// environment or, when unset, the machine gopatina runs on.
func (s *Session) Target() (string, error) {
	goos, goarch := s.Getenv("GOOS"), s.Getenv("GOARCH")
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	if s.Static {
		t, ok := staticTargets[goos+"/"+goarch]
		if !ok {
			return "", fmt.Errorf("-static: GOOS/GOARCH %s/%s has no static target (linux/amd64 and linux/arm64 have)", goos, goarch)
		}
		return t, nil
	}
	t, ok := targets[goos+"/"+goarch]
	if !ok {
		return "", fmt.Errorf("GOOS/GOARCH %s/%s has no Rust target in patina mode yet", goos, goarch)
	}
	return t, nil
}

// cargoEnv is the environment cargo runs with: the caller's, without the
// variables that would change the build behind the crate's back. With them
// removed, the crate's own files (Cargo.toml, Cargo.lock,
// rust-toolchain.toml, .cargo/config.toml) decide the build.
func cargoEnv(env []string) []string {
	drop := map[string]bool{
		"RUSTFLAGS": true, "CARGO_ENCODED_RUSTFLAGS": true, "CARGO_BUILD_RUSTFLAGS": true,
		"RUSTC_WRAPPER": true, "RUSTC_WORKSPACE_WRAPPER": true, "CARGO_BUILD_RUSTC_WRAPPER": true,
		"CARGO_TARGET_DIR": true, "CARGO_BUILD_TARGET_DIR": true, "CARGO_BUILD_TARGET": true,
		"CARGO_INCREMENTAL": true, "CARGO_PROFILE_RELEASE_DEBUG": true, "RUSTDOCFLAGS": true,
	}
	var out []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !drop[name] && !strings.HasPrefix(name, "CARGO_PROFILE_") {
			out = append(out, kv)
		}
	}
	return out
}

// CargoArgs are the arguments of the one cargo command patina mode runs:
// exactly the plain release build of the crate.
var CargoArgs = []string{"build", "--release", "--locked"}

// Cargo builds the crate in dir with cargo build --release --locked and
// returns the path of the binary. cargo's output is shown only if the build
// fails, which is always a GoPatina bug: the borrow checker must reject
// whatever rustc would.
func (s *Session) Cargo(dir string, c *emit.Crate) (string, error) {
	name := s.Getenv(EnvCargo)
	if name == "" {
		name = "cargo"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("patina mode needs a Rust toolchain, and %s was not found\n"+
			"    install Rust from https://rustup.rs (the crate pins Rust %s, which rustup installs on first use),\n"+
			"    or use -patina=check to run the patina checks and build with the Go compiler", name, emit.Toolchain)
	}
	cmd := exec.Command(path, CargoArgs...)
	cmd.Dir = dir
	cmd.Env = cargoEnv(os.Environ())
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cargo build failed: %v\n%s\n"+
			"    the generated Rust did not build; this is a GoPatina bug, please report it with the Go source", err, strings.TrimRight(out.String(), "\n"))
	}
	bin := filepath.Join(dir, "target", c.Target, "release", c.Name+exeSuffix(c.Target))
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("cargo build did not produce %s", bin)
	}
	return bin, nil
}

func exeSuffix(target string) string {
	if strings.Contains(target, "windows") {
		return ".exe"
	}
	return ""
}

// CacheDir returns the directory a crate is built in: under the user's
// cache directory, named after the crate's manifest, so building the same
// program again reuses the previous build.
func (s *Session) CacheDir(c *emit.Crate) (string, error) {
	base := s.Getenv("GOPATINA_CACHE")
	if base == "" {
		d, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(d, "gopatina")
	}
	sum := sha256.Sum256(c.Manifest())
	return filepath.Join(base, "crates", c.Name+"-"+hex.EncodeToString(sum[:8])), nil
}

// Build checks and emits the package in dir, builds it with cargo, and
// copies the binary to out.
func (s *Session) Build(dir, out string) error {
	c, err := s.Emit(dir)
	if err != nil {
		return err
	}
	if c.Library {
		return errLibrary(c)
	}
	crateDir, err := s.CacheDir(c)
	if err != nil {
		return err
	}
	if err := c.Write(crateDir); err != nil {
		return err
	}
	bin, err := s.Cargo(crateDir, c)
	if err != nil {
		return err
	}
	if out == "" {
		out = filepath.Join(s.Cwd, c.Name+exeSuffix(c.Target))
	}
	return copyFile(bin, out)
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, 0o755)
}

// FileSHA256 returns the hex sha256 of the file at path.
func FileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// EmitTo checks and emits the package in dir, and writes the crate to out.
func (s *Session) EmitTo(dir, out string) error {
	c, err := s.Emit(dir)
	if err != nil {
		return err
	}
	if s.APICheck != "" {
		if err := s.checkAPI(c); err != nil {
			return err
		}
	}
	if err := c.Write(out); err != nil {
		return err
	}
	fmt.Fprintf(s.Stderr, "wrote crate %s (%d files, listed in %s)\n", diag.RelPath(s.Cwd, out), len(c.Files), emit.ManifestName)
	return nil
}

// checkAPI compares the library's new api.txt with the one at s.APICheck
// and refuses a removed or changed item in a version cargo would still
// take as compatible (RFC 0014).
func (s *Session) checkAPI(c *emit.Crate) error {
	if !c.Library {
		return errors.New("-api-check applies to a library, whose crate has an api.txt (RFC 0014)")
	}
	old, err := os.ReadFile(s.APICheck)
	if err != nil {
		return err
	}
	var cur string
	for _, f := range c.Files {
		if f.Path == "api.txt" {
			cur = string(f.Data)
		}
	}
	ch := emit.CheckAPI(string(old), cur)
	for _, r := range ch.Removed {
		fmt.Fprintf(s.Stderr, "api: removed or changed: %s\n", r)
	}
	for _, a := range ch.Added {
		fmt.Fprintf(s.Stderr, "api: added: %s\n", a)
	}
	if ch.Breaking {
		return fmt.Errorf("the API of %s removes or changes %d item(s) without a new incompatible version (-version; RFC 0014)", c.Name, len(ch.Removed))
	}
	return nil
}

// errLibrary is the error for building or verifying a binary of a library.
func errLibrary(c *emit.Crate) error {
	return fmt.Errorf("%s is a library, which has no binary: gopatina build -emit-rust writes its crate (RFC 0014)", c.Name)
}

// writeCrate writes a crate; tests replace it to make writing fail.
var writeCrate = (*emit.Crate).Write

// Verify checks both guarantees of docs/REPRODUCIBLE.md for the package in
// dir: emitting it twice, into different directories, gives the same
// manifest; and the binary built by the -patina path is byte-identical to
// the one plain cargo build --release --locked builds from the emitted
// crate in another directory.
func (s *Session) Verify(dir string) error {
	tmp, err := os.MkdirTemp("", "gopatina-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	var crates []*emit.Crate
	var plainBin string
	emitInto := func(sub string) func() error {
		return func() error {
			c, err := s.Emit(dir)
			if err == nil {
				err = writeCrate(c, filepath.Join(tmp, sub, c.Name))
				crates = append(crates, c)
			}
			return err
		}
	}
	steps := []func() error{
		emitInto("emitted"),
		emitInto("plain-cargo-build"),
		func() error { return s.sameEmission(crates[0], crates[1]) },
		func() error {
			return s.Build(dir, filepath.Join(tmp, "patina-"+crates[1].Name+exeSuffix(crates[1].Target)))
		},
		func() (err error) {
			plainBin, err = s.Cargo(filepath.Join(tmp, "plain-cargo-build", crates[1].Name), crates[1])
			return err
		},
		func() error {
			c := crates[1]
			fmt.Fprintf(s.Stdout, "toolchain: Rust %s (rust-toolchain.toml), target %s\n", emit.Toolchain, c.Target)
			return s.compare(filepath.Join(tmp, "patina-"+c.Name+exeSuffix(c.Target)), plainBin)
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// compare reports the sha256 of the -patina binary and the plain cargo
// build, and an error if they differ.
func (s *Session) compare(patinaBin, plainBin string) error {
	a, b, err := sums(patinaBin, plainBin)
	if err != nil {
		return err
	}
	fmt.Fprintf(s.Stdout, "-patina:   %s\n", a)
	fmt.Fprintf(s.Stdout, "cargo:     %s  (cargo %s)\n", b, strings.Join(CargoArgs, " "))
	if a != b {
		return fmt.Errorf("the -patina binary and the plain cargo build differ")
	}
	fmt.Fprintln(s.Stdout, "verified:  the -patina binary is byte-identical to the plain cargo build")
	return nil
}

// sameEmission reports an error if two emissions differ.
func (s *Session) sameEmission(a, b *emit.Crate) error {
	if !bytes.Equal(a.Manifest(), b.Manifest()) {
		return fmt.Errorf("emission is not deterministic: two runs gave different manifests\n%s\n%s", a.Manifest(), b.Manifest())
	}
	sum := sha256.Sum256(a.Manifest())
	fmt.Fprintf(s.Stdout, "emission:  deterministic (manifest sha256 %s, %d files)\n", hex.EncodeToString(sum[:]), len(a.Files))
	return nil
}

// sums returns the sha256 of two files.
func sums(a, b string) (string, string, error) {
	sa, err := FileSHA256(a)
	if err != nil {
		return "", "", err
	}
	sb, err := FileSHA256(b)
	return sa, sb, err
}
