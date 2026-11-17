package main

import "fmt"

type myType struct{ a int }
type my_type struct{ b int }

func main() {
	fmt.Println(myType{1}.a, my_type{2}.b)
}
