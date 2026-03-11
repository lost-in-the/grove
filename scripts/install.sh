#!/bin/sh
# Grove installer — curl -fsSL https://grove.sh/install | sh
# Detects OS/arch, installs via Homebrew or GitHub release, runs grove setup.
set -e

REPO="lost-in-the/grove"

# ─── Helpers ──────────────────────────────────────────────────────────────────

info()  { printf "\033[1;34m=>\033[0m %s\n" "$*"; }
ok()    { printf "\033[1;32m✓\033[0m  %s\n" "$*"; }
warn()  { printf "\033[1;33m!\033[0m  %s\n" "$*" >&2; }
fail()  { printf "\033[1;31m✗\033[0m  %s\n" "$*" >&2; exit 1; }

need() {
    command -v "$1" >/dev/null 2>&1 || fail "Required command not found: $1"
}

# ─── Detect platform ─────────────────────────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       fail "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             fail "Unsupported architecture: $(uname -m)" ;;
    esac
}

# ─── Install methods ─────────────────────────────────────────────────────────

install_brew() {
    info "Installing via Homebrew..."
    brew tap "lost-in-the/tap" 2>/dev/null || true
    brew install grove
    ok "Installed grove via Homebrew"
}

install_release() {
    need curl
    need tar

    OS=$(detect_os)
    ARCH=$(detect_arch)

    info "Detecting latest release..."
    TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | head -1 | cut -d'"' -f4)

    if [ -z "$TAG" ]; then
        fail "Could not determine latest release"
    fi

    VERSION="${TAG#v}"
    ARCHIVE="grove_${VERSION}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading grove ${TAG} for ${OS}/${ARCH}..."
    curl -fsSL "$URL" -o "${TMPDIR}/${ARCHIVE}"

    info "Extracting..."
    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

    # Install to /usr/local/bin or ~/.local/bin
    INSTALL_DIR="/usr/local/bin"
    if [ ! -w "$INSTALL_DIR" ]; then
        INSTALL_DIR="${HOME}/.local/bin"
        mkdir -p "$INSTALL_DIR"
        warn "Installing to ${INSTALL_DIR} (add to PATH if needed)"
    fi

    cp "${TMPDIR}/grove" "${INSTALL_DIR}/grove"
    chmod +x "${INSTALL_DIR}/grove"
    ok "Installed grove ${TAG} to ${INSTALL_DIR}/grove"
}

# ─── Main ─────────────────────────────────────────────────────────────────────

main() {
    info "Installing grove — worktree flow manager"
    echo

    # Prefer Homebrew on macOS
    if command -v brew >/dev/null 2>&1; then
        install_brew
    else
        install_release
    fi

    echo

    # Run setup if possible
    if command -v grove >/dev/null 2>&1; then
        info "Setting up shell integration..."
        grove setup
    else
        warn "grove not found in PATH — add its install directory to PATH, then run: grove setup"
    fi

    echo
    ok "Done! Restart your shell or run: source ~/.zshrc"
    info "Get started: grove init && grove new my-feature"
}

main "$@"
