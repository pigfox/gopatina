package emit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

// emitSource runs the pipeline on one file and returns src/main.rs and the
// emitter's notes, one per line.
func emitSource(t *testing.T, src string) (string, string) {
	t.Helper()
	pkg, err := load.Source("/r", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	if ds := subset.Check(pkg); len(ds) > 0 {
		t.Fatalf("subset: %v", ds)
	}
	res, ds := borrow.Check(pkg)
	if len(ds) > 0 {
		t.Fatalf("borrow: %v", ds)
	}
	c := Emit(pkg, res, Options{Target: goldenTarget, Version: "devel"})
	var notes strings.Builder
	for _, n := range c.Notes {
		fmt.Fprintf(&notes, "%d: %s\n", n.Pos.Line, n.Text)
	}
	for _, f := range c.Files {
		if f.Path == "src/main.rs" {
			return string(f.Data), notes.String()
		}
	}
	t.Fatal("no src/main.rs")
	return "", ""
}

func program(body string) string {
	return "package main\n\nimport \"fmt\"\n\n" + body + "\n"
}

// TestRanges checks the specialization decisions of RFC 0006 that the
// golden programs do not reach.
func TestRanges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		want  []string // in src/main.rs
		not   []string // not in src/main.rs
		notes string   // a substring of the notes, or "" for none at all
	}{
		{
			name: "dropped variable beside a kept one",
			src: program(`func main() {
	s := make([]int, 10)
	t := []int{1, 2, 3}
	for i := 0; i < 10; i++ {
		for k := 0; k < len(t); k++ {
			s[i] += t[k]
		}
	}
	fmt.Println(s)
}`),
			want:  []string{"for i in 0_usize..10", "s[i] += t[k as usize]"},
			notes: "usize for i ",
		},
		{
			name: "two bounds, both proven",
			src: program(`func f(a, b int) int {
	s := make([]int, a)
	t := make([]int, b)
	x := 0
	for i := 0; i < a; i++ {
		for j := 0; j < b; j++ {
			x += s[i] + t[j]
		}
	}
	return x
}

func main() { fmt.Println(f(2, 3)) }`),
			want:  []string{"let a_usize = a as usize;", "let b_usize = b as usize;", "for j in 0..b_usize"},
			notes: "loop specialized: usize for i, j",
		},
		{
			name: "a step that could wrap needs a condition",
			src: program(`func f(n int) int {
	s := make([]int, 1)
	x := 0
	for j := 0; j < n; j += 4611686018427387904 {
		x += s[j*0]
	}
	return x
}

func main() { fmt.Println(f(1)) }`),
			want:  []string{"if n <= 4611686018427387904 {", "let n_usize = n.max(0) as usize;"},
			notes: "loop specialized for n <= 4611686018427387904",
		},
		{
			name: "constants that overflow on their own",
			src: program(`func f(n int) int {
	s := make([]int, 1)
	x := 0
	for i := 0; i < n; i++ {
		x += s[(i+5000000000000000000)+(i+5000000000000000000)]
	}
	return x
}

func main() { fmt.Println(f(0)) }`),
			not:   []string{"usize;"},
			notes: "no single condition",
		},
		{
			name: "loop shapes that are not specialized",
			src: program(`func main() {
	s := make([]int, 10)
	n := len(s)
	for j := 0; j < n; j-- {
		s[j%10]++
		break
	}
	for j := 0; j < n; j -= 1 {
		s[j%10]++
		break
	}
	for j := 0; j < n; {
		s[j]++
		j++
	}
	for j := 0; j < n; j = j + 2 {
		s[j]++
	}
	for j := 0; j < n; j += 2 {
		s[j]++
		j++
	}
	for i := 0; i < 10; i++ {
		p := &i
		s[*p]++
	}
	for i := 0; i < i; i++ {
		s[i]++
	}
	for j := 0; j <= n; j++ {
		s[j%10]++
	}
	fmt.Println(s)
}`),
			not: []string{"usize;", "_usize"},
		},
		{
			name: "make facts: other calls and empty elements",
			src: program(`type empty struct{}

func pick(a, b int) int { return a }

func f(n, m int) int {
	x := pick(n, m)
	e := make([]empty, n)
	s := make([]int, 4)
	for i := 0; i <= n; i++ {
		s[i%4+0] += 1
		x += s[i*0]
	}
	return x + len(e)
}

func main() { fmt.Println(f(3, 1)) }`),
			want:  []string{"let n_usize = n as usize;", "for i in 0..=n_usize"},
			notes: "loop specialized: usize for i",
		},
		{
			name: "comparisons in the fast copy",
			src: program(`func f(n, k int) int {
	s := make([]int, 8)
	x := 0
	for i := 0; i < n; i++ {
		if i < k || k > i || i <= n || n > i || i < n {
			x++
		}
		for j := i; j < i+2; j++ {
			x += s[j%8] + s[i*0]
		}
		x += max(i, 1)
		fmt.Println(i)
		s = append(s, i)
	}
	return x
}

func main() { fmt.Println(f(3, 2)) }`),
			want: []string{
				"(i as i64) < k", "k > i as i64", "(i as i64) <= n", "n_usize > i", "|| i < n_usize",
				"let mut j = i;", "while j < i + 2 {", "j += 1;", "println!(\"{}\", i as i64)", "s.push(i as i64)",
			},
			notes: "loop specialized for n <= 9223372036854775806: usize for i, j",
		},
		{
			name: "a comparison operand is checked like an index",
			src: program(`func f(n, k int) int {
	s := make([]int, 4)
	x := 0
	for i := 0; i < n; i++ {
		if i*i*i*i < k {
			x += s[i%4]
		}
		x += s[i*0]
	}
	return x
}

func main() { fmt.Println(f(3, 2)) }`),
			// i*i*i*i fits only for n <= 55109, under the 2^20 a condition
			// must allow, so the comparison stays an i64 one.
			want:  []string{"if i * i * i * i < k {"},
			not:   []string{"usize;"},
			notes: "no single condition",
		},
		{
			name: "a comment before the nest is kept once",
			src: program(`func f(n int) int {
	s := make([]bool, n)
	x := 0
	// count
	for i := 1; i < n; i++ {
		for j := i * i * i; j < n; j += i {
			if !s[j] { // first time
				s[j] = true
				x++
			}
		}
	}
	return x
}

func main() { fmt.Println(f(30)) }`),
			want:  []string{"    // count\n    if n <= 2097152 {", "if !s[j] {\n", "if !s[j as usize] {\n                    // first time"},
			notes: "loop specialized for n <= 2097152",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs, notes := emitSource(t, tc.src)
			for _, w := range tc.want {
				if !strings.Contains(rs, w) {
					t.Errorf("main.rs lacks %q:\n%s", w, rs)
				}
			}
			for _, w := range tc.not {
				if strings.Contains(rs, w) {
					t.Errorf("main.rs has %q:\n%s", w, rs)
				}
			}
			if tc.notes == "" && notes != "" || !strings.Contains(notes, tc.notes) {
				t.Errorf("notes = %q, want %q", notes, tc.notes)
			}
		})
	}
}

// TestRanges32 checks that on a 32-bit target values must fit in a 32-bit
// usize.
func TestRanges32(t *testing.T) {
	t.Setenv("GOARCH", "386")
	rs, notes := emitSource(t, program(`func f(n int) int {
	s := make([]int, 3)
	x := 0
	for i := 0; i < n; i++ {
		x += s[i]
	}
	return x
}

func main() { fmt.Println(f(3)) }`))
	if !strings.Contains(rs, "if n <= 4294967296 {") || !strings.Contains(notes, "n <= 4294967296") {
		t.Errorf("notes %q\n%s", notes, rs)
	}
}
