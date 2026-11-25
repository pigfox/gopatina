// Package list holds a linked list and a tree whose pointers main builds
// and walks: owned pointers of another package of the program (RFC 0017
// with RFC 0009), a Box with one owner each.
package list

// Node is a link of a list, newest first.
type Node struct {
	V    int
	Name string
	Next *Node
}

// Push puts v in front of head, moving head into the new node.
func Push(head *Node, v int, name string) *Node {
	return &Node{V: v, Name: name, Next: head}
}

// Sum walks the list, a borrow of each node in turn.
func (n *Node) Sum() int {
	t := 0
	for p := n; p != nil; p = p.Next {
		t += p.V
	}
	return t
}

// Bump adds d to every value, changing the list in place.
func (n *Node) Bump(d int) {
	n.V += d
	if n.Next != nil {
		n.Next.Bump(d)
	}
}

// Index returns the position of the first node holding v, or -1.
func Index(head *Node, v int) int {
	i := 0
	for p := head; p != nil; p = p.Next {
		if p.V == v {
			return i
		}
		i++
	}
	return -1
}

// Tree is a binary tree built from fresh nodes.
type Tree struct {
	Left, Right *Tree
	Key         int
}

// Build returns a full tree of the given depth.
func Build(depth, key int) *Tree {
	if depth == 0 {
		return &Tree{Key: key}
	}
	return &Tree{Left: Build(depth-1, 2*key), Right: Build(depth-1, 2*key+1), Key: key}
}

// Count returns the number of nodes.
func (t *Tree) Count() int {
	if t == nil {
		return 0
	}
	return 1 + t.Left.Count() + t.Right.Count()
}
