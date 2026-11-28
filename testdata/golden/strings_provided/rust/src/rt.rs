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

/// Go's strconv.FormatFloat(x, 'g', -1, 64). Rust's {:e} gives the
/// shortest digit count, but where two strings of that length are equally
/// close to x it takes the upper one; Go takes the closest, ties to even,
/// which is x rounded to that many digits, whenever that reads back as x.
fn shortest(x: f64) -> String {
    let e = format!("{x:e}");
    let digits = e.split('e').next().unwrap_or("");
    let n = digits.bytes().filter(u8::is_ascii_digit).count();
    let rounded = format!("{x:.*e}", n.saturating_sub(1));
    let e = if rounded.parse::<f64>() == Ok(x) {
        rounded
    } else {
        e
    };
    layout(&e)
}

/// Lays out Rust's {:e} digits the way Go's %v does: an exponent below
/// 1e-4 and from 1e+06 up, plain digits between.
fn layout(e: &str) -> String {
    let (mantissa, exp) = e.split_once('e').unwrap_or((e, "0"));
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

/// Go's strings.TrimSpace: removes leading and trailing white space, as
/// Go's unicode.IsSpace defines it.
pub fn trim_space(s: &str) -> String {
    s.trim_matches(is_space).to_string()
}

/// Go's unicode.White_Space, written out so the result does not depend on
/// Rust's Unicode version (docs/go-rust-disconnects.md, entry 34).
fn is_space(c: char) -> bool {
    matches!(
        c,
        '\u{0009}'..='\u{000D}'
            | '\u{0020}'
            | '\u{0085}'
            | '\u{00A0}'
            | '\u{1680}'
            | '\u{2000}'..='\u{200A}'
            | '\u{2028}'
            | '\u{2029}'
            | '\u{202F}'
            | '\u{205F}'
            | '\u{3000}'
    )
}

/// A slice printed the way Go's fmt prints one: [a b c].
pub struct Slice<'a, T>(pub &'a [T]);

impl<T: Show> fmt::Display for Slice<'_, T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        slice_fmt(
            f,
            self.0.iter().map(|x| {
                let mut e = String::new();
                x.show(&mut e);
                e
            }),
        )
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

/// A value printed with Go's %v.
pub trait Show {
    fn show(&self, out: &mut String);
}

macro_rules! show_display {
    ($($t:ty),*) => {
        $(impl Show for $t {
            fn show(&self, out: &mut String) {
                out.push_str(&self.to_string());
            }
        })*
    };
}

show_display!(i64, i32, u8, bool, String);

impl Show for f64 {
    fn show(&self, out: &mut String) {
        out.push_str(&Float(*self).to_string());
    }
}

/// Go's unicode.CaseRanges (Unicode 17.0.0): lo, hi, upper and lower
/// deltas; UL marks Go's UpperLower, alternating upper and lower pairs.
const CASE: &[(u32, u32, i32, i32)] = &[
    (0x41, 0x5A, 0, 32),
    (0x61, 0x7A, -32, 0),
    (0xB5, 0xB5, 743, 0),
    (0xC0, 0xD6, 0, 32),
    (0xD8, 0xDE, 0, 32),
    (0xE0, 0xF6, -32, 0),
    (0xF8, 0xFE, -32, 0),
    (0xFF, 0xFF, 121, 0),
    (0x100, 0x12F, UL, UL),
    (0x130, 0x130, 0, -199),
    (0x131, 0x131, -232, 0),
    (0x132, 0x137, UL, UL),
    (0x139, 0x148, UL, UL),
    (0x14A, 0x177, UL, UL),
    (0x178, 0x178, 0, -121),
    (0x179, 0x17E, UL, UL),
    (0x17F, 0x17F, -300, 0),
    (0x180, 0x180, 195, 0),
    (0x181, 0x181, 0, 210),
    (0x182, 0x185, UL, UL),
    (0x186, 0x186, 0, 206),
    (0x187, 0x188, UL, UL),
    (0x189, 0x18A, 0, 205),
    (0x18B, 0x18C, UL, UL),
    (0x18E, 0x18E, 0, 79),
    (0x18F, 0x18F, 0, 202),
    (0x190, 0x190, 0, 203),
    (0x191, 0x192, UL, UL),
    (0x193, 0x193, 0, 205),
    (0x194, 0x194, 0, 207),
    (0x195, 0x195, 97, 0),
    (0x196, 0x196, 0, 211),
    (0x197, 0x197, 0, 209),
    (0x198, 0x199, UL, UL),
    (0x19A, 0x19A, 163, 0),
    (0x19B, 0x19B, 42561, 0),
    (0x19C, 0x19C, 0, 211),
    (0x19D, 0x19D, 0, 213),
    (0x19E, 0x19E, 130, 0),
    (0x19F, 0x19F, 0, 214),
    (0x1A0, 0x1A5, UL, UL),
    (0x1A6, 0x1A6, 0, 218),
    (0x1A7, 0x1A8, UL, UL),
    (0x1A9, 0x1A9, 0, 218),
    (0x1AC, 0x1AD, UL, UL),
    (0x1AE, 0x1AE, 0, 218),
    (0x1AF, 0x1B0, UL, UL),
    (0x1B1, 0x1B2, 0, 217),
    (0x1B3, 0x1B6, UL, UL),
    (0x1B7, 0x1B7, 0, 219),
    (0x1B8, 0x1B9, UL, UL),
    (0x1BC, 0x1BD, UL, UL),
    (0x1BF, 0x1BF, 56, 0),
    (0x1C4, 0x1C4, 0, 2),
    (0x1C5, 0x1C5, -1, 1),
    (0x1C6, 0x1C6, -2, 0),
    (0x1C7, 0x1C7, 0, 2),
    (0x1C8, 0x1C8, -1, 1),
    (0x1C9, 0x1C9, -2, 0),
    (0x1CA, 0x1CA, 0, 2),
    (0x1CB, 0x1CB, -1, 1),
    (0x1CC, 0x1CC, -2, 0),
    (0x1CD, 0x1DC, UL, UL),
    (0x1DD, 0x1DD, -79, 0),
    (0x1DE, 0x1EF, UL, UL),
    (0x1F1, 0x1F1, 0, 2),
    (0x1F2, 0x1F2, -1, 1),
    (0x1F3, 0x1F3, -2, 0),
    (0x1F4, 0x1F5, UL, UL),
    (0x1F6, 0x1F6, 0, -97),
    (0x1F7, 0x1F7, 0, -56),
    (0x1F8, 0x21F, UL, UL),
    (0x220, 0x220, 0, -130),
    (0x222, 0x233, UL, UL),
    (0x23A, 0x23A, 0, 10795),
    (0x23B, 0x23C, UL, UL),
    (0x23D, 0x23D, 0, -163),
    (0x23E, 0x23E, 0, 10792),
    (0x23F, 0x240, 10815, 0),
    (0x241, 0x242, UL, UL),
    (0x243, 0x243, 0, -195),
    (0x244, 0x244, 0, 69),
    (0x245, 0x245, 0, 71),
    (0x246, 0x24F, UL, UL),
    (0x250, 0x250, 10783, 0),
    (0x251, 0x251, 10780, 0),
    (0x252, 0x252, 10782, 0),
    (0x253, 0x253, -210, 0),
    (0x254, 0x254, -206, 0),
    (0x256, 0x257, -205, 0),
    (0x259, 0x259, -202, 0),
    (0x25B, 0x25B, -203, 0),
    (0x25C, 0x25C, 42319, 0),
    (0x260, 0x260, -205, 0),
    (0x261, 0x261, 42315, 0),
    (0x263, 0x263, -207, 0),
    (0x264, 0x264, 42343, 0),
    (0x265, 0x265, 42280, 0),
    (0x266, 0x266, 42308, 0),
    (0x268, 0x268, -209, 0),
    (0x269, 0x269, -211, 0),
    (0x26A, 0x26A, 42308, 0),
    (0x26B, 0x26B, 10743, 0),
    (0x26C, 0x26C, 42305, 0),
    (0x26F, 0x26F, -211, 0),
    (0x271, 0x271, 10749, 0),
    (0x272, 0x272, -213, 0),
    (0x275, 0x275, -214, 0),
    (0x27D, 0x27D, 10727, 0),
    (0x280, 0x280, -218, 0),
    (0x282, 0x282, 42307, 0),
    (0x283, 0x283, -218, 0),
    (0x287, 0x287, 42282, 0),
    (0x288, 0x288, -218, 0),
    (0x289, 0x289, -69, 0),
    (0x28A, 0x28B, -217, 0),
    (0x28C, 0x28C, -71, 0),
    (0x292, 0x292, -219, 0),
    (0x29D, 0x29D, 42261, 0),
    (0x29E, 0x29E, 42258, 0),
    (0x345, 0x345, 84, 0),
    (0x370, 0x373, UL, UL),
    (0x376, 0x377, UL, UL),
    (0x37B, 0x37D, 130, 0),
    (0x37F, 0x37F, 0, 116),
    (0x386, 0x386, 0, 38),
    (0x388, 0x38A, 0, 37),
    (0x38C, 0x38C, 0, 64),
    (0x38E, 0x38F, 0, 63),
    (0x391, 0x3A1, 0, 32),
    (0x3A3, 0x3AB, 0, 32),
    (0x3AC, 0x3AC, -38, 0),
    (0x3AD, 0x3AF, -37, 0),
    (0x3B1, 0x3C1, -32, 0),
    (0x3C2, 0x3C2, -31, 0),
    (0x3C3, 0x3CB, -32, 0),
    (0x3CC, 0x3CC, -64, 0),
    (0x3CD, 0x3CE, -63, 0),
    (0x3CF, 0x3CF, 0, 8),
    (0x3D0, 0x3D0, -62, 0),
    (0x3D1, 0x3D1, -57, 0),
    (0x3D5, 0x3D5, -47, 0),
    (0x3D6, 0x3D6, -54, 0),
    (0x3D7, 0x3D7, -8, 0),
    (0x3D8, 0x3EF, UL, UL),
    (0x3F0, 0x3F0, -86, 0),
    (0x3F1, 0x3F1, -80, 0),
    (0x3F2, 0x3F2, 7, 0),
    (0x3F3, 0x3F3, -116, 0),
    (0x3F4, 0x3F4, 0, -60),
    (0x3F5, 0x3F5, -96, 0),
    (0x3F7, 0x3F8, UL, UL),
    (0x3F9, 0x3F9, 0, -7),
    (0x3FA, 0x3FB, UL, UL),
    (0x3FD, 0x3FF, 0, -130),
    (0x400, 0x40F, 0, 80),
    (0x410, 0x42F, 0, 32),
    (0x430, 0x44F, -32, 0),
    (0x450, 0x45F, -80, 0),
    (0x460, 0x481, UL, UL),
    (0x48A, 0x4BF, UL, UL),
    (0x4C0, 0x4C0, 0, 15),
    (0x4C1, 0x4CE, UL, UL),
    (0x4CF, 0x4CF, -15, 0),
    (0x4D0, 0x52F, UL, UL),
    (0x531, 0x556, 0, 48),
    (0x561, 0x586, -48, 0),
    (0x10A0, 0x10C5, 0, 7264),
    (0x10C7, 0x10C7, 0, 7264),
    (0x10CD, 0x10CD, 0, 7264),
    (0x10D0, 0x10FA, 3008, 0),
    (0x10FD, 0x10FF, 3008, 0),
    (0x13A0, 0x13EF, 0, 38864),
    (0x13F0, 0x13F5, 0, 8),
    (0x13F8, 0x13FD, -8, 0),
    (0x1C80, 0x1C80, -6254, 0),
    (0x1C81, 0x1C81, -6253, 0),
    (0x1C82, 0x1C82, -6244, 0),
    (0x1C83, 0x1C84, -6242, 0),
    (0x1C85, 0x1C85, -6243, 0),
    (0x1C86, 0x1C86, -6236, 0),
    (0x1C87, 0x1C87, -6181, 0),
    (0x1C88, 0x1C88, 35266, 0),
    (0x1C89, 0x1C8A, UL, UL),
    (0x1C90, 0x1CBA, 0, -3008),
    (0x1CBD, 0x1CBF, 0, -3008),
    (0x1D79, 0x1D79, 35332, 0),
    (0x1D7D, 0x1D7D, 3814, 0),
    (0x1D8E, 0x1D8E, 35384, 0),
    (0x1E00, 0x1E95, UL, UL),
    (0x1E9B, 0x1E9B, -59, 0),
    (0x1E9E, 0x1E9E, 0, -7615),
    (0x1EA0, 0x1EFF, UL, UL),
    (0x1F00, 0x1F07, 8, 0),
    (0x1F08, 0x1F0F, 0, -8),
    (0x1F10, 0x1F15, 8, 0),
    (0x1F18, 0x1F1D, 0, -8),
    (0x1F20, 0x1F27, 8, 0),
    (0x1F28, 0x1F2F, 0, -8),
    (0x1F30, 0x1F37, 8, 0),
    (0x1F38, 0x1F3F, 0, -8),
    (0x1F40, 0x1F45, 8, 0),
    (0x1F48, 0x1F4D, 0, -8),
    (0x1F51, 0x1F51, 8, 0),
    (0x1F53, 0x1F53, 8, 0),
    (0x1F55, 0x1F55, 8, 0),
    (0x1F57, 0x1F57, 8, 0),
    (0x1F59, 0x1F59, 0, -8),
    (0x1F5B, 0x1F5B, 0, -8),
    (0x1F5D, 0x1F5D, 0, -8),
    (0x1F5F, 0x1F5F, 0, -8),
    (0x1F60, 0x1F67, 8, 0),
    (0x1F68, 0x1F6F, 0, -8),
    (0x1F70, 0x1F71, 74, 0),
    (0x1F72, 0x1F75, 86, 0),
    (0x1F76, 0x1F77, 100, 0),
    (0x1F78, 0x1F79, 128, 0),
    (0x1F7A, 0x1F7B, 112, 0),
    (0x1F7C, 0x1F7D, 126, 0),
    (0x1F80, 0x1F87, 8, 0),
    (0x1F88, 0x1F8F, 0, -8),
    (0x1F90, 0x1F97, 8, 0),
    (0x1F98, 0x1F9F, 0, -8),
    (0x1FA0, 0x1FA7, 8, 0),
    (0x1FA8, 0x1FAF, 0, -8),
    (0x1FB0, 0x1FB1, 8, 0),
    (0x1FB3, 0x1FB3, 9, 0),
    (0x1FB8, 0x1FB9, 0, -8),
    (0x1FBA, 0x1FBB, 0, -74),
    (0x1FBC, 0x1FBC, 0, -9),
    (0x1FBE, 0x1FBE, -7205, 0),
    (0x1FC3, 0x1FC3, 9, 0),
    (0x1FC8, 0x1FCB, 0, -86),
    (0x1FCC, 0x1FCC, 0, -9),
    (0x1FD0, 0x1FD1, 8, 0),
    (0x1FD8, 0x1FD9, 0, -8),
    (0x1FDA, 0x1FDB, 0, -100),
    (0x1FE0, 0x1FE1, 8, 0),
    (0x1FE5, 0x1FE5, 7, 0),
    (0x1FE8, 0x1FE9, 0, -8),
    (0x1FEA, 0x1FEB, 0, -112),
    (0x1FEC, 0x1FEC, 0, -7),
    (0x1FF3, 0x1FF3, 9, 0),
    (0x1FF8, 0x1FF9, 0, -128),
    (0x1FFA, 0x1FFB, 0, -126),
    (0x1FFC, 0x1FFC, 0, -9),
    (0x2126, 0x2126, 0, -7517),
    (0x212A, 0x212A, 0, -8383),
    (0x212B, 0x212B, 0, -8262),
    (0x2132, 0x2132, 0, 28),
    (0x214E, 0x214E, -28, 0),
    (0x2160, 0x216F, 0, 16),
    (0x2170, 0x217F, -16, 0),
    (0x2183, 0x2184, UL, UL),
    (0x24B6, 0x24CF, 0, 26),
    (0x24D0, 0x24E9, -26, 0),
    (0x2C00, 0x2C2F, 0, 48),
    (0x2C30, 0x2C5F, -48, 0),
    (0x2C60, 0x2C61, UL, UL),
    (0x2C62, 0x2C62, 0, -10743),
    (0x2C63, 0x2C63, 0, -3814),
    (0x2C64, 0x2C64, 0, -10727),
    (0x2C65, 0x2C65, -10795, 0),
    (0x2C66, 0x2C66, -10792, 0),
    (0x2C67, 0x2C6C, UL, UL),
    (0x2C6D, 0x2C6D, 0, -10780),
    (0x2C6E, 0x2C6E, 0, -10749),
    (0x2C6F, 0x2C6F, 0, -10783),
    (0x2C70, 0x2C70, 0, -10782),
    (0x2C72, 0x2C73, UL, UL),
    (0x2C75, 0x2C76, UL, UL),
    (0x2C7E, 0x2C7F, 0, -10815),
    (0x2C80, 0x2CE3, UL, UL),
    (0x2CEB, 0x2CEE, UL, UL),
    (0x2CF2, 0x2CF3, UL, UL),
    (0x2D00, 0x2D25, -7264, 0),
    (0x2D27, 0x2D27, -7264, 0),
    (0x2D2D, 0x2D2D, -7264, 0),
    (0xA640, 0xA66D, UL, UL),
    (0xA680, 0xA69B, UL, UL),
    (0xA722, 0xA72F, UL, UL),
    (0xA732, 0xA76F, UL, UL),
    (0xA779, 0xA77C, UL, UL),
    (0xA77D, 0xA77D, 0, -35332),
    (0xA77E, 0xA787, UL, UL),
    (0xA78B, 0xA78C, UL, UL),
    (0xA78D, 0xA78D, 0, -42280),
    (0xA790, 0xA793, UL, UL),
    (0xA794, 0xA794, 48, 0),
    (0xA796, 0xA7A9, UL, UL),
    (0xA7AA, 0xA7AA, 0, -42308),
    (0xA7AB, 0xA7AB, 0, -42319),
    (0xA7AC, 0xA7AC, 0, -42315),
    (0xA7AD, 0xA7AD, 0, -42305),
    (0xA7AE, 0xA7AE, 0, -42308),
    (0xA7B0, 0xA7B0, 0, -42258),
    (0xA7B1, 0xA7B1, 0, -42282),
    (0xA7B2, 0xA7B2, 0, -42261),
    (0xA7B3, 0xA7B3, 0, 928),
    (0xA7B4, 0xA7C3, UL, UL),
    (0xA7C4, 0xA7C4, 0, -48),
    (0xA7C5, 0xA7C5, 0, -42307),
    (0xA7C6, 0xA7C6, 0, -35384),
    (0xA7C7, 0xA7CA, UL, UL),
    (0xA7CB, 0xA7CB, 0, -42343),
    (0xA7CC, 0xA7DB, UL, UL),
    (0xA7DC, 0xA7DC, 0, -42561),
    (0xA7F5, 0xA7F6, UL, UL),
    (0xAB53, 0xAB53, -928, 0),
    (0xAB70, 0xABBF, -38864, 0),
    (0xFF21, 0xFF3A, 0, 32),
    (0xFF41, 0xFF5A, -32, 0),
    (0x10400, 0x10427, 0, 40),
    (0x10428, 0x1044F, -40, 0),
    (0x104B0, 0x104D3, 0, 40),
    (0x104D8, 0x104FB, -40, 0),
    (0x10570, 0x1057A, 0, 39),
    (0x1057C, 0x1058A, 0, 39),
    (0x1058C, 0x10592, 0, 39),
    (0x10594, 0x10595, 0, 39),
    (0x10597, 0x105A1, -39, 0),
    (0x105A3, 0x105B1, -39, 0),
    (0x105B3, 0x105B9, -39, 0),
    (0x105BB, 0x105BC, -39, 0),
    (0x10C80, 0x10CB2, 0, 64),
    (0x10CC0, 0x10CF2, -64, 0),
    (0x10D50, 0x10D65, 0, 32),
    (0x10D70, 0x10D85, -32, 0),
    (0x118A0, 0x118BF, 0, 32),
    (0x118C0, 0x118DF, -32, 0),
    (0x16E40, 0x16E5F, 0, 32),
    (0x16E60, 0x16E7F, -32, 0),
    (0x16EA0, 0x16EB8, 0, 27),
    (0x16EBB, 0x16ED3, -27, 0),
    (0x1E900, 0x1E921, 0, 34),
    (0x1E922, 0x1E943, -34, 0),
];

/// Go's UpperLower delta (unicode.MaxRune + 1).
const UL: i32 = 0x110000;

/// Go's unicode.ToUpper (upper) or unicode.ToLower of c.
fn go_case(c: char, upper: bool) -> char {
    let r = c as u32;
    let i = CASE.partition_point(|e| e.1 < r);
    let Some(&(lo, _, du, dl)) = CASE.get(i) else {
        return c;
    };
    if r < lo {
        return c;
    }
    let d = if upper { du } else { dl };
    let m = if d == UL {
        lo + (((r - lo) & !1) | u32::from(!upper))
    } else {
        r.wrapping_add_signed(d)
    };
    char::from_u32(m).unwrap_or(c)
}

/// Go's strings.ToLower: unicode.ToLower of every rune.
pub fn to_lower(s: &str) -> String {
    s.chars().map(|c| go_case(c, false)).collect()
}

/// Go's strings.ToUpper: unicode.ToUpper of every rune.
pub fn to_upper(s: &str) -> String {
    s.chars().map(|c| go_case(c, true)).collect()
}

/// Go's case orbits (Unicode 17.0.0): the runes whose unicode.SimpleFold
/// is neither its ToLower nor, when that is itself, its ToUpper.
const FOLD: &[(u32, u32)] = &[
    (0x6B, 0x212A),
    (0x73, 0x17F),
    (0xDF, 0x1E9E),
    (0xE5, 0x212B),
    (0x130, 0x130),
    (0x131, 0x131),
    (0x1C4, 0x1C5),
    (0x1C7, 0x1C8),
    (0x1CA, 0x1CB),
    (0x1F1, 0x1F2),
    (0x390, 0x1FD3),
    (0x3A3, 0x3C2),
    (0x3B0, 0x1FE3),
    (0x3B2, 0x3D0),
    (0x3B5, 0x3F5),
    (0x3B8, 0x3D1),
    (0x3B9, 0x1FBE),
    (0x3BA, 0x3F0),
    (0x3BC, 0xB5),
    (0x3C0, 0x3D6),
    (0x3C1, 0x3F1),
    (0x3C2, 0x3C3),
    (0x3C6, 0x3D5),
    (0x3C9, 0x2126),
    (0x3D1, 0x3F4),
    (0x3F4, 0x398),
    (0x432, 0x1C80),
    (0x434, 0x1C81),
    (0x43E, 0x1C82),
    (0x441, 0x1C83),
    (0x442, 0x1C84),
    (0x44A, 0x1C86),
    (0x463, 0x1C87),
    (0x1C84, 0x1C85),
    (0x1E61, 0x1E9B),
    (0x1FBE, 0x345),
    (0x1FD3, 0x390),
    (0x1FE3, 0x3B0),
    (0x2126, 0x3A9),
    (0x212A, 0x4B),
    (0x212B, 0xC5),
    (0xA64B, 0x1C88),
    (0xFB05, 0xFB06),
    (0xFB06, 0xFB05),
];

/// Go's unicode.SimpleFold: the next rune of c's case orbit.
fn simple_fold(c: char) -> char {
    if let Ok(i) = FOLD.binary_search_by_key(&(c as u32), |e| e.0) {
        return char::from_u32(FOLD[i].1).unwrap_or(c);
    }
    let l = go_case(c, false);
    if l != c { l } else { go_case(c, true) }
}

/// Go's strings.EqualFold: equal under simple Unicode case folding.
pub fn equal_fold(s: &str, t: &str) -> bool {
    let mut t = t.chars();
    for sr in s.chars() {
        let Some(tr) = t.next() else {
            return false;
        };
        if sr == tr {
            continue;
        }
        let (sr, tr) = if tr < sr { (tr, sr) } else { (sr, tr) };
        if tr.is_ascii() {
            if sr.is_ascii_uppercase() && tr == sr.to_ascii_lowercase() {
                continue;
            }
            return false;
        }
        let mut r = simple_fold(sr);
        while r != sr && r < tr {
            r = simple_fold(r);
        }
        if r != tr {
            return false;
        }
    }
    t.next().is_none()
}

/// Go's strings.TrimPrefix.
pub fn trim_prefix(s: &str, prefix: &str) -> String {
    s.strip_prefix(prefix).unwrap_or(s).to_string()
}

/// Go's strings.TrimSuffix.
pub fn trim_suffix(s: &str, suffix: &str) -> String {
    s.strip_suffix(suffix).unwrap_or(s).to_string()
}

/// Go's strings.Split: with an empty separator, one string per rune.
pub fn split(s: &str, sep: &str) -> Vec<String> {
    if sep.is_empty() {
        return s.chars().map(String::from).collect();
    }
    s.split(sep).map(String::from).collect()
}

/// Go's strings.Cut: before and after the first sep, and whether it was found.
pub fn cut(s: &str, sep: &str) -> (String, String, bool) {
    match s.split_once(sep) {
        Some((before, after)) => (before.to_string(), after.to_string(), true),
        None => (s.to_string(), String::new(), false),
    }
}

/// Go's strings.IndexByte: the index of the first c in s, or -1.
pub fn index_byte(s: &str, c: u8) -> i64 {
    s.bytes().position(|b| b == c).map_or(-1, |i| i as i64)
}
