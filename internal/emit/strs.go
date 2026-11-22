package emit

import "strings"

// The runtime items of package strings' first provided API (RFC 0011),
// each written to Go's documented behaviour. The case mapping and folding
// use Go's own tables (casetables.go), not Rust's, whose Unicode version
// differs.
func init() {
	caseMap, caseFold, _ := strings.Cut(caseTables, "/// Go's case orbits")
	caseFold = "/// Go's case orbits" + caseFold
	rtItems = append(rtItems,
		rtItem{
			name: "go_case",
			code: caseMap + `/// Go's UpperLower delta (unicode.MaxRune + 1).
const UL: i32 = 0x110000;

/// Go's unicode.ToUpper (upper) or unicode.ToLower of c.
fn go_case(c: char, upper: bool) -> char {
    let r = c as u32;
    let i = CASE.partition_point(|e| e.1 < r);
    let Some(&(lo, _, du, dl)) = CASE.get(i) else {
        return c;
    };
    if r < lo {
        return c;
    }
    let d = if upper { du } else { dl };
    let m = if d == UL {
        lo + (((r - lo) & !1) | u32::from(!upper))
    } else {
        r.wrapping_add_signed(d)
    };
    char::from_u32(m).unwrap_or(c)
}
`,
		},
		rtItem{
			name: "to_lower",
			deps: []string{"go_case"},
			code: `/// Go's strings.ToLower: unicode.ToLower of every rune.
pub fn to_lower(s: &str) -> String {
    s.chars().map(|c| go_case(c, false)).collect()
}
`,
		},
		rtItem{
			name: "to_upper",
			deps: []string{"go_case"},
			code: `/// Go's strings.ToUpper: unicode.ToUpper of every rune.
pub fn to_upper(s: &str) -> String {
    s.chars().map(|c| go_case(c, true)).collect()
}
`,
		},
		rtItem{
			name: "equal_fold",
			deps: []string{"go_case"},
			code: caseFold + `
/// Go's unicode.SimpleFold: the next rune of c's case orbit.
fn simple_fold(c: char) -> char {
    if let Ok(i) = FOLD.binary_search_by_key(&(c as u32), |e| e.0) {
        return char::from_u32(FOLD[i].1).unwrap_or(c);
    }
    let l = go_case(c, false);
    if l != c { l } else { go_case(c, true) }
}

/// Go's strings.EqualFold: equal under simple Unicode case folding.
pub fn equal_fold(s: &str, t: &str) -> bool {
    let mut t = t.chars();
    for sr in s.chars() {
        let Some(tr) = t.next() else {
            return false;
        };
        if sr == tr {
            continue;
        }
        let (sr, tr) = if tr < sr { (tr, sr) } else { (sr, tr) };
        if tr.is_ascii() {
            if sr.is_ascii_uppercase() && tr == sr.to_ascii_lowercase() {
                continue;
            }
            return false;
        }
        let mut r = simple_fold(sr);
        while r != sr && r < tr {
            r = simple_fold(r);
        }
        if r != tr {
            return false;
        }
    }
    t.next().is_none()
}
`,
		},
		rtItem{
			name: "trim_prefix",
			code: `/// Go's strings.TrimPrefix.
pub fn trim_prefix(s: &str, prefix: &str) -> String {
    s.strip_prefix(prefix).unwrap_or(s).to_string()
}
`,
		},
		rtItem{
			name: "trim_suffix",
			code: `/// Go's strings.TrimSuffix.
pub fn trim_suffix(s: &str, suffix: &str) -> String {
    s.strip_suffix(suffix).unwrap_or(s).to_string()
}
`,
		},
		rtItem{
			name: "split",
			code: `/// Go's strings.Split: with an empty separator, one string per rune.
pub fn split(s: &str, sep: &str) -> Vec<String> {
    if sep.is_empty() {
        return s.chars().map(String::from).collect();
    }
    s.split(sep).map(String::from).collect()
}
`,
		},
		rtItem{
			name: "cut",
			code: `/// Go's strings.Cut: before and after the first sep, and whether it was found.
pub fn cut(s: &str, sep: &str) -> (String, String, bool) {
    match s.split_once(sep) {
        Some((before, after)) => (before.to_string(), after.to_string(), true),
        None => (s.to_string(), String::new(), false),
    }
}
`,
		},
		rtItem{
			name: "index_byte",
			code: `/// Go's strings.IndexByte: the index of the first c in s, or -1.
pub fn index_byte(s: &str, c: u8) -> i64 {
    s.bytes().position(|b| b == c).map_or(-1, |i| i as i64)
}
`,
		},
	)
}
