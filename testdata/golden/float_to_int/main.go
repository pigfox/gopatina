// Float-to-integer conversions as Go's compiler does them on amd64: Go
// leaves an out-of-range value implementation-specific, gc gives MinInt64
// or MinInt32 for NaN and for values that do not fit, and converts to
// uint8 through int32. Rust's as saturates. Found 2026-11-22 while
// planning RFC 0008; the generated tests had no floats.
package main

import "fmt"

func main() {
	z := 0.0
	vals := []float64{
		2.5, -2.5, 300.7, -300.7, -0.9, 3e9 + 5, -3e9 - 5,
		2147483648.5, -2147483648.5, -2147483649.5, 4294967301,
		9.2233720368547758e18, -9.2233720368547758e18, 1e19, 1e300, -1e300,
		2147483647, -2147483648, z / z, 1 / z, -1 / z,
	}
	shortestTies()
	for _, f := range vals {
		fmt.Println(int(f), int64(f), int32(f), uint8(f), int(f*2), uint8(f)+1)
	}
}

// shortestTies prints floats whose shortest digits have two equally close
// candidates; Go takes the even one. Found by the generated tests,
// 2026-11-22 (1284882399712448.25 printed ...483e+15 instead of ...482e+15).
func shortestTies() {
	fmt.Println(1284882399712448.25, 0.125, 2.5e20, 9007199254740993.0)
	x := 1284882399712448.0
	fmt.Println(x+0.25, x+0.75, -(x + 0.25))
}
