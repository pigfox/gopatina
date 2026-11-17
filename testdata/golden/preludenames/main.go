package main

import "fmt"

type String struct{ v string }
type Vec struct{ items []int }
type Option struct{ ok bool }
type Result struct{ code int }
type Box struct{ w, h int }

func (b Box) Area() int   { return b.w * b.h }
func (v *Vec) Push(x int) { v.items = append(v.items, x) }

func Some(x int) Option { return Option{x > 0} }
func Ok(x int) Result   { return Result{x} }

func main() {
	s := String{"hi"}
	v := Vec{}
	v.Push(1)
	v.Push(2)
	b := Box{3, 4}
	o := Some(3)
	r := Ok(9)
	fmt.Println(s.v, v.items, b.Area(), o.ok, r.code)
	None := 5
	Err := "e"
	Self := 1
	Drop := 2
	Clone := 3
	Default := 4
	fmt.Println(None, Err, Self, Drop, Clone, Default)
	var Some2, Vec2 int = 1, 2
	fmt.Println(Some2 + Vec2)
	Iterator := 3
	Into := 4
	From := 5
	Send := 6
	Sync := 7
	Sized := 8
	fmt.Println(Iterator, Into, From, Send, Sync, Sized)
}
