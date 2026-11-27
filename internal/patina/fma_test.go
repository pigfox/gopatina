package patina

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/fma"
)

// TestFusedArm64 checks fma_shapes for arm64: gc's report through the go
// command, every site matched, and Rust that fuses them as gc does.
func TestFusedArm64(t *testing.T) {
	if _, err := exec.LookPath(goCommand); err != nil {
		t.Skip(err)
	}
	s, _, stderr := session(t, "")
	env := map[string]string{"GOOS": "linux", "GOARCH": "arm64"}
	s.Getenv = func(k string) string { return env[k] }
	c, err := s.Emit(golden("fma_shapes"))
	if err != nil {
		t.Fatalf("%v\n%s", err, stderr)
	}
	var main string
	for _, f := range c.Files {
		if f.Path == "src/main.rs" {
			main = string(f.Data)
		}
	}
	for _, want := range []string{
		"// Float multiply-adds are fused (mul_add) exactly where gc go1.",
		"linux/arm64\n",
		"rt::fma_arm64(b, a, c)",     // a*b + c: Rn = b, Rm = a
		"rt::fma_arm64(d, c, a * b)", // a*b + c*d: the right multiply
		"rt::fma_arm64(-b, a, c)",    // c - a*b: FMSUB negates Rn
		"rt::fma_arm64(b, a, -c)",    // a*b - c: FNMSUB negates Ra
		"a * 2.0 + c",                // gc's a+a: not fused
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main.rs lacks %q", want)
		}
	}
}

// TestFMATarget checks what fusion depends on, from the environment and
// the go command.
func TestFMATarget(t *testing.T) {
	s, _, _ := session(t, "")
	defer func(f func(*exec.Cmd) ([]byte, error)) { goEnv = f }(goEnv)
	goEnv = func(*exec.Cmd) ([]byte, error) { return []byte("go1.99.0\nv2\n"), nil }
	env := map[string]string{"GOOS": "linux", "GOARCH": "amd64"}
	s.Getenv = func(k string) string { return env[k] }
	if tg, err := s.FMATarget(); err != nil || tg != (fma.Target{GoVersion: "go1.99.0", GOOS: "linux", GOARCH: "amd64", GOAMD64: "v2"}) {
		t.Errorf("go env's GOAMD64: %+v %v", tg, err)
	}
	env["GOAMD64"] = "v3"
	if tg, _ := s.FMATarget(); tg.GOAMD64 != "v3" {
		t.Errorf("the environment's GOAMD64: %+v", tg)
	}
	env = map[string]string{}
	if tg, _ := s.FMATarget(); tg.GOOS == "" || tg.GOARCH == "" || (tg.GOARCH != "amd64" && tg.GOAMD64 != "") {
		t.Errorf("the host: %+v", tg)
	}
	// No go command: gc's fusion is unknown, so every float + and - is
	// refused.
	goEnv = func(*exec.Cmd) ([]byte, error) { return nil, errors.New("no go") }
	if _, err := s.FMATarget(); err == nil {
		t.Error("no error")
	}
	s2, _, stderr := session(t, "")
	if _, err := s2.Emit(golden("fma_shapes")); err == nil || !strings.Contains(stderr.String(), "which gc may fuse with a multiply for this target, while go env: no go") {
		t.Errorf("%v\n%s", err, stderr)
	}
}
