// Hand-written Rust for the fib benchmark (bench/README.md).

fn fib(n: i64) -> i64 {
    if n < 2 { n } else { fib(n - 1) + fib(n - 2) }
}

fn main() {
    println!("{}", fib(35));
}
