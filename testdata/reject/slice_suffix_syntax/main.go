package main

type box struct{ s []int }

func read([]int)          {}
func readWith([]int, int) {}
func readBytes([]byte)    {}
func size([]int) int      { return 1 }
func low() int            { return 1 }
func (box) read([]int)    {}
func change(s []int) int  { s[0] = 42; return 0 }

func main() {
	s := []int{10, 20, 30}
	q := box{[]int{10, 20, 30}}
	b := []byte{1}
	i := int32(1)
	const decimal = 1.0
	read(s[:])
	read(s[1:2])
	read(s[1:2:3])
	read(s[low():])
	read(s[i:])
	read(s[decimal:])
	read(q.s[1:])
	readWith(s[1:], 1)
	readWith(s[1:], change(s))
	readBytes(b[0:])
	q.read(s[1:])
	size(s[1:])
	_ = size(s[1:])
	v := s[1:]
	read(v)
}
