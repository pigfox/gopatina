//! Package example.com/multi/a, translated by GoPatina from its Go source.

/// Max is a typed constant used by another package.
const MAX: i64 = 10;

/// A Point has a String method, which fmt calls in every package.
#[derive(Clone, Copy, Default, PartialEq)]
pub(crate) struct Point {
    pub(crate) x: i64,
    pub(crate) y: i64,
}

impl Point {
    pub(crate) fn string(self) -> String {
        format!("({}, {})", self.x, self.y)
    }
}

pub(crate) fn init3() {
    println!("init a z");
}

/// New has the same name as b.New.
pub(crate) fn new(x: i64, y: i64) -> Point {
    Point { x, y }
}

/// Sum adds the points.
pub(crate) fn sum(ps: &[Point]) -> Point {
    let mut s: Point = Point::default(); // New has the same name as a.New.
    for p in ps.iter().copied() {
        s.x += p.x;
        s.y += p.y; // Shift shifts by a count known only at run time, through the runtime.
    }
    s
}
