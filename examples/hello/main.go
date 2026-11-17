// Command hello is the first GoPatina example: plain Go that is also inside
// the planned patina subset. See EXAMPLES.md.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Greeting is a small struct holding a string, so it is moved, not copied,
// in patina mode.
type Greeting struct {
	Name  string
	Times int
}

// render builds the greeting text. It is the last user of g, so patina mode
// moves g into it.
func render(g Greeting) string {
	return strings.Repeat("Hello, "+g.Name+"! ", g.Times)
}

func run(w io.Writer) {
	g := Greeting{Name: "Gopher", Times: 2}
	fmt.Fprintln(w, strings.TrimSpace(render(g)))

	names := []string{"Ferris", "Gopher"}
	first := names[0]
	names = append(names, "Gordon") // names changes while first is still in use
	fmt.Fprintln(w, first, len(names))
}

func main() {
	run(os.Stdout)
}
