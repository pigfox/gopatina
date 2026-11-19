//! Runtime support for this crate, written by GoPatina: the few pieces
//! of Go behaviour that have no direct Rust spelling.

use std::fmt;

/// Translates Rust's standard panic text to Go's first line when the
/// operation has a Go spelling. Other panics, including Go's own runtime
/// errors produced by the runtime helpers, are already in Go's form.
fn go_panic_message(msg: &str) -> String {
    if msg == "attempt to divide by zero"
        || msg == "attempt to calculate the remainder with a divisor of zero"
    {
        return String::from("runtime error: integer divide by zero");
    }
    if let Some(rest) = msg.strip_prefix("index out of bounds: the len is ") {
        if let Some((len, index)) = rest.split_once(" but the index is ") {
            if let (Ok(len), Ok(index)) = (len.parse::<u64>(), index.parse::<u64>()) {
                // A negative Go index is emitted as a wrapped usize.
                let signed = index as i64;
                if signed < 0 {
                    return format!("runtime error: index out of range [{signed}]");
                }
                return format!("runtime error: index out of range [{index}] with length {len}");
            }
        }
    }
    msg.to_string()
}

/// Ends the program on a panic the way Go does: the message on standard
/// error, then exit status 2. (Rust's default status is 101.)
pub fn exit_on_panic() {
    std::panic::set_hook(Box::new(|info| {
        let payload = info.payload();
        let msg = if let Some(s) = payload.downcast_ref::<&str>() {
            s.to_string()
        } else if let Some(s) = payload.downcast_ref::<String>() {
            s.clone()
        } else {
            String::from("unknown panic")
        };
        let msg = go_panic_message(&msg);
        let _ = std::io::Write::flush(&mut std::io::stdout());
        eprintln!("panic: {msg}");
        std::process::exit(2);
    }));
}

/// A float64 printed the way Go's fmt prints it: %v uses the shortest
/// digits that read back as the same number, with an exponent below 1e-4
/// and from 1e+06 up; a precision (%.2f) prints fixed digits.
pub struct Float(pub f64);

impl fmt::Display for Float {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let x = self.0;
        if x.is_nan() {
            return pad(f, "NaN");
        }
        if x.is_infinite() {
            return pad(f, if x > 0.0 { "+Inf" } else { "-Inf" });
        }
        if f.precision().is_some() {
            return fmt::Display::fmt(&x, f);
        }
        pad(f, &shortest(x))
    }
}

/// Pads s to the formatter's width with spaces, as Go does for a float
/// shown as text: on the left unless the - flag is given, ignoring the
/// precision and the 0 flag.
fn pad(f: &mut fmt::Formatter<'_>, s: &str) -> fmt::Result {
    let fill = " ".repeat(f.width().unwrap_or(0).saturating_sub(s.chars().count()));
    match f.align() {
        Some(fmt::Alignment::Left) => write!(f, "{s}{fill}"),
        _ => write!(f, "{fill}{s}"),
    }
}

/// Go's strconv.FormatFloat(x, 'g', -1, 64). Rust's {:e} gives the same
/// shortest digits; only the layout differs.
fn shortest(x: f64) -> String {
    let e = format!("{x:e}");
    let (mantissa, exp) = e.split_once('e').unwrap_or((&e, "0"));
    let exp: i32 = exp.parse().unwrap_or(0);
    let (sign, mantissa) = match mantissa.strip_prefix('-') {
        Some(m) => ("-", m),
        None => ("", mantissa),
    };
    let digits = mantissa.replace('.', "");
    let body = if !(-4..6).contains(&exp) {
        let (first, rest) = digits.split_at(1);
        let point = if rest.is_empty() { "" } else { "." };
        let exp_sign = if exp < 0 { '-' } else { '+' };
        format!("{first}{point}{rest}e{exp_sign}{:02}", exp.unsigned_abs())
    } else if exp < 0 {
        format!("0.{}{digits}", "0".repeat((-exp - 1) as usize))
    } else {
        let point = exp as usize + 1;
        if digits.len() <= point {
            format!("{digits}{}", "0".repeat(point - digits.len()))
        } else {
            format!("{}.{}", &digits[..point], &digits[point..])
        }
    };
    format!("{sign}{body}")
}

/// An integer in hexadecimal the way Go's %x prints it: a minus sign and
/// the digits of the absolute value, not two's complement.
pub fn hex(x: i64, upper: bool) -> String {
    let digits = if upper {
        format!("{:X}", x.unsigned_abs())
    } else {
        format!("{:x}", x.unsigned_abs())
    };
    if x < 0 { format!("-{digits}") } else { digits }
}

/// The character with code point x, as Go's %c prints it: U+FFFD for a
/// value that is not a valid code point.
pub fn rune(x: i64) -> char {
    u32::try_from(x)
        .ok()
        .and_then(char::from_u32)
        .unwrap_or('\u{FFFD}')
}
