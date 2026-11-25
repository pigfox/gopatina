package borrow

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/pigfox/gopatina/internal/subset"
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
		whole := op.excl && w.throughMaybeNil(top.e)
		for j := i + 1; j < len(o.ops); j++ {
			later := o.ops[j]
			conflict, copyOnly, twice := false, true, false
			for k := later.from; k < later.to; k++ {
				ev := w.evs[k]
				if ev.v != top.v || !overlaps(ev.path, top.path) && !whole {
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
				case whole && !overlaps(ev.path, top.path):
					// The lend goes through an owned pointer that may be
					// nil, which Rust unwraps whole (v.as_mut().unwrap().f):
					// a read of another field comes first, a string's as a
					// copy (RFC 0017); a change is refused.
					conflict = true
					copyOnly = copyOnly && !changes
				case op.excl && op.explicit:
					conflict = true
					if changes || held {
						copyOnly = false
					}
					twice = twice || held && !changes
				case changes && !op.excl && KindOf(a.pkg.Info.Types[top.e].Type) == Value:
					// A string (or a struct of strings and numbers) read
					// before the change: a copy taken first keeps Go's meaning.
					conflict = true
				case changes:
					conflict, copyOnly = true, false
				case op.excl && held:
					conflict, copyOnly, twice = true, false, true
				}
			}
			if !conflict {
				continue
			}
			if copyOnly && call != nil && stmtLevel {
				a.res.Hoist[call] = true
				if !op.excl {
					a.fix(top.pos, fmt.Sprintf("copied %s before calling %s: a later argument changes %s; %s", trimElem(top.path), name, rootName(top.path), copyReason(a.pkg.Info.TypeOf(top.e))))
				}
				continue
			}
			p := top.path
			if twice && call != nil {
				// One object lent twice, once to be changed: two names for
				// it during the call (RFC 0017).
				a.report(pos, a.errorf(pos, "%s is lent to %s to be changed while another argument also lends %s", trimElem(p), name, rootName(p)).With(
					"in Rust, and in patina mode, an object changed through one name has no other name meanwhile (RFC 0017)",
					fmt.Sprintf("pass %s once, or copy what the other argument needs from it into a variable first", rootName(p))))
				continue
			}
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

// throughMaybeNil reports whether place e is reached through an owned
// pointer variable that may be nil (an Option<Box<T>> or Option<&mut T>,
// RFC 0017): Rust's place unwraps the variable itself.
func (w *walker) throughMaybeNil(e ast.Expr) bool {
	plan := subset.Ptrs(w.a.pkg)
	for {
		switch x := ast.Unparen(stripAddr(e)).(type) {
		case *ast.SelectorExpr:
			if id, ok := ast.Unparen(x.X).(*ast.Ident); ok {
				v := w.localVar(id)
				return v != nil && plan.Owned(v.Type()) && !plan.NonNilVar(v)
			}
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		default:
			return false
		}
	}
}

func trimElem(path string) string {
	if len(path) > 2 && path[len(path)-2:] == "[]" {
		return path[:len(path)-2]
	}
	return path
}
