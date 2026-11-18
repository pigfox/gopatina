package main

func keep(s []int) []int {
	return s
}

func forward(s []int) {
	_ = keep(s)
}

func main() {
	s := []int{10, 20, 30}
	forward(s[1:])
}
