# Contributing to GoPatina

Thank you for your interest in GoPatina. The project is at the design (RFC)
stage, so the most valuable contributions right now are careful reading,
questions and critique of [RFC 0001](docs/rfcs/0001-gopatina.md), followed by
small, well-tested changes to the prototype CLI.

By participating you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Ground rules

- **Every change lands through a pull request.** Nobody pushes to `main`
  directly, maintainers included. `main` is protected: it requires a pull
  request, passing CI, a linear history and the project lead's approval.
- **Nothing merges without the project lead's approval.** Every pull request,
  including the maintainer's own and those opened by automation such as
  Dependabot, is merged only after Peter Sjolin has approved it. Passing CI is
  necessary but never sufficient. An approval is needed again after any new
  push to the branch.
- **CI for outside contributors runs after approval.** GitHub Actions runs on
  a pull request from a fork only after a maintainer has approved the run, so
  your first push may show checks waiting; that is expected.
- **Pull requests are squash-merged.** The pull request title becomes the commit
  subject on `main`, so it must follow [Conventional Commits](#commit-messages).
- **Every commit is signed off** under the [Developer Certificate of
  Origin](#developer-certificate-of-origin-dco).
- **Language and semantics changes go through an RFC** first (see
  [RFC process](#rfc-process)).

## Workflow

1. Fork `pigfox/gopatina` and clone your fork.
2. Create a topic branch from `main`, named after the change:
   `feat/short-name`, `fix/short-name`, `docs/short-name`, `ci/short-name`.
3. Make your change. Keep it focused: one logical change per pull request.
4. Run the same checks CI runs:

   ```sh
   gofmt -l .          # must print nothing
   go vet ./...
   go test -race -coverprofile=coverage.out ./...
   go tool cover -func=coverage.out | tail -n 1   # must say 100.0%
   ```

   If your change touches the compiler (internal/subset, internal/borrow,
   internal/emit), also run the golden and Rust tests:

   ```sh
   go test ./internal/emit -update     # rewrite testdata/golden/*/rust; review the diff
   go test ./internal/borrow -update   # rewrite the explain.txt and want.txt files
   GOPATINA_RUST_TESTS=1 go test ./internal/patina   # build and run every golden program
   ```

   If your change touches code generation (the emitters, the native backend
   or the runtime), also run the benchmarks against hand-written Rust and put
   the results in the pull request description (see
   [Quality and performance](#quality-and-performance)).

5. Commit with a sign-off (`git commit -s`) and a Conventional Commit message.
6. Push the branch to your fork and open a pull request against `main`. Fill in
   the pull request template.
7. Address review feedback with new commits. The branch is squashed on merge,
   so there is no need to rewrite history during review.
8. When CI is green and the project lead has approved the pull request, the
   project lead merges it.

## Quality and performance

The full policy is in [docs/QUALITY.md](docs/QUALITY.md). In short:

- **Coverage stays at 100%.** CI fails if unit-test statement coverage drops
  below 100.0%. New code comes with tests that assert its behaviour.
- **Each phase's test layers are required**: golden, compile and differential
  tests for the emitters, rejection and fuzz tests for ownership inference,
  and so on, as listed in the policy.
- **Benchmarks run when code generation changes.** Pull requests that change
  code generation include the benchmark ratios from CI against equivalent
  hand-written Rust built with the latest stable toolchain (with its
  `rustc --version`), and explain any ratio that gets worse beyond noise.
  Official numbers are measured only after merging, on the maintainer's test
  server. See [bench/README.md](bench/README.md).

### Pull request checklist

- [ ] Title is a Conventional Commit; every commit is signed off.
- [ ] `gofmt -l .` prints nothing; `go vet ./...` and `go test -race ./...` pass.
- [ ] Coverage is still 100.0%.
- [ ] If code generation changed: benchmark results against Rust are in the
      description.
- [ ] Docs and `CHANGELOG.md` updated; semantics changes reference an accepted
      RFC.

## Commit messages

GoPatina uses [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>(<optional scope>): <short summary in the imperative mood>

<body: what changed and why, wrapped at 72 columns>

Signed-off-by: Your Name <you@example.com>
```

Types in use: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`,
`ci`, `chore`, `revert`. Mark breaking changes with `!` after the type
(`feat!: ...`) and a `BREAKING CHANGE:` footer.

Examples:

- `feat(cli): pass build flags through to the go tool`
- `docs(rfc): clarify ownership rules for slices`
- `ci: test on macOS`

## Developer Certificate of Origin (DCO)

GoPatina does not use a CLA. Instead, every commit must carry a
`Signed-off-by` line, certifying that you wrote the change or otherwise have
the right to submit it under the project's license, as described in the
[Developer Certificate of Origin 1.1](https://developercertificate.org/).

Add the line automatically with:

```sh
git commit -s
```

The name and email must match the commit author. If you forgot, amend with
`git commit --amend -s` (or `git rebase --signoff main` for several commits).

## RFC process

Changes to the language subset accepted in patina mode, its ownership rules,
code generation strategy, or anything else a GoPatina user would have to learn
need an RFC before code:

1. Open an issue with the **RFC proposal** template to gauge interest.
2. Copy [`docs/rfcs/0000-template.md`](docs/rfcs/0000-template.md) to
   `docs/rfcs/0000-short-title.md` and fill it in.
3. Open a pull request titled `docs(rfc): <title>`. The RFC number is assigned
   when it is merged.
4. Discussion happens on the pull request. An RFC is accepted when the project
   lead merges it (see [GOVERNANCE.md](GOVERNANCE.md)), and rejected or
   postponed when the pull request is closed with a written summary of why.

Bug fixes, tests, documentation and CLI improvements that do not change
semantics do not need an RFC.

## Licensing

GoPatina is dual-licensed under [MIT](LICENSE-MIT) or
[Apache-2.0](LICENSE-APACHE), at your option. Unless you explicitly state
otherwise, any contribution intentionally submitted for inclusion in the work
by you, as defined in the Apache-2.0 license, shall be dual licensed as above,
without any additional terms or conditions.

## Reporting security issues

Please do not open public issues for vulnerabilities. See
[SECURITY.md](SECURITY.md).
