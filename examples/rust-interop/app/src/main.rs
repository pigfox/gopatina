//! quote: prices an order under a rules file.
//!
//! usage: quote <rules-file> <order-file>

use std::process::ExitCode;
use std::{env, fs};

fn main() -> ExitCode {
    let args: Vec<String> = env::args().skip(1).collect();
    let [rules_path, order_path] = &args[..] else {
        eprintln!("usage: quote <rules-file> <order-file>");
        return ExitCode::from(2);
    };
    let result = fs::read_to_string(rules_path)
        .and_then(|rules| Ok((rules, fs::read_to_string(order_path)?)))
        .map_err(|e| e.into())
        .and_then(|(rules, order)| quote::run(&rules, &order));
    match result {
        Ok(receipt) => {
            print!("{receipt}");
            ExitCode::SUCCESS
        }
        Err(e) => {
            eprintln!("quote: {e}");
            ExitCode::FAILURE
        }
    }
}
