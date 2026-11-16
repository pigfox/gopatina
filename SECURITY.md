# Security Policy

## Supported versions

GoPatina is pre-alpha. Only the latest commit on `main` is supported; there
are no maintained release branches yet.

## Reporting a vulnerability

Please **do not** report security vulnerabilities through public issues,
discussions or pull requests.

Report them privately through GitHub's security advisory form:
<https://github.com/pigfox/gopatina/security/advisories/new>

Please include:

- a description of the issue and its impact,
- steps to reproduce, or a proof of concept,
- the affected commit or version, and your platform and Go version.

You can expect an acknowledgement within 7 days. We will keep you informed
while we investigate, agree a disclosure date with you, and credit you in the
advisory unless you prefer otherwise.

## Scope

The prototype CLI delegates to the Go toolchain installed on your machine.
Vulnerabilities in Go itself should be reported to the Go project
(<https://go.dev/doc/security/policy>), and in Rust to the Rust project
(<https://www.rust-lang.org/policies/security>).
