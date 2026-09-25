#!/usr/bin/env bash
# validate-herdr.sh — End-to-end validation of grove's herdr backend against a
# live herdr server.
#
# Usage: scripts/validate-herdr.sh [--keep] [--plugin] [--stop-server]
#   --keep          leave the scratch project and herdr workspaces behind
#   --plugin        also link integrations/herdr and exercise its hooks; the
#                   herdr server's PATH must resolve `grove` to this build
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
# never touches workspaces, worktrees, or panes belonging to the user. The
# named-session checks start a session of their own (grove-validate-<pid>) and
# stop and delete it afterwards.
#
# --plugin links the checkout's plugin into herdr's registry for the run and
# unlinks it after. It refuses to run if lost-in-the.grove is already
# installed, rather than replace the user's copy.
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
PLUGIN=0
for arg in "$@"; do
  case "$arg" in
    --keep)        KEEP=1 ;;
    --plugin)      PLUGIN=1 ;;
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
# lives inside $LAB. An optional argument names the session to look in.
#
# Never sweep `workspace list` wholesale. A herdr server is shared — the user's
# own workspaces, their running agents, and the pane this script is executing in
# all live in that same list, and closing them is not recoverable. Both sides
# are realpath-resolved because macOS hands out $TMPDIR under /var/folders while
# herdr reports the /private/var form.
lab_workspace_ids() {
  local sess=() ; [ -n "${1:-}" ] && sess=(--session "$1")
  herdr "${sess[@]}" workspace list 2>/dev/null | LAB="$LAB" python3 -c '
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

# Closes this script's workspaces — in the ambient session and in every other
# running one, since a checkout under $LAB can be open in several (the
# named-session checks open one on purpose; an interrupted run may leave one
# behind, and grove would rightly adopt it on the next run).
close_lab_workspaces() {
  local sess
  for sess in "" $(herdr session list --json 2>/dev/null | python3 -c '
import json, sys
try:
    print(" ".join(s["name"] for s in json.load(sys.stdin)["sessions"] if s["running"]))
except Exception:
    pass
'); do
    lab_workspace_ids "$sess" | while read -r id; do
      if [ -n "$sess" ]; then
        [ -n "$id" ] && herdr --session "$sess" workspace close "$id" >/dev/null 2>&1
      else
        [ -n "$id" ] && herdr workspace close "$id" >/dev/null 2>&1
      fi
    done
  done
}

# ws_field SESSION CHECKOUT FIELD — one field of the workspace whose checkout
# is CHECKOUT, in SESSION ("" for the ambient one). FIELD is a workspace key
# (workspace_id, label, focused) or "tokens" (as JSON). Empty when none.
ws_field() {
  local sess=() ; [ -n "$1" ] && sess=(--session "$1")
  herdr "${sess[@]}" workspace list 2>/dev/null | WANT="$2" FIELD="$3" python3 -c '
import json, os, sys
want = os.path.realpath(os.environ["WANT"])
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for w in d.get("result", {}).get("workspaces", []):
    path = (w.get("worktree") or {}).get("checkout_path")
    if path and os.path.realpath(path) == want:
        v = w.get(os.environ["FIELD"])
        print(json.dumps(v or {}) if os.environ["FIELD"] == "tokens" else v)
        break
'
}

# ws_count SESSION CHECKOUT — how many workspaces SESSION has for CHECKOUT.
ws_count() {
  local sess=() ; [ -n "$1" ] && sess=(--session "$1")
  herdr "${sess[@]}" workspace list 2>/dev/null | WANT="$2" python3 -c '
import json, os, sys
want = os.path.realpath(os.environ["WANT"])
try:
    d = json.load(sys.stdin)
except Exception:
    print(0); sys.exit(0)
print(sum(1 for w in d.get("result", {}).get("workspaces", [])
          if (w.get("worktree") or {}).get("checkout_path")
          and os.path.realpath(w["worktree"]["checkout_path"]) == want))
'
}

# root_pane WORKSPACE_ID — the workspace's first pane.
root_pane() {
  herdr pane list --workspace "$1" 2>/dev/null | python3 -c '
import json, sys
try:
    print(json.load(sys.stdin)["result"]["panes"][0]["pane_id"])
except Exception:
    pass
'
}

# pane_cwd PANE_ID — the pane's foreground working directory, resolved.
pane_cwd() {
  herdr pane get "$1" 2>/dev/null | python3 -c '
import json, os, sys
try:
    p = json.load(sys.stdin)["result"]["pane"]
    print(os.path.realpath(p.get("foreground_cwd") or p.get("cwd") or ""))
except Exception:
    pass
'
}

# state_has NAME — whether the scratch project's grove state tracks NAME.
state_has() {
  NAME="$1" python3 -c '
import json, os, sys
s = json.load(open(".grove/state.json"))
sys.exit(0 if os.environ["NAME"] in s.get("worktrees", {}) else 1)
'
}

# wait_for DESCRIPTION CMD... — poll CMD (up to ~5s): plugin hooks and pane
# shells run asynchronously. CMD runs in this shell, so it may be a function.
wait_for() {
  local what="$1"; shift
  for _ in $(seq 1 50); do
    if "$@"; then ok "$what"; return 0; fi
    sleep 0.1
  done
  bad "$what"
  return 1
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
# means the worktree still resolves to a live session — active (herdr's
# server-wide focused workspace) or open is focus's business, not identity's,
# so asserting a specific one of the two makes the check flap with focus.
WS=$(wslist | grep '|demo-alpha|' | cut -d'|' -f1 | head -n1)
herdr workspace rename "$WS" "renamed-by-the-user" >/dev/null 2>&1
S=$("$GROVE" ls | awk '$1=="alpha"{print $4}')
case "$S" in
  active|open) ok "relabelling a workspace does not desync grove" ;;
  *) bad "relabelling a workspace does not desync grove" "got [$S] want [active|open]" ;;
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
# status proves it; active-vs-open only reflects where focus happens to sit
# (see the relabelling check above).
S=$("$GROVE" ls | awk '$1=="renamed"{print $4}')
case "$S" in
  active|open) ok "renamed worktree still resolves" ;;
  *) bad "renamed worktree still resolves" "got [$S] want [active|open]" ;;
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

sect "grove open with a session command"
# `pane run` prints nothing on success; grove once took that silence for a
# failure and reported "failed to create session" after the command had run.
cp .grove/config.toml "$LAB/config.toml.orig"
printf '\n[session]\ncommand = "echo grove-open-ok"\n' >> .grove/config.toml
git add -A && git commit -qm "session command" >/dev/null 2>&1
OUT=$(GROVE_SHELL=1 timeout 20 "$GROVE" open cmdcheck 2>&1)
check "grove open exits cleanly" "$?" "0"
if printf '%s' "$OUT" | grep -q "running 'echo grove-open-ok'"; then
  ok "grove open reports the command it started"
else
  bad "grove open reports the command it started" "$OUT"
fi
CMD_WS=$(ws_field "" "$LAB/demo-cmdcheck" workspace_id)
CMD_PANE=$(root_pane "$CMD_WS")
command_ran() { herdr pane read "$CMD_PANE" --source recent 2>/dev/null | grep -q '^grove-open-ok'; }
wait_for "the session command ran in the new pane" command_ran
cp "$LAB/config.toml.orig" .grove/config.toml
git add -A && git commit -qm "drop session command" >/dev/null 2>&1

sect "directory drift correction"
# `pane process-info` nests its process list under process_info; grove once
# read an always-empty list, so no pane ever looked like a shell and drift was
# never corrected under herdr.
CMD_REAL=$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$LAB/demo-cmdcheck")
drifted()   { [ "$(pane_cwd "$CMD_PANE")" = "/" ]; }
back_home() { [ "$(pane_cwd "$CMD_PANE")" = "$CMD_REAL" ]; }
herdr pane run "$CMD_PANE" "cd /" >/dev/null 2>&1
wait_for "the pane drifts away" drifted
HERDR_ENV=1 timeout 20 "$GROVE" to cmdcheck >/dev/null 2>&1
wait_for "grove to cds a drifted shell back to the worktree" back_home

sect "coding-agent state"
# `pane report-agent` stands in for a real agent. Only observed states render.
herdr pane report-agent --source grove-validate --agent claude --state blocked "$CMD_PANE" >/dev/null 2>&1
check "grove ls shows the blocked agent" "$("$GROVE" ls | awk '$1=="cmdcheck"{print $5}')" "blocked"
AGENT_JSON=$("$GROVE" ls --json | python3 -c '
import json, sys
for w in json.load(sys.stdin)["worktrees"]:
    if w["name"] == "cmdcheck":
        print(w.get("agent", ""))
')
check "grove ls --json carries the agent state" "$AGENT_JSON" "blocked"
herdr pane report-agent --source grove-validate --agent claude --state unknown "$CMD_PANE" >/dev/null 2>&1

sect "repository workspace"
# herdr opens the repository's own checkout as the parent workspace it groups
# the worktrees under; `grove to root` must land there, not open another.
(cd "$LAB/demo-cmdcheck" && HERDR_ENV=1 timeout 20 "$GROVE" to root >/dev/null 2>&1)
check "grove to root succeeds" "$?" "0"
check "the repository has exactly one workspace" "$(ws_count "" "$DEMO")" "1"

sect "named sessions"
# herdr sessions are independent servers; `worktree open` checks only its own.
# grove must adopt a workspace another session already has rather than open a
# duplicate, and `grove rm` must close it wherever it is.
SESS="grove-validate-$$"
session_up() {
  herdr session list --json 2>/dev/null | SESS="$SESS" python3 -c '
import json, os, sys
d = json.load(sys.stdin)
sys.exit(0 if any(s["name"] == os.environ["SESS"] and s["running"] for s in d["sessions"]) else 1)
'
}
( herdr --session "$SESS" server >"$LAB/session.log" 2>&1 & )
wait_for "a second named session starts" session_up
HERDR_SESSION="$SESS" "$GROVE" new gamma >/dev/null 2>&1
check "the worktree opens in the other session" "$(ws_count "$SESS" "$LAB/demo-gamma")" "1"
OUT=$(GROVE_SHELL=1 timeout 20 "$GROVE" to gamma 2>&1)
if printf '%s' "$OUT" | grep -q "herdr --session $SESS"; then
  ok "outside herdr, grove to attaches the session that has it"
else
  bad "outside herdr, grove to attaches the session that has it" "$OUT"
fi
check "no duplicate opens in the ambient session" "$(ws_count "" "$LAB/demo-gamma")" "0"
AMBIENT_SOCK=$(herdr status server --json 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin).get("socket") or "")')
OUT=$(HERDR_ENV=1 HERDR_SOCKET_PATH="$AMBIENT_SOCK" GROVE_SHELL=1 timeout 20 "$GROVE" to gamma 2>&1)
if printf '%s' "$OUT" | grep -q "already open in herdr session"; then
  ok "inside another session, grove to says where it is open"
else
  bad "inside another session, grove to says where it is open" "$OUT"
fi
check "still no duplicate in the ambient session" "$(ws_count "" "$LAB/demo-gamma")" "0"
"$GROVE" rm gamma --force >/dev/null 2>&1
check "grove rm closes the workspace in the other session" "$(ws_count "$SESS" "$LAB/demo-gamma")" "0"
herdr session stop "$SESS" >/dev/null 2>&1
herdr session delete "$SESS" >/dev/null 2>&1

sect "plugin hooks"
if [ "$PLUGIN" -eq 0 ]; then
  printf '  \033[33mSKIP\033[0m  plugin checks — pass --plugin to link integrations/herdr for the run.\n'
elif herdr plugin list 2>/dev/null | grep -q 'lost-in-the.grove'; then
  printf '  \033[33mSKIP\033[0m  lost-in-the.grove is already installed; not replacing it.\n'
else
  herdr plugin link "$PROJECT_DIR/integrations/herdr" >/dev/null 2>&1
  # A worktree made by herdr itself, as its UI does — kept inside $LAB.
  herdr worktree create --cwd "$DEMO" --branch herdr-made --path "$LAB/demo-herdr-made" --no-focus >/dev/null 2>&1
  marked() { [ "$(ws_field "" "$LAB/demo-herdr-made" tokens)" = '{"grove": "untracked"}' ]; }
  wait_for "a herdr-made worktree gets the untracked marker" marked
  "$GROVE" adopt "$LAB/demo-herdr-made" >/dev/null 2>&1
  check "grove adopt applies the canonical label" "$(ws_field "" "$LAB/demo-herdr-made" label)" "demo-herdr-made"
  check "grove adopt clears the marker" "$(ws_field "" "$LAB/demo-herdr-made" tokens)" "{}"
  HM_WS=$(ws_field "" "$LAB/demo-herdr-made" workspace_id)
  herdr worktree remove --workspace "$HM_WS" --force >/dev/null 2>&1
  untracked_again() { ! state_has herdr-made; }
  wait_for "removing it through herdr drops grove's state entry" untracked_again
  herdr plugin unlink lost-in-the.grove >/dev/null 2>&1
fi

sect "degradation with the server stopped"
if [ "$STOP_SERVER" -eq 0 ]; then
  printf '  \033[33mSKIP\033[0m  2 checks — stopping the server would kill every pane it hosts.\n'
  printf '        Re-run with --stop-server against a server you are not working in.\n'
else
# Close this run's workspaces first: herdr persists and restores its session
# across a restart, and a restored workspace whose checkout is gone keeps its
# label — which grove's rename fallback would match on the next run.
close_lab_workspaces
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
