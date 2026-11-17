package main

import "fmt"

type myType struct{ aB, a_b, ab int }
type my_type struct{ x int }

func doThing(v int) int  { return v + 1 }
func do_thing(v int) int { return v + 2 }

func main() {
	myVar := 1
	my_var := 2
	MyVar := 3
	fmt.Println(myVar, my_var, MyVar)
	t := myType{1, 2, 3}
	u := my_type{9}
	fmt.Println(t.aB, t.a_b, t.ab, u.x, t.aB+t.a_b)
	fmt.Println(doThing(1), do_thing(1))
	XMLHttp := 1
	xml_http := 2
	XmlHttp := 3
	fmt.Println(XMLHttp, xml_http, XmlHttp)
	a1B := 1
	a_1b := 2
	a1b := 3
	fmt.Println(a1B, a_1b, a1b)
	_x := 4
	x_ := 5
	fmt.Println(_x, x_)
}
