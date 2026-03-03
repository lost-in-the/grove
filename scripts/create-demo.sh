#!/usr/bin/env bash
# create-demo.sh — Set up demo fixture for VHS tape recordings.
# Creates a demo-ready directory at /tmp/grove-demo with the built grove binary.
#
# Idempotent: delegates to create-fixture.sh which handles cleanup.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DEMO_DIR="/tmp/grove-demo"
BINARY="$PROJECT_DIR/bin/grove"

if [ ! -f "$BINARY" ]; then
  echo "Error: binary not found at $BINARY"
  echo "Run 'make build' first."
  exit 1
fi

echo "Creating demo fixture at $DEMO_DIR..."
"$SCRIPT_DIR/create-fixture.sh" "$DEMO_DIR"

echo "Installing grove binary into demo fixture..."
mkdir -p "$DEMO_DIR/rails-app/bin"
cp "$BINARY" "$DEMO_DIR/rails-app/bin/grove"

echo ""
echo "Demo ready at: $DEMO_DIR/rails-app"
echo "Binary: $DEMO_DIR/rails-app/bin/grove"
