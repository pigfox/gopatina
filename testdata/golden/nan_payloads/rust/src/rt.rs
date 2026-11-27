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

/// a + b as gc computes it, NaN included (nan).
pub fn add_nan(a: f64, b: f64) -> f64 {
    nan(a + b, &[a, b])
}

/// a - b as gc computes it, NaN included (nan).
pub fn sub_nan(a: f64, b: f64) -> f64 {
    nan(a - b, &[a, b])
}

/// a * b as gc computes it, NaN included (nan).
pub fn mul_nan(a: f64, b: f64) -> f64 {
    nan(a * b, &[a, b])
}

/// a / b as gc computes it, NaN included (nan).
pub fn div_nan(a: f64, b: f64) -> f64 {
    nan(a / b, &[a, b])
}

/// math.Sqrt as gc computes it, NaN included (nan).
pub fn sqrt_nan(x: f64) -> f64 {
    nan(x.sqrt(), &[x])
}

/// float32(x) as gc computes it on amd64 and arm64 (CVTSD2SS, FCVT): a
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

/// float64(x) of a float32 as gc computes it (CVTSS2SD, FCVT): a NaN keeps
/// its sign and payload, quiet.
pub fn to_f64(x: f32) -> f64 {
    if x.is_nan() {
        let b = u64::from(x.to_bits());
        let payload = (b & 0x003f_ffff) << 29;
        return f64::from_bits(((b >> 31) << 63) | 0x7ff8_0000_0000_0000 | payload);
    }
    f64::from(x)
}
