package emit

//go:generate go run ./internal/gencase casetables.go

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
`,
	},
	{
		name: "Float32",
		uses: []string{"fmt"},
		deps: []string{"Float"},
		code: `/// A float32 printed the way Go's fmt prints it: %v uses the shortest
/// digits that read back as the same float32 (FormatFloat(x, 'g', -1, 32)),
/// laid out as for a float64; a precision prints the exact value's digits.
pub struct Float32(pub f32);

impl fmt::Display for Float32 {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let x = self.0;
        if x.is_nan() || x.is_infinite() || f.precision().is_some() {
            return fmt::Display::fmt(&Float(f64::from(x)), f);
        }
        let e = format!("{x:e}");
        let digits = e.split('e').next().unwrap_or("");
        let n = digits.bytes().filter(u8::is_ascii_digit).count();
        let rounded = format!("{x:.*e}", n.saturating_sub(1));
        let e = if rounded.parse::<f32>() == Ok(x) {
            rounded
        } else {
            e
        };
        pad(f, &layout(&e))
    }
}
`,
	},
	{
		name: "shift",
		code: `/// Go's x << s and x >> s for a count known only at run time: at and past
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
`,
	},
	{
		name: "add64",
		code: `/// math/bits.Add64 as gc computes it: sum and carry out of x + y +
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
`,
	},
	{
		name: "sub64",
		code: `/// math/bits.Sub64 as gc computes it: difference and borrow out of x - y -
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
`,
	},
	{
		name: "mul64",
		code: `/// math/bits.Mul64: the high and low halves of the 128-bit product.
pub fn mul64(x: u64, y: u64) -> (u64, u64) {
    let p = u128::from(x) * u128::from(y);
    ((p >> 64) as u64, p as u64)
}
`,
	},
	{
		name: "uindex",
		code: `/// An index of Go type uint64 (uint, uintptr): out of range, Go prints it
/// as an unsigned number, which Rust's own message, read as Go's signed
/// index, would not.
pub fn uindex(i: u64, len: usize) -> usize {
    if i >= len as u64 {
        panic!("gopatina-user-panic:runtime error: index out of range [{i}] with length {len}");
    }
    i as usize
}
`,
	},
	{
		name: "f64_to_u64",
		deps: []string{"f64_to_i64"},
		code: `/// A float64 converted to a uint64 the way Go's compiler converts it on
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
`,
	},
	{
		name: "f64_to_i64",
		code: `/// A float64 converted to an int64 the way Go's compiler converts it on
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
`,
	},
	{
		name: "f64_to_i32",
		code: `/// A float64 converted to an int32 the way Go's compiler converts it on
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
		name: "hex_int",
		code: `/// An integer in hexadecimal with Go's # and 0 flags: a minus sign, then
/// 0x (0X) for #, then the digits of the absolute value padded with zeros
/// to zero_width, of which the sign takes one place and the prefix none.
pub fn hex_int(x: i128, upper: bool, sharp: bool, zero_width: usize) -> String {
    let mut digits = if upper {
        format!("{:X}", x.unsigned_abs())
    } else {
        format!("{:x}", x.unsigned_abs())
    };
    let want = zero_width.saturating_sub(usize::from(x < 0));
    if digits.len() < want {
        digits = "0".repeat(want - digits.len()) + &digits;
    }
    let prefix = match (sharp, upper) {
        (false, _) => "",
        (true, false) => "0x",
        (true, true) => "0X",
    };
    let sign = if x < 0 { "-" } else { "" };
    format!("{sign}{prefix}{digits}")
}
`,
	},
	{
		name: "hex_bytes",
		code: `/// Bytes in hexadecimal the way Go's %x prints a string or []byte: two
/// digits a byte, after 0x (0X) for the # flag unless there are none.
pub fn hex_bytes(b: &[u8], upper: bool, sharp: bool) -> String {
    let mut s = String::new();
    if sharp && !b.is_empty() {
        s.push_str(if upper { "0X" } else { "0x" });
    }
    for x in b {
        if upper {
            s.push_str(&format!("{x:02X}"));
        } else {
            s.push_str(&format!("{x:02x}"));
        }
    }
    s
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
		deps: []string{"Show", "slice_fmt"},
		code: `/// A slice printed the way Go's fmt prints one: [a b c].
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
`,
	},
	{
		name: "SliceMap",
		uses: []string{"fmt"},
		deps: []string{"slice_fmt"},
		code: `/// A slice printed the way Go's fmt prints one, each element through its
/// String or Error method.
pub struct SliceMap<'a, T, F: Fn(&T) -> String>(pub &'a [T], pub F);

impl<T, F: Fn(&T) -> String> fmt::Display for SliceMap<'_, T, F> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        slice_fmt(f, self.0.iter().map(&self.1))
    }
}
`,
	},
	{
		name: "slice_fmt",
		uses: []string{"fmt"},
		code: `/// Writes [a b c]. Go applies a width to each element, not to the whole
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
		name: "show_ints",
		deps: []string{"Show"},
		code: `show_display!(i8, i16, u16, u32, u64);
`,
	},
	{
		name: "show_f32",
		deps: []string{"Show", "Float32"},
		code: `impl Show for f32 {
    fn show(&self, out: &mut String) {
        out.push_str(&Float32(*self).to_string());
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
		name: "Ptr",
		uses: []string{"any::Any", "cell::Cell", "cell::RefCell", "rc::Rc"},
		code: `/// A Go pointer that escapes (RFC 0013): nil, or a cell shared by every
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
`,
	},
	{
		name: "Own",
		uses: []string{"any::Any", "cell::Cell", "cell::RefCell"},
		code: `/// A Go pointer that is never shared (RFC 0016): nil, or a box with one
/// owner. Nothing writes through it once it is made, so a read is a plain
/// borrow of the box.
pub struct Own<T: 'static>(Option<Box<T>>);

impl<T> Own<T> {
    /// A new box holding v: Go's &T{...}.
    pub fn new(v: T) -> Self {
        Own(Some(Box::new(v)))
    }

    pub fn nil() -> Self {
        Own(None)
    }

    pub fn is_nil(&self) -> bool {
        self.0.is_none()
    }

    /// The value pointed at, for reading through the pointer.
    pub fn get(&self) -> &T {
        match &self.0 {
            Some(b) => b,
            None => panic!(
                "gopatina-user-panic:runtime error: invalid memory address or nil pointer dereference"
            ),
        }
    }
}

impl<T: Clone> Clone for Own<T> {
    fn clone(&self) -> Self {
        Own(self.0.clone())
    }
}

impl<T> Default for Own<T> {
    fn default() -> Self {
        Own(None)
    }
}

/// Go compares pointers by the variable they point at.
impl<T> PartialEq for Own<T> {
    fn eq(&self, other: &Self) -> bool {
        match (&self.0, &other.0) {
            (Some(a), Some(b)) => std::ptr::eq::<T>(&**a, &**b),
            (None, None) => true,
            _ => false,
        }
    }
}

thread_local! {
    static OWN_DEPTH: Cell<u32> = const { Cell::new(0) };
    static OWN_PENDING: RefCell<Vec<Box<dyn Any>>> = const { RefCell::new(Vec::new()) };
}

/// Freeing a box frees the boxes it owns, one call deeper each; past a
/// fixed depth they are freed in a loop instead, so freeing a long list
/// cannot overflow the stack.
impl<T> Drop for Own<T> {
    fn drop(&mut self) {
        let Some(b) = self.0.take() else {
            return;
        };
        let depth = OWN_DEPTH.get();
        if depth >= 1024 {
            OWN_PENDING.with_borrow_mut(|p| p.push(b));
            return;
        }
        OWN_DEPTH.set(depth + 1);
        drop(b);
        if depth == 0 {
            while let Some(b) = OWN_PENDING.with_borrow_mut(|p| p.pop()) {
                drop(b);
            }
        }
        OWN_DEPTH.set(depth);
    }
}
`,
	},
	{
		name: "show_ptr",
		deps: []string{"Ptr"},
		code: `/// %v of a pointer through its String or Error method. As Go's fmt does,
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
