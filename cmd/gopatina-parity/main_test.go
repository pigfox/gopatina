package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The stand-ins for go, gopatina and cargo act on markers in the program's
// source, so each step can be made to fail.
const fakeGo = `#!/bin/sh
[ "$1" = env ] && { [ -n "$FAKE_GOVERSION" ] && { echo "$FAKE_GOVERSION"; exit 0; }; echo go1.30; exit 0; }
grep -q 'go 1.30' go.mod || { echo "wrong go.mod"; exit 1; }
while [ $# -gt 0 ]; do if [ "$1" = -o ]; then out="$2"; fi; shift; done
grep -q FAKE_GO_BROKEN main.go && { echo broken; exit 1; }
cp main.go "$out.src"
cat > "$out" <<EOF
#!/bin/sh
src="$out.src"
EOF
cat >> "$out" <<'EOF'
grep -q FAKE_SLOW "$src" && sleep 5
grep -q FAKE_RANDOM "$src" && { echo $$; exit 0; }
echo hello
grep -q FAKE_PANIC "$src" && { echo "panic: go" >&2; exit 2; }
exit 0
EOF
chmod +x "$out"
`

const fakeGopatina = `#!/bin/sh
while [ $# -gt 0 ]; do if [ "$1" = -o ]; then out="$2"; fi; shift; done
grep -q FAKE_REFUSE main.go && { echo './main.go:3:5: Node is copied at line 12 into "s" with ` + "`x`" + `'; echo '    a note'; echo './main.go:4:1: Node is copied at line 13 into "t" with ` + "`y`" + `'; exit 1; }
grep -q FAKE_CRASH main.go && { echo 'panic: Node out of range [3]'; exit 2; }
grep -q FAKE_STDREFUSE main.go && { echo '/usr/local/go/src/math/sin.go:3:4: converting unsafe.Pointer'; echo '/usr/local/go/src/math/unsafe.go:5:6: an array type'; exit 1; }
mkdir -p "$out/.cargo"
cp main.go "$out/main.go"
if grep -q FAKE_NONAME main.go; then echo '[package]' > "$out/Cargo.toml"; else printf '[package]\nname = "app"\n' > "$out/Cargo.toml"; fi
grep -q FAKE_NOTARGET main.go || echo 'target = "tt"' > "$out/.cargo/config.toml"
exit 0
`

const fakeCargo = `#!/bin/sh
src=main.go
case "$1" in
clippy) grep -q FAKE_LINT "$src" && { echo 'error: this borrows ` + "`x`" + `'; exit 101; }; exit 0 ;;
esac
grep -q FAKE_RUSTFAIL "$src" && { echo 'warning: unused'; echo 'error[E0308]: mismatched types ` + "`i64`" + `'; exit 101; }
name=$(sed -n 's/^name = "\(.*\)"/\1/p' Cargo.toml); [ -n "$name" ] || name=main
triple=$(sed -n 's/^target = "\(.*\)"/\1/p' .cargo/config.toml 2>/dev/null)
dir="$CARGO_TARGET_DIR/$triple/release"; mkdir -p "$dir"
cp "$src" "$dir/$name.src"
cat > "$dir/$name" <<EOF
#!/bin/sh
src="$dir/$name.src"
EOF
cat >> "$dir/$name" <<'EOF'
grep -q FAKE_DIFF "$src" && { echo other; exit 0; }
grep -q FAKE_EXIT "$src" && { echo hello; exit 7; }
echo hello
grep -q FAKE_PANIC "$src" && { if grep -q FAKE_OTHERPANIC "$src"; then echo "panic: rust" >&2; else echo "panic: go" >&2; fi; exit 2; }
exit 0
EOF
chmod +x "$dir/$name"
`

// tools writes the stand-ins and returns the flags that name them.
func tools(t *testing.T) []string {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	var flags []string
	for name, script := range map[string]string{"go": fakeGo, "gopatina": fakeGopatina, "cargo": fakeCargo} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		flags = append(flags, "-"+name, path)
	}
	return flags[:len(flags):len(flags)] // appending never shares it
}

// corpus writes programs, name to source, and returns its directory.
func corpus(t *testing.T, progs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range progs {
		path := filepath.Join(dir, filepath.FromSlash(name)+".go.txt")
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func prog(marks ...string) string {
	return "//go:build norun || OMIT\n\npackage main\n\nfunc main() {} // " + strings.Join(marks, " ") + "\n"
}

func parity(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// TestStages takes a program through each step's success and failure.
func TestStages(t *testing.T) {
	dir := corpus(t, map[string]string{
		"a/hello":      prog(),
		"a/lint":       prog("FAKE_LINT"),
		"a/notarget":   prog("FAKE_NOTARGET", "FAKE_NONAME"),
		"a/panic":      prog("FAKE_PANIC"),
		"b/std":        "package main\n\nimport \"example.com/x\"\n\nfunc main() { x.F() }\n",
		"b/net":        "package main\n\nimport \"net/http\"\n\nfunc main() { http.Get(\"\") }\n",
		"b/exec":       "package main\n\nimport \"os/exec\"\n\nfunc main() { exec.Command(\"ls\") }\n",
		"b/signal":     "package main\n\nimport (\n\t\"fmt\"\n\t\"os/signal\"\n)\n\nfunc main() { fmt.Println(signal.Notify) }\n",
		"b/parse":      "package main\n\nfunc main( {\n",
		"b/gobroken":   prog("FAKE_GO_BROKEN"),
		"b/slow":       prog("FAKE_SLOW"),
		"b/random":     prog("FAKE_RANDOM"),
		"c/refuse":     "package main\n\ntype Node struct{ V int }\n\nvar count int\n\nfunc main() {\nouter:\n\tfor i, v := range []int{count} {\n\t\tn := Node{V: v + i}\n\t\t_ = n\n\t\tbreak outer\n\t}\n} // FAKE_REFUSE\n",
		"c/crash":      "package main\n\ntype Node int\n\nfunc main() {} // FAKE_CRASH\n",
		"c/stdrefuse":  prog("FAKE_STDREFUSE"),
		"c/rustfail":   prog("FAKE_RUSTFAIL"),
		"c/diff":       prog("FAKE_DIFF"),
		"c/exit":       prog("FAKE_EXIT"),
		"c/otherpanic": prog("FAKE_PANIC", "FAKE_OTHERPANIC"),
	})
	os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("# known to vary\n\na/listed: ranges over a map\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "listed.go.txt"), []byte(prog()), 0o644)
	md := filepath.Join(t.TempDir(), "r.md")
	js := filepath.Join(t.TempDir(), "r.json")
	code, out, errs := parity(append(tools(t), "-corpus", dir, "-timeout", "1s", "-md", md, "-json", js)...)
	if code != 0 || out != "" {
		t.Fatalf("code %d, out %q, stderr %s", code, out, errs)
	}
	data, _ := os.ReadFile(js)
	var r report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	stage := map[string]string{}
	for _, res := range r.Results {
		stage[res.Name] = res.Stage + " " + res.Reason
	}
	want := map[string]string{
		"a/hello":      "matched ",
		"a/listed":     "skipped ranges over a map (skip.txt)",
		"a/lint":       "matched ",
		"a/notarget":   "matched ",
		"a/panic":      "matched ",
		"b/std":        "skipped imports example.com/x, not in the standard library",
		"b/net":        "skipped uses the network (net/http)",
		"b/exec":       "skipped runs other programs (os/exec)",
		"b/signal":     "skipped waits for signals (os/signal)",
		"b/parse":      "skipped does not parse",
		"b/gobroken":   "skipped does not build with Go",
		"b/slow":       "skipped does not finish within 1s with Go",
		"b/random":     "skipped its output differs between Go runs (clock, randomness or environment)",
		"c/refuse":     "refused ",
		"c/crash":      "crashed ",
		"c/stdrefuse":  "refused ",
		"c/rustfail":   "not compiled ",
		"c/diff":       "mismatched ",
		"c/exit":       "mismatched ",
		"c/otherpanic": "mismatched ",
	}
	for name, w := range want {
		if stage[name] != w {
			t.Errorf("%s: %q, want %q", name, stage[name], w)
		}
	}
	if r.Programs != 20 || r.Skipped != 9 || r.Tested != 11 || r.Accepted != 8 || r.Compiled != 7 || r.Matched != 4 || r.Linted != 7 || r.LintClean != 6 {
		t.Errorf("counts %+v", r)
	}
	if r.Rates["matched"].(float64) != 36.4 || r.Rates["lint_clean"].(float64) != 85.7 {
		t.Errorf("rates %v", r.Rates)
	}
	prints := map[string]int{}
	for _, cl := range r.Clusters {
		prints[cl.Fingerprint] = len(cl.Programs)
	}
	for fp, n := range map[string]int{
		"_ is copied at line N into \"S\" with `_`":                              1,
		"gopatina: panic: Node out of range [N]":                                 1,
		"uses the standard package math, which patina mode does not support yet": 1,
		"rustc: error[E0308]: mismatched types `_`":                              1,
		"clippy: error: this borrows `_`":                                        1,
		"output: standard output differs":                                        1,
		"output: exit status differs":                                            1,
		"output: first line of standard error differs":                           1,
	} {
		if prints[fp] != n {
			t.Errorf("cluster %q: %d programs, want %d (%v)", fp, prints[fp], n, prints)
		}
	}
	mdText, _ := os.ReadFile(md)
	for _, s := range []string{"11 tested, 9 skipped", "| output matches Go | 4 | 36.4% |", "## Failure clusters", "| 1 | does not parse |"} {
		if !strings.Contains(string(mdText), s) {
			t.Errorf("markdown lacks %q:\n%s", s, mdText)
		}
	}
	if !strings.Contains(errs, "a/hello: matched") {
		t.Errorf("no progress: %s", errs)
	}
}

// TestOnlyAndStdout checks -only and the report on standard output, in a
// working directory of the run's own.
func TestOnlyAndStdout(t *testing.T) {
	dir := corpus(t, map[string]string{"x/one": prog(), "y/two": prog()})
	code, out, _ := parity(append(tools(t), "-corpus", dir, "-only", "y/", "-lint=false", "-work", t.TempDir())...)
	var r report
	json.Unmarshal([]byte(out), &r)
	if code != 0 || r.Programs != 1 || r.Results[0].Name != "y/two" || r.Linted != 0 || r.Rates["lint_clean"] != nil {
		t.Errorf("code %d, report %s", code, out)
	}
}

// TestBaseline checks the comparison with a previous report.
func TestBaseline(t *testing.T) {
	dir := corpus(t, map[string]string{"p/ok": prog(), "p/bad": prog("FAKE_DIFF")})
	base := func(matched int) string {
		path := filepath.Join(t.TempDir(), "base.json")
		data, _ := json.Marshal(report{Accepted: 2, Compiled: 2, Matched: matched})
		os.WriteFile(path, data, 0o644)
		return path
	}
	args := append(tools(t), "-corpus", dir, "-lint=false")
	if code, _, errs := parity(append(args, "-baseline", base(1))...); code != 0 {
		t.Errorf("at the baseline: %d %s", code, errs)
	}
	if code, _, errs := parity(append(args, "-baseline", base(2))...); code != 3 || !strings.Contains(errs, "matched 1 programs (baseline 2)") {
		t.Errorf("below the baseline: %d %s", code, errs)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte("{"), 0o644)
	for _, b := range []string{bad, filepath.Join(t.TempDir(), "missing.json")} {
		if code, _, _ := parity(append(args, "-baseline", b)...); code != 1 {
			t.Errorf("baseline %s: code %d", b, code)
		}
	}
}

// TestErrors checks bad usage and the errors that stop a run.
func TestErrors(t *testing.T) {
	ts := tools(t)
	dir := corpus(t, map[string]string{"p/ok": prog()})
	noDir := filepath.Join(t.TempDir(), "none")
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o644)
	unreadable := corpus(t, map[string]string{"p/secret": prog()})
	badSkip := corpus(t, map[string]string{"p/ok": prog()})
	os.WriteFile(filepath.Join(badSkip, "skip.txt"), []byte("p/ok\n"), 0o644)
	dirSkip := corpus(t, map[string]string{"p/ok": prog()})
	os.Mkdir(filepath.Join(dirSkip, "skip.txt"), 0o755)
	os.Chmod(filepath.Join(unreadable, "p", "secret.go.txt"), 0)
	for _, tc := range []struct {
		name string
		args []string
		env  string
		code int
	}{
		{"bad flag", []string{"-nope"}, "", 2},
		{"extra argument", []string{"x"}, "", 2},
		{"no corpus", append(ts, "-corpus", noDir), "", 1},
		{"no temp dir", append(ts, "-corpus", dir), noDir, 1},
		{"work is a file", append(ts, "-corpus", dir, "-work", file), "", 1},
		{"unwritable json", append(ts, "-corpus", dir, "-json", noDir+"/r.json"), "", 1},
		{"unwritable md", append(ts, "-corpus", dir, "-md", noDir+"/r.md"), "", 1},
		{"no go version", append(ts, "-corpus", dir), "", 1},
		{"bad skip.txt", append(ts, "-corpus", badSkip), "", 1},
		{"skip.txt a directory", append(ts, "-corpus", dirSkip), "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv("TMPDIR", tc.env)
			}
			if tc.name == "no go version" {
				t.Setenv("FAKE_GOVERSION", "devel")
			}
			if code, _, errs := parity(tc.args...); code != tc.code {
				t.Errorf("code %d, want %d: %s", code, tc.code, errs)
			}
		})
	}
	if os.Geteuid() != 0 {
		if code, _, errs := parity(append(ts, "-corpus", unreadable)...); code != 1 || !strings.Contains(errs, "p/secret") {
			t.Errorf("unreadable program: %d %s", code, errs)
		}
	}
}

// TestNoneTested checks the rates when every program is skipped.
func TestNoneTested(t *testing.T) {
	r := summarize([]result{{Name: "s", Stage: skipped, Reason: "why"}})
	if r.Rates["matched"] != nil || !strings.Contains(markdown(r), "| of 0 tested | programs | rate |") || !strings.Contains(markdown(r), "n/a") {
		t.Errorf("%+v\n%s", r, markdown(r))
	}
}

// TestClusters checks the clusters' order and the Markdown's examples.
func TestClusters(t *testing.T) {
	var rs []result
	for _, n := range []string{"a", "b", "c", "d"} {
		rs = append(rs, result{Name: n, Stage: refused, Fingerprints: []string{"x | y"}})
	}
	rs = append(rs, result{Name: "e", Stage: refused, Fingerprints: []string{"b"}}, result{Name: "f", Stage: refused, Fingerprints: []string{"a"}})
	r := summarize(rs)
	if r.Clusters[0].Fingerprint != "x | y" || r.Clusters[1].Fingerprint != "a" || r.Clusters[2].Fingerprint != "b" {
		t.Errorf("order %v", r.Clusters)
	}
	if md := markdown(r); !strings.Contains(md, "| 4 | x \\| y | a, b, c |") {
		t.Errorf("markdown:\n%s", md)
	}
}

// TestHelpers checks the helpers' edge cases.
func TestHelpers(t *testing.T) {
	if got := firstMatch([]byte("one\ntwo"), "error"); got != "one" {
		t.Errorf("firstMatch: %q", got)
	}
	if o, _ := execute(filepath.Join(t.TempDir(), "missing"), t.TempDir(), 1e9); o.code != -1 {
		t.Errorf("execute missing: %+v", o)
	}
	if fps := fingerprints([]string{"a 1", "a 2"}, nil); len(fps) != 1 || fps[0] != "a N" {
		t.Errorf("fingerprints %v", fps)
	}
}

// TestMain runs main with arguments that end in bad usage.
func TestMainExit(t *testing.T) {
	saved, savedArgs := exit, os.Args
	defer func() { exit, os.Args = saved, savedArgs }()
	got := -1
	exit = func(code int) { got = code }
	os.Args = []string{"gopatina-parity", "-nope"}
	os.Stderr, _ = os.Open(os.DevNull)
	main()
	if got != 2 {
		t.Errorf("exit %d", got)
	}
}
