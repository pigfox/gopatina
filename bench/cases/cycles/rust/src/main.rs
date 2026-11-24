// Hand-written Rust for the cycles benchmark (bench/README.md): a doubly
// linked list as a Rust developer writes one, its nodes in one Vec and
// linked by index, so dropping the Vec frees the list.

struct Node {
    v: i64,
    prev: Option<usize>,
    next: Option<usize>,
}

fn build(n: i64) -> Vec<Node> {
    let mut nodes = vec![Node {
        v: 0,
        prev: None,
        next: None,
    }];
    let mut tail = 0;
    for i in 1..n {
        nodes.push(Node {
            v: i,
            prev: Some(tail),
            next: None,
        });
        let x = nodes.len() - 1;
        nodes[tail].next = Some(x);
        tail = x;
    }
    nodes
}

fn walk(nodes: &[Node]) -> i64 {
    let mut s = 0;
    let mut last = 0;
    let mut p = Some(0);
    while let Some(i) = p {
        s += nodes[i].v;
        last = i;
        p = nodes[i].next;
    }
    let mut q = Some(last);
    while let Some(i) = q {
        s -= nodes[i].v / 2;
        q = nodes[i].prev;
    }
    s
}

fn main() {
    let mut total = 0;
    for _ in 0..300 {
        total += walk(&build(10000));
    }
    println!("{total}");
}
