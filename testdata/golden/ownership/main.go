// Command ownership exercises the borrow checker's automatic decisions:
// moves, copies of strings and small structs, and inferred borrows.
package main

import "fmt"

type Config struct {
	Name  string
	Level int
}

type Registry struct {
	names []string
	cfgs  []Config
}

func (r *Registry) Add(name string) {
	r.names = append(r.names, name)
}

func (r *Registry) AddConfig(c Config) {
	r.cfgs = append(r.cfgs, c)
}

func (r *Registry) Count() int {
	return len(r.names) + len(r.cfgs)
}

func label(c Config) string {
	return fmt.Sprintf("%s/%d", c.Name, c.Level)
}

func upgrade(c Config) Config {
	c.Level++
	return c
}

func lengths(words []string) []int {
	var out []int
	for _, w := range words {
		out = append(out, len(w))
	}
	return out
}

func main() {
	var reg Registry
	name := "alpha"
	reg.Add(name)
	reg.Add(name + "-2")
	fmt.Println(name)

	cfg := Config{Name: "base", Level: 1}
	reg.AddConfig(cfg)
	up := upgrade(cfg)
	fmt.Println(label(cfg), label(up), reg.Count())

	words := []string{"one", "three"}
	fmt.Println(lengths(words), words)
	last := words[len(words)-1]
	words = append(words, "five")
	fmt.Println(last, len(words))

	moved := words
	fmt.Println(len(moved))

	title := "draft"
	final := title
	title = "v2"
	fmt.Println(final, title)
}
