package borrow

import (
	"go/ast"
	"go/token"
	"go/types"
)

// A Mode is how a parameter or receiver is passed.
type Mode int

const (
	// ByValue: a Copy value, copied in as in Go.
	ByValue Mode = iota
	// Ref: a shared borrow; the callee only reads the value (&T, &str,
	// &[T]).
	Ref
	// RefMut: an exclusive borrow; the callee changes the value in place,
	// and Go's caller sees the change (&mut T, &mut [T]).
	RefMut
	// Owned: the callee keeps the value, or changes its own copy (T). The
	// caller moves the value in, or copies it when it still needs it.
	Owned
)

// A Param is the borrow checker's decision for one parameter or receiver.
type Param struct {
	Var  *types.Var
	Kind Kind
	Mode Mode
	// Mut: the callee changes its own copy, so the Rust binding is mut.
	Mut bool

	// What the callee does with the parameter.
	changesFields bool // writes a field of its own copy
	changesShared bool // writes memory the caller can see (elements, *p)
	replacesSlice bool // replaces the header through a pointer to a slice
	reassigned    bool // assigns the whole parameter, or appends to it
	escapes       bool // stores, returns or hands on the value
	firstEscape   token.Pos
	owned         bool // an owned pointer (RFC 0016): lent, never kept
}

// Keeps reports whether the callee keeps the value after it returns, or
// changes memory the caller shares. A copy handed to such a parameter would
// not behave like Go's shared one.
func (p *Param) Keeps() bool { return p.escapes || p.changesShared }

// A Func is the borrow checker's view of one function or method.
type Func struct {
	Obj    *types.Func
	Decl   *ast.FuncDecl
	Recv   *Param // nil for a function
	Params []*Param
}

// An Action says how a value is handed to a new owner.
type Action int

const (
	// Move: the value is moved; the old variable is not used again.
	Move Action = iota + 1
	// Clone: the value is copied, because the old one is still needed and
	// a copy keeps Go's meaning.
	Clone
	// Lend: a copy of an interface value only read while its source is
	// unchanged is a borrow of the source instead (let b = &a): no clone.
	Lend
)

// A Pointer describes a local variable of pointer type.
type Pointer struct {
	// Target is the variable it points to, or nil when it was initialised
	// with &T{...}, in which case it owns the value.
	Target    *types.Var
	Exclusive bool   // it is used to change the target
	path      string // the place within Target, such as b.items
	pos       token.Pos
}

// A RangeForm says how a range loop over a slice is written in Rust.
type RangeForm int

const (
	// RangeIter iterates over a borrow of the slice: the body does not
	// change it.
	RangeIter RangeForm = iota
	// RangeIndex iterates over the indices of the slice's length when the
	// loop starts, reading each element by index, because the body changes
	// the slice.
	RangeIndex
)

// A Fix is a decision the borrow checker made on its own, reported by
// -explain.
type Fix struct {
	Pos  token.Position
	Text string
}

// Result is the outcome of the borrow checker for a package.
type Result struct {
	Funcs map[*types.Func]*Func
	// Actions holds the decision for every place (a variable, field or
	// element of a non-Copy type) handed to a new owner.
	Actions map[ast.Expr]Action
	// Mut holds every local variable and parameter the program changes.
	Mut map[*types.Var]bool
	// Pointers holds every local variable of pointer type.
	Pointers map[*types.Var]*Pointer
	// Ranges holds the form of every range loop over a slice.
	Ranges map[*ast.RangeStmt]RangeForm
	// RangeCopy holds range loops whose value variable is changed in the
	// body, so it is copied out of the element.
	RangeCopy map[*ast.RangeStmt]bool
	// RangeMut holds range loops over interface values whose body changes
	// the objects they hold: each element lent mutably (iter_mut).
	RangeMut map[*ast.RangeStmt]bool
	// RangeMutVars holds the value variables of those loops: &mut borrows
	// of the elements.
	RangeMutVars map[*types.Var]bool
	// Hoist holds calls whose arguments are evaluated into temporaries
	// first, because the call also changes a variable that a later
	// argument reads.
	Hoist map[*ast.CallExpr]bool
	// Suffixes holds approved read-only suffix arguments keyed by their call.
	Suffixes map[*ast.CallExpr]*ast.SliceExpr
	// RefElems holds range variables bound to a borrow of the element, not
	// a copy: the loop does not change the slice or the variable.
	RefElems map[*types.Var]bool
	// CopyFirst holds the places handed to a call that is not the program's
	// (a print) through an exclusive local borrow whose source the same
	// statement reads after: read into a local before the statement, so the
	// borrow ends first (RFC 0017).
	CopyFirst map[ast.Expr]bool
	// Lent holds the local variables declared as a copy of an interface
	// value (b := a) that only read it while a does not change: borrows
	// of a, with no clone.
	Lent map[*types.Var]bool
	// Counted holds 3-clause for loops written as a Rust range,
	// for i in a..b: i is not changed in the body, and b does not change.
	Counted map[*ast.ForStmt]bool
	// Fixes lists the automatic decisions, sorted by position.
	Fixes []Fix

	params map[*types.Var]*Param
}

// Param returns the decision for parameter or receiver v, or nil if v is
// not one.
func (r *Result) Param(v *types.Var) *Param { return r.params[v] }

// IfaceMut reports whether interface method m, called through an
// interface value, may change the value's dynamic value: some dynamic type
// of the program implements m with a receiver it changes through (a
// pointer receiver lent exclusively). Such an interface value is lent
// mutably to the call (RFC 0012 with RFC 0017).
func (r *Result) IfaceMut(dyn []types.Type, m *types.Func) bool {
	for _, t := range dyn {
		obj, _, _ := types.LookupFieldOrMethod(t, false, m.Pkg(), m.Name())
		impl, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		if fn := r.Funcs[impl]; fn != nil && fn.Recv != nil && fn.Recv.Mode == RefMut {
			return true
		}
	}
	return false
}

// IfaceKeeps returns a dynamic type of the program that implements
// interface method m with a pointer receiver the method keeps (returns or
// stores), or nil. Called through an interface, which owns that object,
// the method would give it a second owner (RFC 0017).
func (r *Result) IfaceKeeps(dyn []types.Type, m *types.Func) types.Type {
	for _, t := range dyn {
		obj, _, _ := types.LookupFieldOrMethod(t, false, m.Pkg(), m.Name())
		impl, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		if fn := r.Funcs[impl]; fn != nil && fn.Recv != nil && fn.Recv.Mode == Owned && isPointerT(fn.Recv.Var.Type()) {
			return t
		}
	}
	return nil
}
