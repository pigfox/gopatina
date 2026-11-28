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

/// math/bits.Add64 as gc computes it: sum and carry out of x + y +
/// carry. Go defines it only for a carry of 0 or 1; outside that, gc on
/// x86-64 counts the carry as 1 when its low 32 bits are not zero (it
/// negates the carry as a 32-bit value to set the flag ADC reads), measured
/// with Go 1.27.1; elsewhere any carry other than 0 counts as 1 (untested).
pub fn add64(x: u64, y: u64, carry: u64) -> (u64, u64) {
    let c = if cfg!(target_arch = "x86_64") {
        u64::from(carry as u32 != 0)
    } else {
        u64::from(carry != 0)
    };
    let (s, o1) = x.overflowing_add(y);
    let (s, o2) = s.overflowing_add(c);
    (s, u64::from(o1 | o2))
}

/// math/bits.Sub64 as gc computes it: difference and borrow out of x - y -
/// borrow, with the borrow outside 0 and 1 counted as for Add64.
pub fn sub64(x: u64, y: u64, borrow: u64) -> (u64, u64) {
    let b = if cfg!(target_arch = "x86_64") {
        u64::from(borrow as u32 != 0)
    } else {
        u64::from(borrow != 0)
    };
    let (d, o1) = x.overflowing_sub(y);
    let (d, o2) = d.overflowing_sub(b);
    (d, u64::from(o1 | o2))
}

/// math/bits.Mul64: the high and low halves of the 128-bit product.
pub fn mul64(x: u64, y: u64) -> (u64, u64) {
    let p = u128::from(x) * u128::from(y);
    ((p >> 64) as u64, p as u64)
}
