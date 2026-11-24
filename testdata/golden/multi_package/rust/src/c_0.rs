//! Package example.com/multi/c, translated by GoPatina from its Go source.

pub(crate) fn init1() {
    println!("init c");
}

/// New is the third New; String is a Rust name too.
pub(crate) fn new() -> i64 {
    string()
}

pub(crate) fn string() -> i64 {
    42
}

/// trace prints as a package-level variable initializes.
pub(crate) fn trace(s: &str) -> i64 {
    println!("init var {}", s);
    s.len() as i64
}

pub(crate) static REGISTERED: std::sync::LazyLock<i64> =
    std::sync::LazyLock::new(|| trace("registered"));

pub(crate) fn init_vars() {
    std::sync::LazyLock::force(&REGISTERED);
}
