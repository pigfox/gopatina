package emit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// caseReport lists, for every code point, what the case functions do with
// it: each rune ToLower or ToUpper changes, and each pair of runes near
// each other (or a case pair apart) that EqualFold holds equal. Go and the
// emitted Rust must print it identically.
func caseReport() string {
	var b strings.Builder
	near := func(r rune) []rune {
		return []rune{r + 1, r ^ 1, r + 32, r - 32, unicode.ToLower(r), unicode.ToUpper(r), unicode.SimpleFold(r)}
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		s := string(r)
		if l, u := strings.ToLower(s), strings.ToUpper(s); l != s || u != s {
			fmt.Fprintf(&b, "c %X %s %s\n", r, hexRunes(l), hexRunes(u))
		}
		for _, t := range near(r) {
			if t != r && t >= 0 && t <= unicode.MaxRune && (t < 0xD800 || t > 0xDFFF) && strings.EqualFold(s, string(t)) {
				fmt.Fprintf(&b, "f %X %X\n", r, t)
			}
		}
	}
	return b.String()
}

func hexRunes(s string) string {
	var p []string
	for _, r := range s {
		p = append(p, fmt.Sprintf("%X", r))
	}
	return strings.Join(p, ".")
}

const caseMain = `mod rt;

fn hex(s: &str) -> String {
    s.chars()
        .map(|c| format!("{:X}", c as u32))
        .collect::<Vec<_>>()
        .join(".")
}

fn main() {
    let mut out = String::new();
    for c in (0..=0x10FFFFu32).filter_map(char::from_u32) {
        let s = c.to_string();
        let (l, u) = (rt::to_lower(&s), rt::to_upper(&s));
        if l != s || u != s {
            out += &format!("c {:X} {} {}\n", c as u32, hex(&l), hex(&u));
        }
        let r = c as i64;
        let lower = rt::to_lower(&s).chars().next().unwrap() as i64;
        let upper = rt::to_upper(&s).chars().next().unwrap() as i64;
        for t in [r + 1, r ^ 1, r + 32, r - 32, lower, upper, rt::fold_next(c) as i64] {
            if t == r {
                continue;
            }
            if let Some(tc) = u32::try_from(t).ok().and_then(char::from_u32) {
                if rt::equal_fold(&s, &tc.to_string()) {
                    out += &format!("f {:X} {:X}\n", c as u32, t);
                }
            }
        }
    }
    print!("{out}");
}
`

// TestCaseTablesRust builds the emitted case functions and compares them
// with Go's on every code point. It needs cargo (GOPATINA_RUST_TESTS=1)
// and a toolchain whose Unicode version is the tables'.
func TestCaseTablesRust(t *testing.T) {
	if os.Getenv("GOPATINA_RUST_TESTS") != "1" {
		t.Skip("set GOPATINA_RUST_TESTS=1 to build and run the Rust crates")
	}
	if unicode.Version != UnicodeVersion {
		t.Skipf("toolchain Unicode %s, tables %s", unicode.Version, UnicodeVersion)
	}
	dir := t.TempDir()
	rt := rtModule(map[string]bool{"to_lower": true, "to_upper": true, "equal_fold": true}) +
		"\n/// For the test: Go's unicode.SimpleFold.\npub fn fold_next(c: char) -> char {\n    simple_fold(c)\n}\n"
	files := map[string]string{
		"Cargo.toml":  "[package]\nname = \"casecheck\"\nversion = \"0.0.0\"\nedition = \"2024\"\n\n[profile.dev]\nopt-level = 1\n",
		"src/main.rs": caseMain,
		"src/rt.rs":   rt,
	}
	for name, data := range files {
		os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("cargo", "run", "--quiet", "--offline")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	got, err := cmd.Output()
	if err != nil {
		t.Fatalf("cargo run: %v", err)
	}
	want := caseReport()
	if string(got) == want {
		return
	}
	g, w := strings.Split(string(got), "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		if i >= len(g) || i >= len(w) || g[i] != w[i] {
			t.Fatalf("Rust and Go differ at line %d of %d: rust %q, go %q", i+1, len(w), at(g, i), at(w, i))
		}
	}
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "(end)"
}
