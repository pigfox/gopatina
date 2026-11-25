//! Runtime support for this crate, written by GoPatina: the few pieces
//! of Go behaviour that have no direct Rust spelling.

use std::fmt;
use std::sync::atomic::AtomicU64;
use std::sync::atomic::Ordering;

/// A Go error value made by errors.New or fmt.Errorf. Functions that
/// return (T, error) in Go return Result<T, Error> here. Each one is a new
/// pointer in Go, equal only to itself and its copies; id stands for that
/// identity. Errors compare by that identity (id), never by message.
#[derive(Clone, Debug)]
pub struct Error {
    message: String,
    id: u64,
}

impl Error {
    pub fn new(message: impl Into<String>) -> Error {
        Error {
            message: message.into(),
            id: error_id(),
        }
    }

    pub fn id(&self) -> u64 {
        self.id
    }
}

/// A new error identity.
pub fn error_id() -> u64 {
    static NEXT: AtomicU64 = AtomicU64::new(0);
    NEXT.fetch_add(1, Ordering::Relaxed)
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.pad(&self.message)
    }
}
