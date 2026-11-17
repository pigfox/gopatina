// Command ownership shows a value that is still used after it was handed
// off. It is valid Go, and plain Go runs it. In patina mode it is rejected,
// because the two copies of m share Body's backing array. See EXAMPLES.md.
package main

import (
	"fmt"
	"io"
	"os"
)

// Message is queued for sending.
type Message struct {
	To   string
	Body []byte
}

// Outbox holds queued messages.
type Outbox struct {
	queued []Message
}

// Queue stores m in the outbox. The outbox now owns a copy of m, and that
// copy shares m.Body's backing array.
func (o *Outbox) Queue(m Message) {
	o.queued = append(o.queued, m)
}

func run(w io.Writer) {
	var out Outbox
	m := Message{To: "ops", Body: []byte("disk almost full")}
	out.Queue(m)
	m.Body[0] = 'D' // also changes the queued message

	fmt.Fprintf(w, "queued: %s\n", out.queued[0].Body)
	fmt.Fprintf(w, "local:  %s\n", m.Body)
}

func main() {
	run(os.Stdout)
}
