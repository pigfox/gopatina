// Command owned_local_borrows takes the address of local values and uses
// the pointer only within the function, beside pointers of the same types
// that escape. Rust lends the value for the pointer's life: let p = &mut x
// where something changes through it, &x where it only reads, and the
// value itself is not used until the pointer's last use (RFC 0017).
package main

import "fmt"

// Counter counts.
type Counter struct{ N int }

// Inc changes the counter through its receiver.
func (c *Counter) Inc() { c.N++ }

func newCounter() *Counter { return &Counter{} }

func newInt(v int) *int { return &v }

// add changes a number through its pointer.
func add(p *int, v int) { *p += v }

// read reads through the pointer.
func read(p *int) int { return *p }

func two() int { return 2 }

func main() {
	fmt.Println(newCounter().N, *newInt(0)) // pointers of both types escape

	x := 5
	p := &x
	*p = 7
	*p++
	add(p, two())
	fmt.Println(read(p), x)

	y := 3
	q := &y
	fmt.Println(*q+read(q), y)

	var c Counter
	pc := &c
	pc.Inc()
	(*pc).N += 10
	pc.N++
	fmt.Println(pc.N)
	fmt.Println(c.N)
}
