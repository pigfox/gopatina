//! GoPatina's machinery for interface and error values (RFC 0012),
//! private to this library crate (RFC 0014).

#[allow(unused_imports)]
use super::*;

/// The dynamic types of the program's interface values (RFC 0012).
#[derive(Clone)]
pub enum Dyn {
    ErrorsErrorString(rt::Error),
    FmtWrapError(WrapError),
}

/// An interface value: its dynamic value, or None for a nil interface. The
/// value cannot be changed through the interface, so a copy is a clone, as
/// Go copies it.
pub type Iface = Option<Box<Dyn>>; // boxed: a dynamic value can hold one

/// Converts a value to an interface value.
pub fn boxed(d: Dyn) -> Iface {
    Some(Box::new(d))
}

/// Go's == on two dynamic values: the same type and equal values; a type
/// that is not comparable panics.
impl PartialEq for Dyn {
    fn eq(&self, other: &Self) -> bool {
        match (self, other) {
            (Dyn::ErrorsErrorString(a), Dyn::ErrorsErrorString(b)) => a.id() == b.id(),
            (Dyn::FmtWrapError(a), Dyn::FmtWrapError(b)) => a.id() == b.id(),
            _ => false,
        }
    }
}

/// The Go name of a dynamic type, as %T prints it.
pub fn iface_name(d: Option<&Dyn>) -> &'static str {
    match d {
        None => "<nil>",
        Some(Dyn::ErrorsErrorString(_)) => "*errors.errorString",
        Some(Dyn::FmtWrapError(_)) => "*fmt.wrapError",
    }
}

/// %T of an interface value.
pub fn iface_type(x: &Iface) -> &'static str {
    iface_name(x.as_deref())
}

/// %v of an interface value.
pub fn iface_show(x: &Iface) -> String {
    match x.as_deref() {
        None => String::from("<nil>"),
        Some(Dyn::ErrorsErrorString(v)) => v.to_string(),
        Some(Dyn::FmtWrapError(v)) => v.msg.clone(),
    }
}

/// The panic of a failed x.(T) to a concrete type T.
pub fn iface_assert_fail(d: Option<&Dyn>, iface: &str, want: &str) -> ! {
    match d {
        None => panic!("interface conversion: {iface} is nil, not {want}"),
        Some(_) => panic!(
            "interface conversion: {iface} is {}, not {want}",
            iface_name(d)
        ),
    }
}

/// x.(I) to an interface type I: x, or Go's panic.
pub fn iface_assert(x: Iface, missing: fn(&Dyn) -> Option<&'static str>, want: &str) -> Iface {
    match x.as_deref() {
        None => panic!("interface conversion: interface is nil, not {want}"),
        Some(d) => {
            if let Some(m) = missing(d) {
                panic!(
                    "interface conversion: {} is not {want}: missing method {m}",
                    iface_name(Some(d))
                )
            }
        }
    }
    x
}

/// v, ok := x.(I) to an interface type I.
pub fn iface_try(x: &Iface, missing: fn(&Dyn) -> Option<&'static str>) -> (Iface, bool) {
    match x.as_deref() {
        Some(d) if missing(d).is_none() => (x.clone(), true),
        _ => (None, false),
    }
}

/// Go's *fmt.wrapError: fmt.Errorf's value with one %w. Each one is a
/// new pointer in Go; id stands for that identity.
#[derive(Clone)]
pub struct WrapError {
    msg: String,
    err: Iface,
    id: u64,
}

impl WrapError {
    fn id(&self) -> u64 {
        self.id
    }
}

/// fmt.Errorf with %w: the message, wrapping err.
pub fn go_wrap(msg: String, err: Iface) -> Iface {
    let id = rt::error_id();
    boxed(Dyn::FmtWrapError(WrapError { msg, err, id }))
}

/// %w of an error: its text, or %!w(<nil>).
pub fn iface_w(x: &Iface) -> String {
    if x.is_none() {
        return String::from("%!w(<nil>)");
    }
    iface_show(x)
}

/// errors.Is: whether err or an error it wraps is target, or says it is.
pub fn iface_is(err: &Iface, target: &Iface) -> bool {
    if err.is_none() || target.is_none() {
        return err.is_none() && target.is_none();
    }
    let mut err = err.clone();
    loop {
        if err == *target {
            return true;
        }
        match iface_unwrap(&err) {
            Some(next) if next.is_some() => err = next,
            _ => return false,
        }
    }
}

/// Unwrap() error of the dynamic value: the error it wraps, or None if it has
/// no such method.
pub fn iface_unwrap(x: &Iface) -> Option<Iface> {
    match x.as_deref() {
        Some(Dyn::FmtWrapError(v)) => Some(v.err.clone()),
        _ => None,
    }
}

/// errors.New: a new error with the message, equal only to itself.
pub fn go_error(message: impl Into<String>) -> Iface {
    boxed(Dyn::ErrorsErrorString(rt::Error::new(message)))
}

/// return v..., err: Ok when err is nil, else Err.
pub fn iface_result<T>(v: T, err: Iface) -> Result<T, Iface> {
    match err {
        None => Ok(v),
        e => Err(e),
    }
}
