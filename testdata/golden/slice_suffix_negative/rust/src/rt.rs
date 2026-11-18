//! Runtime support for this crate, written by GoPatina: the few pieces
//! of Go behaviour that have no direct Rust spelling.

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
        let _ = std::io::Write::flush(&mut std::io::stdout());
        eprintln!("panic: {msg}");
        std::process::exit(2);
    }));
}

/// Borrows s[low:] after checking Go's signed low bound against len(s).
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
