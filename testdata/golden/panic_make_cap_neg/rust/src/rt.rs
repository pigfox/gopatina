//! Runtime support for this crate, written by GoPatina: the few pieces
//! of Go behaviour that have no direct Rust spelling.

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
        // Go's runtime indents each line after the first.
        return rest.replace('\n', "\n\t");
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

/// Go's largest allocation, in bytes (runtime.maxAlloc).
#[cfg(target_pointer_width = "64")]
const MAX_ALLOC: u64 = 1 << 48;
#[cfg(not(target_pointer_width = "64"))]
const MAX_ALLOC: u64 = u32::MAX as u64;

/// Go's makeslice check for make([]T, len, cap) with elements of size
/// bytes: the same tests in the same order, and the same panics.
fn make_check(len: i64, cap: i64, size: u64) {
    let too_big = |n: i64| {
        let (mem, overflow) = (n as u64).overflowing_mul(size);
        overflow || mem > MAX_ALLOC
    };
    if too_big(cap) || len < 0 || len > cap {
        if too_big(len) || len < 0 {
            panic!("runtime error: makeslice: len out of range");
        }
        panic!("runtime error: makeslice: cap out of range");
    }
}

/// The capacity of make([]T, 0, cap), after Go's checks.
pub fn make_cap(cap: i64, size: u64) -> usize {
    make_check(0, cap, size);
    cap as usize
}
