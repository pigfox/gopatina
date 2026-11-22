//! Package example.com/multi/b, translated by GoPatina from its Go source.

use crate::rt;

pub(crate) fn init4() {
    println!("init b");
}

/// Kind is a named type with a String method, used from main.
pub(crate) type Kind = i64;

const BIG: i64 = 7;

pub(crate) fn kind_string(k: i64) -> String {
    if k > 5 {
        return String::from("big");
    }
    String::from("small")
}

/// Scale multiplies a point of package a.
pub(crate) fn scale(p: crate::a_2::Point, k: i64) -> crate::a_2::Point {
    crate::a_2::Point {
        x: p.x * k,
        y: p.y * k,
    }
}

/// New has the same name as a.New.
pub(crate) fn new() -> String {
    String::from("b.New")
}

/// Shift shifts by a count known only at run time, through the runtime.
pub(crate) fn shift(x: i64, s: i64) -> i64 {
    rt::shl(x, s as i128)
}
