package emit

import (
	"go/types"
	"sort"
	"strings"
)

// showFn names the function that renders %v of a struct of named type t,
// show_<type>, and records that structShowItems writes it.
func (g *gen) showFn(t types.Type) string {
	obj := types.Unalias(t).(*types.Named).Obj()
	if name, ok := g.shows[obj]; ok {
		return name
	}
	if g.shows == nil {
		g.shows = map[*types.TypeName]string{}
	}
	taken := map[string]bool{}
	for _, n := range g.funcs {
		taken[n] = true
	}
	for _, n := range g.shows {
		taken[n] = true
	}
	name := "show_" + strings.ToLower(g.types[obj])
	for taken[name] {
		name += "_"
	}
	g.shows[obj] = name
	return name
}

// structShowItems writes a show_ function for each struct type printed
// with %v: fmt's {f1 f2 ...}, each field by its own %v (the checker has
// limited them to the printable basic types, slices of them and such
// structs). A nested struct's function is written too.
func (g *gen) structShowItems() {
	written := map[*types.TypeName]bool{}
	for {
		var next []*types.TypeName
		for obj := range g.shows {
			if !written[obj] {
				next = append(next, obj)
			}
		}
		if len(next) == 0 {
			return
		}
		sort.Slice(next, func(i, j int) bool { return g.shows[next[i]] < g.shows[next[j]] })
		for _, obj := range next {
			written[obj] = true
			g.structShow(obj)
		}
	}
}

// structShow writes the show_ function of struct type obj.
func (g *gen) structShow(obj *types.TypeName) {
	t := obj.Type()
	st := t.Underlying().(*types.Struct)
	if g.out.Len() > 0 {
		g.line("")
	}
	g.line("/// %v of a " + obj.Name() + ": {f1 f2 ...}, each field by its %v.")
	g.line("fn " + g.shows[obj] + "(s: &" + g.rustType(t) + ") -> String {")
	g.ind++
	if st.NumFields() == 0 {
		g.line("String::from(\"{}\")")
	} else {
		holes := make([]string, st.NumFields())
		args := []node{}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			holes[i] = "{}"
			args = append(args, g.showField(f.Type(), "s."+g.fields[f]))
		}
		format := text("\"{{" + strings.Join(holes, " ") + "}}\"")
		g.stmtLine("", &call{head: "format!", open: "(", close: ")", args: append([]node{format}, args...)}, "")
	}
	g.ind--
	g.line("}")
}

// showField renders %v of the field at place p, of type t, as a Display.
func (g *gen) showField(t types.Type, p string) node {
	switch u := t.Underlying().(type) {
	case *types.Struct:
		return &call{head: g.showFn(t), open: "(", close: ")", args: []node{text("&" + p)}}
	case *types.Slice:
		g.rt["show_slice"] = true
		return &call{head: "rt::show_slice", open: "(", close: ")", args: []node{text("&" + p), text("|e| " + g.show(u.Elem(), "e"))}}
	}
	switch b := t.Underlying().(*types.Basic); {
	case b.Kind() == types.Float32:
		g.rt["Float32"] = true
		return &call{head: "rt::Float32", open: "(", close: ")", args: []node{text(p)}}
	case b.Info()&types.IsFloat != 0:
		g.rt["Float"] = true
		return &call{head: "rt::Float", open: "(", close: ")", args: []node{text(p)}}
	}
	return text(p)
}
