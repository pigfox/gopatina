#!/usr/bin/env bash
# merge-pr.sh: squash-merge a pull request only if the squash commit will
# carry its Signed-off-by line (CONTRIBUTING.md, Developer Certificate of
# Origin), and check afterwards that the commit on main does.
#
# usage: scripts/merge-pr.sh [--admin] [--dry-run] <pr-number> [<body-file>]
#
# The squash commit's subject is the pull request's title with " (#N)". Its
# body is <body-file> if given, otherwise the body of the pull request's
# head commit (everything after the subject line), which is what a
# one-commit pull request needs.
#
# It refuses, and merges nothing, when:
# - the title is not a Conventional Commit;
# - any check on the head commit has not passed;
# - the body has no "Signed-off-by: <author>" line for the head commit's
#   author.
# It merges with --match-head-commit, so a push after the checks ran is
# not merged. --admin is passed on to gh pr merge. --dry-run prints the
# subject and body and stops. After merging it fetches main and exits 1 if
# main's new commit lacks the line.
set -euo pipefail

admin=()
dry=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --admin) admin=(--admin); shift ;;
    --dry-run) dry=1; shift ;;
    *) break ;;
  esac
done
if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: $0 [--admin] [--dry-run] <pr-number> [<body-file>]" >&2
  exit 2
fi
pr="$1"
case "$pr" in
  '' | *[!0-9]*) echo "merge-pr: not a pull request number: $pr" >&2; exit 2 ;;
esac

head="$(gh pr view "$pr" --json headRefOid --jq .headRefOid)"
title="$(gh pr view "$pr" --json title --jq .title)"
pattern='^(feat|fix|docs|test|refactor|perf|build|ci|chore|revert)(\([a-z0-9._/-]+\))?!?: .+'
if ! printf '%s' "$title" | grep -Eq "$pattern"; then
  echo "merge-pr: the title is not a Conventional Commit: $title" >&2
  exit 1
fi

# shellcheck disable=SC2016 # $c is jq's, not the shell's
pending="$(gh pr view "$pr" --json statusCheckRollup --jq '[.statusCheckRollup[] | select((.conclusion // "") as $c | ($c != "SUCCESS" and $c != "SKIPPED" and $c != "NEUTRAL")) | (.name // .context)] | join(", ")')"
if [ -n "$pending" ]; then
  echo "merge-pr: checks not passed on $head: $pending" >&2
  exit 1
fi

git fetch -q origin "$head" 2>/dev/null || git fetch -q origin "pull/$pr/head"
author="$(git show -s --format='%an <%ae>' "$head")"
body="$(mktemp)"
trap 'rm -f "$body"' EXIT
if [ "$#" -eq 2 ]; then
  cat "$2" >"$body"
else
  git show -s --format=%B "$head" | tail -n +3 >"$body"
fi
if ! grep -Fqx "Signed-off-by: $author" "$body"; then
  echo "merge-pr: the squash body has no 'Signed-off-by: $author' line" >&2
  exit 1
fi

subject="$title (#$pr)"
if [ "$dry" -eq 1 ]; then
  printf '%s\n\n' "$subject"
  cat "$body"
  exit 0
fi
gh pr merge "$pr" --squash "${admin[@]}" --match-head-commit "$head" --subject "$subject" --body-file "$body"

git fetch -q origin main
merged="$(git rev-parse origin/main)"
if ! git show -s --format=%B "$merged" | grep -Fqx "Signed-off-by: $author"; then
  echo "merge-pr: main's commit $merged has no 'Signed-off-by: $author' line" >&2
  exit 1
fi
echo "$merged"
