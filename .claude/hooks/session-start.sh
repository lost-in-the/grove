#!/bin/bash
# SessionStart hook for Claude Code on the web.
#
# The web container images ship a Go older than the version go.mod requires,
# so GOTOOLCHAIN=auto downloads the required toolchain into the module cache.
# That download is the slim distribution: pkg/tool contains only the core
# build tools. `go tool <x>` builds omitted tools on demand, but
# `go test -cover` (what `make test` runs) resolves covdata directly in
# pkg/tool and fails with `go: no such tool "covdata"` for packages that have
# no test files. This hook materializes the missing tools from the
# toolchain's bundled source so `make test` works out of the box.
set -euo pipefail

# Local sessions use a full Go install — nothing to do.
if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

cd "$CLAUDE_PROJECT_DIR"

# Warm the module cache; also triggers the toolchain auto-download so the
# GOROOT probed below is the one go.mod selects.
go mod download

GOROOT_ACTIVE="$(go env GOROOT)"
TOOLDIR="$GOROOT_ACTIVE/pkg/tool/$(go env GOOS)_$(go env GOARCH)"
# The toolchain version in use (e.g. go1.25.8). Pinning GOTOOLCHAIN to it for
# the builds below stops the GOROOT/src go.mod from pulling down yet another
# toolchain.
GOVER="$(go env GOVERSION)"

missing=()
for t in addr2line buildid covdata doc fix nm objdump pack pprof test2json trace; do
  [ -x "$TOOLDIR/$t" ] || missing+=("$t")
done

if [ "${#missing[@]}" -gt 0 ]; then
  echo "Building ${#missing[@]} missing Go tool(s) into $TOOLDIR: ${missing[*]}"
  # The module-cache toolchain is read-only; open it up just for the copy.
  chmod u+w "$TOOLDIR"
  trap 'chmod u-w "$TOOLDIR"' EXIT
  for t in "${missing[@]}"; do
    (cd "$GOROOT_ACTIVE/src" && GOTOOLCHAIN="$GOVER" go build -o "$TOOLDIR/$t" "cmd/$t")
  done
fi

# Fail loudly now (rather than mid-session) if the covdata path still breaks.
go test -cover -run TestNothingMatchesThisName ./internal/log/ > /dev/null
echo "Go toolchain ready: $GOVER ($(command -v golangci-lint > /dev/null && golangci-lint version --short 2> /dev/null || echo 'golangci-lint missing — make lint falls back to go vet'))"
