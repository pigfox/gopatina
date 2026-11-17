// Command fmtverbs exercises the fmt verbs patina mode supports.
package main

import (
	"fmt"
	"os"
)

func main() {
	n, neg := 42, -255
	f := 3.14159
	s := "go"
	ok := true
	var b byte = 'A'
	fmt.Printf("%d|%5d|%-5d|%05d|%+d\n", n, n, n, n, n)
	fmt.Printf("%x|%X|%x|%4x|%x\n", n, n, neg, b, 255)
	fmt.Printf("%c|%c|%3c|%c\n", b, 0x4E16, 'x', -1)
	fmt.Printf("%s|%6s|%-6s|%.1s\n", s, s, s, "hello")
	fmt.Printf("%t|%6t|%v\n", ok, ok, !ok)
	fmt.Printf("%f|%.2f|%8.3f|%-8.1f|%+.1f|%08.2f\n", f, f, f, f, f, -f)
	fmt.Printf("%v|%v|%v|%5v|%v\n", f, 1e-7, 2.0, 1.5, 100000000.0)
	fmt.Printf("100%% done, {braces} kept\n")
	msg := fmt.Sprintf("%s has %d items", s, n)
	fmt.Println(msg, len(msg))
	fmt.Print("a", "b", 1, 2, "c", 3.5, "\n")
	fmt.Print(1, 2, "\n")
	line := fmt.Sprintln("x", 1, true)
	fmt.Print(line)
	fmt.Println(fmt.Sprint("p", 7, 8))
	fmt.Fprintln(os.Stderr, "to stderr", n)
	fmt.Fprintf(os.Stdout, "to stdout %d\n", n)
	fmt.Println()
}
