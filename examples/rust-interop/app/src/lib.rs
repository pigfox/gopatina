//! Prices an order under a rules file. Reading the order and printing the
//! receipt are written here in Rust; the rules engine is the `rules`
//! crate, which GoPatina generates from the Go package `go/rules`.

use std::error::Error;
use std::fmt;

/// A malformed line of an order file.
#[derive(Debug, PartialEq, Eq)]
pub struct OrderError {
    line: usize,
}

impl fmt::Display for OrderError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "order line {}: expected <sku> <quantity> <cents>",
            self.line
        )
    }
}

impl Error for OrderError {}

/// Reads an order: one `<sku> <quantity> <cents>` line per item, with blank
/// lines and `#` comments ignored.
pub fn parse_order(text: &str) -> Result<Vec<rules::Line>, OrderError> {
    let mut lines = Vec::new();
    for (i, raw) in text.lines().enumerate() {
        let line = raw.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let err = || OrderError { line: i + 1 };
        let fields: Vec<&str> = line.split_whitespace().collect();
        let [sku, quantity, cents] = fields[..] else {
            return Err(err());
        };
        lines.push(rules::Line {
            sku: sku.to_string(),
            quantity: number(quantity).ok_or_else(err)?,
            cents: number(cents).ok_or_else(err)?,
        });
    }
    Ok(lines)
}

/// A whole number of one to nine digits, as the Go original reads it.
fn number(s: &str) -> Option<i64> {
    if s.is_empty() || s.len() > 9 || !s.bytes().all(|b| b.is_ascii_digit()) {
        return None;
    }
    s.parse().ok()
}

/// Formats cents as dollars and cents.
fn money(cents: i64) -> String {
    format!("{}.{:02}", cents / 100, cents % 100)
}

/// The receipt for a priced order.
pub fn receipt(q: &rules::Quote) -> String {
    let mut out = String::new();
    for (label, cents) in [
        ("subtotal", q.subtotal),
        ("discount", -q.discount),
        ("shipping", q.shipping),
        ("tax", q.tax),
        ("total", q.total),
    ] {
        let amount = if cents < 0 {
            format!("-{}", money(-cents))
        } else {
            money(cents)
        };
        out.push_str(&format!("{label:<10}{amount:>10}\n"));
    }
    let applied = if q.applied.is_empty() {
        "none".to_string()
    } else {
        q.applied.join("; ")
    };
    out.push_str(&format!("applied: {applied}\n"));
    out
}

/// Prices the order text under the rules text and returns the receipt.
pub fn run(rules_text: &str, order_text: &str) -> Result<String, Box<dyn Error>> {
    let rules = rules::parse(rules_text)?;
    let order = parse_order(order_text)?;
    let quote = rules::price(&order, &rules)?;
    Ok(receipt(&quote))
}

#[cfg(test)]
mod tests {
    use super::*;

    const RULES: &str = "discount bulk 10 5\nshipping 499 free-over 5000\ntax 8\n";

    #[test]
    fn prices_an_order() {
        let out = run(RULES, "apple 12 50\npear 2 80\n").unwrap();
        assert!(out.contains("total          12.87"), "{out}");
        assert!(out.ends_with("applied: 5% off apple\n"), "{out}");
    }

    #[test]
    fn free_shipping_over_the_threshold() {
        let out = run(RULES, "tv 1 59900\n").unwrap();
        assert!(out.contains("shipping        0.00"), "{out}");
        assert!(out.contains("free shipping"), "{out}");
    }

    #[test]
    fn reports_a_bad_order_line() {
        let err = parse_order("apple 12 50\npear two 80\n").unwrap_err();
        assert_eq!(err, OrderError { line: 2 });
        assert_eq!(
            err.to_string(),
            "order line 2: expected <sku> <quantity> <cents>"
        );
    }

    #[test]
    fn rules_errors_are_go_errors() {
        let err = rules::parse("tax eight").unwrap_err();
        assert!(err.is(&rules::err_syntax()));
        assert_eq!(
            err.to_string(),
            "line 1: rules: syntax error: bad number eight"
        );
        let empty = rules::price(&[], &[]).unwrap_err();
        assert_eq!(empty, rules::err_empty());
    }
}
