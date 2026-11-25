// Command owned_packages builds and walks pointers whose types another
// package of the program declares (RFC 0009): each is owned, a Box, as a
// type of main is (RFC 0017), not a shared cell.
package main

import (
	"fmt"

	"example.com/ownedpkgs/list"
)

// Stack keeps a list of another package as its field.
type Stack struct {
	Top  *list.Node
	Size int
}

func (s *Stack) Push(v int) {
	s.Top = list.Push(s.Top, v, fmt.Sprint("s", v))
	s.Size++
}

func main() {
	var head *list.Node
	for i := 1; i <= 4; i++ {
		head = list.Push(head, i, fmt.Sprint("n", i))
	}
	fmt.Println(head.Sum(), head.V, head.Name, head.Next.Next.V)
	head.Bump(10)
	fmt.Println(head.Sum(), head.Next.Name)
	if i := list.Index(head, 12); i >= 0 {
		fmt.Println("found at", i)
	}
	fmt.Println(list.Index(head, 99), head.Next == head, head.Next.Next != nil)
	s := &Stack{}
	s.Push(7)
	s.Push(8)
	fmt.Println(s.Size, s.Top.V, s.Top.Next.Name, s.Top.Sum())
	t := list.Build(3, 1)
	fmt.Println(t.Count(), t.Left.Right.Key, t.Right.Right.Right.Key)
}
