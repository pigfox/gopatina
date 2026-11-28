package emit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// toolchainFile is toolchain.json, the one place the project's toolchain
// versions are written. Everything else that names them is a copy, and
// TestToolchainDrift fails when a copy disagrees. scripts/toolchain-bump.sh
// rewrites every copy at once.
type toolchainFile struct {
	Rust struct {
		Pinned  string `json:"pinned"`
		MSRV    string `json:"msrv"`
		Edition string `json:"edition"`
	} `json:"rust"`
	Go struct {
		Module string `json:"module"`
		Pinned string `json:"pinned"`
	} `json:"go"`
}

func readToolchain(t *testing.T) toolchainFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root(), "toolchain.json"))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var tc toolchainFile
	if err := dec.Decode(&tc); err != nil {
		t.Fatalf("toolchain.json: %v", err)
	}
	if tc.Rust.Pinned == "" || tc.Rust.MSRV == "" || tc.Rust.Edition == "" || tc.Go.Module == "" || tc.Go.Pinned == "" {
		t.Fatalf("toolchain.json: every field must be set: %+v", tc)
	}
	return tc
}

// goVersionFile is how actions/setup-go spells Go release p in
// .go-version: 1.27.4 as it is, a release candidate 1.28rc1 as 1.28.0-rc.1.
func goVersionFile(p string) string {
	if major, rc, ok := strings.Cut(p, "rc"); ok {
		return major + ".0-rc." + rc
	}
	return p
}

// field returns the value of the first `key = "value"` line in a TOML file.
func field(data []byte, key string) string {
	m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + ` = "([^"]*)"`).FindSubmatch(data)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func TestGoVersionFile(t *testing.T) {
	for in, want := range map[string]string{"1.27.4": "1.27.4", "1.28rc1": "1.28.0-rc.1", "1.28rc12": "1.28.0-rc.12"} {
		if got := goVersionFile(in); got != want {
			t.Errorf("goVersionFile(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToolchainDrift(t *testing.T) {
	tc := readToolchain(t)
	if Toolchain != tc.Rust.Pinned {
		t.Errorf("internal/emit Toolchain = %q, toolchain.json rust.pinned = %q", Toolchain, tc.Rust.Pinned)
	}
	if MSRV != tc.Rust.MSRV {
		t.Errorf("internal/emit MSRV = %q, toolchain.json rust.msrv = %q", MSRV, tc.Rust.MSRV)
	}
	if got := field([]byte(cargoToml("x", "0.0.0", false, Options{})), "edition"); got != tc.Rust.Edition {
		t.Errorf("emitted Cargo.toml edition = %q, toolchain.json rust.edition = %q", got, tc.Rust.Edition)
	}

	// Every committed crate GoPatina emitted pins the same toolchain.
	var pins []string
	for _, pattern := range []string{"testdata/golden/*/rust", "testdata/golden-lib/*/rust", "examples/*/rust", "examples/rust-interop/rules"} {
		dirs, err := filepath.Glob(filepath.Join(root(), pattern))
		if err != nil {
			t.Fatal(err)
		}
		pins = append(pins, dirs...)
	}
	if len(pins) < 3 {
		t.Fatalf("found only %d committed crates", len(pins))
	}
	for _, dir := range pins {
		rel, _ := filepath.Rel(root(), dir)
		tt, err := os.ReadFile(filepath.Join(dir, "rust-toolchain.toml"))
		if err != nil {
			t.Error(err)
			continue
		}
		if got := field(tt, "channel"); got != tc.Rust.Pinned {
			t.Errorf("%s/rust-toolchain.toml channel = %q, toolchain.json rust.pinned = %q", rel, got, tc.Rust.Pinned)
		}
		ct, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
		if err != nil {
			t.Error(err)
			continue
		}
		if got := field(ct, "rust-version"); got != tc.Rust.MSRV {
			t.Errorf("%s/Cargo.toml rust-version = %q, toolchain.json rust.msrv = %q", rel, got, tc.Rust.MSRV)
		}
		if got := field(ct, "edition"); got != tc.Rust.Edition {
			t.Errorf("%s/Cargo.toml edition = %q, toolchain.json rust.edition = %q", rel, got, tc.Rust.Edition)
		}
	}

	gomod, err := os.ReadFile(filepath.Join(root(), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if m := regexp.MustCompile(`(?m)^go (\S+)$`).FindSubmatch(gomod); m == nil || string(m[1]) != tc.Go.Module {
		t.Errorf("go.mod's go directive is not toolchain.json go.module = %q", tc.Go.Module)
	}

	// CI's Go is the pinned one (actions/setup-go reads .go-version), and
	// the fusion sites gc reported were recorded with it.
	gv, err := os.ReadFile(filepath.Join(root(), ".go-version"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(gv)), goVersionFile(tc.Go.Pinned); got != want {
		t.Errorf(".go-version = %q, toolchain.json go.pinned %q wants %q", got, tc.Go.Pinned, want)
	}
	sites, err := filepath.Glob(filepath.Join(root(), "testdata", "golden", "*", "fma-sites.txt"))
	if err != nil || len(sites) == 0 {
		t.Fatalf("no fma-sites.txt: %v", err)
	}
	for _, f := range sites {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "# go") && !strings.HasPrefix(line, "# go"+tc.Go.Pinned+" ") {
				rel, _ := filepath.Rel(root(), f)
				t.Errorf("%s records %q, not go.pinned go%s (go test ./internal/patina -run TestFMASites -update-fma-sites)", rel, line, tc.Go.Pinned)
			}
		}
	}

	// The workflows read toolchain.json; none writes a version of its own.
	flows, err := filepath.Glob(filepath.Join(root(), ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range flows {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range []string{tc.Rust.Pinned, `"` + tc.Rust.MSRV + `"`, "go-version: " + tc.Go.Pinned, "go-version: \"" + tc.Go.Pinned} {
			if bytes.Contains(data, []byte(v)) {
				t.Errorf("%s names %s; read it from toolchain.json instead", filepath.Base(f), v)
			}
		}
	}

	// The documents that state the current pin.
	for file, wants := range map[string][]string{
		"README.md": {
			"emitted crates pin " + tc.Rust.Pinned + ";",
			"| " + tc.Rust.MSRV + " | " + tc.Rust.Edition + " |",
		},
		filepath.Join("docs", "patina-subset.md"): {
			"| `rust-toolchain.toml` | Rust " + tc.Rust.Pinned + ", minimal profile",
			"edition " + tc.Rust.Edition + ", `rust-version = \"" + tc.Rust.MSRV + "\"`",
		},
	} {
		data, err := os.ReadFile(filepath.Join(root(), file))
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range wants {
			if !strings.Contains(string(data), w) {
				t.Errorf("%s does not say %q (toolchain.json)", file, w)
			}
		}
	}
}
