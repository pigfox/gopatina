package borrow

import (
	"fmt"
	"github.com/pigfox/gopatina/internal/subset"
	"go/scanner"
	"go/token"
	"go/types"
	"strings"
)

// handOff decides how the place in ev is handed to its new owner: moved
// when nothing uses it afterwards, copied when a copy keeps Go's meaning,
// and otherwise rejected with an error.
func (l *live) handOff(ev event, lv set) {
	a := l.a
	t := a.pkg.Info.Types[ev.e].Type
	k := KindOf(t)
	if k == Error || k == Writer || k == PointerKind {
		return
	}
	next, usedLater := lv[ev.v]
	through := ev.ref || (ev.indirect && !ev.elem)
	needCopy := usedLater || ev.force || ev.elem || through
	if !needCopy {
		a.res.Actions[ev.e] = Move
		if ev.whole && ev.into != "a return" && ev.into != "a composite literal" && ev.into != "append" {
			a.fix(ev.pos, fmt.Sprintf("moved %s into %s: %s is not used afterwards", ev.path, ev.into, ev.path))
		}
		return
	}
	if subset.Ptrs(a.pkg).Owned(t) {
		l.ownedHandOff(ev, usedLater, next)
		return
	}
	what := exprString(ev.e)
	why := ""
	switch {
	case ev.readBy != "":
		why = fmt.Sprintf("%s is also read by an earlier operand of %s", ev.v.Name(), ev.readBy)
	case ev.force:
		why = fmt.Sprintf("%s is also borrowed by an earlier argument of %s", ev.v.Name(), ev.into)
	case ev.elem:
		why = fmt.Sprintf("%s is an element of %s, and an element cannot be moved out", what, rootPath(ev.path))
	case through:
		why = fmt.Sprintf("%s is borrowed, so its value cannot be moved out", ev.v.Name())
	default:
		why = fmt.Sprintf("%s is used again on line %d", ev.v.Name(), a.line(next.pos))
	}
	if k == Shared && (ev.param == nil || ev.param.Keeps()) {
		a.res.Actions[ev.e] = Clone // keeps the emitter total; the error stops the build
		l.sharedHandOff(ev, what, why, usedLater, next, t)
		return
	}
	a.res.Actions[ev.e] = Clone
	reason := copyReason(t)
	if k == Shared {
		reason = fmt.Sprintf("%s neither keeps it nor changes what it shares, so a copy behaves like Go's", ev.into)
	}
	a.fix(ev.pos, fmt.Sprintf("copied %s into %s: %s; %s", what, ev.into, why, reason))
}

// copyReason says why a copy of a value of type t, which shares nothing that
// can change, keeps the program's meaning.
func copyReason(t types.Type) string {
	switch {
	case heapPtrType(t):
		return "a copy of a pointer points at the same variable, as Go's does (RFC 0013)"
	case isStruct(t):
		return "Go copies structs too, and nothing in this one can be shared, so a copy keeps the program's meaning"
	case subset.IsIface(t):
		return "a copy of an interface value holds the same value, which no copy can change, so a copy keeps the program's meaning"
	}
	return "Go strings are immutable, so a copy keeps the program's meaning"
}

func isStruct(t types.Type) bool {
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

// rootPath returns the slice an element path is in: m.Body for m.Body[].
func rootPath(path string) string { return path[:strings.LastIndex(path, "[]")] }

// sharedHandOff reports a value that shares memory and would need a copy.
func (l *live) sharedHandOff(ev event, what, why string, usedLater bool, next use, t types.Type) {
	a := l.a
	line := ev.intoLine
	copyHint := fmt.Sprintf("hand over a copy instead: %s", copyExpr(what, t))
	switch {
	case usedLater && !ev.elem && !ev.ref && !ev.force:
		changed := next.kind == evMut
		msg := fmt.Sprintf("%s is still used after it was handed to %s on line %d", what, ev.into, line)
		later := fmt.Sprintf("%s is used here, but %s now owns it", what, ev.into)
		if changed {
			p := strings.TrimSuffix(next.path, "[]")
			msg = fmt.Sprintf("%s is changed after %s was handed to %s on line %d", p, what, ev.into, line)
			later = fmt.Sprintf("this also changes the copy %s kept, which shares %s's memory", ev.into, p)
		}
		keeps := fmt.Sprintf("%s now holds %s", ev.into, what)
		if ev.param != nil {
			keeps = fmt.Sprintf("%s keeps %s", ev.into, what)
			if !ev.param.escapes {
				keeps = fmt.Sprintf("%s changes what %s shares", ev.into, what)
			}
		}
		first, second := a.srcLine(ev.pos), a.srcLine(next.pos)
		w := max(len(first), len(second))
		d := a.errorf(next.pos, "%s", msg)
		a.report(next.pos, d.With(
			fmt.Sprintf("    %-*s  // line %d: %s", w, first, line, keeps),
			fmt.Sprintf("    %-*s  // line %d: %s", w, second, a.line(next.pos), later),
			fmt.Sprintf("patina mode does not copy %s on its own: a copy would not share that memory, so the program could behave differently", what),
			fmt.Sprintf("if %s should keep the original, hand it a copy: %s", ev.into, copyExpr(what, t)),
			fmt.Sprintf("or finish using %s before handing it over", what)))
	default:
		a.report(ev.pos, a.errorf(ev.pos, "%s cannot be handed to %s without a copy: %s", what, ev.into, why).With(
			fmt.Sprintf("in Go the copy would share %s's memory, so a change through one is seen through the other;", what),
			"patina mode does not copy such values on its own, because a copy would change what the program does",
			copyHint))
	}
}

// srcLine returns the source line at pos, without its indentation and
// without a trailing comment.
func (a *analyzer) srcLine(pos token.Pos) string {
	p := a.pkg.Fset.Position(pos)
	lines := strings.Split(string(a.pkg.Src[p.Filename]), "\n")
	line := lines[p.Line-1]
	var sc scanner.Scanner
	fset := token.NewFileSet()
	f := fset.AddFile("", -1, len(line))
	sc.Init(f, []byte(line), nil, scanner.ScanComments)
	for {
		tpos, tok, _ := sc.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT {
			line = line[:f.Offset(tpos)]
			break
		}
	}
	return strings.TrimSpace(line)
}

// copyExpr writes Go code that makes an independent copy of the value of
// expression x, of type t: a slice, or a struct that holds one.
func copyExpr(x string, t types.Type) string {
	if u, ok := t.Underlying().(*types.Slice); ok {
		if b, ok := u.Elem().(*types.Basic); ok && b.Kind() == types.Uint8 {
			return "bytes.Clone(" + x + ")"
		}
		if KindOf(u.Elem()) == Shared {
			return "a deep copy of " + x
		}
		return "slices.Clone(" + x + ")"
	}
	u := t.Underlying().(*types.Struct)
	var fields []string
	for i := 0; i < u.NumFields(); i++ {
		f := u.Field(i)
		v := x + "." + f.Name()
		if KindOf(f.Type()) == Shared {
			v = copyExpr(v, f.Type())
		}
		fields = append(fields, f.Name()+": "+v)
	}
	return types.Unalias(t).(*types.Named).Obj().Name() + "{" + strings.Join(fields, ", ") + "}"
}

// ownedHandOff reports an owned pointer (RFC 0016) that is still used
// after it moves: one owner per object means its old name is dead once it
// moves (RFC 0017). A copy would copy the whole object, which Go never
// does for a pointer. It is moved all the same, so the emitter stays
// total; the error stops the build.
func (l *live) ownedHandOff(ev event, usedLater bool, next use) {
	a := l.a
	a.res.Actions[ev.e] = Move
	elem := strings.TrimPrefix(types.TypeString(a.pkg.Info.Types[ev.e].Type, types.RelativeTo(a.pkg.Types)), "*")
	// The subset moves an owned pointer only out of a local variable (a
	// field, an element or a parameter moved shares its type), so what
	// is left to refuse is a later use.
	if usedLater {
		a.report(next.pos, a.errorf(next.pos, "%s is used after its %s moved into %s on line %d", ev.v.Name(), elem, ev.into, a.line(ev.pos)).With(
			fmt.Sprintf("in patina mode an object has one owner (RFC 0017): once %s moves, only its new owner names the %s", ev.v.Name(), elem),
			fmt.Sprintf("use the %s through its new owner, or give %s its own %s", elem, ev.v.Name(), elem)))
	}
}
