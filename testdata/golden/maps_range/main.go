// Command maps_range writes range over a map (RFC 0019, step 3). Go
// leaves the order unspecified and randomizes it, so everything printed
// here is the same in any order: sums, counts, the largest value, maps
// built from the entries (printed with their keys sorted), and keys sorted
// before they are printed.
package main

import "fmt"

type stock struct {
	count int
	cents int // an integer: a float product summed may be fused on arm64
	sale  bool
}

var prices = map[string]float64{"apple": 1.25, "pear": 2.5, "fig": 0.75}

func main() {
	ages := map[string]int{"ann": 31, "bob": 27, "cy": 45}
	total, oldest, short := 0, 0, 0
	for name, age := range ages {
		total += age
		if age > oldest {
			oldest = age
		}
		if len(name) < 3 {
			short++
		}
	}
	fmt.Println(total, oldest, short)

	n := 0
	for range ages {
		n++
	}
	keys := []string{}
	for k := range ages {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	fmt.Println(n, keys)

	sum := 0.0
	for _, p := range prices {
		sum += p // exact in binary, so the same in any order
	}
	fmt.Println(sum)

	byLen := map[int]int{}
	for k, v := range ages {
		byLen[len(k)] += v
	}
	inv := map[int]string{}
	for k, v := range ages {
		inv[v] = k
	}
	fmt.Println(byLen, inv)

	stocks := map[string]stock{"a": {2, 150, true}, "b": {3, 50, false}}
	worth, onSale := 0, 0
	for _, s := range stocks {
		worth += s.count * s.cents
		if s.sale {
			onSale++
		}
	}
	fmt.Println(worth, onSale)

	for k, v := range ages {
		v *= 2
		if k == "bob" {
			fmt.Println(k, v)
		}
	}
}
