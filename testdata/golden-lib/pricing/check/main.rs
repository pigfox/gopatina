// Prints what the generated pricing crate computes on the inputs
// check/main.go gives the Go package: the two must print the same.

use pricing::{Order, bulk, label, tag};

fn main() {
    // Order has an unexported field, so Rust code starts from its zero
    // value, as Go code outside the package does.
    let mut o = Order::default();
    o.min_qty = 10;
    o.percent = 5;
    o.add("apple".into(), 12, 50);
    o.add("pear".into(), 2, 80);
    let total = o.total();
    println!("{} {}", total.as_ref().unwrap(), total.is_ok());
    println!("{} {}", label(&o.lines[0]), -1234);
    println!("{} {}", bulk(&o.lines, 10).len(), tag(o.clone(), "rush".into()));
    let err = Order::default().total().unwrap_err();
    println!("{} {}", err, err == pricing::err_empty());
    let re = pricing::reprice(&o.lines, 5, 10, true);
    let ok = re.is_ok();
    let re = re.unwrap();
    println!("{} {} {} {}", re.len(), re[0].price, re[1].price, ok);
    let err = pricing::reprice(&o.lines, -60, 0, false).unwrap_err();
    println!("{err}");
    o.add("plum".into(), 0, 30);
    let err = o.total().unwrap_err();
    let quantity = pricing::err_quantity();
    println!("{} {} {}", err, err.is(&quantity), err.unwrap() == Some(quantity));
}
