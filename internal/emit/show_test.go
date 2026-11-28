package emit

import (
	"strings"
	"testing"

	"github.com/pigfox/gopatina/internal/borrow"
	"github.com/pigfox/gopatina/internal/load"
	"github.com/pigfox/gopatina/internal/subset"
)

// A struct's show_ function does not take the name of a function of the
// program.
func TestShowFnName(t *testing.T) {
	src := "package main\n\nimport \"fmt\"\n\ntype point struct{ X int }\n\nfunc show_point() int { return 1 }\n\nfunc main() { fmt.Println(point{show_point()}) }\n"
	pkg, err := load.Source("/r", "main.go", src, "go1.25")
	if err != nil {
		t.Fatal(err)
	}
	if ds := subset.Check(pkg); len(ds) > 0 {
		t.Fatal(ds)
	}
	res, _ := borrow.Check(pkg)
	var main string
	for _, f := range Emit(pkg, res, Options{Target: goldenTarget, Version: "devel"}).Files {
		if f.Path == "src/main.rs" {
			main = string(f.Data)
		}
	}
	if !strings.Contains(main, "fn show_point_(s: &Point) -> String {") || !strings.Contains(main, "show_point_(&Point { x: show_point() })") {
		t.Errorf("main.rs:\n%s", main)
	}
}
