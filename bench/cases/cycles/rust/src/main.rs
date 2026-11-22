// Hand-written Rust for the cycles benchmark (bench/README.md): a doubly
// linked list as a Rust developer writes one, with Weak back pointers, so
// dropping a list frees it.

use std::cell::RefCell;
use std::rc::{Rc, Weak};

struct Node {
    v: i64,
    prev: Weak<RefCell<Node>>,
    next: Option<Rc<RefCell<Node>>>,
}

fn build(n: i64) -> Rc<RefCell<Node>> {
    let head = Rc::new(RefCell::new(Node {
        v: 0,
        prev: Weak::new(),
        next: None,
    }));
    let mut tail = Rc::clone(&head);
    for i in 1..n {
        let x = Rc::new(RefCell::new(Node {
            v: i,
            prev: Rc::downgrade(&tail),
            next: None,
        }));
        tail.borrow_mut().next = Some(Rc::clone(&x));
        tail = x;
    }
    head
}

fn walk(head: &Rc<RefCell<Node>>) -> i64 {
    let mut s = 0;
    let mut p = Some(Rc::clone(head));
    let mut last = Rc::clone(head);
    while let Some(n) = p {
        s += n.borrow().v;
        p = n.borrow().next.clone();
        last = n;
    }
    let mut q = Some(last);
    while let Some(n) = q {
        s -= n.borrow().v / 2;
        q = n.borrow().prev.upgrade();
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
