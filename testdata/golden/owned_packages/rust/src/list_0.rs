//! Package example.com/ownedpkgs/list, translated by GoPatina from its Go source.

/// Node is a link of a list, newest first.
#[derive(Clone, Default, PartialEq)]
pub(crate) struct Node {
    pub(crate) v: i64,
    pub(crate) name: String,
    pub(crate) next: Option<Box<Node>>,
}

/// Frees a chain one link at a time, so a long one cannot overflow
/// the stack.
impl Drop for Node {
    fn drop(&mut self) {
        let mut next = self.next.take();
        while let Some(mut link) = next {
            next = link.next.take();
        }
    }
}

impl Node {
    /// Sum walks the list, a borrow of each node in turn.
    pub(crate) fn sum(n: Option<&Node>) -> i64 {
        let mut t: i64 = 0;
        let mut p = n;
        while let Some(node) = p {
            t += node.v;
            p = node.next.as_deref();
        }
        t
    }

    /// Bump adds d to every value, changing the list in place.
    pub(crate) fn bump(&mut self, d: i64) {
        self.v += d;
        if let Some(next) = self.next.as_deref_mut() {
            next.bump(d);
        }
    }
}

/// Push puts v in front of head, moving head into the new node.
pub(crate) fn push(head: Option<Box<Node>>, v: i64, name: String) -> Box<Node> {
    Box::new(Node {
        v,
        name,
        next: head,
    })
}

/// Index returns the position of the first node holding v, or -1.
pub(crate) fn index(head: Option<&Node>, v: i64) -> i64 {
    let mut i: i64 = 0;
    let mut p = head;
    while let Some(node) = p {
        if node.v == v {
            return i;
        }
        i += 1;
        p = node.next.as_deref();
    }
    -1
}

/// Tree is a binary tree built from fresh nodes.
#[derive(Clone, Default, PartialEq)]
pub(crate) struct Tree {
    pub(crate) left: Option<Box<Tree>>,
    pub(crate) right: Option<Box<Tree>>,
    pub(crate) key: i64,
}

impl Tree {
    /// Count returns the number of nodes.
    pub(crate) fn count(t: Option<&Tree>) -> i64 {
        if t.is_none() {
            return 0;
        }
        1 + Tree::count(t.unwrap().left.as_deref()) + Tree::count(t.unwrap().right.as_deref())
    }
}

/// Build returns a full tree of the given depth.
pub(crate) fn build(depth: i64, key: i64) -> Box<Tree> {
    if depth == 0 {
        return Box::new(Tree {
            key,
            ..Default::default()
        });
    }
    Box::new(Tree {
        left: Some(build(depth - 1, 2 * key)),
        right: Some(build(depth - 1, 2 * key + 1)),
        key,
    })
}
