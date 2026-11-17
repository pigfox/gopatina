package main

import (
	"fmt"
	"io"
	"os"
)

func emit(w io.Writer, n int) {
	for i := 0; i < n; i++ {
		fmt.Fprintf(w, "[%d]", i)
	}
	fmt.Fprintln(w)
}

func report(w io.Writer, name string, vals []float64) {
	fmt.Fprint(w, name, ":")
	for i, v := range vals {
		if i > 0 {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, "%6.2f", v)
	}
	fmt.Fprintln(w)
}

func main() {
	emit(os.Stdout, 4)
	emit(os.Stderr, 2)
	report(os.Stdout, "vals", []float64{1, 2.5, -3.14159, 1e3})
	fmt.Fprint(os.Stdout, 1, 2, "a", "b", 3, 4.5, "\n")
	fmt.Print(1, 2, "a", "b", 3, 4.5, true, false, "\n")
	fmt.Println(fmt.Sprint(1, 2), fmt.Sprint("a", 1, 2, "b"), fmt.Sprintln("x", 1) == "x 1\n")
}
