// Hand-written Rust for the suffix-sum benchmark (bench/README.md). The
// checksum borrows s[start..] the same way the GoPatina case passes s[start:].

fn checksum(s: &[i64]) {
    let mut total: i64 = 0;
    for &v in s.iter() {
        total = total.wrapping_mul(33).wrapping_add(v);
    }
    println!("{}", total);
}

fn main() {
    const N: usize = 20_000_000;
    const START: usize = 1_000_000;
    let mut s: Vec<i64> = vec![0; N];
    for i in 0..N {
        s[i] = (i % 101) as i64 - 50;
    }
    checksum(&s[START..]);
}
