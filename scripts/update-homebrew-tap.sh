#!/usr/bin/env bash
# update-homebrew-tap.sh — publish a grove release's formula to the Homebrew tap.
#
# Usage: TAP_GITHUB_TOKEN=... scripts/update-homebrew-tap.sh <tag>
#
# The tap's default branch only accepts changes through a pull request (a
# repository rule: a direct write is refused with HTTP 409, which is how the
# v0.11.0 release's formula update failed). So this renders the formula from
# .github/formula.rb.tmpl, commits it to a grove-<version> branch on the tap,
# and opens a pull request (or reuses the open one). It never merges. The run
# prints a notice with the pull request link, and a person merges it.
#
# Safe to re-run: a tap already at this formula is a no-op, and an existing
# branch or pull request is reused rather than duplicated.
#
# TAP_GITHUB_TOKEN needs Contents and Pull requests write access to the tap
# (to push the branch and open the pull request).
#
# Overrides (tests point these at a fake API): TAP_REPO, SOURCE_REPO,
# GITHUB_API, TARBALL_URL.

set -euo pipefail

TAG="${1:?usage: update-homebrew-tap.sh <tag>}"
: "${TAP_GITHUB_TOKEN:?TAP_GITHUB_TOKEN is required}"

VERSION="${TAG#v}"
TAP_REPO="${TAP_REPO:-lost-in-the/homebrew-tap}"
SOURCE_REPO="${SOURCE_REPO:-lost-in-the/grove}"
API="${GITHUB_API:-https://api.github.com}"
TARBALL_URL="${TARBALL_URL:-https://github.com/${SOURCE_REPO}/archive/refs/tags/${TAG}.tar.gz}"
FORMULA_PATH="Formula/grove.rb"
BRANCH="grove-${VERSION}"
TITLE="grove ${VERSION}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEMPLATE="${SCRIPT_DIR}/../.github/formula.rb.tmpl"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fail() { echo "::error::$1" >&2; [ -n "${2:-}" ] && echo "$2" >&2; exit 1; }

# api METHOD PATH [JSON_BODY] — sets HTTP_CODE and RESPONSE. Runs in this
# shell (never in a command substitution) so both stay visible to the caller.
api() {
  local method="$1" path="$2" body="${3:-}" out
  local args=(-sS -w '\n%{http_code}' -X "$method"
    -H "Authorization: token ${TAP_GITHUB_TOKEN}"
    -H "Accept: application/vnd.github+json")
  if [ -n "$body" ]; then
    args+=(-H "Content-Type: application/json" -d "$body")
  fi
  out="$(curl "${args[@]}" "${API}${path}")" || fail "request failed: ${method} ${path}"
  HTTP_CODE="${out##*$'\n'}"
  RESPONSE="${out%$'\n'*}"
}

# expect CODE... — fail unless the last response had one of these statuses.
expect() {
  local code
  for code in "$@"; do
    [ "$HTTP_CODE" = "$code" ] && return 0
  done
  fail "unexpected HTTP ${HTTP_CODE} from the GitHub API" "$RESPONSE"
}

# file_on REF — sets FILE_SHA and FILE_CONTENT (base64) for the formula on REF,
# both empty when the file does not exist there. Not for use in a command
# substitution: a failed request must end the script, not read as "no file".
file_on() {
  FILE_SHA="" FILE_CONTENT=""
  api GET "/repos/${TAP_REPO}/contents/${FORMULA_PATH}?ref=$1"
  [ "$HTTP_CODE" = "404" ] && return 0
  expect 200
  FILE_SHA="$(jq -r .sha <<<"$RESPONSE")"
  FILE_CONTENT="$(jq -r '.content | gsub("\n"; "")' <<<"$RESPONSE")"
}

# same_content BASE64 — whether it decodes to exactly the rendered formula.
same_content() {
  [ -n "$1" ] && cmp -s <(base64 -d <<<"$1") "$WORK/grove.rb"
}

# --- render the formula -------------------------------------------------------

# -f so a 404/5xx fails instead of hashing an error page, and refuse an empty
# download: a wrong digest publishes a formula every `brew install` rejects.
curl -fsSL --retry 3 --retry-delay 2 "$TARBALL_URL" -o "$WORK/source.tar.gz" \
  || fail "could not download the source tarball: $TARBALL_URL"
[ -s "$WORK/source.tar.gz" ] || fail "downloaded source tarball is empty: $TARBALL_URL"
SHA256="$(sha256sum "$WORK/source.tar.gz" | awk '{print $1}')"

sed -e "s/VERSION/${VERSION}/g" -e "s/SHA256/${SHA256}/g" "$TEMPLATE" > "$WORK/grove.rb"
FORMULA_B64="$(base64 -w 0 "$WORK/grove.rb")"

# --- nothing to do if the tap already has it ------------------------------------

api GET "/repos/${TAP_REPO}"
expect 200
BASE="$(jq -r .default_branch <<<"$RESPONSE")"

file_on "$BASE"
if same_content "$FILE_CONTENT"; then
  echo "${TAP_REPO} ${BASE} already has the grove ${VERSION} formula; nothing to do."
  exit 0
fi

# --- branch with the formula -----------------------------------------------------

api GET "/repos/${TAP_REPO}/git/ref/heads/${BASE}"
expect 200
BASE_SHA="$(jq -r .object.sha <<<"$RESPONSE")"

api POST "/repos/${TAP_REPO}/git/refs" \
  "$(jq -n --arg ref "refs/heads/${BRANCH}" --arg sha "$BASE_SHA" '{ref: $ref, sha: $sha}')"
if [ "$HTTP_CODE" = "422" ] && grep -q "Reference already exists" <<<"$RESPONSE"; then
  echo "Reusing branch ${BRANCH} from an earlier run."
else
  expect 201
fi

file_on "$BRANCH"
if same_content "$FILE_CONTENT"; then
  echo "${BRANCH} already carries the grove ${VERSION} formula."
else
  body="$(jq -n --arg message "$TITLE" --arg content "$FORMULA_B64" \
    --arg branch "$BRANCH" --arg sha "$FILE_SHA" \
    '{message: $message, content: $content, branch: $branch}
     + (if $sha == "" then {} else {sha: $sha} end)')"
  api PUT "/repos/${TAP_REPO}/contents/${FORMULA_PATH}" "$body"
  expect 200 201
  echo "Committed the grove ${VERSION} formula to ${BRANCH}."
fi

# --- pull request ------------------------------------------------------------------

api GET "/repos/${TAP_REPO}/pulls?state=open&head=${TAP_REPO%%/*}:${BRANCH}"
expect 200
PR_NUMBER="$(jq -r '.[0].number // empty' <<<"$RESPONSE")"
PR_URL="$(jq -r '.[0].html_url // empty' <<<"$RESPONSE")"

if [ -z "$PR_NUMBER" ]; then
  pr_body="Updates the \`grove\` formula to [${TAG}](https://github.com/${SOURCE_REPO}/releases/tag/${TAG}).

Generated by grove's release workflow from \`.github/formula.rb.tmpl\`: source tarball SHA-256 \`${SHA256}\`."
  api POST "/repos/${TAP_REPO}/pulls" \
    "$(jq -n --arg title "$TITLE" --arg head "$BRANCH" --arg base "$BASE" --arg body "$pr_body" \
      '{title: $title, head: $head, base: $base, body: $body}')"
  expect 201
  PR_NUMBER="$(jq -r .number <<<"$RESPONSE")"
  PR_URL="$(jq -r .html_url <<<"$RESPONSE")"
  echo "Opened ${PR_URL}"
else
  echo "Reusing open pull request ${PR_URL}"
fi


# --- hand off --------------------------------------------------------------------------

# Nothing went wrong, so this is a notice, not a warning. The script never
# merges: a person does, from the link.
msg="Homebrew: ${PR_URL} is waiting for a manual merge to publish grove ${VERSION} to the tap."
echo "::notice::$msg"
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  echo "$msg" >> "$GITHUB_STEP_SUMMARY"
fi
