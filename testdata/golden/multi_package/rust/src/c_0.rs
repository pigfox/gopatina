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
