// Command maps_local writes local maps (RFC 0019, step 1): make and
// literals, reads that give the zero value for a missing key, writes,
// comma-ok, delete, len, and printing, which sorts the keys as fmt does.
package main

import "fmt"

func main() {
	m := make(map[string]int)
	m["Answer"] = 42
	fmt.Println("The value:", m["Answer"])
	m["Answer"] = 48
	fmt.Println("The value:", m["Answer"])
	delete(m, "Answer")
	fmt.Println("The value:", m["Answer"])
	v, ok := m["Answer"]
	fmt.Println("The value:", v, "Present?", ok)

	counts := map[string]int{"b": 2, "a": 1}
	counts["c"]++
	counts["a"] += 10
	word := "b"
	counts[word] *= 3
	counts["b"]--
	fmt.Println(counts, len(counts), counts[word])
	if _, ok := counts["z"]; !ok {
		fmt.Println("no z")
	}

	squares := make(map[int]int, 4)
	for i := 3; i >= -1; i-- {
		squares[i] = i * i
		fmt.Print(squares[i], " ")
	}
	fmt.Println()
	fmt.Println(squares)
	fmt.Printf("%v %d\n", squares, squares[2])

	names := map[int]string{}
	names[7] = "seven"
	if n, found := names[8]; !found {
		fmt.Printf("8 missing, zero value %q\n", n)
		n = "eight"
		names[8] = n
	}
	seen := map[bool]float64{true: 1.5}
	seen[false] = 0.25
	fmt.Println(seen, names, fmt.Sprint(len(names)))
}
