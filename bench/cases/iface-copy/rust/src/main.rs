// Hand-written Rust for the iface-copy benchmark (bench/README.md): the
// shapes as an enum, as a Rust developer writes a closed set, and the copy
// a borrow, since it is only read.

enum Shape {
    Rect { w: i64, h: i64, name: String },
    Circle { r: i64, name: String },
}

impl Shape {
    fn area(&self) -> i64 {
        match self {
            Shape::Rect { w, h, name } => w * h + name.len() as i64,
            Shape::Circle { r, name } => 3 * r * r + name.len() as i64,
        }
    }
}

fn measure(s: &Shape) -> i64 {
    s.area()
}

fn main() {
    let n: i64 = 200_000_000;
    let mut a = Shape::Rect {
        w: 2,
        h: 3,
        name: String::from("a rectangle with a name"),
    };
    let mut total: i64 = 0;
    for i in 0..n {
        let b = &a;
        total += measure(b) + i % 3;
        if i % 1000 == 0 {
            if i % 2000 == 0 {
                a = Shape::Circle {
                    r: i % 7,
                    name: String::from("a circle"),
                };
            } else {
                a = Shape::Rect {
                    w: i % 5,
                    h: 3,
                    name: String::from("a rectangle with a name"),
                };
            }
        }
    }
    println!("{total}");
}
