package main

import "fmt"

type Counter struct {
	n    int
	hist []int
}

func (c *Counter) Inc(by int) {
	c.n += by
	c.hist = append(c.hist, c.n)
}

func (c Counter) Get() int { return c.n }

func bump(c *Counter, times int) {
	for i := 0; i < times; i++ {
		c.Inc(i)
	}
}

func main() {
	cs := []Counter{{}, {n: 10}, {n: 20}}
	for i := 0; i < len(cs); i++ {
		cs[i].Inc(i + 1)
	}
	for _, c := range cs {
		c.Inc(100)
		fmt.Println(c.Get(), c.hist)
	}
	for i := range cs {
		cs[i].Inc(1000)
		fmt.Println(cs[i].Get(), len(cs[i].hist))
	}
	c := cs[0]
	bump(&c, 2)
	fmt.Println(cs[0].n, c.Get())
}
