package borrow

import (
	"fmt"
	"go/ast"
	"go/token"
)

// An operand is one argument (or the receiver) of a call, or one operand
// of an operator that borrows both of its operands, as Rust sees it.
type operand struct {
	from, to int  // its events, w.evs[from:to]
	top      int  // the event of the place it borrows for the whole call, or -1
	excl     bool // top is borrowed exclusively
	explicit bool // top is an explicit &mut argument, not an autoref receiver
}

// operands notes where each operand's events start and end.
type operands struct {
	w   *walker
	ops []operand
}

func (w *walker) operands() *operands { return &operands{w: w} }

// add walks one operand with f. The operand borrows its place for the whole
// call when borrow is true, exclusively when excl is true.
func (o *operands) add(e ast.Expr, borrow, excl, explicit bool, f func()) {
	from := len(o.w.evs)
	f()
	op := operand{from: from, to: len(o.w.evs), top: -1, excl: excl, explicit: explicit}
	if borrow {
		if ev, ok := o.w.place(ast.Unparen(stripAddr(e))); ok {
			for i := from; i < len(o.w.evs); i++ {
				if o.w.evs[i].e == ev.e && o.w.evs[i].v == ev.v {
					op.top = i
				}
			}
		}
	}
	o.ops = append(o.ops, op)
}

// check reports operands that Rust would see borrowed while another
// operand changes the same variable. call is the call (nil for an
// operator), name what it calls, and stmtLevel reports whether the call is
// the whole expression of its statement, so its arguments can be computed
// into temporaries first.
func (o *operands) check(call *ast.CallExpr, name string, pos token.Pos, stmtLevel bool) {
	w := o.w
	a := w.a
	for i, op := range o.ops {
		if op.top < 0 {
			continue
		}
		top := w.evs[op.top]
		for j := i + 1; j < len(o.ops); j++ {
			later := o.ops[j]
			conflict, copyOnly := false, true
			for k := later.from; k < later.to; k++ {
				ev := w.evs[k]
				if ev.v != top.v || !overlaps(ev.path, top.path) {
					continue
				}
				if ev.kind == evConsume && !ev.force && KindOf(a.pkg.Info.Types[ev.e].Type) == Value {
					// Handed by value to a call inside a later operand, as
					// in f(s, g(s)): Go passes a copy, so the earlier read
					// sees the same value, and so does a copy here.
					w.evs[k].force = true
					w.evs[k].readBy = name
					continue
				}
				changes := ev.kind == evMut || ev.kind == evBorrow || (ev.kind == evConsume && !ev.force) || ev.kind == evDef
				held := later.top == k && KindOf(a.pkg.Info.Types[ev.e].Type) != Copy
				switch {
				case op.excl && op.explicit:
					conflict = true
					if changes || held {
						copyOnly = false
					}
				case changes && !op.excl && KindOf(a.pkg.Info.Types[top.e].Type) == Value:
					// A string (or a struct of strings and numbers) read
					// before the change: a copy taken first keeps Go's meaning.
					conflict = true
				case changes, op.excl && held:
					conflict, copyOnly = true, false
				}
			}
			if !conflict {
				continue
			}
			if copyOnly && call != nil && stmtLevel {
				a.res.Hoist[call] = true
				if !op.excl {
					a.fix(top.pos, fmt.Sprintf("copied %s before calling %s: a later argument changes %s; Go strings are immutable, so a copy keeps the program's meaning", trimElem(top.path), name, rootName(top.path)))
				}
				continue
			}
			p := top.path
			if call == nil {
				a.report(pos, a.errorf(pos, "%s is read by %s, and another operand changes %s", trimElem(p), name, rootName(p)).With(
					"Go does not specify whether the read comes before or after the change, so patina mode does not guess",
					fmt.Sprintf("compute the operand that changes %s into a variable in its own statement first", rootName(p))))
				continue
			}
			a.report(pos, a.errorf(pos, "%s is passed to %s, and a later argument changes %s before the call", trimElem(p), name, rootName(p)).With(
				"patina mode keeps a value passed to a call in place until the call runs, so nothing else may change it meanwhile",
				fmt.Sprintf("compute the argument that changes %s into a variable in its own statement first", rootName(p))))
		}
	}
}

func trimElem(path string) string {
	if len(path) > 2 && path[len(path)-2:] == "[]" {
		return path[:len(path)-2]
	}
	return path
}
