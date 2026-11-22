package emit

// The runtime items of time.Duration's methods (RFC 0011), each
// transcribed from Go's time package; integer overflow wraps, as in Go.
func init() {
	rtItems = append(rtItems,
		rtItem{
			name: "duration_string",
			code: `/// Go's time.Duration.String: 1h2m3.5s, 1.5ms, 2µs, 0s.
pub fn duration_string(d: i64) -> String {
    let mut buf = [0u8; 32];
    let mut w = buf.len();
    let neg = d < 0;
    let mut u = d.unsigned_abs();
    if u < 1_000_000_000 {
        let prec;
        w -= 1;
        buf[w] = b's';
        w -= 1;
        if u == 0 {
            buf[w] = b'0';
            return String::from("0s");
        } else if u < 1_000 {
            prec = 0;
            buf[w] = b'n';
        } else if u < 1_000_000 {
            prec = 3;
            w -= 1;
            buf[w..w + 2].copy_from_slice("µ".as_bytes());
        } else {
            prec = 6;
            buf[w] = b'm';
        }
        (w, u) = duration_frac(&mut buf, w, u, prec);
        w = duration_int(&mut buf, w, u);
    } else {
        w -= 1;
        buf[w] = b's';
        (w, u) = duration_frac(&mut buf, w, u, 9);
        w = duration_int(&mut buf, w, u % 60);
        u /= 60;
        if u > 0 {
            w -= 1;
            buf[w] = b'm';
            w = duration_int(&mut buf, w, u % 60);
            u /= 60;
            if u > 0 {
                w -= 1;
                buf[w] = b'h';
                w = duration_int(&mut buf, w, u);
            }
        }
    }
    if neg {
        w -= 1;
        buf[w] = b'-';
    }
    String::from_utf8_lossy(&buf[w..]).into_owned()
}

/// Go's time.fmtFrac: the fraction of v/10^prec, without trailing zeros,
/// written before buf[w]; returns the new start and v/10^prec.
fn duration_frac(buf: &mut [u8], mut w: usize, mut v: u64, prec: usize) -> (usize, u64) {
    let mut print = false;
    for _ in 0..prec {
        let digit = v % 10;
        print = print || digit != 0;
        if print {
            w -= 1;
            buf[w] = digit as u8 + b'0';
        }
        v /= 10;
    }
    if print {
        w -= 1;
        buf[w] = b'.';
    }
    (w, v)
}

/// Go's time.fmtInt: v in decimal, written before buf[w].
fn duration_int(buf: &mut [u8], mut w: usize, mut v: u64) -> usize {
    if v == 0 {
        w -= 1;
        buf[w] = b'0';
    }
    while v > 0 {
        w -= 1;
        buf[w] = (v % 10) as u8 + b'0';
        v /= 10;
    }
    w
}
`,
		},
		rtItem{
			name: "duration_nanoseconds",
			code: `/// Go's time.Duration.Nanoseconds.
pub fn duration_nanoseconds(d: i64) -> i64 {
    d
}
`,
		},
		rtItem{
			name: "duration_microseconds",
			code: `/// Go's time.Duration.Microseconds.
pub fn duration_microseconds(d: i64) -> i64 {
    d / 1_000
}
`,
		},
		rtItem{
			name: "duration_milliseconds",
			code: `/// Go's time.Duration.Milliseconds.
pub fn duration_milliseconds(d: i64) -> i64 {
    d / 1_000_000
}
`,
		},
		rtItem{
			name: "duration_seconds",
			code: `/// Go's time.Duration.Seconds.
pub fn duration_seconds(d: i64) -> f64 {
    (d / 1_000_000_000) as f64 + (d % 1_000_000_000) as f64 / 1e9
}
`,
		},
		rtItem{
			name: "duration_minutes",
			code: `/// Go's time.Duration.Minutes.
pub fn duration_minutes(d: i64) -> f64 {
    (d / 60_000_000_000) as f64 + (d % 60_000_000_000) as f64 / 6e10
}
`,
		},
		rtItem{
			name: "duration_hours",
			code: `/// Go's time.Duration.Hours.
pub fn duration_hours(d: i64) -> f64 {
    (d / 3_600_000_000_000) as f64 + (d % 3_600_000_000_000) as f64 / 3.6e12
}
`,
		},
		rtItem{
			name: "duration_truncate",
			code: `/// Go's time.Duration.Truncate.
pub fn duration_truncate(d: i64, m: i64) -> i64 {
    if m <= 0 {
        return d;
    }
    d.wrapping_sub(d % m)
}
`,
		},
		rtItem{
			name: "duration_round",
			code: `/// Go's time.Duration.Round: halfway values round away from zero, and
/// a result past the range saturates.
pub fn duration_round(d: i64, m: i64) -> i64 {
    if m <= 0 {
        return d;
    }
    let less_than_half = |x: i64| (x as u64).wrapping_add(x as u64) < m as u64;
    let mut r = d % m;
    if d < 0 {
        r = r.wrapping_neg();
        if less_than_half(r) {
            return d.wrapping_add(r);
        }
        let d1 = d.wrapping_sub(m).wrapping_add(r);
        if d1 < d {
            return d1;
        }
        return i64::MIN;
    }
    if less_than_half(r) {
        return d - r;
    }
    let d1 = d.wrapping_add(m).wrapping_sub(r);
    if d1 > d {
        return d1;
    }
    i64::MAX
}
`,
		},
		rtItem{
			name: "duration_abs",
			code: `/// Go's time.Duration.Abs: the largest Duration for the smallest.
pub fn duration_abs(d: i64) -> i64 {
    if d >= 0 {
        d
    } else if d == i64::MIN {
        i64::MAX
    } else {
        -d
    }
}
`,
		},
	)
}
