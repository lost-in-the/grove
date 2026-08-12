#!/usr/bin/env bash
# validate-herdr.sh — End-to-end validation of grove's herdr backend against a
# live herdr server.
#
# Usage: scripts/validate-herdr.sh [--keep] [--stop-server]
#   --keep          leave the scratch project and herdr workspaces behind
#   --stop-server   also run the degradation checks, which STOP the herdr server
#
# Covers what unit tests can't: that grove and herdr actually agree about
# worktree identity, that the ownership boundary holds (grove owns worktree
# lifecycle, herdr only ever adopts), and that a stopped server degrades instead
# of hanging. See docs/HERDR_INTEGRATION.md.
#
# Requires: herdr on PATH, git. Builds grove from the current checkout.
#
# Safe against a live server by default. Every test runs in a throwaway repo
# under $TMPDIR, and the only workspaces this script ever closes are the ones
# whose checkout lives inside that directory — see close_lab_workspaces. It
# never touches workspaces, worktrees, or panes belonging to the user.
#
# The two degradation checks are the exception: they stop the herdr server,
# which kills every pane it is running. They are therefore opt-in behind
# --stop-server. Do not pass it against a server you are working in.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
LAB="${TMPDIR:-/tmp}/grove-herdr-validation"
DEMO="$LAB/demo"
GROVE="$LAB/grove"
KEEP=0
STOP_SERVER=0
for arg in "$@"; do
  case "$arg" in
    --keep)        KEEP=1 ;;
    --stop-server) STOP_SERVER=1 ;;
    *) printf 'unknown option: %s\n' "$arg" >&2; exit 2 ;;
  esac
done

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

# Emits "id|label|checkout_path|focused" — ONLY for workspaces whose checkout
# resolves inside $LAB. Assertions must never match by label across the whole
# server: a live server carries the user's own workspaces, including debris
# from older runs (e.g. a dead "demo-alpha" with no checkout at all), and a
# bare `grep demo-` counts those too. The same lab-scoping rule the cleanup
# path follows applies to every count and every id this script derives.
#
# A workspace whose recorded checkout went stale still matches as long as the
# dead path is under $LAB — which is exactly the case after `grove rename`,
# where herdr keeps the pre-rename path.
wslist() {
  herdr workspace list 2>/dev/null | LAB="$LAB" python3 -c '
import json, os, sys
lab = os.path.realpath(os.environ["LAB"])
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for w in d.get("result", {}).get("workspaces", []):
    path = (w.get("worktree") or {}).get("checkout_path")
    if not path:
        continue
    real = os.path.realpath(path)
    if real != lab and not real.startswith(lab + os.sep):
        continue
    print(w["workspace_id"], w["label"], path, w["focused"], sep="|")
'
}

# Emits the workspace ids this script is responsible for: those whose checkout
# lives inside $LAB.
#
# Never sweep `workspace list` wholesale. A herdr server is shared — the user's
# own workspaces, their running agents, and the pane this script is executing in
# all live in that same list, and closing them is not recoverable. Both sides
# are realpath-resolved because macOS hands out $TMPDIR under /var/folders while
# herdr reports the /private/var form.
lab_workspace_ids() {
  herdr workspace list 2>/dev/null | LAB="$LAB" python3 -c '
import json, os, sys
lab = os.path.realpath(os.environ["LAB"])
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for w in d.get("result", {}).get("workspaces", []):
    path = (w.get("worktree") or {}).get("checkout_path")
    if not path:
        continue
    real = os.path.realpath(path)
    if real == lab or real.startswith(lab + os.sep):
        print(w["workspace_id"])
'
}

close_lab_workspaces() {
  lab_workspace_ids | while read -r id; do
    [ -n "$id" ] && herdr workspace close "$id" >/dev/null 2>&1
  done
}

# --- scratch project ---------------------------------------------------------

close_lab_workspaces
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
# path, so relabelling behind grove's back must not desync it. "Not desynced"
# means the worktree still resolves to a live session — attached or detached
# is the client's business (a live GUI focuses freshly opened workspaces; the
# headless server this script was first verified against focuses nothing), so
# asserting a specific one of the two makes the check flap with focus.
WS=$(wslist | grep '|demo-alpha|' | cut -d'|' -f1 | head -n1)
herdr workspace rename "$WS" "renamed-by-the-user" >/dev/null 2>&1
S=$("$GROVE" ls | awk '$1=="alpha"{print $4}')
case "$S" in
  attached|detached) ok "relabelling a workspace does not desync grove" ;;
  *) bad "relabelling a workspace does not desync grove" "got [$S] want [attached|detached]" ;;
esac
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
# fallback instead. That self-healing is the thing being asserted — any live
# status proves it; attached-vs-detached only reflects where the client's
# focus happens to sit (see the relabelling check above).
S=$("$GROVE" ls | awk '$1=="renamed"{print $4}')
case "$S" in
  attached|detached) ok "renamed worktree still resolves" ;;
  *) bad "renamed worktree still resolves" "got [$S] want [attached|detached]" ;;
esac

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
if [ "$STOP_SERVER" -eq 0 ]; then
  printf '  \033[33mSKIP\033[0m  2 checks — stopping the server would kill every pane it hosts.\n'
  printf '        Re-run with --stop-server against a server you are not working in.\n'
else
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
fi

# --- summary -----------------------------------------------------------------

printf '\n\033[1mpassed: %d   failed: %d\033[0m\n' "$PASS" "$FAIL"

if [ "$KEEP" -eq 1 ]; then
  printf 'scratch project kept at %s\n' "$DEMO"
else
  # Close the workspaces this run opened before deleting the checkouts they
  # point at, so the server is left exactly as it was found. Pointless once the
  # server is down, and lab_workspace_ids would just come back empty.
  [ "$STOP_SERVER" -eq 0 ] && close_lab_workspaces
  cd "$PROJECT_DIR" || true
  rm -rf "$LAB"
fi

if [ "$STOP_SERVER" -eq 1 ]; then
  printf 'note: the herdr server was stopped by the last check — restart it with `herdr server`\n'
fi

[ "$FAIL" -eq 0 ]
