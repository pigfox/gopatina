// Prints what the generated inventory crate computes on the inputs
// check/main.go gives the Go package: the two must print the same.

use inventory::{Item, apply};

fn main() {
    let items = vec![
        Item {
            sku: "bolt".into(),
            count: 10,
        },
        Item {
            sku: "nut".into(),
            count: 2,
        },
    ];
    let skus: Vec<String> = ["bolt", "nut", "bolt"].map(String::from).to_vec();
    let result = apply(&items, &skus, &[-3, 5, 4]);
    let ok = result.is_ok();
    let out = result.expect("enough stock");
    println!("{} {} {} {}", out[0].sku, out[0].count, out[1].count, ok);
    let skus: Vec<String> = ["nut", "nut"].map(String::from).to_vec();
    let err = apply(&items, &skus, &[-1, -2]).unwrap_err();
    println!("failed: {err}");
    println!("{}", inventory::report(&items, &["bolt".to_string()], &[-20]));
}
