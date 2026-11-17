package main

import "fmt"

const (
	A = iota
	B
	C
	_
	E
)

const (
	KB int64 = 1 << (10 * (iota + 1))
	MB
	GB
)

const Pi = 3.14159
const Name = "const-name"
const Big = 1 << 40
const Mask uint8 = 0xF0

type Color int

const (
	Red Color = iota
	Green
	Blue
)

func (c Color) String() string {
	switch c {
	case Red:
		return "red"
	case Green:
		return "green"
	}
	return "blue"
}

func main() {
	fmt.Println(A, B, C, E, KB, MB, GB, Pi, Name, Big, Mask)
	var r float64 = 2
	fmt.Println(Pi*r*r, Big/3, Mask>>4, Mask|1)
	c := Green
	fmt.Println(c.String(), Blue.String(), int(c)+1)
	const local = 7
	var u uint8 = local * 30
	fmt.Println(u, local/2, local/2.0)
}
