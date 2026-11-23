// Prints what the generated wordstat crate computes on the inputs
// check/main.go gives the Go package: the two must print the same.

fn main() {
    let words = wordstat::words("the cat and the dog and the bird");
    println!("{} {} {}", words.len(), words[0], wordstat::MAX_WORDS);
    for c in wordstat::top(&words) {
        println!("{} {}", c.word, c.n);
    }
}
