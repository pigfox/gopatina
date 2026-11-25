// More of RFC 0013: methods of named non-struct types through heap
// pointers, value methods, multiple assignments, nil and cells.
package main

import "fmt"

type Num int

func (n *Num) Inc() { *n++ }

func (n Num) Get() int { return int(n) * 10 }

func newNum(v Num) *Num { return &v }

type Name string

func (n Name) Up() string { return string(n) + "!" }

func newName() *Name {
	n := Name("go")
	return &n
}

type Nums []int

func (ns Nums) Set() { ns[0] = 9 }

func newNums() *Nums {
	ns := Nums{1, 2}
	return &ns
}

type In struct{ n int }

func (i In) Get() int { return i.n + 1 }

func (i *In) Inc() { i.n++ }

type Bag struct{ s []int }

func (b Bag) Sum() int {
	t := 0
	for _, x := range b.s {
		t += x
	}
	return t
}

type Counter struct {
	hits int
	name string
}

func (c Counter) Double() int { return c.hits * 2 }

func (c *Counter) Hit() int {
	c.hits++
	return c.hits
}

func (c *Counter) Self() *Counter { return c }

func show(c *Counter) string { return fmt.Sprint(c.hits) }

type Tree struct {
	Left, Right *Tree
	Key         int
	in          In
	bag         Bag
	items       []int
	name        string
}

// depth tests its fields for nil inside the cell, without copying them.
func (t *Tree) depth() int {
	if t.Left == nil && t.Right == nil {
		return 1
	}
	d := 0
	if t.Left != nil {
		d = t.Left.depth()
	}
	return d + 1
}

func leaf(k int) *Tree { return &Tree{Key: k, bag: Bag{s: []int{k, k}}, items: []int{k}, name: "t"} }

func push(t *Tree, k int) *Tree {
	if t == nil {
		return leaf(k)
	}
	t.Left = push(t.Left, k)
	return t
}

type Big struct {
	name string
	n    int
}

func boxed(b Big) *Big { return &b }

func pairOf(t *Tree) (int, int) { return t.Key, t.Key + 1 }

func addN(s []int, n int, name string) { s[0] += n + len(name) }

func split(t *Tree) (int, int) {
	return t.Key, pairSum(t)
}

func pairSum(t *Tree) int {
	t.Key++
	return t.Key
}

func xs() []int { return []int{4, 5} }

func keepSlice(p *[]int) *[]int { return p }

func keepNames(p *[]string) *[]string { return p }

func ptrTo(x int) *int { return &x }

// ints returns two new ints, declared together: a and b live in cells.
func ints() (*int, *int) {
	a, b := 4, 5
	return &a, &b
}

func set(s []int) { s[1] = 50 }

func main() {
	n := newNum(4)
	n.Inc()
	fmt.Println(*n, n.Get())
	nm := newName()
	fmt.Println(nm.Up())
	ns := newNums()
	ns.Set()
	fmt.Println(*ns)
	x1, x2 := ints()
	*x1 += *x2
	fmt.Println(*x1, *x2)

	forest := push(push(push(nil, 3), 2), 1)
	fmt.Println(forest.Left.Left.Key, forest.Left.Right == nil)
	fmt.Println(forest.Left.Left.Left == nil, nil == forest.Left.Right)
	fmt.Println(forest.depth(), forest.Left.Left.depth())
	tr := forest.Left.Left
	fmt.Println(forest.Left.Left.Key+tr.Key, tr == forest.Left.Left) // tr's last use: forest changes next
	forest.in.Inc()
	(&forest.in).Inc()
	fmt.Println(forest.in.Get(), forest.bag.Sum(), forest.name[0], forest.name == forest.Left.Left.name)
	addN(forest.items, forest.Left.Left.Key, "t") // the Key is read before forest.items is lent
	fmt.Println(forest.items)

	var c Counter
	c.Hit()
	(&c).Hit()
	self := c.Self()
	fmt.Println(c.Double(), self.Double(), show(&c), self.hits)

	b := boxed(Big{name: "b", n: 1})
	b.n += 2
	fmt.Println(b.name, b.n)

	k, extra := pairOf(forest)
	cell := &k
	fmt.Println(*cell, extra, *ptrTo(6))
	k, extra = pairOf(forest.Left.Left)
	k, more := pairOf(forest)
	fmt.Println(k, more, extra)
	k, fresh := pairOf(forest.Left.Left)
	cell2 := &fresh
	*cell2 += k
	fmt.Println(k, *cell2)
	fmt.Println(fresh)
	forest.Key, _ = pairOf(forest.Left.Left)
	forest.Key, extra = 7, forest.Key
	fmt.Println(forest.Key, extra)
	s1, s2 := split(forest)
	fmt.Println(s1, s2)

	list := xs()
	keepSlice(&list)
	set(list)
	fmt.Println(list)
	names := []string{"x", "y"}
	keepNames(&names)
	all := ""
	for _, nm := range names { // names lives in a cell: a copy of it is ranged over
		all += nm
	}
	fmt.Println(all, len(names))
	// cx is changed through cp, and used by name only once cp is done
	// with: cp is a borrow of cx (testdata/reject/owned_address_kept has
	// cx used beside cp, two owners).
	cx := 5
	cp := &cx
	*cp = 7
	fmt.Println(*cp)
	fmt.Println(cx)
}
