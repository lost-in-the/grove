#!/usr/bin/env bash
# validate-herdr.sh — End-to-end validation of grove's herdr backend against a
# live herdr server.
#
# Usage: scripts/validate-herdr.sh [--keep]
#   --keep   leave the scratch project and herdr workspaces behind for poking at
#
# Covers what unit tests can't: that grove and herdr actually agree about
# worktree identity, that the ownership boundary holds (grove owns worktree
# lifecycle, herdr only ever adopts), and that a stopped server degrades instead
# of hanging. See docs/HERDR_INTEGRATION.md.
#
# Requires: herdr on PATH, git. Builds grove from the current checkout.
#
# Safe by construction: every test runs in a throwaway repo under $TMPDIR, and
# the script never touches worktrees outside it. It does stop the herdr server
# as its final check, so don't run it against a server you're using.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
LAB="${TMPDIR:-/tmp}/grove-herdr-validation"
DEMO="$LAB/demo"
GROVE="$LAB/grove"
KEEP=0
[ "${1:-}" = "--keep" ] && KEEP=1

PASS=0
FAIL=0

ok()   { printf '  \033[32mPASS\033[0m  %s\n' "$1"; PASS=$((PASS + 1)); }
bad()  { printf '  \033[31mFAIL\033[0m  %s\n' "$1"; [ -n "${2:-}" ] && printf '        %s\n' "$2"; FAIL=$((FAIL + 1)); }
sect() { printf '\n\033[1m%s\033[0m\n' "$1"; }
check(){ if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "got [$2] want [$3]"; fi; }

die() { printf '\033[31mError:\033[0m %s\n' "$1" >&2; exit 1; }

# --- preflight ---------------------------------------------------------------

command -v herdr >/dev/null 2>&1 || die "herdr not found in PATH — install it from https://herdr.dev"
command -v git   >/dev/null 2>&1 || die "git not found in PATH"

# `herdr status server` exits 0 whether or not a server is up — it succeeded at
# reporting the status — so the state has to come from its output.
if ! herdr status server 2>&1 | grep -q '^status: running'; then
  die "no herdr server running — start one with 'herdr server' (headless) or 'herdr'"
fi

printf 'grove:  building from %s\n' "$PROJECT_DIR"
mkdir -p "$LAB"
(cd "$PROJECT_DIR" && go build -o "$GROVE" ./cmd/grove) || die "go build failed"
printf 'herdr:  %s\n' "$(herdr --version 2>&1)"

# Emits "id|label|checkout_path|focused" per workspace.
wslist() {
  herdr workspace list 2>/dev/null | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for w in d.get("result", {}).get("workspaces", []):
    wt = w.get("worktree") or {}
    print(w["workspace_id"], w["label"], wt.get("checkout_path", "-"), w["focused"], sep="|")
'
}

close_all_workspaces() {
  wslist | cut -d'|' -f1 | while read -r id; do
    [ -n "$id" ] && herdr workspace close "$id" >/dev/null 2>&1
  done
}

# --- scratch project ---------------------------------------------------------

close_all_workspaces
rm -rf "$DEMO" "$LAB"/demo-*
mkdir -p "$DEMO"
cd "$DEMO" || die "cannot enter $DEMO"

git init -q -b main
git config user.email validate@example.com
git config user.name "Herdr Validation"
echo "scratch" > README.md
git add -A && git commit -qm "init"

"$GROVE" init >/dev/null 2>&1 || die "grove init failed"
printf '[mux]\nbackend = "herdr"\n' >> .grove/config.toml
git add -A && git commit -qm "grove config" >/dev/null 2>&1

export GROVE_NO_COLOR=1

# --- checks ------------------------------------------------------------------

sect "backend resolution"
BACKEND=$("$GROVE" config 2>/dev/null | awk '/\[mux\]/{f=1;next} f&&/backend:/{print $2; exit}')
check "config reports the herdr backend" "$BACKEND" "herdr"

sect "create"
OUT=$("$GROVE" new alpha 2>&1)
if printf '%s' "$OUT" | grep -q "Created herdr session 'demo-alpha'"; then
  ok "grove new adopts the checkout as a herdr workspace"
else
  bad "grove new adopts the checkout as a herdr workspace" "$OUT"
fi

"$GROVE" new beta >/dev/null 2>&1
check "both worktrees have workspaces" "$(wslist | grep -c 'demo-')" "2"

sect "identity is the checkout path, not the label"
# herdr labels are cosmetic and user-renameable; grove keys on the checkout
# path, so relabelling behind grove's back must not desync it.
WS=$(wslist | grep '|demo-alpha|' | cut -d'|' -f1)
herdr workspace rename "$WS" "renamed-by-the-user" >/dev/null 2>&1
check "relabelling a workspace does not desync grove" \
  "$("$GROVE" ls | awk '$1=="alpha"{print $4}')" "detached"
herdr workspace rename "$WS" "demo-alpha" >/dev/null 2>&1

sect "idempotency"
# `herdr worktree open` reuses an existing workspace for the same checkout.
"$GROVE" new alpha >/dev/null 2>&1
check "re-creating does not duplicate the workspace" "$(wslist | grep -c 'demo-alpha')" "1"

sect "rename"
"$GROVE" rename beta renamed >/dev/null 2>&1
if wslist | grep -q 'demo-renamed'; then
  ok "grove rename relabels the workspace"
else
  bad "grove rename relabels the workspace" "$(wslist)"
fi
# herdr's checkout provenance goes stale here; grove resolves via the name
# fallback instead. That self-healing is the thing being asserted.
check "renamed worktree still resolves" \
  "$("$GROVE" ls | awk '$1=="renamed"{print $4}')" "detached"

sect "switch"
HERDR_ENV=1 timeout 20 "$GROVE" to renamed >/dev/null 2>&1
check "grove to focuses the workspace" "$(wslist | grep 'demo-renamed' | cut -d'|' -f4)" "True"

sect "remove"
"$GROVE" rm renamed --force >/dev/null 2>&1
if wslist | grep -q 'demo-renamed'; then
  bad "grove rm closes the workspace" "$(wslist)"
else
  ok "grove rm closes the workspace"
fi
if [ -d "$LAB/demo-renamed" ]; then
  bad "grove rm removed the checkout" "$LAB/demo-renamed still present"
else
  ok "grove rm removed the checkout"
fi
if git -C "$DEMO" worktree list | grep -q 'demo-renamed'; then
  bad "git worktree deregistered" "still registered"
else
  ok "git worktree deregistered"
fi

sect "ownership boundary"
# Adopting must never create a checkout — grove owns worktree lifecycle.
git -C "$DEMO" worktree add -q -b outside "$LAB/demo-outside" 2>/dev/null
BEFORE=$(git -C "$DEMO" worktree list | wc -l | tr -d ' ')
herdr worktree open --cwd "$DEMO" --path "$LAB/demo-outside" --label demo-outside --no-focus >/dev/null 2>&1
AFTER=$(git -C "$DEMO" worktree list | wc -l | tr -d ' ')
check "adopting a checkout creates no new git worktree" "$BEFORE" "$AFTER"

sect "degradation with the server stopped"
herdr server stop >/dev/null 2>&1
sleep 1
DOWN="$LAB/down.txt"
timeout 10 "$GROVE" ls > "$DOWN" 2>&1
check "grove ls succeeds with no server" "$?" "0"
if grep -q "none" "$DOWN"; then
  ok "sessions report none rather than erroring"
else
  bad "sessions report none rather than erroring" "$(cat "$DOWN")"
fi
# Capture first rather than piping: `grove doctor` exits non-zero when checks
# fail — which is the whole point here — and under pipefail that would sink the
# pipeline even when grep matched.
DOCTOR_OUT=$(timeout 30 "$GROVE" doctor 2>&1)
if printf '%s' "$DOCTOR_OUT" | grep -q "Herdr server"; then
  ok "doctor flags the stopped server"
else
  bad "doctor flags the stopped server" "no 'Herdr server' line in doctor output"
fi

# --- summary -----------------------------------------------------------------

printf '\n\033[1mpassed: %d   failed: %d\033[0m\n' "$PASS" "$FAIL"

if [ "$KEEP" -eq 1 ]; then
  printf 'scratch project kept at %s\n' "$DEMO"
else
  cd "$PROJECT_DIR" || true
  rm -rf "$LAB"
fi

printf 'note: the herdr server was stopped by the last check — restart it with `herdr server`\n'

[ "$FAIL" -eq 0 ]
