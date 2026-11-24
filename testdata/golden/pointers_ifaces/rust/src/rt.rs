//! Runtime support for this crate, written by GoPatina: the few pieces
//! of Go behaviour that have no direct Rust spelling.

use std::any::Any;
use std::cell::Cell;
use std::cell::RefCell;
use std::fmt;
use std::rc::Rc;

/// Ends the program on a panic the way Go does: the message on standard
/// error, then exit status 2. (Rust's default status is 101.)
pub fn exit_on_panic() {
    std::panic::set_hook(Box::new(|info| {
        let payload = info.payload();
        let raw = if let Some(s) = payload.downcast_ref::<&str>() {
            s.to_string()
        } else if let Some(s) = payload.downcast_ref::<String>() {
            s.clone()
        } else {
            String::from("unknown panic")
        };
        let msg = go_panic_message(&raw);
        let _ = std::io::Write::flush(&mut std::io::stdout());
        eprintln!("panic: {msg}");
        std::process::exit(2);
    }));
}

/// Panics from a Go panic(...) call carry a marker and are printed verbatim.
/// Rust's own runtime panics carry no marker; the messages Go programs can
/// observe from them are translated to Go's wording here.
fn go_panic_message(msg: &str) -> String {
    const USER: &str = "gopatina-user-panic:";
    if let Some(rest) = msg.strip_prefix(USER) {
        return rest.to_string();
    }
    match msg {
        "attempt to divide by zero"
        | "attempt to calculate the remainder with a divisor of zero" => {
            return String::from("runtime error: integer divide by zero");
        }
        // A nil owned pointer read through: Option::unwrap on None.
        "called `Option::unwrap()` on a `None` value" => {
            return String::from(
                "runtime error: invalid memory address or nil pointer dereference",
            );
        }
        _ => {}
    }
    const INDEX: &str = "index out of bounds: the len is ";
    let parts = || {
        let rest = msg.strip_prefix(INDEX)?;
        let (len, index) = rest.split_once(" but the index is ")?;
        Some((len.parse::<u64>().ok()?, index.parse::<u64>().ok()?))
    };
    if let Some((len, index)) = parts() {
        // Go's signed index reaches Rust as a wrapped usize.
        let signed = index as i64;
        if signed < 0 {
            return format!("runtime error: index out of range [{signed}]");
        }
        return format!("runtime error: index out of range [{index}] with length {len}");
    }
    msg.to_string()
}

/// A slice printed the way Go's fmt prints one, each element through its
/// String or Error method.
pub struct SliceMap<'a, T, F: Fn(&T) -> String>(pub &'a [T], pub F);

impl<T, F: Fn(&T) -> String> fmt::Display for SliceMap<'_, T, F> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        slice_fmt(f, self.0.iter().map(&self.1))
    }
}

/// Writes [a b c]. Go applies a width to each element, not to the whole
/// slice.
fn slice_fmt(f: &mut fmt::Formatter<'_>, elems: impl Iterator<Item = String>) -> fmt::Result {
    let mut s = String::from("[");
    for (i, e) in elems.enumerate() {
        if i > 0 {
            s.push(' ');
        }
        let fill = " ".repeat(f.width().unwrap_or(0).saturating_sub(e.chars().count()));
        match f.align() {
            Some(fmt::Alignment::Left) => {
                s.push_str(&e);
                s.push_str(&fill);
            }
            _ => {
                s.push_str(&fill);
                s.push_str(&e);
            }
        }
    }
    s.push(']');
    f.write_str(&s)
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
            None => panic!(
                "gopatina-user-panic:runtime error: invalid memory address or nil pointer dereference"
            ),
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

/// %v of a pointer through its String or Error method. As Go's fmt does,
/// a method that panics for a nil pointer prints <nil>.
pub fn show_ptr<T>(p: &Ptr<T>, show: impl FnOnce(&Ptr<T>) -> String) -> String {
    if !p.is_nil() {
        return show(p);
    }
    let hook = std::panic::take_hook();
    std::panic::set_hook(Box::new(|_| {}));
    let shown = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| show(p)));
    std::panic::set_hook(hook);
    shown.unwrap_or_else(|_| String::from("<nil>"))
}
