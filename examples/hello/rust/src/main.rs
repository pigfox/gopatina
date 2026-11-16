// Planned output of: gopatina build -emit-rust ./examples/hello
// Generated from examples/hello/main.go. Design, not yet implemented.

/// Greeting is a small struct holding a string, so it is moved, not copied,
/// in patina mode.
pub struct Greeting {
    pub name: String,
    pub times: i64,
}

/// render builds the greeting text. It is the last user of g, so patina mode
/// moves g into it.
fn render(g: Greeting) -> String {
    format!("Hello, {}! ", g.name).repeat(g.times as usize)
}

fn run() {
    let g = Greeting { name: String::from("Gopher"), times: 2 };
    println!("{}", render(g).trim());

    let mut names: Vec<String> = vec![String::from("Ferris"), String::from("Gopher")];
    // patina: cloned names[0] (names changes below while first is in use)
    let first = names[0].clone();
    names.push(String::from("Gordon")); // names changes while first is still in use
    println!("{} {}", first, names.len());
}

fn main() {
    run();
}
