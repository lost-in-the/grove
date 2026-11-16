#!/usr/bin/env bash
# test-update-homebrew-tap.sh — drive update-homebrew-tap.sh through each case
# against a fake GitHub API (scripts/testdata/fake_github_api.py).
#
# The release job that runs the real script only fires on a tag push, so a
# mistake there would first surface in a release. CI runs this on every PR.
#
# Requires: bash, curl, jq, python3.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$SCRIPT_DIR/update-homebrew-tap.sh"
TEMPLATE="$SCRIPT_DIR/../.github/formula.rb.tmpl"
WORK="$(mktemp -d)"
PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')"
API="http://127.0.0.1:$PORT"
export no_proxy="127.0.0.1,localhost" NO_PROXY="127.0.0.1,localhost"

python3 "$SCRIPT_DIR/testdata/fake_github_api.py" "$PORT" & FAKE=$!
trap 'kill $FAKE 2>/dev/null; rm -rf "$WORK"' EXIT
for _ in $(seq 1 50); do curl -fsS "$API/__state" >/dev/null 2>&1 && break; sleep 0.1; done

# A stand-in source tarball; only its digest matters.
echo "grove source" > "$WORK/src.txt"
tar -czf "$WORK/source.tar.gz" -C "$WORK" src.txt
SHA="$(sha256sum "$WORK/source.tar.gz" | awk '{print $1}')"
render() { sed -e "s/VERSION/$1/g" -e "s/SHA256/$2/g" "$TEMPLATE"; }
render 0.11.0 "$SHA" > "$WORK/new.rb"
render 0.10.0 "$(printf '%064d' 0)" > "$WORK/old.rb"
NEW="$(cat "$WORK/new.rb")"
OLD="$(cat "$WORK/old.rb")"

PASS=0; FAIL=0
ok()  { echo "  PASS $1"; PASS=$((PASS+1)); }
bad() { echo "  FAIL $1"; [ -n "${2:-}" ] && echo "$2" | sed 's/^/       /'; FAIL=$((FAIL+1)); }
reset() { curl -fsS -X POST "$API/__reset" -d "$1" >/dev/null; }
st() { curl -fsS "$API/__state" | jq -r "$1"; }
cfg() { jq -n --rawfile f "$1" "{main_formula: \$f} + $2"; }
run() {
  TAP_GITHUB_TOKEN=t GITHUB_API="$API" TARBALL_URL="file://$WORK/source.tar.gz" \
    "$SCRIPT" v0.11.0 >"$WORK/out.txt" 2>&1
  echo $?
}
out() { cat "$WORK/out.txt"; }

# The script never merges. Count the requests that would: the GraphQL
# auto-merge mutation and the direct merge.
merge_calls() { st '[.calls[] | select(test("graphql|/merge$"))] | length'; }
PR_URL="https://github.com/lost-in-the/homebrew-tap/pull/6"
unset TAP_MERGE
export GITHUB_STEP_SUMMARY="$WORK/summary.txt"

echo "1. fresh release: the pull request is opened and left for a person"
: > "$WORK/summary.txt"
reset "$(cfg "$WORK/old.rb" '{}')"
rc=$(run)
[ "$rc" = 0 ] && ok "exits 0" || bad "exits 0 (got $rc)" "$(out)"
[ "$(st '.files["grove-0.11.0"]')" = "$NEW" ] && ok "branch carries the rendered formula" || bad "branch formula"
[ "$(st '.files.main')" = "$OLD" ] && ok "main untouched (no direct write)" || bad "main was written"
[ "$(st '.pulls | length')" = 1 ] && ok "one pull request" || bad "pull requests: $(st '.pulls | length')"
[ "$(st '.pulls[0].title')" = "grove 0.11.0" ] && ok "titled 'grove 0.11.0'" || bad "title"
[ "$(merge_calls)" = 0 ] && ok "no auto-merge or merge request reached the API" || bad "merge calls: $(st '.calls')"
grep -q "^::notice::.*${PR_URL}.*manual merge" "$WORK/out.txt" && ok "notice names the pull request and the manual merge" || bad "notice" "$(out)"
grep -q "::warning::" "$WORK/out.txt" && bad "must not warn: nothing went wrong" "$(out)" || ok "no warning"
grep -q "${PR_URL}.*manual merge" "$WORK/summary.txt" && ok "step summary says the same" || bad "summary" "$(cat "$WORK/summary.txt")"

echo "2. re-run with the branch and pull request already there"
before_puts=$(st '[.calls[] | select(startswith("PUT"))] | length')
rc=$(run)
[ "$rc" = 0 ] && ok "exits 0" || bad "exits 0 (got $rc)" "$(out)"
[ "$(st '[.calls[] | select(startswith("PUT"))] | length')" = "$before_puts" ] && ok "no second commit" || bad "committed again"
[ "$(st '.pulls | length')" = 1 ] && ok "pull request reused, not duplicated" || bad "duplicated PR"
grep -q "Reusing branch" "$WORK/out.txt" && grep -q "Reusing open pull request ${PR_URL}" "$WORK/out.txt" && ok "reports the reuse" || bad "reuse message" "$(out)"
[ "$(merge_calls)" = 0 ] && ok "still no merge request" || bad "merge calls: $(st '.calls')"

echo "3. tap already at this formula"
reset "$(cfg "$WORK/new.rb" '{}')"
rc=$(run)
[ "$rc" = 0 ] && ok "exits 0" || bad "exits 0 (got $rc)" "$(out)"
[ "$(st '[.calls[] | select(startswith("POST"))] | length')" = 0 ] && ok "no branch or pull request calls" || bad "made calls: $(st '.calls')"
grep -q "nothing to do" "$WORK/out.txt" && ok "says nothing to do" || bad "message" "$(out)"

echo "4. TAP_MERGE=merge in the environment is ignored"
: > "$WORK/summary.txt"
reset "$(cfg "$WORK/old.rb" '{}')"
export TAP_MERGE=merge
rc=$(run)
unset TAP_MERGE
[ "$rc" = 0 ] && ok "exits 0" || bad "exits 0 (got $rc)" "$(out)"
[ "$(st '.pulls | length')" = 1 ] && ok "pull request opened" || bad "pull requests: $(st '.pulls | length')"
[ "$(merge_calls)" = 0 ] && ok "no merge request reached the API" || bad "merge calls: $(st '.calls')"
[ "$(st '.files.main')" = "$OLD" ] && ok "main untouched" || bad "main changed"
grep -q "^::notice::.*manual merge" "$WORK/out.txt" && ok "still asks for a manual merge" || bad "notice" "$(out)"

echo "5. token cannot write"
reset "$(cfg "$WORK/old.rb" '{put_status: 403}')"
rc=$(run)
[ "$rc" != 0 ] && ok "fails" || bad "should fail"
grep -q "::error::unexpected HTTP 403" "$WORK/out.txt" && ok "names the HTTP status" || bad "error" "$(out)"
[ "$(st '.pulls | length')" = 0 ] && ok "opens no PR for an empty branch" || bad "opened a PR"

echo "6. reading the tap's formula fails (must not be taken for 'no file')"
reset "$(cfg "$WORK/old.rb" '{contents_status: 500}')"
rc=$(run)
[ "$rc" != 0 ] && ok "fails" || bad "should fail" "$(out)"
[ "$(st '[.calls[] | select(startswith("POST"))] | length')" = 0 ] && ok "goes no further" || bad "carried on: $(st '.calls')"

echo "7. source tarball missing"
reset "$(cfg "$WORK/old.rb" '{}')"
TAP_GITHUB_TOKEN=t GITHUB_API="$API" TARBALL_URL="file://$WORK/nope.tar.gz" "$SCRIPT" v0.11.0 >"$WORK/out.txt" 2>&1; rc=$?
[ "$rc" != 0 ] && grep -q "could not download the source tarball" "$WORK/out.txt" && ok "fails before touching the tap" || bad "tarball failure" "$(out)"
[ "$(st '.calls | length')" = 0 ] && ok "no API calls" || bad "calls: $(st '.calls')"

echo "8. a tag with build metadata is refused before anything is read or written"
reset "$(cfg "$WORK/old.rb" '{}')"
TAP_GITHUB_TOKEN=t GITHUB_API="$API" TARBALL_URL="file://$WORK/source.tar.gz" "$SCRIPT" v0.11.0+build.1 >"$WORK/out.txt" 2>&1; rc=$?
[ "$rc" != 0 ] && ok "fails" || bad "should fail" "$(out)"
grep -q "::error::unsupported release tag 'v0.11.0+build.1'" "$WORK/out.txt" && ok "names the tag" || bad "error" "$(out)"
[ "$(st '.calls | length')" = 0 ] && ok "no API calls" || bad "calls: $(st '.calls')"

echo
echo "passed: $PASS  failed: $FAIL"
[ "$FAIL" = 0 ]
