package main

import "fmt"

func match(loop int) int {
	type_ := loop + 1
	return type_
}

func fn(mod, impl, crate int) int {
	return mod + impl*crate
}

func main() {
	type_ := 1
	fn_ := 2
	match_ := match(3)
	loop := 4
	mod := 5
	self := 6
	ref := 7
	move := 8
	impl := 9
	crate := 10
	super := 11
	where := 12
	trait := 13
	use := 14
	async := 15
	await := 16
	dyn := 17
	box := 18
	abstract := 19
	become := 20
	final := 21
	try := 22
	yield := 23
	unsafe := 24
	static := 25
	pub := 26
	mut := 27
	let := 28
	enum := 29
	fmt.Println(type_, fn_, match_, loop, mod, self, ref, move, impl, crate, super, where, trait, use, async, await, dyn, box, abstract, become, final, try, yield, unsafe, static, pub, mut, let, enum)
	fmt.Println(fn(mod, impl, crate))
	as, in, u8, i32, f64, usize, str, bool_ := 1, 2, 3, 4, 5, 6, 7, 8
	fmt.Println(as, in, u8, i32, f64, usize, str, bool_)
}
