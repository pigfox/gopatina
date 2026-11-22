//! Package example.com/errpkgs/parse, translated by GoPatina from its Go source.

use crate::rt;

/// Digit returns the value of a decimal digit.
pub(crate) fn digit(c: u8) -> Result<i64, crate::Iface> {
    if c < b'0' || c > b'9' {
        return Err(crate::go_error(format!(
            "not a digit: {}",
            rt::rune(c as i64),
        )));
    }
    Ok((c - b'0') as i64)
}

/// Number parses a string of digits, passing Digit's errors on.
pub(crate) fn number(s: &str) -> Result<i64, crate::Iface> {
    if s.is_empty() {
        return Err(crate::go_error("empty"));
    }
    let mut n: i64 = 0;
    for i in 0..s.len() as i64 {
        let d = digit(s.as_bytes()[i as usize])?;
        n = n * 10 + d;
    }
    Ok(n)
}

/// Check reports whether n is small.
pub(crate) fn check(n: i64) -> Result<(), crate::Iface> {
    if n > 99 {
        return Err(crate::go_error(format!("too big: {}", n)));
    }
    Ok(())
}

/// Parser parses with a limit.
#[derive(Clone, Copy, Default, PartialEq)]
pub(crate) struct Parser {
    pub(crate) limit: i64,
}

impl Parser {
    /// Parse parses s and checks it against the limit.
    pub(crate) fn parse(self, s: &str) -> Result<i64, crate::Iface> {
        let n = number(s)?;
        if n > self.limit {
            return Err(crate::go_error(format!("over {}", self.limit)));
        }
        Ok(n)
    }
}
