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

/// Whether Go's math uses FMA, as internal/cpu decides at start-up: the
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

/// Go's math.Exp, bit for bit: on amd64 its assembly (exp_amd64.s,
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
    let fma = fma_usable();
    let mut r;
    let mut p;
    if fma {
        r = (-kf).mul_add(LN2U, x);
        r = (-kf).mul_add(LN2L, r);
        r *= 0.0625;
        p = C[0];
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
        r = r.mul_add(t, 1.0);
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
