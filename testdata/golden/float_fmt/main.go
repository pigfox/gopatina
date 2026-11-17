package main

import "fmt"

func main() {
	fs := []float64{0, 1, -1, 0.1, 1e21, 1e20, 1e-7, 123456789.123, 1.0 / 3, 2.5e-10, 100000, 1e6, 12345678, 5e-324, 1.7976931348623157e308}
	for i := 0; i < len(fs); i++ {
		f := fs[i]
		fmt.Println(f)
		fmt.Printf("%v|%f|%.2f|%8.3f|%-10.1f|%+.0f|%010.4f\n", f, f, f, f, f, f, f)
	}
	nz := 0.0
	nz = -nz
	fmt.Println(nz, 1/nz > 0)
	fmt.Printf("%v %f\n", nz, nz)
	z := 0.0
	fmt.Println(1/z, -1/z, z/z)
	fmt.Printf("%5.1f|%-8v|%+v\n", 1/z, -1/z, z/z)
}
