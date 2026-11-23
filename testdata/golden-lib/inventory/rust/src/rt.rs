//! Runtime support for this crate, written by GoPatina: the few pieces
//! of Go behaviour that have no direct Rust spelling.

use std::any::Any;
use std::cell::Cell;
use std::cell::RefCell;
use std::fmt;
use std::rc::Rc;

/// A Go error value made by errors.New or fmt.Errorf. Functions that
/// return (T, error) in Go return Result<T, Error> here.
#[derive(Clone, Debug, PartialEq)]
pub struct Error {
    message: String,
}

impl Error {
    pub fn new(message: impl Into<String>) -> Error {
        Error {
            message: message.into(),
        }
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.pad(&self.message)
    }
}

/// A Go pointer that escapes (RFC 0013): nil, or a cell shared by every
/// copy of the pointer. Each access borrows the cell for that access only,
/// so no borrow is held while other code runs.
pub struct Ptr<T: 'static>(Option<Shared<T>>);

/// The one place the sharing is chosen: Rc now; Arc and a lock once
/// goroutines come (RFC 0002).
type Shared<T> = Rc<RefCell<T>>;

impl<T> Ptr<T> {
    /// A new cell holding v: Go's &T{...}, or a variable whose address
    /// escapes.
    pub fn new(v: T) -> Self {
        Ptr(Some(Rc::new(RefCell::new(v))))
    }

    pub fn nil() -> Self {
        Ptr(None)
    }

    pub fn is_nil(&self) -> bool {
        self.0.is_none()
    }

    fn cell(&self) -> &RefCell<T> {
        match &self.0 {
            Some(c) => c,
            None => panic!("runtime error: invalid memory address or nil pointer dereference"),
        }
    }

    /// Reads through the pointer.
    pub fn with<R>(&self, f: impl FnOnce(&T) -> R) -> R {
        f(&self.cell().borrow())
    }

    /// Writes through the pointer.
    pub fn with_mut<R>(&self, f: impl FnOnce(&mut T) -> R) -> R {
        f(&mut self.cell().borrow_mut())
    }
}

impl<T: Clone> Ptr<T> {
    /// A copy of the value pointed at: Go's *p.
    pub fn load(&self) -> T {
        self.cell().borrow().clone()
    }
}

impl<T> Clone for Ptr<T> {
    fn clone(&self) -> Self {
        Ptr(self.0.clone())
    }
}

impl<T> Default for Ptr<T> {
    fn default() -> Self {
        Ptr(None)
    }
}

/// Go compares pointers by the variable they point at.
impl<T> PartialEq for Ptr<T> {
    fn eq(&self, other: &Self) -> bool {
        match (&self.0, &other.0) {
            (Some(a), Some(b)) => Rc::ptr_eq(a, b),
            (None, None) => true,
            _ => false,
        }
    }
}

/// The last pointer to a cell frees it through release, so freeing a long
/// list frees one cell at a time, as Go's collector does, instead of one
/// call deeper per cell (which would overflow the stack).
impl<T> Drop for Ptr<T> {
    fn drop(&mut self) {
        let Some(c) = self.0.take() else {
            return;
        };
        if Rc::strong_count(&c) == 1 {
            release(c);
        }
    }
}

thread_local! {
    static PENDING: RefCell<Vec<Rc<dyn Any>>> = const { RefCell::new(Vec::new()) };
    static RELEASING: Cell<bool> = const { Cell::new(false) };
}

/// Frees cell c, and the cells freeing it frees, in a loop.
fn release(c: Rc<dyn Any>) {
    PENDING.with_borrow_mut(|p| p.push(c));
    if RELEASING.get() {
        return; // the loop below, further up, frees it
    }
    RELEASING.set(true);
    while let Some(c) = PENDING.with_borrow_mut(|p| p.pop()) {
        drop(c);
    }
    RELEASING.set(false);
}
