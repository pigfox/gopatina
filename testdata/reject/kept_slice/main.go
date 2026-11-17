// Command kept_slice hands a slice to a function that keeps it, then
// changes the slice: in Go both share the backing array.
package main

import "fmt"

type Log struct {
	lines [][]byte
}

func (l *Log) Add(line []byte) {
	l.lines = append(l.lines, line)
}

func main() {
	var log Log
	buf := []byte("first")
	log.Add(buf)
	buf[0] = 'F'
	fmt.Println(len(log.lines), len(buf))
}
