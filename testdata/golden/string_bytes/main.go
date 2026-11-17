package main

import "fmt"

func rev(s string) string {
	r := ""
	for _, c := range s {
		r = fmt.Sprintf("%c", c) + r
	}
	return r
}

func count(s string) (int, int) {
	b, r := 0, 0
	for i := range s {
		_ = i
		r++
	}
	for i := 0; i < len(s); i++ {
		b++
	}
	return b, r
}

func main() {
	words := []string{"abc", "héé", "日本語", "😀x", ""}
	for _, w := range words {
		b, r := count(w)
		fmt.Println(w, rev(w), b, r)
	}
	s := "日本語"
	fmt.Println(s[0], s[1], len(s), s[len(s)-1])
	cmp := "é" > "z"
	fmt.Println(cmp, "日" < "本", "a"+"é" == "aé")
	var bs []byte
	for i := 0; i < 5; i++ {
		bs = append(bs, byte('a'+i))
	}
	fmt.Println(len(bs), bs[2], bs)
	str := ""
	for _, b := range bs {
		str += fmt.Sprintf("%c", b)
	}
	fmt.Println(str)
}
