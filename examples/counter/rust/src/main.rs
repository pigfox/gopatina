// Planned output of: gopatina build -emit-rust ./examples/counter
// Generated from examples/counter/main.go. Design, not yet implemented.

use std::sync::{Arc, Mutex};
use tokio::task::JoinSet;

/// count starts workers goroutines, each adding per_worker to a shared total.
// patina: async because it waits for goroutines (wg.Wait)
async fn count(workers: i64, per_worker: i64) -> i64 {
    // patina: mu guards total, so they become one Arc<Mutex<i64>>
    let total = Arc::new(Mutex::new(0_i64));
    // patina: sync.WaitGroup -> JoinSet
    let mut wg = JoinSet::new();
    for _ in 0..workers {
        let total = Arc::clone(&total);
        // patina: go func() { ... }() -> a task on the multi-threaded runtime
        wg.spawn(async move {
            for _ in 0..per_worker {
                *total.lock().unwrap() += 1;
            }
        });
    }
    while let Some(done) = wg.join_next().await {
        done.unwrap();
    }
    *total.lock().unwrap()
}

async fn run() {
    println!("total: {}", count(4, 1000).await);
}

#[tokio::main(flavor = "multi_thread")]
async fn main() {
    run().await;
}
