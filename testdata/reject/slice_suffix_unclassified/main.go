package main

func discard(s []int) {
	_ = s
}

func convert(s []int) {
	_ = []int(s)
}

func main() {
	s := []int{10, 20, 30}
	discard(s[1:])
	convert(s[1:])
}
