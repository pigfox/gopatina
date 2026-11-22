// Command time_duration exercises the first provided type, time.Duration
// (RFC 0011): its constants, arithmetic and conversions, every supported
// method at the edges of its units and of the int64 range, and printing
// through String directly, with widths and in slices.
package main

import (
	"fmt"
	"time"
)

// timeout scales a base duration, as programs do.
func timeout(attempt int) time.Duration {
	return time.Duration(attempt*attempt) * 250 * time.Millisecond
}

func main() {
	ds := []time.Duration{
		0, 1, 999, 1000, 1001, 1500, 999999, time.Millisecond, 1500 * time.Microsecond,
		time.Second - 1, time.Second, 61 * time.Second, time.Hour + time.Minute + 1500*time.Millisecond,
		-1, -time.Second, 100 * time.Hour, 1<<63 - 1, -1 << 63,
	}
	for _, d := range ds {
		fmt.Println(d, d.Nanoseconds(), d.Microseconds(), d.Milliseconds(), d.Seconds(), d.Minutes(), d.Hours(), d.Abs())
	}
	fmt.Println(len(ds), ds[17] < ds[16])
	fmt.Printf("%v|%s|%d|%8v|%-8v|\n", ds[5], ds[12], ds[12], time.Second, time.Millisecond)
	short := []time.Duration{time.Nanosecond, time.Microsecond, time.Minute}
	fmt.Println(short)
	fmt.Println(fmt.Sprint(time.Hour), fmt.Sprintf("%v", 90*time.Minute), time.Duration(42).String())

	for _, m := range []time.Duration{0, -1, 1, time.Millisecond, time.Second, time.Hour, 7} {
		d := 1499*time.Millisecond + 500*time.Microsecond
		fmt.Println(m, d.Round(m), d.Truncate(m), (-d).Round(m), (-d).Truncate(m))
	}
	big, small := time.Duration(1<<63-1), time.Duration(-1<<63)
	fmt.Println(big.Round(time.Hour), small.Round(time.Hour), big.Truncate(time.Hour), small.Truncate(time.Hour))
	fmt.Println((2500 * time.Millisecond).Round(time.Second), (-2500 * time.Millisecond).Round(time.Second))

	total := time.Duration(0)
	for i := 1; i <= 4; i++ {
		total += timeout(i)
	}
	fmt.Println(total, total > 5*time.Second, total/time.Second, total%time.Second, int64(total))
	fmt.Println((total * 3).Round(time.Second).String(), time.Duration(float64(time.Second)*1.5))
}
