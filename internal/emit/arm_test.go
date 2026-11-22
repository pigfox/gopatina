package emit

import (
	"strings"
	"testing"
)

// TestBareArms checks that an arm whose only statement is a match or a
// loop is written the way rustfmt writes it, without a block, and that
// every other arm keeps its block.
func TestBareArms(t *testing.T) {
	rs, _ := emitSource(t, program(`func f(a, b int) int {
	n := 0
	switch a {
	case 0:
		switch b {
		case 1:
			n++
		}
	case 1:
		for {
			n++
			break
		}
	case 2:
		// a comment keeps the block
		switch b {
		case 1:
			n++
		}
	case 3:
		switch {
		case b > 1:
			n++
		}
	case 4:
		for n < 3 {
			n++
		}
	case 5:
		n += 2
	case 6:
		switch m := b * 2; m {
		case 2:
			n++
		}
	}
	return n
}

func main() { fmt.Println(f(0, 1)) }`))
	for _, want := range []string{
		"        0 => match b {\n",
		"        },\n        1 => loop {\n",
		"        2 => {\n            // a comment keeps the block\n            match b {",
		"        3 => {\n            if b > 1 {",
		"        4 => {\n            while n < 3 {",
		"        5 => {\n            n += 2;",
		"        6 => {\n            let m = b * 2;\n            match m {\n",
	} {
		if !strings.Contains(rs, want) {
			t.Errorf("main.rs lacks %q:\n%s", want, rs)
		}
	}
}
