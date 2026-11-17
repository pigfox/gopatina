// Hand-written Rust for the sieve benchmark (bench/README.md).

fn count_primes(n: usize) -> usize {
    let mut composite = vec![false; n];
    let mut count = 0;
    for i in 2..n {
        if composite[i] {
            continue;
        }
        count += 1;
        let mut j = i * i;
        while j < n {
            composite[j] = true;
            j += i;
        }
    }
    count
}

fn main() {
    println!("{}", count_primes(20_000_000));
}
