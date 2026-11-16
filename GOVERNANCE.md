# Governance

GoPatina is a young project, and its governance is deliberately simple. It
will be revisited as the contributor base grows.

## Model: benevolent dictator, for now

The project lead is **Peter Sjolin** ([@pigfox](https://github.com/pigfox)),
who founded the project. The project lead:

- sets the project's direction and roadmap,
- accepts or rejects RFCs,
- has the final say when consensus cannot be reached,
- manages releases, repository settings and access.

Day-to-day decisions are made in the open, on issues and pull requests, and
aim for rough consensus among the people doing the work. The project lead
explains decisions in writing.

## Maintainers

Maintainers can review and merge pull requests. The current maintainers are
listed in [`.github/CODEOWNERS`](.github/CODEOWNERS). A contributor with a
sustained record of high-quality contributions and reviews may be invited to
become a maintainer by the project lead.

## Decision process

| Change                                   | Needs                                   |
| ---------------------------------------- | --------------------------------------- |
| Bug fix, test, docs, CLI improvement     | A pull request with passing CI          |
| Language subset, semantics, code gen     | An accepted RFC, then pull requests     |
| Governance, license, code of conduct     | A pull request merged by the lead       |

## Evolving this document

When the project has several active maintainers, this document will be
replaced, through a pull request, with a model that shares decision-making
(for example, a small core team deciding by lazy consensus).

## Code of conduct

Everyone participating in GoPatina is expected to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).
