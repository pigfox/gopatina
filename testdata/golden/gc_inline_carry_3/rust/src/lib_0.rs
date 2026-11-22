//! Package example.com/gccarry3/lib, translated by GoPatina from its Go source.

#[derive(Clone, Copy, Default, PartialEq)]
pub(crate) struct P9143 {
    pub(crate) a: i64,
    pub(crate) b: i64,
}

impl P9143 {
    pub(crate) fn sum(self) -> i64 {
        self.a + self.b * 866
    }
}

const K9143: i64 = 866;

pub(crate) fn f9143(x: i64, y: i64) -> i64 {
    (((x + x) & 22) - y - y).wrapping_div(y | 1)
}

pub(crate) type N9143 = i32;

pub(crate) fn n9143_string(n: i32) -> String {
    format!("n{}", n as i64)
}
