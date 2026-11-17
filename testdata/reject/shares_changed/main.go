// Command shares_changed hands a message to a function that changes the
// bytes it shares with the caller, then uses the message again.
package main

import "fmt"

type Msg struct {
	To   string
	Body []byte
}

func scrub(m Msg) {
	m.Body[0] = '-'
	m.To = ""
}

func main() {
	m := Msg{To: "ops", Body: []byte("abc")}
	scrub(m)
	fmt.Println(m.To, len(m.Body))
}
