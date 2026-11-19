# Governance

GoPatina is a young project, and its governance is deliberately simple. It
will be revisited as the contributor base grows.

## Model: benevolent dictator, for now

The project lead is **Peter Sjolin** ([@pigfox](https://github.com/pigfox)),
who founded the project. The project lead:

- sets the project's direction and roadmap,
- approves every pull request before it is merged,
- accepts or rejects RFCs,
- has the final say when consensus cannot be reached,
- manages releases, repository settings and access.

Day-to-day decisions are made in the open, on issues and pull requests, and
aim for rough consensus among the people doing the work. The project lead
explains decisions in writing.

## Maintainers

Maintainers review pull requests. The current maintainers are listed in
[`.github/CODEOWNERS`](.github/CODEOWNERS). A contributor with a
sustained record of high-quality contributions and reviews may be invited to
become a maintainer by the project lead.

## Approval of changes

Every pull request is merged only after the project lead, Peter Sjolin, has
approved it. This applies to every change: the maintainer's own pull
requests, contributions from forks, and pull requests opened by automation
(Dependabot and similar). Passing CI is required but is never enough on its
own; no change merges on green CI alone. A new push to a pull request needs a
new approval.

GitHub Actions runs on pull requests from outside contributors only after a
maintainer approves the run.

These rules are enforced by `main`'s branch protection: one required
approval, code-owner review, approval required after the last push, and the
required CI checks.

### The lead's own pull requests

GitHub does not let an author approve their own pull request, so the lead's
own changes cannot satisfy the required-approval rule through the review UI.
For those, the lead's recorded approval is a comment on the pull request,
written from the lead's account, that states the approval and the commit it
applies to; the merge is then performed with the administrator override. The
comment is the approval record and is kept for audit, exactly as a review
would be. Any push after that comment needs a new comment.

This is a narrower exception than it sounds: it does not let anyone but the
lead merge, it still requires the required CI checks to pass first, and it
keeps the approval in writing on the pull request.

## Decision process

| Change                                   | Needs                                   |
| ---------------------------------------- | --------------------------------------- |
| Bug fix, test, docs, CLI improvement     | A pull request with passing CI, approved by the lead |
| Language subset, semantics, code gen     | An accepted RFC, then approved pull requests |
| Governance, license, code of conduct     | A pull request approved and merged by the lead |

## Evolving this document

When the project has several active maintainers, this document will be
replaced, through a pull request, with a model that shares decision-making
(for example, a small core team deciding by lazy consensus).

## Code of conduct

Everyone participating in GoPatina is expected to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).
