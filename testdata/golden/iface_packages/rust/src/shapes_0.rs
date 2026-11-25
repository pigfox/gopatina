//! Package example.com/ifacepkgs/shapes, translated by GoPatina from its Go source.

/// Shape is anything with an area and a name.
pub(crate) type Shape = crate::Iface;

/// Rect is a rectangle.
#[derive(Clone, Copy, Default, PartialEq)]
pub(crate) struct Rect {
    pub(crate) w: i64,
    pub(crate) h: i64,
}

impl Rect {
    /// Area is the rectangle's area.
    pub(crate) fn area(self) -> i64 {
        self.w * self.h
    }

    /// Name is "rect".
    pub(crate) fn name(self) -> String {
        String::from("rect")
    }
}

/// Unit is a square of side n.
pub(crate) type Unit = i64;

/// Area is the square's area.
pub(crate) fn unit_area(u: i64) -> i64 {
    u * u
}

/// Name is "unit".
pub(crate) fn unit_name(u: i64) -> String {
    String::from("unit")
}

/// Total adds the areas, and reports the kinds it saw.
pub(crate) fn total(all: &[crate::Iface]) -> i64 {
    let mut t: i64 = 0;
    for s in all {
        {
            let dyn_x = s;
            match dyn_x.as_ref() {
                Some(crate::Dyn::ShapesRect(_)) => {
                    println!("a rect of {}", crate::dyn_area(s));
                }
                _ => {
                    println!("a {} of {}", crate::dyn_name(s), crate::dyn_area(s));
                }
            }
        }
        t += crate::dyn_area(s);
    }
    t
}

/// Biggest returns the shape with the largest area, or nil.
pub(crate) fn biggest(all: &[crate::Iface]) -> crate::Iface {
    let mut best: crate::Iface = None;
    for s in all {
        if best.is_none() || crate::dyn_area(s) > crate::dyn_area(&best) {
            best = s.clone();
        }
    }
    best
}
