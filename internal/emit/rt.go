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
        "called ` + "`Option::unwrap()`" + ` on a ` + "`None`" + ` value" => {
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
		name: "floor",
		code: `/// Go's math.Floor: the greatest integer value at most x. A NaN comes back
/// quiet, as from Go's ROUNDSD on amd64 and FRINT on arm64.
pub fn floor(x: f64) -> f64 {
    if x.is_nan() { quiet(x) } else { x.floor() }
}
`,
		deps: []string{"quiet"},
	},
	{
		name: "ceil",
		code: `/// Go's math.Ceil: the least integer value at least x. A NaN comes back
/// quiet, as from Go's ROUNDSD on amd64 and FRINT on arm64.
pub fn ceil(x: f64) -> f64 {
    if x.is_nan() { quiet(x) } else { x.ceil() }
}
`,
		deps: []string{"quiet"},
	},
	{
		name: "trunc",
		code: `/// Go's math.Trunc: the integer value of x, rounded toward zero. A NaN comes back
/// quiet, as from Go's ROUNDSD on amd64 and FRINT on arm64.
pub fn trunc(x: f64) -> f64 {
    if x.is_nan() { quiet(x) } else { x.trunc() }
}
`,
		deps: []string{"quiet"},
	},
	{
		name: "quiet",
		code: `/// The NaN x with its quiet bit set, its sign and payload kept.
fn quiet(x: f64) -> f64 {
    f64::from_bits(x.to_bits() | 0x0008_0000_0000_0000)
}
`,
	},
	{
		name: "fma_hw",
		code: `/// x*y + z rounded once: by the processor's VFMADD when it has FMA
/// (std::arch's detection, then code compiled for the feature), by
/// mul_add (the library's fma, maybe in software) when it does not. The
/// value is the same either way; only the speed differs.
#[cfg(target_arch = "x86_64")]
fn fma_hw(x: f64, y: f64, z: f64) -> f64 {
    #[target_feature(enable = "fma")]
    unsafe fn vfmadd(x: f64, y: f64, z: f64) -> f64 {
        x.mul_add(y, z)
    }
    if std::is_x86_feature_detected!("fma") {
        // SAFETY: the processor has FMA.
        unsafe { vfmadd(x, y, z) }
    } else {
        x.mul_add(y, z)
    }
}

#[cfg(not(target_arch = "x86_64"))]
fn fma_hw(x: f64, y: f64, z: f64) -> f64 {
    x.mul_add(y, z)
}
`,
	},
	{
		name: "fma_hw_f32",
		code: `/// fma_hw for float32: VFMADD...SS when the processor has FMA.
#[cfg(target_arch = "x86_64")]
fn fma_hw_f32(x: f32, y: f32, z: f32) -> f32 {
    #[target_feature(enable = "fma")]
    unsafe fn vfmadd(x: f32, y: f32, z: f32) -> f32 {
        x.mul_add(y, z)
    }
    if std::is_x86_feature_detected!("fma") {
        // SAFETY: the processor has FMA.
        unsafe { vfmadd(x, y, z) }
    } else {
        x.mul_add(y, z)
    }
}

#[cfg(not(target_arch = "x86_64"))]
fn fma_hw_f32(x: f32, y: f32, z: f32) -> f32 {
    x.mul_add(y, z)
}
`,
	},
	{
		name: "fma_amd64",
		code: `/// gc's fused multiply-add on amd64 (VFMADD231SD, GOAMD64=v3): x*y + z
/// rounded once. A NaN operand gives the first NaN of x, y and z, quiet;
/// an invalid operation (Inf*0, Inf-Inf), the default NaN. mul_add alone
/// leaves both to the library or the instruction LLVM picks.
pub fn fma_amd64(x: f64, y: f64, z: f64) -> f64 {
    for v in [x, y, z] {
        if v.is_nan() {
            return f64::from_bits(v.to_bits() | 0x0008_0000_0000_0000);
        }
    }
    let r = fma_hw(x, y, z);
    if r.is_nan() {
        f64::from_bits(0xfff8_0000_0000_0000)
    } else {
        r
    }
}
`,
		deps: []string{"fma_hw"},
	},
	{
		name: "fma_arm64",
		code: `/// gc's fused multiply-add on arm64 (FMADD; FMSUB and FNMSUB negate an
/// operand first): a + n*m rounded once, with Arm's FPMulAdd NaN rules:
/// the first signaling NaN of a, n and m, then the first quiet one, each
/// quiet; the default NaN for a quiet NaN a with Inf*0, and for an invalid
/// operation.
pub fn fma_arm64(n: f64, m: f64, a: f64) -> f64 {
    let signaling = |v: f64| v.is_nan() && v.to_bits() & 0x0008_0000_0000_0000 == 0;
    for v in [a, n, m] {
        if signaling(v) {
            return f64::from_bits(v.to_bits() | 0x0008_0000_0000_0000);
        }
    }
    let inf_zero = (n.is_infinite() && m == 0.0) || (n == 0.0 && m.is_infinite());
    if a.is_nan() && inf_zero {
        return f64::from_bits(0x7ff8_0000_0000_0000);
    }
    for v in [a, n, m] {
        if v.is_nan() {
            return v;
        }
    }
    let r = n.mul_add(m, a);
    if r.is_nan() {
        f64::from_bits(0x7ff8_0000_0000_0000)
    } else {
        r
    }
}
`,
	},
	{
		name: "fma_amd64_f32",
		code: `/// fma_amd64 for float32 (VFMADD231SS): x*y + z
/// rounded once. A NaN operand gives the first NaN of x, y and z, quiet;
/// an invalid operation (Inf*0, Inf-Inf), the default NaN. mul_add alone
/// leaves both to the library or the instruction LLVM picks.
pub fn fma_amd64_f32(x: f32, y: f32, z: f32) -> f32 {
    for v in [x, y, z] {
        if v.is_nan() {
            return f32::from_bits(v.to_bits() | 0x0040_0000);
        }
    }
    let r = fma_hw_f32(x, y, z);
    if r.is_nan() {
        f32::from_bits(0xffc0_0000)
    } else {
        r
    }
}
`,
		deps: []string{"fma_hw_f32"},
	},
	{
		name: "fma_arm64_f32",
		code: `/// fma_arm64 for float32 (FMADDS; FMSUBS and FNMSUBS negate an
/// operand first): a + n*m rounded once, with Arm's FPMulAdd NaN rules:
/// the first signaling NaN of a, n and m, then the first quiet one, each
/// quiet; the default NaN for a quiet NaN a with Inf*0, and for an invalid
/// operation.
pub fn fma_arm64_f32(n: f32, m: f32, a: f32) -> f32 {
    let signaling = |v: f32| v.is_nan() && v.to_bits() & 0x0040_0000 == 0;
    for v in [a, n, m] {
        if signaling(v) {
            return f32::from_bits(v.to_bits() | 0x0040_0000);
        }
    }
    let inf_zero = (n.is_infinite() && m == 0.0) || (n == 0.0 && m.is_infinite());
    if a.is_nan() && inf_zero {
        return f32::from_bits(0x7fc0_0000);
    }
    for v in [a, n, m] {
        if v.is_nan() {
            return v;
        }
    }
    let r = n.mul_add(m, a);
    if r.is_nan() {
        f32::from_bits(0x7fc0_0000)
    } else {
        r
    }
}
`,
	},
	{
		name: "nan",
		code: `/// The result of gc's float operation (ADDSD, SUBSD, MULSD, DIVSD, SQRTSD
/// on amd64; FADD, FSUB, FMUL, FDIV, FSQRT on arm64) when r, its IEEE
/// result, may be NaN and a bit-level read may see it. rustc may fold a
/// NaN to its own (RFC 3514); this gives the hardware's. A result that is
/// not NaN is r.
fn nan(r: f64, ops: &[f64]) -> f64 {
    if !r.is_nan() {
        return r;
    }
    // Arm's FPProcessNaNs: the first signaling NaN of Rn and Rm, then the
    // first quiet one; x86: the first NaN operand. Both make it quiet.
    #[cfg(target_arch = "aarch64")]
    for &v in ops {
        if v.is_nan() && v.to_bits() & 0x0008_0000_0000_0000 == 0 {
            return f64::from_bits(v.to_bits() | 0x0008_0000_0000_0000);
        }
    }
    for &v in ops {
        if v.is_nan() {
            return f64::from_bits(v.to_bits() | 0x0008_0000_0000_0000);
        }
    }
    // An invalid operation: the default NaN.
    if cfg!(target_arch = "aarch64") {
        f64::from_bits(0x7ff8_0000_0000_0000)
    } else {
        f64::from_bits(0xfff8_0000_0000_0000)
    }
}
`,
	},
	{
		name: "add_nan",
		code: `/// a + b as gc computes it, NaN included (nan).
pub fn add_nan(a: f64, b: f64) -> f64 {
    nan(a + b, &[a, b])
}
`,
		deps: []string{"nan"},
	},
	{
		name: "sub_nan",
		code: `/// a - b as gc computes it, NaN included (nan).
pub fn sub_nan(a: f64, b: f64) -> f64 {
    nan(a - b, &[a, b])
}
`,
		deps: []string{"nan"},
	},
	{
		name: "mul_nan",
		code: `/// a * b as gc computes it, NaN included (nan).
pub fn mul_nan(a: f64, b: f64) -> f64 {
    nan(a * b, &[a, b])
}
`,
		deps: []string{"nan"},
	},
	{
		name: "div_nan",
		code: `/// a / b as gc computes it, NaN included (nan).
pub fn div_nan(a: f64, b: f64) -> f64 {
    nan(a / b, &[a, b])
}
`,
		deps: []string{"nan"},
	},
	{
		name: "sqrt_nan",
		code: `/// math.Sqrt as gc computes it, NaN included (nan).
pub fn sqrt_nan(x: f64) -> f64 {
    nan(x.sqrt(), &[x])
}
`,
		deps: []string{"nan"},
	},
	{
		name: "nan_f32",
		code: `/// The result of gc's float operation (ADDSD, SUBSD, MULSD, DIVSD, SQRTSD
/// on amd64; FADD, FSUB, FMUL, FDIV, FSQRT on arm64) when r, its IEEE
/// result, may be NaN and a bit-level read may see it. rustc may fold a
/// NaN to its own (RFC 3514); this gives the hardware's. A result that is
/// not NaN is r.
fn nan_f32(r: f32, ops: &[f32]) -> f32 {
    if !r.is_nan() {
        return r;
    }
    // Arm's FPProcessNaNs: the first signaling NaN of Rn and Rm, then the
    // first quiet one; x86: the first NaN operand. Both make it quiet.
    #[cfg(target_arch = "aarch64")]
    for &v in ops {
        if v.is_nan() && v.to_bits() & 0x0040_0000 == 0 {
            return f32::from_bits(v.to_bits() | 0x0040_0000);
        }
    }
    for &v in ops {
        if v.is_nan() {
            return f32::from_bits(v.to_bits() | 0x0040_0000);
        }
    }
    // An invalid operation: the default NaN.
    if cfg!(target_arch = "aarch64") {
        f32::from_bits(0x7fc0_0000)
    } else {
        f32::from_bits(0xffc0_0000)
    }
}
`,
	},
	{
		name: "add_nan_f32",
		code: `/// a + b as gc computes it, NaN included (nan_f32).
pub fn add_nan_f32(a: f32, b: f32) -> f32 {
    nan_f32(a + b, &[a, b])
}
`,
		deps: []string{"nan_f32"},
	},
	{
		name: "sub_nan_f32",
		code: `/// a - b as gc computes it, NaN included (nan_f32).
pub fn sub_nan_f32(a: f32, b: f32) -> f32 {
    nan_f32(a - b, &[a, b])
}
`,
		deps: []string{"nan_f32"},
	},
	{
		name: "mul_nan_f32",
		code: `/// a * b as gc computes it, NaN included (nan_f32).
pub fn mul_nan_f32(a: f32, b: f32) -> f32 {
    nan_f32(a * b, &[a, b])
}
`,
		deps: []string{"nan_f32"},
	},
	{
		name: "div_nan_f32",
		code: `/// a / b as gc computes it, NaN included (nan_f32).
pub fn div_nan_f32(a: f32, b: f32) -> f32 {
    nan_f32(a / b, &[a, b])
}
`,
		deps: []string{"nan_f32"},
	},
	{
		name: "sqrt_nan_f32",
		code: `/// math.Sqrt as gc computes it, NaN included (nan_f32).
pub fn sqrt_nan_f32(x: f32) -> f32 {
    nan_f32(x.sqrt(), &[x])
}
`,
		deps: []string{"nan_f32"},
	},
	{
		name: "to_f32",
		code: `/// float32(x) as gc computes it on amd64 and arm64 (CVTSD2SS, FCVT): a
/// NaN keeps its sign and the top of its payload, quiet; rustc may fold
/// it to its own NaN.
pub fn to_f32(x: f64) -> f32 {
    if x.is_nan() {
        let b = x.to_bits();
        let sign = ((b >> 63) as u32) << 31;
        return f32::from_bits(sign | 0x7fc0_0000 | ((b >> 29) as u32 & 0x003f_ffff));
    }
    x as f32
}
`,
	},
	{
		name: "to_f64",
		code: `/// float64(x) of a float32 as gc computes it (CVTSS2SD, FCVT): a NaN keeps
/// its sign and payload, quiet.
pub fn to_f64(x: f32) -> f64 {
    if x.is_nan() {
        let b = u64::from(x.to_bits());
        let payload = (b & 0x003f_ffff) << 29;
        return f64::from_bits(((b >> 31) << 63) | 0x7ff8_0000_0000_0000 | payload);
    }
    f64::from(x)
}
`,
	},
	{
		name: "fma_usable",
		code: `/// Whether Go's math uses FMA, as internal/cpu decides at start-up: the
/// processor has AVX and FMA, and GODEBUG does not turn either off
/// (cpu.avx=off, cpu.fma=off, cpu.all=off; the last setting wins, and on
/// enables only what the processor has).
#[cfg(not(target_arch = "aarch64"))]
fn fma_usable() -> bool {
    static USABLE: std::sync::OnceLock<bool> = std::sync::OnceLock::new();
    *USABLE.get_or_init(|| {
        let (mut avx, mut fma) = (true, true);
        for field in std::env::var("GODEBUG").unwrap_or_default().split(',') {
            let setting = field.strip_prefix("cpu.").and_then(|f| f.split_once('='));
            let Some((key, value)) = setting else {
                continue;
            };
            let on = match value {
                "on" => true,
                "off" => false,
                _ => continue,
            };
            match key {
                "all" => (avx, fma) = (on, on),
                "avx" => avx = on,
                "fma" => fma = on,
                _ => {}
            }
        }
        avx && fma && std::is_x86_feature_detected!("avx") && std::is_x86_feature_detected!("fma")
    })
}
`,
	},
	{
		name: "fma_usable_v3",
		code: `/// Whether Go's math uses FMA in a program built with GOAMD64=v3 or above,
/// which requires AVX and FMA: GODEBUG cannot turn them off there.
#[cfg(not(target_arch = "aarch64"))]
fn fma_usable() -> bool {
    std::is_x86_feature_detected!("avx") && std::is_x86_feature_detected!("fma")
}
`,
	},
	{
		name: "trig",
		code: `// sin.go's and trig_reduce.go's constants, bit for bit.
const TRIG_PI4A: f64 = f64::from_bits(0x3fe9_21fb_4000_0000);
const TRIG_PI4B: f64 = f64::from_bits(0x3e64_442d_0000_0000);
const TRIG_PI4C: f64 = f64::from_bits(0x3ce8_4698_98cc_5170);
const TRIG_FOUR_OVER_PI: f64 = f64::from_bits(0x3ff4_5f30_6dc9_c883);
const TRIG_PI4: f64 = f64::from_bits(0x3fe9_21fb_5444_2d18);
const TRIG_SIN: [f64; 6] = [
    f64::from_bits(0x3de5_d8fd_1fd1_9ccd),
    f64::from_bits(0xbe5a_e5e5_a929_1f5d),
    f64::from_bits(0x3ec7_1de3_567d_48a1),
    f64::from_bits(0xbf2a_01a0_19bf_df03),
    f64::from_bits(0x3f81_1111_1110_f7d0),
    f64::from_bits(0xbfc5_5555_5555_5548),
];
const TRIG_COS: [f64; 6] = [
    f64::from_bits(0xbda8_fa49_a086_1a9b),
    f64::from_bits(0x3e21_ee9d_7b4e_3f05),
    f64::from_bits(0xbe92_7e4f_7eac_4bc6),
    f64::from_bits(0x3efa_01a0_19c8_44f5),
    f64::from_bits(0xbf56_c16c_16c1_4f91),
    f64::from_bits(0x3fa5_5555_5555_554b),
];
const TRIG_MPI4: [u64; 20] = [
    0x0000_0000_0000_0001,
    0x45f3_06dc_9c88_2a53,
    0xf84e_afa3_ea69_bb81,
    0xb6c5_2b32_7887_2083,
    0xfca2_c757_bd77_8ac3,
    0x6e48_dc74_849b_a5c0,
    0x0c92_5dd4_13a3_2439,
    0xfc3b_d639_6253_4e7d,
    0xd104_6bea_5d76_8909,
    0xd338_e04d_68be_fc82,
    0x7323_ac73_06a6_73e9,
    0x3908_bf17_7bf2_5076,
    0x3ff1_2fff_bc0b_301f,
    0xde5e_2316_b414_da3e,
    0xda6c_fd9e_4f96_136e,
    0x9e8c_7ecd_3cbf_d45a,
    0xea4f_758f_d7cb_e2f6,
    0x7a0e_73ef_14a5_25d4,
    0xd7f6_bf62_3f1a_ba10,
    0xac06_608d_f8f6_d757,
];

/// Go's math.Sin, bit for bit (sin.go, which gc compiles on every target).
pub fn sin(x: f64) -> f64 {
    trig(x, false)
}

/// Go's math.Cos, bit for bit (sin.go).
pub fn cos(x: f64) -> f64 {
    trig(x, true)
}

/// a*b + c, fused (rounded once) where gc fuses that site for the target,
/// and rounded twice where it does not.
#[inline(always)]
fn trig_fm(fused: bool, a: f64, b: f64, c: f64) -> f64 {
    if fused { a.mul_add(b, c) } else { a * b + c }
}

/// sin.go's sin (cos false) and cos (cos true). gc fuses, on arm64, the
/// reduction's three steps (TRIG_FUSE_REDUCE), every step of both
/// polynomials and cos's 1 - zz/2 (TRIG_FUSE_POLY), and each polynomial's
/// outer add (TRIG_FUSE_OUTER); at GOAMD64=v3 only the outer add; below v3
/// nothing.
fn trig(x: f64, cos: bool) -> f64 {
    let (fr, fp, fo) = (TRIG_FUSE_REDUCE, TRIG_FUSE_POLY, TRIG_FUSE_OUTER);
    if x.is_nan() {
        // Sin returns a NaN as it is; Cos returns Go's NaN.
        return if cos {
            f64::from_bits(0x7ff8_0000_0000_0001)
        } else {
            x
        };
    }
    if x.is_infinite() {
        return f64::from_bits(0x7ff8_0000_0000_0001);
    }
    if !cos && x == 0.0 {
        return x;
    }
    let mut sign = false;
    let mut x = x;
    if cos {
        x = x.abs();
    } else if x < 0.0 {
        x = -x;
        sign = true;
    }
    let (mut j, z);
    if x >= 536_870_912.0 {
        // reduceThreshold, 1 << 29.
        (j, z) = trig_reduce(x);
    } else {
        j = (x * TRIG_FOUR_OVER_PI) as u64;
        let mut y = j as f64;
        if j & 1 == 1 {
            j += 1;
            y += 1.0;
        }
        j &= 7;
        let mut r = trig_fm(fr, -y, TRIG_PI4A, x);
        r = trig_fm(fr, -y, TRIG_PI4B, r);
        z = trig_fm(fr, -y, TRIG_PI4C, r);
    }
    if j > 3 {
        j -= 4;
        sign = !sign;
    }
    if cos && j > 1 {
        sign = !sign;
    }
    let zz = z * z;
    // Cos takes sin's polynomial for octants 1 and 2, Sin cos's.
    let y = if (j == 1 || j == 2) != cos {
        let mut p = TRIG_COS[0];
        for c in &TRIG_COS[1..] {
            p = trig_fm(fp, p, zz, *c);
        }
        let a = trig_fm(fp, -0.5, zz, 1.0);
        trig_fm(fo, zz * zz, p, a)
    } else {
        let mut p = TRIG_SIN[0];
        for c in &TRIG_SIN[1..] {
            p = trig_fm(fp, p, zz, *c);
        }
        trig_fm(fo, z * zz, p, z)
    };
    if sign { -y } else { y }
}

/// Go's x << n and x >> n for an unsigned 64-bit x: 0 once n reaches 64.
fn trig_shl(x: u64, n: u32) -> u64 {
    x.checked_shl(n).unwrap_or(0)
}

fn trig_shr(x: u64, n: u32) -> u64 {
    x.checked_shr(n).unwrap_or(0)
}

/// trig_reduce.go's trigReduce: x reduced modulo Pi/4 with Payne and
/// Hanek's method, for x of at least 2**29, as the octant j and z.
fn trig_reduce(x: f64) -> (u64, f64) {
    if x < TRIG_PI4 {
        return (0, x);
    }
    let mut ix = x.to_bits();
    let exp = ((ix >> 52) & 0x7ff) as i64 - 1023 - 52;
    ix &= !(0x7ff << 52);
    ix |= 1 << 52;
    let (digit, bitshift) = (((exp + 61) / 64) as usize, ((exp + 61) % 64) as u32);
    let m = &TRIG_MPI4;
    let z0 = trig_shl(m[digit], bitshift) | trig_shr(m[digit + 1], 64 - bitshift);
    let z1 = trig_shl(m[digit + 1], bitshift) | trig_shr(m[digit + 2], 64 - bitshift);
    let z2 = trig_shl(m[digit + 2], bitshift) | trig_shr(m[digit + 3], 64 - bitshift);
    let z2hi = ((u128::from(z2) * u128::from(ix)) >> 64) as u64;
    let z1p = u128::from(z1) * u128::from(ix);
    let (z1hi, z1lo) = ((z1p >> 64) as u64, z1p as u64);
    let z0lo = z0.wrapping_mul(ix);
    let (lo, c) = z1lo.overflowing_add(z2hi);
    let hi = z0lo.wrapping_add(z1hi).wrapping_add(u64::from(c));
    let mut j = hi >> 61;
    let mut hi = (hi << 3) | (lo >> 61);
    let lz = hi.leading_zeros();
    let e = 1023 - (u64::from(lz) + 1);
    hi = trig_shl(hi, lz + 1) | trig_shr(lo, 64 - (lz + 1));
    hi = trig_shr(hi, 64 - 52);
    hi |= e << 52;
    let mut z = f64::from_bits(hi);
    if j & 1 == 1 {
        j += 1;
        j &= 7;
        z -= 1.0;
    }
    (j, z * TRIG_PI4)
}
`,
	},
	{
		name: "trig_fuse",
		code: `/// Which of sin.go's multiply-adds gc fuses (rt::trig): on arm64 all of
/// them; on amd64 none, below GOAMD64=v3.
#[cfg(target_arch = "aarch64")]
const TRIG_FUSE_REDUCE: bool = true;
#[cfg(not(target_arch = "aarch64"))]
const TRIG_FUSE_REDUCE: bool = false;
const TRIG_FUSE_POLY: bool = TRIG_FUSE_REDUCE;
const TRIG_FUSE_OUTER: bool = TRIG_FUSE_REDUCE;
`,
	},
	{
		name: "trig_fuse_v3",
		code: `/// Which of sin.go's multiply-adds gc fuses (rt::trig): on arm64 all of
/// them; on amd64 each polynomial's outer add, from GOAMD64=v3.
#[cfg(target_arch = "aarch64")]
const TRIG_FUSE_REDUCE: bool = true;
#[cfg(not(target_arch = "aarch64"))]
const TRIG_FUSE_REDUCE: bool = false;
const TRIG_FUSE_POLY: bool = TRIG_FUSE_REDUCE;
const TRIG_FUSE_OUTER: bool = true;
`,
	},
	{
		name: "pow",
		code: `/// Go's math.Pow, bit for bit (pow.go, which gc compiles on every target):
/// Exp and Log for the fraction of y, then squarings for its integer part.
pub fn pow(x: f64, y: f64) -> f64 {
    if y == 0.0 || x == 1.0 {
        return 1.0;
    }
    if y == 1.0 {
        return x;
    }
    if x.is_nan() || y.is_nan() {
        return f64::from_bits(0x7ff8_0000_0000_0001);
    }
    if x == 0.0 {
        let odd = x.is_sign_negative() && pow_odd_int(y);
        if y < 0.0 {
            return if odd {
                f64::NEG_INFINITY
            } else {
                f64::INFINITY
            };
        }
        return if odd { x } else { 0.0 };
    }
    if y.is_infinite() {
        return if x == -1.0 {
            1.0
        } else if (x.abs() < 1.0) == (y == f64::INFINITY) {
            0.0
        } else {
            f64::INFINITY
        };
    }
    if x.is_infinite() {
        if x == f64::NEG_INFINITY {
            return pow(1.0 / x, -y);
        }
        return if y < 0.0 { 0.0 } else { f64::INFINITY };
    }
    if y == 0.5 {
        return sqrt_nan(x);
    }
    if y == -0.5 {
        let s = sqrt_nan(x);
        return nan(1.0 / s, &[1.0, s]);
    }
    // Modf(Abs(y)): Trunc, and the fraction, which is not negative here.
    let mut yi = y.abs().trunc();
    let mut yf = y.abs() - yi;
    if yf != 0.0 && x < 0.0 {
        return f64::from_bits(0x7ff8_0000_0000_0001);
    }
    if yi >= 9_223_372_036_854_775_808.0 {
        // 1 << 63: a large even integer, so the result overflows or
        // underflows, except for x == -1.
        return if x == -1.0 {
            1.0
        } else if (x.abs() < 1.0) == (y > 0.0) {
            0.0
        } else {
            f64::INFINITY
        };
    }
    let mut a1 = 1.0;
    let mut ae: i64 = 0;
    if yf != 0.0 {
        if yf > 0.5 {
            yf -= 1.0;
            yi += 1.0;
        }
        a1 = exp(yf * log(x));
    }
    let (mut x1, mut xe) = pow_frexp(x);
    let mut i = yi as i64;
    while i != 0 {
        if !(-4096..=4096).contains(&xe) {
            // xe would overflow the shift below; the result underflows or
            // overflows in pow_ldexp.
            ae += xe;
            break;
        }
        if i & 1 == 1 {
            a1 *= x1;
            ae += xe;
        }
        x1 *= x1;
        xe <<= 1;
        if x1 < 0.5 {
            // x1 += x1. gc fuses it with the square before it on arm64 and
            // from GOAMD64=v3, computing old*old + x1 rounded once; that is
            // always 2*x1: the exact value is within ulp(x1)/2, a quarter
            // of ulp(2*x1), of 2*x1, which is a float64. So both are this.
            x1 += x1;
            xe -= 1;
        }
        i >>= 1;
    }
    if y < 0.0 {
        a1 = 1.0 / a1;
        ae = -ae;
    }
    pow_ldexp(a1, ae)
}

/// pow.go's isOddInt.
fn pow_odd_int(x: f64) -> bool {
    if x.abs() >= 9_007_199_254_740_992.0 {
        // 1 << 53: every float64 from here on is an even integer.
        return false;
    }
    let xi = x.trunc();
    x - xi == 0.0 && (xi as i64) & 1 == 1
}

/// Go's math.Frexp (frexp.go): a subnormal f is made normal first.
fn pow_frexp(f: f64) -> (f64, i64) {
    if f == 0.0 || f.is_infinite() || f.is_nan() {
        return (f, 0);
    }
    let (f, mut e) = if f.abs() < f64::MIN_POSITIVE {
        (f * 4_503_599_627_370_496.0, -52)
    } else {
        (f, 0)
    };
    let mut b = f.to_bits();
    e += ((b >> 52) & 0x7ff) as i64 - 1022;
    b &= !(0x7ff << 52);
    b |= 1022 << 52;
    (f64::from_bits(b), e)
}

/// Go's math.Ldexp (ldexp.go): frac * 2**exp, through the subnormals.
fn pow_ldexp(frac: f64, exp: i64) -> f64 {
    if frac == 0.0 || frac.is_infinite() || frac.is_nan() {
        return frac;
    }
    let (frac, e) = if frac.abs() < f64::MIN_POSITIVE {
        (frac * 4_503_599_627_370_496.0, -52)
    } else {
        (frac, 0)
    };
    let mut exp = exp + e;
    let mut x = frac.to_bits();
    exp += ((x >> 52) & 0x7ff) as i64 - 1023;
    if exp < -1075 {
        return 0.0f64.copysign(frac);
    }
    if exp > 1023 {
        return if frac < 0.0 {
            f64::NEG_INFINITY
        } else {
            f64::INFINITY
        };
    }
    let mut m = 1.0;
    if exp < -1022 {
        exp += 53;
        m = 1.0 / 9_007_199_254_740_992.0;
    }
    x &= !(0x7ff << 52);
    x |= ((exp + 1023) as u64) << 52;
    m * f64::from_bits(x)
}
`,
		deps: []string{"exp", "log", "sqrt_nan", "nan"},
	},
	{
		name: "exp",
		code: `/// Go's math.Exp, bit for bit: on amd64 its assembly (exp_amd64.s,
/// Shibata's method), with fused multiply-adds where Go uses them
/// (fma_usable); on arm64 its assembly (exp_arm64.s), which always fuses.
pub fn exp(x: f64) -> f64 {
    #[cfg(target_arch = "aarch64")]
    return exp_arm64(x);
    #[cfg(not(target_arch = "aarch64"))]
    return exp_amd64(x);
}

#[cfg(not(target_arch = "aarch64"))]
fn exp_amd64(x: f64) -> f64 {
    // exp_amd64.s's constants, bit for bit.
    const LOG2E: f64 = f64::from_bits(0x3ff7_1547_652b_82fe);
    const LN2U: f64 = f64::from_bits(0x3fe6_2e42_fefa_3000);
    const LN2L: f64 = f64::from_bits(0x3d53_de6a_f278_ece6);
    const OVERFLOW: f64 = f64::from_bits(0x4086_2e42_fefa_39ef);
    const C: [f64; 7] = [
        f64::from_bits(0x3efa_01a0_1a01_a01a),
        f64::from_bits(0x3f2a_01a0_1a01_a01a),
        f64::from_bits(0x3f56_c16c_16c1_6c17),
        f64::from_bits(0x3f81_1111_1111_1111),
        f64::from_bits(0x3fa5_5555_5555_5555),
        f64::from_bits(0x3fc5_5555_5555_5555),
        0.5,
    ];
    let b = x.to_bits();
    if b & !(1 << 63) >= 0x7ff0_0000_0000_0000 {
        // NaN, returned as it is, and the infinities.
        return if b == 0xfff0_0000_0000_0000 { 0.0 } else { x };
    }
    if x > OVERFLOW {
        return f64::INFINITY;
    }
    // CVTSD2SL rounds to nearest even; out of range it is i32::MIN.
    let k = (LOG2E * x).round_ties_even() as i32;
    let kf = f64::from(k);
    // exp_amd64.s's fused path, compiled for AVX and FMA so each mul_add
    // is one VFMADD in line, not a call to the library's fma.
    #[target_feature(enable = "avx,fma")]
    unsafe fn fused(x: f64, kf: f64) -> f64 {
        let mut r = (-kf).mul_add(LN2U, x);
        r = (-kf).mul_add(LN2L, r);
        r *= 0.0625;
        let mut p = C[0];
        for c in &C[1..] {
            p = p.mul_add(r, *c);
        }
        p = p.mul_add(r, 1.0);
        r *= p;
        for _ in 0..3 {
            let t = r + 2.0;
            r *= t;
        }
        let t = r + 2.0;
        r.mul_add(t, 1.0)
    }
    let mut r;
    let mut p;
    if fma_usable() {
        // SAFETY: fma_usable found AVX and FMA on this processor.
        r = unsafe { fused(x, kf) };
    } else {
        r = x - LN2U * kf;
        r -= LN2L * kf;
        r *= 0.0625;
        p = C[0];
        for c in &C[1..] {
            p = p * r + *c;
        }
        p = p * r + 1.0;
        r *= p;
        for _ in 0..4 {
            let t = 2.0 + r;
            r *= t;
        }
        r += 1.0;
    }
    // r * 2**k, through the subnormal range as Go does.
    let mut e = i64::from(k) + 0x3ff;
    if e <= 0 {
        if e < -52 {
            return 0.0;
        }
        e += 0x3fe;
        r *= f64::from_bits((e as u64) << 52);
        e = 1;
    } else if e >= 0x7ff {
        return f64::INFINITY;
    }
    r * f64::from_bits((e as u64) << 52)
}

#[cfg(target_arch = "aarch64")]
fn exp_arm64(x: f64) -> f64 {
    // exp_arm64.s's constants, bit for bit.
    const LN2HI: f64 = f64::from_bits(0x3fe6_2e42_fee0_0000);
    const LN2LO: f64 = f64::from_bits(0x3dea_39ef_3579_3c76);
    const LOG2E: f64 = f64::from_bits(0x3ff7_1547_652b_82fe);
    const OVERFLOW: f64 = f64::from_bits(0x4086_2e42_fefa_39ef);
    const UNDERFLOW: f64 = f64::from_bits(0xc087_4910_d52d_3051);
    const P: [f64; 5] = [
        f64::from_bits(0x3e66_3769_72be_a4d0),
        f64::from_bits(0xbebb_bd41_c5d2_6bf1),
        f64::from_bits(0x3f11_566a_af25_de2c),
        f64::from_bits(0xbf66_c16c_16be_bd93),
        f64::from_bits(0x3fc5_5555_5555_5555),
    ];
    if x.is_nan() {
        return x;
    }
    if x > OVERFLOW {
        return f64::INFINITY;
    }
    if x < UNDERFLOW {
        return 0.0;
    }
    if x.abs() < f64::from_bits(0x3e30_0000_0000_0000) {
        return x + 1.0;
    }
    // k = Log2e*x -+ 0.5, truncated (FCVTZSD).
    let k = if x < 0.0 {
        LOG2E.mul_add(x, -0.5)
    } else {
        LOG2E.mul_add(x, 0.5)
    };
    let ki = k as i64;
    let kf = ki as f64;
    let hi = (-kf).mul_add(LN2HI, x);
    let lo = kf * LN2LO;
    let r = hi - lo;
    let t = r * r;
    let mut c = t.mul_add(P[0], P[1]);
    for p in &P[2..] {
        c = t.mul_add(c, *p);
    }
    let c = (-t).mul_add(c, r);
    let y = 1.0 - ((lo - (r * c) / (2.0 - c)) - hi);
    // Ldexp(y, k), inline as Go writes it.
    let b = y.to_bits();
    let mut e = (b >> 52) as i64 + ki;
    let mut m = 1.0;
    if e < 1 {
        e += 52;
        m = f64::from_bits(0x3cb0_0000_0000_0000);
    }
    f64::from_bits(((e as u64) << 52) | (b & 0x000f_ffff_ffff_ffff)) * m
}
`,
	},
	{
		name: "log",
		code: `/// Go's math.Log, bit for bit: on amd64 its assembly (log_amd64.s), on
/// arm64 its Go (log.go) as gc compiles it there, fusing ten of its
/// multiply-adds.
pub fn log(x: f64) -> f64 {
    #[cfg(target_arch = "aarch64")]
    return log_arm64(x);
    #[cfg(not(target_arch = "aarch64"))]
    return log_amd64(x);
}

/// log.go's and log_amd64.s's constants, bit for bit.
const LOG_LN2HI: f64 = f64::from_bits(0x3fe6_2e42_fee0_0000);
const LOG_LN2LO: f64 = f64::from_bits(0x3dea_39ef_3579_3c76);
const LOG_HSQRT2: f64 = f64::from_bits(0x3fe6_a09e_667f_3bcd);
const LOG_L: [f64; 7] = [
    f64::from_bits(0x3fe5_5555_5555_5593),
    f64::from_bits(0x3fd9_9999_9997_fa04),
    f64::from_bits(0x3fd2_4924_9422_9359),
    f64::from_bits(0x3fcc_71c5_1d8e_78af),
    f64::from_bits(0x3fc7_4664_96cb_03de),
    f64::from_bits(0x3fc3_9a09_d078_c69f),
    f64::from_bits(0x3fc2_f112_df3e_5244),
];

#[cfg(not(target_arch = "aarch64"))]
fn log_amd64(x: f64) -> f64 {
    let [l1, l2, l3, l4, l5, l6, l7] = LOG_L;
    let b = x.to_bits();
    if b & !(1 << 63) == 0 {
        return f64::NEG_INFINITY;
    }
    if (b as i64) < 0 {
        return f64::from_bits(0x7ff8_0000_0000_0001); // also for a negative NaN
    }
    if b >= 0x7ff0_0000_0000_0000 {
        return x; // +Inf, or NaN as it is
    }
    // The assembly's Frexp, which leaves a subnormal x as it is.
    let mut f1 = f64::from_bits((b & 0x000f_ffff_ffff_ffff) | 0x3fe0_0000_0000_0000);
    let mut k = f64::from(((b >> 52) & 0x7ff) as i32 - 0x3fe);
    if f1 <= LOG_HSQRT2 {
        // CMPSD's not-less-than: <=, where log.go has <.
        k -= 1.0;
        f1 *= 2.0;
    }
    let f = f1 - 1.0;
    let s = f / (2.0 + f);
    let s2 = s * s;
    let s4 = s2 * s2;
    let t1 = s2 * (((l7 * s4 + l5) * s4 + l3) * s4 + l1);
    let t2 = s4 * ((l6 * s4 + l4) * s4 + l2);
    let r = t1 + t2;
    let hfsq = 0.5 * f * f;
    k * LOG_LN2HI - ((hfsq - (s * (r + hfsq) + LOG_LN2LO * k)) - f)
}

#[cfg(target_arch = "aarch64")]
fn log_arm64(x: f64) -> f64 {
    let [l1, l2, l3, l4, l5, l6, l7] = LOG_L;
    if x.is_nan() || x == f64::INFINITY {
        return x;
    }
    if x < 0.0 {
        return f64::from_bits(0x7ff8_0000_0000_0001);
    }
    if x == 0.0 {
        return f64::NEG_INFINITY;
    }
    // Frexp (frexp.go), a subnormal x made normal first.
    let (mut b, mut e) = (x.to_bits(), 0);
    if x.abs() < f64::MIN_POSITIVE {
        b = (x * (1u64 << 52) as f64).to_bits();
        e = -52;
    }
    e += ((b >> 52) & 0x7ff) as i64 - 1022;
    let mut f1 = f64::from_bits((b & !(0x7ff << 52)) | (1022 << 52));
    if f1 < LOG_HSQRT2 {
        f1 *= 2.0;
        e -= 1;
    }
    let f = f1 - 1.0;
    let k = e as f64;
    let s = f / (2.0 + f);
    let s2 = s * s;
    let s4 = s2 * s2;
    // gc's fused sites in log.go (fma-sites from -d=fmahash): 124:17, :25,
    // :32; 125:17, :25; 126:10; 128:17, :26, :36, :40.
    let p1 = s4.mul_add(s4.mul_add(s4.mul_add(l7, l5), l3), l1);
    let q = s4.mul_add(s4.mul_add(l6, l4), l2);
    let t2 = s4 * q;
    let r = s2.mul_add(p1, t2); // t1 + t2: t1's multiply fused
    let half = 0.5 * f;
    let a = half.mul_add(f, r); // hfsq+R: hfsq's multiply fused
    let c = s.mul_add(a, k * LOG_LN2LO); // s*(hfsq+R) + k*Ln2Lo: s's fused
    let d = half.mul_add(f, -c); // hfsq - (...): FNMSUB
    let e = d - f;
    k.mul_add(LOG_LN2HI, -e) // k*Ln2Hi - (...): FNMSUB
}
`,
	},
	{
		name: "inf",
		code: `/// Go's math.Inf: positive infinity if sign >= 0, negative infinity
/// otherwise.
pub fn inf(sign: i64) -> f64 {
    if sign >= 0 {
        f64::INFINITY
    } else {
        f64::NEG_INFINITY
    }
}
`,
	},
	{
		name: "is_inf",
		code: `/// Go's math.IsInf: whether f is an infinity, of either sign when sign
/// is 0, positive when sign > 0, negative when sign < 0.
pub fn is_inf(f: f64, sign: i64) -> bool {
    sign >= 0 && f == f64::INFINITY || sign <= 0 && f == f64::NEG_INFINITY
}
`,
	},
	{
		name: "Defers",
		code: `/// The calls a function defers, run last first when it returns or as a
/// panic unwinds through it, as Go runs them.
#[derive(Default)]
pub struct Defers(Vec<String>);

impl Defers {
    /// Defers printing text, made when the defer statement runs, as Go
    /// evaluates a deferred call's arguments.
    pub fn print(&mut self, text: String) {
        self.0.push(text);
    }
}

impl Drop for Defers {
    fn drop(&mut self) {
        while let Some(text) = self.0.pop() {
            print!("{text}");
        }
    }
}
`,
	},
	{
		name: "run_main",
		deps: []string{"exit_on_panic"},
		code: `/// Runs main as Go does when the program defers calls: a panic unwinds,
/// running them, and then its message goes to standard error and the
/// program exits with status 2.
pub fn run_main(main: impl FnOnce()) {
    static MESSAGE: std::sync::Mutex<String> = std::sync::Mutex::new(String::new());
    std::panic::set_hook(Box::new(|info| {
        let payload = info.payload();
        let raw = if let Some(s) = payload.downcast_ref::<&str>() {
            s.to_string()
        } else if let Some(s) = payload.downcast_ref::<String>() {
            s.clone()
        } else {
            String::from("unknown panic")
        };
        if let Ok(mut m) = MESSAGE.lock() {
            *m = go_panic_message(&raw);
        }
    }));
    if std::panic::catch_unwind(std::panic::AssertUnwindSafe(main)).is_err() {
        let _ = std::io::Write::flush(&mut std::io::stdout());
        let msg = MESSAGE.lock().map(|m| m.clone()).unwrap_or_default();
        eprintln!("panic: {msg}");
        std::process::exit(2);
    }
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
		name: "show_slice",
		code: `/// %v of a slice inside another value (a struct's field, an interface's
/// dynamic value): its elements' %v, in brackets.
pub fn show_slice<T>(v: &[T], show: impl Fn(&T) -> String) -> String {
    let parts: Vec<String> = v.iter().map(show).collect();
    format!("[{}]", parts.join(" "))
}
`,
	},
	{
		name: "quote",
		code: `/// Go's strconv.Quote of s, which fmt's %q prints: s in double quotes, with
/// " and \ escaped, \a \b \f \n \r \t \v, and every rune strconv.IsPrint
/// rejects as \xNN (below a space, and DEL), \uNNNN or \UNNNNNNNN. A patina
/// string is valid UTF-8, so no byte is escaped on its own.
pub fn quote(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' | '\\' => {
                out.push('\\');
                out.push(c);
            }
            _ if is_print(c as u32) => out.push(c),
            '\u{7}' => out.push_str("\\a"),
            '\u{8}' => out.push_str("\\b"),
            '\u{c}' => out.push_str("\\f"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '\u{b}' => out.push_str("\\v"),
            _ if c < ' ' || c == '\u{7f}' => out.push_str(&format!("\\x{:02x}", c as u32)),
            _ if (c as u32) < 0x10000 => out.push_str(&format!("\\u{:04x}", c as u32)),
            _ => out.push_str(&format!("\\U{:08x}", c as u32)),
        }
    }
    out.push('"');
    out
}

/// Go's strconv.IsPrint: a letter, mark, number, punctuation, symbol or the
/// ASCII space, by the Unicode tables of the Go release the crate was made
/// with (strconv/isprint.go).
fn is_print(r: u32) -> bool {
    if r <= 0xff {
        if (0x20..=0x7e).contains(&r) {
            return true;
        }
        if (0xa1..=0xff).contains(&r) {
            return r != 0xad;
        }
        return false;
    }
    if r < 0x10000 {
        let rr = r as u16;
        let i = IS_PRINT16.partition_point(|&x| x < rr);
        if i >= IS_PRINT16.len() || rr < IS_PRINT16[i & !1] || IS_PRINT16[i | 1] < rr {
            return false;
        }
        let j = IS_NOT_PRINT16.partition_point(|&x| x < rr);
        return j >= IS_NOT_PRINT16.len() || IS_NOT_PRINT16[j] != rr;
    }
    let i = IS_PRINT32.partition_point(|&x| x < r);
    if i >= IS_PRINT32.len() || r < IS_PRINT32[i & !1] || IS_PRINT32[i | 1] < r {
        return false;
    }
    if r >= 0x20000 {
        return true;
    }
    let rr = (r - 0x10000) as u16;
    let j = IS_NOT_PRINT32.partition_point(|&x| x < rr);
    j >= IS_NOT_PRINT32.len() || IS_NOT_PRINT32[j] != rr
}
`,
		deps: []string{"isprint"},
	},
	{
		name: "isprint",
		code: `// strconv/isprint.go's tables, of the Go release the crate was made with:
// ranges of printable runes, and the unprintable ones inside them.
const IS_PRINT16: [u16; 412] = [
    0x0020, 0x007e, 0x00a1, 0x0377, 0x037a, 0x037f, 0x0384, 0x0556, 0x0559, 0x058a, 0x058d, 0x05c7,
    0x05d0, 0x05ea, 0x05ef, 0x05f4, 0x0606, 0x070d, 0x0710, 0x074a, 0x074d, 0x07b1, 0x07c0, 0x07fa,
    0x07fd, 0x082d, 0x0830, 0x085b, 0x085e, 0x086a, 0x0870, 0x088f, 0x0897, 0x098c, 0x098f, 0x0990,
    0x0993, 0x09b2, 0x09b6, 0x09b9, 0x09bc, 0x09c4, 0x09c7, 0x09c8, 0x09cb, 0x09ce, 0x09d7, 0x09d7,
    0x09dc, 0x09e3, 0x09e6, 0x09fe, 0x0a01, 0x0a0a, 0x0a0f, 0x0a10, 0x0a13, 0x0a39, 0x0a3c, 0x0a42,
    0x0a47, 0x0a48, 0x0a4b, 0x0a4d, 0x0a51, 0x0a51, 0x0a59, 0x0a5e, 0x0a66, 0x0a76, 0x0a81, 0x0ab9,
    0x0abc, 0x0acd, 0x0ad0, 0x0ad0, 0x0ae0, 0x0ae3, 0x0ae6, 0x0af1, 0x0af9, 0x0b0c, 0x0b0f, 0x0b10,
    0x0b13, 0x0b39, 0x0b3c, 0x0b44, 0x0b47, 0x0b48, 0x0b4b, 0x0b4d, 0x0b55, 0x0b57, 0x0b5c, 0x0b63,
    0x0b66, 0x0b77, 0x0b82, 0x0b8a, 0x0b8e, 0x0b95, 0x0b99, 0x0b9f, 0x0ba3, 0x0ba4, 0x0ba8, 0x0baa,
    0x0bae, 0x0bb9, 0x0bbe, 0x0bc2, 0x0bc6, 0x0bcd, 0x0bd0, 0x0bd0, 0x0bd7, 0x0bd7, 0x0be6, 0x0bfa,
    0x0c00, 0x0c39, 0x0c3c, 0x0c4d, 0x0c55, 0x0c5d, 0x0c60, 0x0c63, 0x0c66, 0x0c6f, 0x0c77, 0x0cb9,
    0x0cbc, 0x0ccd, 0x0cd5, 0x0cd6, 0x0cdc, 0x0ce3, 0x0ce6, 0x0cf3, 0x0d00, 0x0d4f, 0x0d54, 0x0d63,
    0x0d66, 0x0d96, 0x0d9a, 0x0dbd, 0x0dc0, 0x0dc6, 0x0dca, 0x0dca, 0x0dcf, 0x0ddf, 0x0de6, 0x0def,
    0x0df2, 0x0df4, 0x0e01, 0x0e3a, 0x0e3f, 0x0e5b, 0x0e81, 0x0ebd, 0x0ec0, 0x0ed9, 0x0edc, 0x0edf,
    0x0f00, 0x0f6c, 0x0f71, 0x0fda, 0x1000, 0x10c7, 0x10cd, 0x10cd, 0x10d0, 0x124d, 0x1250, 0x125d,
    0x1260, 0x128d, 0x1290, 0x12b5, 0x12b8, 0x12c5, 0x12c8, 0x1315, 0x1318, 0x135a, 0x135d, 0x137c,
    0x1380, 0x1399, 0x13a0, 0x13f5, 0x13f8, 0x13fd, 0x1400, 0x169c, 0x16a0, 0x16f8, 0x1700, 0x1715,
    0x171f, 0x1736, 0x1740, 0x1753, 0x1760, 0x1773, 0x1780, 0x17dd, 0x17e0, 0x17e9, 0x17f0, 0x17f9,
    0x1800, 0x1819, 0x1820, 0x1878, 0x1880, 0x18aa, 0x18b0, 0x18f5, 0x1900, 0x192b, 0x1930, 0x193b,
    0x1940, 0x1940, 0x1944, 0x196d, 0x1970, 0x1974, 0x1980, 0x19ab, 0x19b0, 0x19c9, 0x19d0, 0x19da,
    0x19de, 0x1a1b, 0x1a1e, 0x1a7c, 0x1a7f, 0x1a89, 0x1a90, 0x1a99, 0x1aa0, 0x1aad, 0x1ab0, 0x1add,
    0x1ae0, 0x1aeb, 0x1b00, 0x1bf3, 0x1bfc, 0x1c37, 0x1c3b, 0x1c49, 0x1c4d, 0x1c8a, 0x1c90, 0x1cba,
    0x1cbd, 0x1cc7, 0x1cd0, 0x1cfa, 0x1d00, 0x1f15, 0x1f18, 0x1f1d, 0x1f20, 0x1f45, 0x1f48, 0x1f4d,
    0x1f50, 0x1f7d, 0x1f80, 0x1fd3, 0x1fd6, 0x1fef, 0x1ff2, 0x1ffe, 0x2010, 0x2027, 0x2030, 0x205e,
    0x2070, 0x2071, 0x2074, 0x209c, 0x20a0, 0x20c1, 0x20d0, 0x20f0, 0x2100, 0x218b, 0x2190, 0x2429,
    0x2440, 0x244a, 0x2460, 0x2b73, 0x2b76, 0x2cf3, 0x2cf9, 0x2d27, 0x2d2d, 0x2d2d, 0x2d30, 0x2d67,
    0x2d6f, 0x2d70, 0x2d7f, 0x2d96, 0x2da0, 0x2e5d, 0x2e80, 0x2ef3, 0x2f00, 0x2fd5, 0x2ff0, 0x3096,
    0x3099, 0x30ff, 0x3105, 0x31e5, 0x31ef, 0xa48c, 0xa490, 0xa4c6, 0xa4d0, 0xa62b, 0xa640, 0xa6f7,
    0xa700, 0xa7dc, 0xa7f1, 0xa82c, 0xa830, 0xa839, 0xa840, 0xa877, 0xa880, 0xa8c5, 0xa8ce, 0xa8d9,
    0xa8e0, 0xa953, 0xa95f, 0xa97c, 0xa980, 0xa9d9, 0xa9de, 0xaa36, 0xaa40, 0xaa4d, 0xaa50, 0xaa59,
    0xaa5c, 0xaac2, 0xaadb, 0xaaf6, 0xab01, 0xab06, 0xab09, 0xab0e, 0xab11, 0xab16, 0xab20, 0xab6b,
    0xab70, 0xabed, 0xabf0, 0xabf9, 0xac00, 0xd7a3, 0xd7b0, 0xd7c6, 0xd7cb, 0xd7fb, 0xf900, 0xfa6d,
    0xfa70, 0xfad9, 0xfb00, 0xfb06, 0xfb13, 0xfb17, 0xfb1d, 0xfdcf, 0xfdf0, 0xfe19, 0xfe20, 0xfe6b,
    0xfe70, 0xfefc, 0xff01, 0xffbe, 0xffc2, 0xffc7, 0xffca, 0xffcf, 0xffd2, 0xffd7, 0xffda, 0xffdc,
    0xffe0, 0xffee, 0xfffc, 0xfffd,
];
const IS_NOT_PRINT16: [u16; 132] = [
    0x00ad, 0x038b, 0x038d, 0x03a2, 0x0530, 0x0590, 0x061c, 0x06dd, 0x083f, 0x085f, 0x08e2, 0x0984,
    0x09a9, 0x09b1, 0x09de, 0x0a04, 0x0a29, 0x0a31, 0x0a34, 0x0a37, 0x0a3d, 0x0a5d, 0x0a84, 0x0a8e,
    0x0a92, 0x0aa9, 0x0ab1, 0x0ab4, 0x0ac6, 0x0aca, 0x0b00, 0x0b04, 0x0b29, 0x0b31, 0x0b34, 0x0b5e,
    0x0b84, 0x0b91, 0x0b9b, 0x0b9d, 0x0bc9, 0x0c0d, 0x0c11, 0x0c29, 0x0c45, 0x0c49, 0x0c57, 0x0c5b,
    0x0c8d, 0x0c91, 0x0ca9, 0x0cb4, 0x0cc5, 0x0cc9, 0x0cdf, 0x0cf0, 0x0d0d, 0x0d11, 0x0d45, 0x0d49,
    0x0d80, 0x0d84, 0x0db2, 0x0dbc, 0x0dd5, 0x0dd7, 0x0e83, 0x0e85, 0x0e8b, 0x0ea4, 0x0ea6, 0x0ec5,
    0x0ec7, 0x0ecf, 0x0f48, 0x0f98, 0x0fbd, 0x0fcd, 0x10c6, 0x1249, 0x1257, 0x1259, 0x1289, 0x12b1,
    0x12bf, 0x12c1, 0x12d7, 0x1311, 0x1680, 0x176d, 0x1771, 0x180e, 0x191f, 0x1a5f, 0x1b4d, 0x1f58,
    0x1f5a, 0x1f5c, 0x1f5e, 0x1fb5, 0x1fc5, 0x1fdc, 0x1ff5, 0x208f, 0x2d26, 0x2da7, 0x2daf, 0x2db7,
    0x2dbf, 0x2dc7, 0x2dcf, 0x2dd7, 0x2ddf, 0x2e9a, 0x3000, 0x3040, 0x3130, 0x318f, 0x321f, 0xa9ce,
    0xa9ff, 0xab27, 0xab2f, 0xfb37, 0xfb3d, 0xfb3f, 0xfb42, 0xfb45, 0xfe53, 0xfe67, 0xfe75, 0xffe7,
];
const IS_PRINT32: [u32; 566] = [
    0x10000, 0x1004d, 0x10050, 0x1005d, 0x10080, 0x100fa, 0x10100, 0x10102, 0x10107, 0x10133,
    0x10137, 0x1019c, 0x101a0, 0x101a0, 0x101d0, 0x101fd, 0x10280, 0x1029c, 0x102a0, 0x102d0,
    0x102e0, 0x102fb, 0x10300, 0x10323, 0x1032d, 0x1034a, 0x10350, 0x1037a, 0x10380, 0x103c3,
    0x103c8, 0x103d5, 0x10400, 0x1049d, 0x104a0, 0x104a9, 0x104b0, 0x104d3, 0x104d8, 0x104fb,
    0x10500, 0x10527, 0x10530, 0x10563, 0x1056f, 0x105bc, 0x105c0, 0x105f3, 0x10600, 0x10736,
    0x10740, 0x10755, 0x10760, 0x10767, 0x10780, 0x107ba, 0x10800, 0x10805, 0x10808, 0x10838,
    0x1083c, 0x1083c, 0x1083f, 0x1089e, 0x108a7, 0x108af, 0x108e0, 0x108f5, 0x108fb, 0x1091b,
    0x1091f, 0x10939, 0x1093f, 0x10959, 0x10980, 0x109b7, 0x109bc, 0x109cf, 0x109d2, 0x10a06,
    0x10a0c, 0x10a35, 0x10a38, 0x10a3a, 0x10a3f, 0x10a48, 0x10a50, 0x10a58, 0x10a60, 0x10a9f,
    0x10ac0, 0x10ae6, 0x10aeb, 0x10af6, 0x10b00, 0x10b35, 0x10b39, 0x10b55, 0x10b58, 0x10b72,
    0x10b78, 0x10b91, 0x10b99, 0x10b9c, 0x10ba9, 0x10baf, 0x10c00, 0x10c48, 0x10c80, 0x10cb2,
    0x10cc0, 0x10cf2, 0x10cfa, 0x10d27, 0x10d30, 0x10d39, 0x10d40, 0x10d65, 0x10d69, 0x10d85,
    0x10d8e, 0x10d8f, 0x10e60, 0x10ead, 0x10eb0, 0x10eb1, 0x10ec2, 0x10ec7, 0x10ed0, 0x10ed8,
    0x10efa, 0x10f27, 0x10f30, 0x10f59, 0x10f70, 0x10f89, 0x10fb0, 0x10fcb, 0x10fe0, 0x10ff6,
    0x11000, 0x1104d, 0x11052, 0x11075, 0x1107f, 0x110c2, 0x110d0, 0x110e8, 0x110f0, 0x110f9,
    0x11100, 0x11147, 0x11150, 0x11176, 0x11180, 0x111f4, 0x11200, 0x11241, 0x11280, 0x112a9,
    0x112b0, 0x112ea, 0x112f0, 0x112f9, 0x11300, 0x1130c, 0x1130f, 0x11310, 0x11313, 0x11344,
    0x11347, 0x11348, 0x1134b, 0x1134d, 0x11350, 0x11350, 0x11357, 0x11357, 0x1135d, 0x11363,
    0x11366, 0x1136c, 0x11370, 0x11374, 0x11380, 0x1138b, 0x1138e, 0x113c2, 0x113c5, 0x113d8,
    0x113e1, 0x113e2, 0x11400, 0x11461, 0x11480, 0x114c7, 0x114d0, 0x114d9, 0x11580, 0x115b5,
    0x115b8, 0x115dd, 0x11600, 0x11644, 0x11650, 0x11659, 0x11660, 0x1166c, 0x11680, 0x116b9,
    0x116c0, 0x116c9, 0x116d0, 0x116e3, 0x11700, 0x1171a, 0x1171d, 0x1172b, 0x11730, 0x11746,
    0x11800, 0x1183b, 0x118a0, 0x118f2, 0x118ff, 0x11906, 0x11909, 0x11909, 0x1190c, 0x11938,
    0x1193b, 0x11946, 0x11950, 0x11959, 0x119a0, 0x119a7, 0x119aa, 0x119d7, 0x119da, 0x119e4,
    0x11a00, 0x11a47, 0x11a50, 0x11aa2, 0x11ab0, 0x11af8, 0x11b00, 0x11b09, 0x11b60, 0x11b67,
    0x11bc0, 0x11be1, 0x11bf0, 0x11bf9, 0x11c00, 0x11c45, 0x11c50, 0x11c6c, 0x11c70, 0x11c8f,
    0x11c92, 0x11cb6, 0x11d00, 0x11d36, 0x11d3a, 0x11d47, 0x11d50, 0x11d59, 0x11d60, 0x11d98,
    0x11da0, 0x11da9, 0x11db0, 0x11ddb, 0x11de0, 0x11de9, 0x11ee0, 0x11ef8, 0x11f00, 0x11f3a,
    0x11f3e, 0x11f5a, 0x11fb0, 0x11fb0, 0x11fc0, 0x11ff1, 0x11fff, 0x12399, 0x12400, 0x12474,
    0x12480, 0x12543, 0x12f90, 0x12ff2, 0x13000, 0x1342f, 0x13440, 0x13455, 0x13460, 0x143fa,
    0x14400, 0x14646, 0x16100, 0x16139, 0x16800, 0x16a38, 0x16a40, 0x16a69, 0x16a6e, 0x16ac9,
    0x16ad0, 0x16aed, 0x16af0, 0x16af5, 0x16b00, 0x16b45, 0x16b50, 0x16b77, 0x16b7d, 0x16b8f,
    0x16d40, 0x16d79, 0x16e40, 0x16e9a, 0x16ea0, 0x16eb8, 0x16ebb, 0x16ed3, 0x16f00, 0x16f4a,
    0x16f4f, 0x16f87, 0x16f8f, 0x16f9f, 0x16fe0, 0x16fe4, 0x16ff0, 0x16ff6, 0x17000, 0x18cd5,
    0x18cff, 0x18d1e, 0x18d80, 0x18df2, 0x1aff0, 0x1b122, 0x1b132, 0x1b132, 0x1b150, 0x1b152,
    0x1b155, 0x1b155, 0x1b164, 0x1b167, 0x1b170, 0x1b2fb, 0x1bc00, 0x1bc6a, 0x1bc70, 0x1bc7c,
    0x1bc80, 0x1bc88, 0x1bc90, 0x1bc99, 0x1bc9c, 0x1bc9f, 0x1cc00, 0x1ccfc, 0x1cd00, 0x1ceb3,
    0x1ceba, 0x1ced0, 0x1cee0, 0x1cef0, 0x1cf00, 0x1cf2d, 0x1cf30, 0x1cf46, 0x1cf50, 0x1cfc3,
    0x1d000, 0x1d0f5, 0x1d100, 0x1d126, 0x1d129, 0x1d172, 0x1d17b, 0x1d1ea, 0x1d200, 0x1d245,
    0x1d2c0, 0x1d2d3, 0x1d2e0, 0x1d2f3, 0x1d300, 0x1d356, 0x1d360, 0x1d378, 0x1d400, 0x1d49f,
    0x1d4a2, 0x1d4a2, 0x1d4a5, 0x1d4a6, 0x1d4a9, 0x1d50a, 0x1d50d, 0x1d546, 0x1d54a, 0x1d6a5,
    0x1d6a8, 0x1d7cb, 0x1d7ce, 0x1da8b, 0x1da9b, 0x1daaf, 0x1df00, 0x1df1e, 0x1df25, 0x1df2a,
    0x1e000, 0x1e018, 0x1e01b, 0x1e02a, 0x1e030, 0x1e06d, 0x1e08f, 0x1e08f, 0x1e100, 0x1e12c,
    0x1e130, 0x1e13d, 0x1e140, 0x1e149, 0x1e14e, 0x1e14f, 0x1e290, 0x1e2ae, 0x1e2c0, 0x1e2f9,
    0x1e2ff, 0x1e2ff, 0x1e4d0, 0x1e4f9, 0x1e5d0, 0x1e5fa, 0x1e5ff, 0x1e5ff, 0x1e6c0, 0x1e6f5,
    0x1e6fe, 0x1e6ff, 0x1e7e0, 0x1e8c4, 0x1e8c7, 0x1e8d6, 0x1e900, 0x1e94b, 0x1e950, 0x1e959,
    0x1e95e, 0x1e95f, 0x1ec71, 0x1ecb4, 0x1ed01, 0x1ed3d, 0x1ee00, 0x1ee24, 0x1ee27, 0x1ee3b,
    0x1ee42, 0x1ee42, 0x1ee47, 0x1ee54, 0x1ee57, 0x1ee64, 0x1ee67, 0x1ee9b, 0x1eea1, 0x1eebb,
    0x1eef0, 0x1eef1, 0x1f000, 0x1f02b, 0x1f030, 0x1f093, 0x1f0a0, 0x1f0ae, 0x1f0b1, 0x1f0f5,
    0x1f100, 0x1f1ad, 0x1f1e6, 0x1f202, 0x1f210, 0x1f23b, 0x1f240, 0x1f248, 0x1f250, 0x1f251,
    0x1f260, 0x1f265, 0x1f300, 0x1f6d8, 0x1f6dc, 0x1f6ec, 0x1f6f0, 0x1f6fc, 0x1f700, 0x1f7d9,
    0x1f7e0, 0x1f7eb, 0x1f7f0, 0x1f7f0, 0x1f800, 0x1f80b, 0x1f810, 0x1f847, 0x1f850, 0x1f859,
    0x1f860, 0x1f887, 0x1f890, 0x1f8ad, 0x1f8b0, 0x1f8bb, 0x1f8c0, 0x1f8c1, 0x1f8d0, 0x1f8d8,
    0x1f900, 0x1fa57, 0x1fa60, 0x1fa6d, 0x1fa70, 0x1fa7c, 0x1fa80, 0x1fa8a, 0x1fa8e, 0x1fac8,
    0x1facd, 0x1fadc, 0x1fadf, 0x1faea, 0x1faef, 0x1faf8, 0x1fb00, 0x1fbfa, 0x20000, 0x2a6df,
    0x2a700, 0x2b81d, 0x2b820, 0x2cead, 0x2ceb0, 0x2ebe0, 0x2ebf0, 0x2ee5d, 0x2f800, 0x2fa1d,
    0x30000, 0x3134a, 0x31350, 0x33479, 0xe0100, 0xe01ef,
];
const IS_NOT_PRINT32: [u16; 120] = [
    0x000c, 0x0027, 0x003b, 0x003e, 0x018f, 0x039e, 0x057b, 0x058b, 0x0593, 0x0596, 0x05a2, 0x05b2,
    0x05ba, 0x0786, 0x07b1, 0x0809, 0x0836, 0x0856, 0x08f3, 0x0a04, 0x0a14, 0x0a18, 0x0e7f, 0x0eaa,
    0x10bd, 0x1135, 0x11e0, 0x1212, 0x1287, 0x1289, 0x128e, 0x129e, 0x1304, 0x1329, 0x1331, 0x1334,
    0x133a, 0x138a, 0x138f, 0x13b6, 0x13c1, 0x13c6, 0x13cb, 0x13d6, 0x145c, 0x1914, 0x1917, 0x1936,
    0x1c09, 0x1c37, 0x1ca8, 0x1d07, 0x1d0a, 0x1d3b, 0x1d3e, 0x1d66, 0x1d69, 0x1d8f, 0x1d92, 0x1f11,
    0x246f, 0x6a5f, 0x6abf, 0x6b5a, 0x6b62, 0xaff4, 0xaffc, 0xafff, 0xd455, 0xd49d, 0xd4ad, 0xd4ba,
    0xd4bc, 0xd4c4, 0xd506, 0xd515, 0xd51d, 0xd53a, 0xd53f, 0xd545, 0xd551, 0xdaa0, 0xe007, 0xe022,
    0xe025, 0xe6df, 0xe7e7, 0xe7ec, 0xe7ef, 0xe7ff, 0xee04, 0xee20, 0xee23, 0xee28, 0xee33, 0xee38,
    0xee3a, 0xee48, 0xee4a, 0xee4c, 0xee50, 0xee53, 0xee58, 0xee5a, 0xee5c, 0xee5e, 0xee60, 0xee63,
    0xee6b, 0xee73, 0xee78, 0xee7d, 0xee7f, 0xee8a, 0xeea4, 0xeeaa, 0xf0c0, 0xf0d0, 0xfac7, 0xfb93,
];
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
		uses: []string{"fmt", "sync::atomic::AtomicU64", "sync::atomic::Ordering"},
		code: `/// A Go error value made by errors.New or fmt.Errorf. Functions that
/// return (T, error) in Go return Result<T, Error> here. Each one is a new
/// pointer in Go, equal only to itself and its copies; id stands for that
/// identity. Errors compare by that identity (id), never by message.
#[derive(Clone, Debug)]
pub struct Error {
    message: String,
    id: u64,
}

impl Error {
    pub fn new(message: impl Into<String>) -> Error {
        Error {
            message: message.into(),
            id: error_id(),
        }
    }

    pub fn id(&self) -> u64 {
        self.id
    }
}

/// A new error identity.
pub fn error_id() -> u64 {
    static NEXT: AtomicU64 = AtomicU64::new(0);
    NEXT.fetch_add(1, Ordering::Relaxed)
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.pad(&self.message)
    }
}
`,
	},
	{
		name: "show_ref",
		code: `/// %v of an owned pointer, or a borrow of one, through a String or Error
/// method that takes &self: <nil> for a nil pointer, as Go's fmt prints
/// for a method that dereferences it.
pub fn show_ref<P: std::ops::Deref<Target = T>, T: ?Sized>(
    p: &Option<P>,
    show: impl FnOnce(&T) -> String,
) -> String {
    match p {
        Some(v) => show(v),
        None => String::from("<nil>"),
    }
}
`,
	},
	{
		name: "show_opt",
		code: `/// %v of an owned pointer, or a borrow of one, through its String or
/// Error method. As Go's fmt does, a method that panics for a nil pointer
/// prints <nil>.
pub fn show_opt<P>(p: &Option<P>, show: impl FnOnce(&Option<P>) -> String) -> String {
    if p.is_some() {
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
