// Hand-written Rust for the iface-keep benchmark (bench/README.md): the
// shape kept by value in a Vec, cloned for each copy, as a Rust developer
// keeps copies of a value.

#[derive(Clone)]
struct Rect {
    w: i64,
    h: i64,
    name: String,
}

impl Rect {
    fn area(&self) -> i64 {
        self.w * self.h + self.name.len() as i64
    }
}

fn main() {
    let (rounds, n): (i64, i64) = (40, 100_000);
    let mut total: i64 = 0;
    for r in 0..rounds {
        let mut a = Rect {
            w: r % 5,
            h: 3,
            name: String::from("a rectangle with a name"),
        };
        let mut kept: Vec<Rect> = Vec::new();
        for i in 0..n {
            kept.push(a.clone());
            if i % 100 == 0 {
                a = Rect {
                    w: i % 7,
                    h: r % 3,
                    name: String::from("a rectangle with a name"),
                };
            }
        }
        for s in &kept {
            total += s.area();
        }
    }
    println!("{total}");
}
