// The Rust a person writes for main.go: the reference GoPatina's Rust is
// measured against (closeness.txt). It is bench/cases/binary-trees/rust
// with the same smaller size as main.go.

struct Node {
    left: Option<Box<Node>>,
    right: Option<Box<Node>>,
}

fn bottom_up(depth: i64) -> Box<Node> {
    if depth <= 0 {
        return Box::new(Node {
            left: None,
            right: None,
        });
    }
    Box::new(Node {
        left: Some(bottom_up(depth - 1)),
        right: Some(bottom_up(depth - 1)),
    })
}

fn check(n: &Node) -> i64 {
    match (&n.left, &n.right) {
        (Some(l), Some(r)) => 1 + check(l) + check(r),
        _ => 1,
    }
}

fn main() {
    let max_depth = 10;
    let long = bottom_up(max_depth);
    let mut d = 4;
    while d <= max_depth {
        let iters = 1i64 << (max_depth - d + 4);
        let mut total = 0;
        for _ in 0..iters {
            total += check(&bottom_up(d));
        }
        println!("{iters} trees of depth {d} check: {total}");
        d += 2;
    }
    println!(
        "long lived tree of depth {max_depth} check: {}",
        check(&long)
    );
}
