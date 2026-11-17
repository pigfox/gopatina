package main

import "fmt"

type Inner struct {
	Name  string
	Tags  []string
	Score float64
}

type Outer struct {
	ID    int
	In    Inner
	Items []Inner
	Flag  bool
}

func (o *Outer) Add(n string, s float64) {
	o.Items = append(o.Items, Inner{Name: n, Score: s})
	o.In.Tags = append(o.In.Tags, n)
}

func (o Outer) Total() float64 {
	t := 0.0
	for _, it := range o.Items {
		t += it.Score
	}
	return t
}

func main() {
	o := Outer{ID: 1, In: Inner{Name: "root"}}
	for i := 0; i < 4; i++ {
		o.Add(fmt.Sprintf("n%d", i), float64(i)*1.5)
	}
	fmt.Println(o.Total(), o.In.Tags, len(o.Items))
	p := o
	p.In.Name = "copy"
	p.Items[0].Name = "shared?"
	fmt.Println(o.In.Name, p.In.Name, o.Items[0].Name)
	fmt.Printf("%v %v\n", o.In.Name, o.In.Score)
	fmt.Printf("%+v\n", p.In.Tags)

}
