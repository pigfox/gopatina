//! Package example.com/sentinels/store, translated by GoPatina from its Go source.

thread_local! {
    pub(crate) static ERR_MISSING: crate::Iface = crate::go_error("missing");
}

thread_local! {
    pub(crate) static ERR_LOCKED: crate::Iface = crate::go_wrap(
        format!(
            "locked: {}",
            crate::iface_w(&ERR_MISSING.with(Clone::clone)),
        ),
        ERR_MISSING.with(Clone::clone),
    );
}

/// Get returns the value of key, or one of the sentinel errors.
pub(crate) fn get(key: &str) -> Result<i64, crate::Iface> {
    match key {
        "a" => {
            return Ok(1);
        }
        "l" => {
            return crate::iface_result(0, ERR_LOCKED.with(Clone::clone));
        }
        _ => {}
    } // Default is an interface value of another kind.
    crate::iface_result(0, ERR_MISSING.with(Clone::clone))
}

pub(crate) fn init_vars() {
    ERR_MISSING.with(|_| {});
    ERR_LOCKED.with(|_| {});
}
