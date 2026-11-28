//! Runtime support for this crate, written by GoPatina: the few pieces
//! of Go behaviour that have no direct Rust spelling.

use std::fmt;

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

/// The length of make([]T, n), after Go's checks.
pub fn make_len(n: i64, size: u64) -> usize {
    make_check(n, n, size);
    n as usize
}

/// Go's x << s and x >> s for a count known only at run time: at and past
/// the width, 0 (or -1 for a negative signed x shifted right); a negative
/// count panics, as in Go.
pub fn shl<T: Shift>(x: T, s: i128) -> T {
    x.go_shl(shift_count(s))
}

pub fn shr<T: Shift>(x: T, s: i128) -> T {
    x.go_shr(shift_count(s))
}

fn shift_count(s: i128) -> u32 {
    if s < 0 {
        panic!("gopatina-user-panic:runtime error: negative shift amount");
    }
    u32::try_from(s).unwrap_or(u32::MAX)
}

pub trait Shift: Copy {
    fn go_shl(self, s: u32) -> Self;
    fn go_shr(self, s: u32) -> Self;
}

macro_rules! shift_signed {
    ($($t:ty),*) => {
        $(impl Shift for $t {
            fn go_shl(self, s: u32) -> Self {
                self.checked_shl(s).unwrap_or(0)
            }
            fn go_shr(self, s: u32) -> Self {
                self.checked_shr(s).unwrap_or(self >> (<$t>::BITS - 1))
            }
        })*
    };
}

macro_rules! shift_unsigned {
    ($($t:ty),*) => {
        $(impl Shift for $t {
            fn go_shl(self, s: u32) -> Self {
                self.checked_shl(s).unwrap_or(0)
            }
            fn go_shr(self, s: u32) -> Self {
                self.checked_shr(s).unwrap_or(0)
            }
        })*
    };
}

shift_signed!(i8, i16, i32, i64);
shift_unsigned!(u8, u16, u32, u64);

/// A float64 converted to a uint64 the way Go's compiler converts it on
/// this target. On x86-64 gc converts a value below 2^63 as an int64 (so a
/// negative one wraps) and any other as the int64 conversion of x - 2^63
/// with the top bit set: 2^63 for NaN, the infinities and values of 2^64
/// and above. Elsewhere, Rust's saturating conversion (gc's arm64 FCVTZU).
pub fn f64_to_u64(x: f64) -> u64 {
    if cfg!(target_arch = "x86_64") {
        if x < 9223372036854775808.0 {
            f64_to_i64(x) as u64
        } else {
            (f64_to_i64(x - 9223372036854775808.0) as u64) | (1 << 63)
        }
    } else {
        x as u64
    }
}

/// A float64 converted to an int64 the way Go's compiler converts it on
/// this target. Go leaves an out-of-range value implementation-specific: on
/// x86-64 gc truncates with CVTTSD2SQ, which gives i64::MIN for NaN and for
/// any value that does not fit; elsewhere (arm64) it truncates with
/// saturation and gives 0 for NaN, which is Rust's own conversion.
pub fn f64_to_i64(x: f64) -> i64 {
    #[cfg(target_arch = "x86_64")]
    if !(-9223372036854775808.0..9223372036854775808.0).contains(&x) {
        return i64::MIN;
    }
    x as i64
}

/// A float64 converted to an int32 the way Go's compiler converts it on
/// this target: i32::MIN for NaN and for a value whose truncation does not
/// fit, on x86-64 (CVTTSD2SL); Rust's saturating conversion elsewhere. Go
/// converts to the narrower integer types through this, then truncates.
pub fn f64_to_i32(x: f64) -> i32 {
    #[cfg(target_arch = "x86_64")]
    if !(x > -2147483649.0 && x < 2147483648.0) {
        return i32::MIN;
    }
    x as i32
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
