//! Runtime support for this crate, written by GoPatina: the few pieces
//! of Go behaviour that have no direct Rust spelling.

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
