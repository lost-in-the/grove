#!/usr/bin/env bash
# Fails when a tracked (or untracked, non-ignored) file contains a shape that
# usually means a real person's machine or mailbox leaked into the repo.
#
# Shapes only: this script is public, so it must never name private words.
# Private names belong in the secret-backed pii-check workflow.
set -u

cd "$(git rev-parse --show-toplevel)" || exit 2

# Paths where contact or ownership info is expected.
exempt=(
  ':!CODE_OF_CONDUCT.md'
  ':!SECURITY.md'
  ':!LICENSE'
  ':!CHANGELOG.md'
  ':!.github/CODEOWNERS'
  ':!go.mod'
  ':!go.sum'
  ':!scripts/check-leaks.sh'
)

status=0

# scan <label> <ERE to find> <ERE of matches to allow>
# Prints file:line for every found match that is not allowed.
scan() {
  local label=$1 find=$2 allow=$3 out
  out=$(git grep --untracked -nIEo -e "$find" -- . "${exempt[@]}" | grep -Ev "$allow") || true
  if [ -n "$out" ]; then
    # Print the location only. CI logs are public, so echoing the matched
    # text would publish it a second time.
    printf '%s\n' "$out" | cut -d: -f1,2 | sed "s/^/[$label] /"
    status=1
  fi
}

scan 'home-dir' \
  '/Users/[A-Za-z0-9._-]+' \
  ':/Users/(you|dev|user|example)$'

scan 'home-dir' \
  '/home/[A-Za-z0-9._-]+' \
  ':/home/(user|dev|u|runner)$'

scan 'work-path' \
  '~/work($|[^A-Za-z0-9_-])' \
  '^$'

scan 'email' \
  '[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}' \
  ':git@github\.com$|@(example\.com|test\.com|users\.noreply\.github\.com)$'

if [ "$status" -ne 0 ]; then
  echo
  echo "check-leaks: found text that looks personal. Use a neutral placeholder" >&2
  echo "(/home/dev, /Users/you, ~/code/..., user@example.com)." >&2
fi
exit "$status"
