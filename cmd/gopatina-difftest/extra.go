package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/pigfox/gopatina/internal/progen"
)

// Two checks beside a batch's program comparison:
//
//   - lint (every -lint'th batch): the batch's generated crate must pass
//     cargo clippy -D warnings, as the golden crates must; running a
//     program does not show a crate that does not lint.
//   - library (every -libraries'th batch): a generated library package
//     (progen.LibraryPackage) is emitted as a crate (RFC 0014), and a Rust
//     program calling it must print what a Go program calling the package
//     prints.
//
// A failure of either is a defect, kept with what reproduces it.

// due reports whether the check run every `every` batches runs for the
// batch starting at program first.
func due(c config, every, first int) bool {
	return every > 0 && ((first-c.start)/c.per)%every == 0
}

// inputLints are the lints a faithful translation of a random program
// trips because the Go program itself says it: x % 1, x = x, x - x, a
// swap written as two assignments, an if nested in an if, ReplaceAll of a
// text with itself, else { if }, x * -1, statements after a panic.
// Hand-written Rust translating the same Go would trip them too, so they
// are allowed in the batches; everything else is a GoPatina defect. The
// golden crates lint with no allowances.
var inputLints = []string{
	"clippy::almost_swapped", "clippy::collapsible_else_if", "clippy::collapsible_if",
	"clippy::erasing_op", "clippy::identity_op", "clippy::manual_swap",
	"clippy::misrefactored_assign_op", "clippy::modulo_one", "clippy::neg_multiply",
	"clippy::no_effect_replace", "clippy::self_assignment", "unreachable_code",
}

// lintBatch emits the batch's crate and runs clippy on it. It returns the
// clippy output on a failure, and an error when cargo cannot run clippy at
// all (a machine without clippy), which is not a GoPatina defect.
func lintBatch(c config, worker int, dir string) (failed bool, out []byte, err error) {
	crate := filepath.Join(dir, "crate")
	goEnv := []string{"GOCACHE=" + c.gocache}
	if out, err := build(c.buildLimit(), dir, goEnv, c.gopatina, "build", "-emit-rust", "-o", crate, "."); err != nil {
		return true, out, nil
	}
	target := "CARGO_TARGET_DIR=" + filepath.Join(c.work, "lint-target-"+strconv.Itoa(worker))
	args := []string{"clippy", "--release", "--quiet", "--", "-D", "warnings"}
	for _, l := range inputLints {
		args = append(args, "-A", l)
	}
	out, err = build(c.buildLimit(), crate, []string{target}, c.cargo, args...)
	if _, ran := err.(*exec.ExitError); err != nil && !ran {
		return false, nil, fmt.Errorf("%s could not run: %v", c.cargo, err)
	}
	if err != nil && bytes.Contains(out, []byte("is not installed for the toolchain")) {
		return false, nil, fmt.Errorf("clippy is not installed for the crate's toolchain:\n%s", out)
	}
	return err != nil, out, nil
}

// libraryCheck runs program first's library check. It returns what to keep
// when the check fails, and an error when the Go side fails, which no
// generated library may do.
func libraryCheck(c config, worker, first int) (failed bool, files map[string][]byte, err error) {
	dir := filepath.Join(c.work, fmt.Sprintf("lib%d", first))
	defer os.RemoveAll(dir)
	pkg := progen.LibraryPackage(c.seed, first)
	goCheck := progen.LibraryCheckGo(c.seed, first, "libcheck/"+progen.LibraryName)
	rsCheck := progen.LibraryCheckRust(c.seed, first)
	ws := filepath.Join(dir, "ws")
	srcs := map[string]string{
		"go.mod":                       "module libcheck\n\ngo 1.25\n",
		progen.LibraryName + "/lib.go": pkg,
		"check/main.go":                goCheck,
		"ws/Cargo.toml":                "[workspace]\nmembers = [\"" + progen.LibraryName + "\", \"check\"]\nresolver = \"3\"\n\n[profile.dev.package." + progen.LibraryName + "]\noverflow-checks = false\n",
		"ws/check/Cargo.toml":          "[package]\nname = \"check\"\nversion = \"0.1.0\"\nedition = \"2024\"\n\n[dependencies]\n" + progen.LibraryName + " = { path = \"../" + progen.LibraryName + "\" }\n",
		"ws/check/src/main.rs":         rsCheck,
	}
	for name, data := range srcs {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(data), 0o644) // a failure shows in the go build below
	}
	keepFiles := func(more map[string][]byte) map[string][]byte {
		files := map[string][]byte{"lib.go": []byte(pkg), "check.go": []byte(goCheck), "check.rs": []byte(rsCheck)}
		for k, v := range more {
			files[k] = v
		}
		return files
	}
	goEnv := []string{"GOCACHE=" + c.gocache}
	goBin := filepath.Join(dir, "check-go")
	if out, err := build(c.buildLimit(), dir, goEnv, c.goCmd, "build", "-o", goBin, "./check"); err != nil {
		return false, nil, fmt.Errorf("library %d: go build: %v\n%s", first, err, out)
	}
	want, _, code := execute(goBin, c.timeout)
	if code != 0 {
		return false, nil, fmt.Errorf("library %d: the Go check exited %d", first, code)
	}
	if out, err := build(c.buildLimit(), dir, goEnv, c.gopatina, "build", "-emit-rust", "-o", filepath.Join(ws, progen.LibraryName), "./"+progen.LibraryName); err != nil {
		return true, keepFiles(map[string][]byte{"emit.txt": out}), nil
	}
	target := "CARGO_TARGET_DIR=" + filepath.Join(c.work, "lib-target-"+strconv.Itoa(worker))
	if out, err := build(c.buildLimit(), ws, []string{target}, c.cargo, "build", "--quiet"); err != nil {
		if _, ran := err.(*exec.ExitError); !ran {
			return false, nil, fmt.Errorf("%s could not run: %v", c.cargo, err)
		}
		return true, keepFiles(map[string][]byte{"cargo-build.txt": out}), nil
	}
	got, stderr, code := execute(filepath.Join(c.work, "lib-target-"+strconv.Itoa(worker), "debug", "check"), c.timeout)
	if code != 0 || !bytes.Equal(got, want) {
		return true, keepFiles(map[string][]byte{"go.out": want, "rust.out": got, "rust.err": stderr}), nil
	}
	return false, nil, nil
}

// keepLibrary keeps a failing library check under -failures.
func keepLibrary(c config, first int, files map[string][]byte) {
	dir := filepath.Join(c.failed, fmt.Sprintf("seed%d-lib%d", c.seed, first))
	os.MkdirAll(dir, 0o755)
	for name, data := range files {
		os.WriteFile(filepath.Join(dir, name), data, 0o644)
	}
}
