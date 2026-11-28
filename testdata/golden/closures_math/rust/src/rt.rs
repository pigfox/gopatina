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

/// Go's math.Floor: the greatest integer value at most x. A NaN comes back
/// quiet, as from Go's ROUNDSD on amd64 and FRINT on arm64.
pub fn floor(x: f64) -> f64 {
    if x.is_nan() { quiet(x) } else { x.floor() }
}

/// Go's math.Ceil: the least integer value at least x. A NaN comes back
/// quiet, as from Go's ROUNDSD on amd64 and FRINT on arm64.
pub fn ceil(x: f64) -> f64 {
    if x.is_nan() { quiet(x) } else { x.ceil() }
}

/// Go's math.Trunc: the integer value of x, rounded toward zero. A NaN comes back
/// quiet, as from Go's ROUNDSD on amd64 and FRINT on arm64.
pub fn trunc(x: f64) -> f64 {
    if x.is_nan() { quiet(x) } else { x.trunc() }
}

/// The NaN x with its quiet bit set, its sign and payload kept.
fn quiet(x: f64) -> f64 {
    f64::from_bits(x.to_bits() | 0x0008_0000_0000_0000)
}

/// The result of gc's float operation (ADDSD, SUBSD, MULSD, DIVSD, SQRTSD
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

/// math.Sqrt as gc computes it, NaN included (nan).
pub fn sqrt_nan(x: f64) -> f64 {
    nan(x.sqrt(), &[x])
}

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

// sin.go's and trig_reduce.go's constants, bit for bit.
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

/// Which of sin.go's multiply-adds gc fuses (rt::trig): on arm64 all of
/// them; on amd64 none, below GOAMD64=v3.
#[cfg(target_arch = "aarch64")]
const TRIG_FUSE_REDUCE: bool = true;
#[cfg(not(target_arch = "aarch64"))]
const TRIG_FUSE_REDUCE: bool = false;
const TRIG_FUSE_POLY: bool = TRIG_FUSE_REDUCE;
const TRIG_FUSE_OUTER: bool = TRIG_FUSE_REDUCE;

/// Go's math.Pow, bit for bit (pow.go, which gc compiles on every target):
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

/// Go's math.Log, bit for bit: on amd64 its assembly (log_amd64.s), on
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
