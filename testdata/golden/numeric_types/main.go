// The remaining integer types and float32 (RFC 0008): arithmetic at each
// type's edges, conversions between every width, float-to-integer
// conversions as gc does them on amd64, float32 arithmetic, constants and
// printing, and an out-of-range uint64 index, printed as Go prints it.
package main

import "fmt"

func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func mix64(x uint64) uint64 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	return x
}

func edges() {
	var a int8 = -128
	var b int16 = 32767
	var c uint16 = 65535
	var d uint32 = 1 << 31
	var e uint64 = 1<<64 - 1
	var f uint = 3
	var g uintptr = 10
	neg := int8(-1)
	fmt.Println(a/neg, a%neg, -a, a-1, b+1, c+1, d*2, e+1, e/3, f-4, g<<62, -e)
	fmt.Println(a>>8, a>>7, b<<16, c>>16, d<<32, e>>64, e<<1, int16(c), uint8(b), int8(d>>24))
	var z uint32
	z--
	fmt.Println(z, fnv32("gopatina"), mix64(42), min(c, 7), max(e, 1))
}

func conversions() {
	vals := []int64{-1, 255, 256, -129, 70000, -70000, 1 << 40, -1 << 63}
	for _, v := range vals {
		fmt.Println(int8(v), int16(v), uint16(v), uint32(v), uint64(v), uint(v), uintptr(v), float32(v), float64(uint64(v)))
	}
}

func floats() {
	zero := 0.0
	fs := []float64{-2.5, 300.7, 70000.5, -70000.5, 3e9 + 5, 4294967301, -1, 9.3e18, 1e19, 1.8e19, 2e19, 1e300, zero / zero, 1 / zero, -1 / zero}
	for _, x := range fs {
		fmt.Println(int8(x), int16(x), uint16(x), uint32(x), uint64(x), uint(x), uintptr(x))
	}
	var a float32 = 0.1
	b := a * 3
	c := float32(16777217)
	var z32 float32
	big32 := float32(1e38)
	f1, f2, f3 := float32(3e9), float32(-2.5), float32(1e20)
	d := float32(1) / z32
	fmt.Println(a, b, c, d, -d, z32/z32, float64(a), float32(0.1)+float32(0.2), a/3, big32*10)
	fmt.Printf("%v|%.3f|%8.2f|%-9v|%f|\n", b, b, b, a, float32(2)/3)
	fmt.Println(int(f1), int32(f2), uint8(f2), uint64(f3), int16(f1))
	fmt.Println([]float32{1.5, 0.1, 1e10}, []uint64{1<<64 - 1, 0}, []int8{-1, 127})
	const third float32 = 1.0 / 3
	fmt.Println(third, float32(16777216.5), float32(3.4e38))
}

func verbs() {
	var q uint64 = 1<<64 - 1
	var u uint16 = 65535
	var m int8 = -128
	var w uint32 = 65
	fmt.Printf("%x %X %d %c %#x %08x|%x|%5d|%-5d|%+d\n", q, u, m, w, w, u, m, m, m, m)
}

func main() {
	edges()
	conversions()
	floats()
	verbs()
	// A slice literal printed without a type from its context (GoPatina
	// built an i32 literal until 2026-11-22, which did not compile).
	fmt.Println([]int{1 << 40, 2}, len([]int{-1 << 63}))
	s := []int{1, 2, 3}
	var big uint64 = 1<<63 + 5
	fmt.Println(s[big])
}
