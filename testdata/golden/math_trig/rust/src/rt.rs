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

/// a * b as gc computes it, NaN included (nan).
pub fn mul_nan(a: f64, b: f64) -> f64 {
    nan(a * b, &[a, b])
}

/// a / b as gc computes it, NaN included (nan).
pub fn div_nan(a: f64, b: f64) -> f64 {
    nan(a / b, &[a, b])
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
