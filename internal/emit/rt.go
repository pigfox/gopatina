package emit

import (
	"sort"
	"strings"
)

// The runtime support module, src/rt.rs, holds the few pieces of Go
// behaviour that have no direct Rust spelling. Only the pieces a program
// uses are written, in a fixed order.

type rtItem struct {
	name string
	uses []string // the std items it needs
	deps []string // other runtime items it needs
	code string
}

var rtItems = []rtItem{
	{
		name: "exit_on_panic",
		code: `/// Ends the program on a panic the way Go does: the message on standard
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
/// Rust's own runtime panics carry no marker; the two messages Go programs
/// can observe from phase 1 operations are translated to Go's wording here.
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
`,
	},
	{
		name: "slice_suffix",
		code: `/// Borrows s[low:] after checking Go's signed low bound against len(s).
pub fn slice_suffix(s: &[i64], low: i64) -> &[i64] {
    if low < 0 {
        panic!("runtime error: slice bounds out of range [{low}:]");
    }
    let len = s.len();
    if low as u64 > len as u64 {
        panic!("runtime error: slice bounds out of range [{low}:{len}]");
    }
    &s[low as usize..]
}
`,
	},
	{
		name: "make_check",
		code: `/// Go's largest allocation, in bytes (runtime.maxAlloc).
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
`,
	},
	{
		name: "make_len",
		deps: []string{"make_check"},
		code: `/// The length of make([]T, n), after Go's checks.
pub fn make_len(n: i64, size: u64) -> usize {
    make_check(n, n, size);
    n as usize
}
`,
	},
	{
		name: "make_len_cap",
		deps: []string{"make_check"},
		code: `/// The length of make([]T, len, cap), after Go's checks.
pub fn make_len_cap(len: i64, cap: i64, size: u64) -> usize {
    make_check(len, cap, size);
    len as usize
}
`,
	},
	{
		name: "make_cap",
		deps: []string{"make_check"},
		code: `/// The capacity of make([]T, 0, cap), after Go's checks.
pub fn make_cap(cap: i64, size: u64) -> usize {
    make_check(0, cap, size);
    cap as usize
}
`,
	},
	{
		name: "Float",
		uses: []string{"fmt"},
		code: `/// A float64 printed the way Go's fmt prints it: %v uses the shortest
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
`,
	},
	{
		name: "hex",
		code: `/// An integer in hexadecimal the way Go's %x prints it: a minus sign and
/// the digits of the absolute value, not two's complement.
pub fn hex(x: i64, upper: bool) -> String {
    let digits = if upper {
        format!("{:X}", x.unsigned_abs())
    } else {
        format!("{:x}", x.unsigned_abs())
    };
    if x < 0 { format!("-{digits}") } else { digits }
}
`,
	},
	{
		name: "rune",
		code: `/// The character with code point x, as Go's %c prints it: U+FFFD for a
/// value that is not a valid code point.
pub fn rune(x: i64) -> char {
    u32::try_from(x)
        .ok()
        .and_then(char::from_u32)
        .unwrap_or('\u{FFFD}')
}
`,
	},
	{
		name: "runes",
		code: `/// The byte offset and code point of each character of s, as Go's
/// for i, r := range s produces them.
pub fn runes(s: &str) -> impl Iterator<Item = (i64, i32)> + '_ {
    s.char_indices().map(|(i, c)| (i as i64, c as i32))
}
`,
	},
	{
		name: "trim_space",
		code: `/// Go's strings.TrimSpace: removes leading and trailing white space, as
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
`,
	},
	{
		name: "Slice",
		uses: []string{"fmt"},
		deps: []string{"Show"},
		code: `/// A slice printed the way Go's fmt prints one: [a b c].
pub struct Slice<'a, T>(pub &'a [T]);

impl<T: Show> fmt::Display for Slice<'_, T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let mut s = String::from("[");
        for (i, x) in self.0.iter().enumerate() {
            if i > 0 {
                s.push(' ');
            }
            x.show(&mut s);
        }
        s.push(']');
        f.pad(&s)
    }
}
`,
	},
	{
		name: "Show",
		deps: []string{"Float"},
		code: `/// A value printed with Go's %v.
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
`,
	},
	{
		name: "Error",
		uses: []string{"fmt"},
		code: `/// A Go error value made by errors.New or fmt.Errorf. Functions that
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
`,
	},
	{
		name: "write_bytes",
		code: `/// Writes the parts one after the other, as Go's fmt does when it prints
/// a []byte with %s: the bytes as they are, valid UTF-8 or not.
pub fn write_bytes(w: &mut dyn std::io::Write, parts: &[&[u8]]) {
    for part in parts {
        let _ = w.write_all(part);
    }
}
`,
	},
}

// rtModule returns the source of src/rt.rs holding the named items and
// what they depend on, or "" if none are used.
func rtModule(used map[string]bool) string {
	need := map[string]bool{}
	var add func(string)
	add = func(name string) {
		if need[name] {
			return
		}
		need[name] = true
		for _, it := range rtItems {
			if it.name == name {
				for _, d := range it.deps {
					add(d)
				}
			}
		}
	}
	for name := range used {
		add(name)
	}
	if len(need) == 0 {
		return ""
	}
	uses := map[string]bool{}
	var parts []string
	for _, it := range rtItems {
		if need[it.name] {
			parts = append(parts, it.code)
			for _, u := range it.uses {
				uses[u] = true
			}
		}
	}
	var b strings.Builder
	b.WriteString("//! Runtime support for this crate, written by GoPatina: the few pieces\n")
	b.WriteString("//! of Go behaviour that have no direct Rust spelling.\n\n")
	var us []string
	for u := range uses {
		us = append(us, "use std::"+u+";\n")
	}
	sort.Strings(us)
	if len(us) > 0 {
		b.WriteString(strings.Join(us, ""))
		b.WriteString("\n")
	}
	b.WriteString(strings.Join(parts, "\n"))
	return b.String()
}
