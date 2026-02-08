#!/usr/bin/env bash
# create-fixture.sh — Create a persistent test fixture for grove TUI testing.
# Usage: scripts/create-fixture.sh [path]
# Default path: /tmp/grove-test-fixture
#
# Idempotent: removes and recreates if the fixture already exists.

set -euo pipefail

FIXTURE_DIR="${1:-/tmp/grove-test-fixture}"
REPO_DIR="$FIXTURE_DIR/rails-app"

# Clean up existing fixture
if [ -d "$FIXTURE_DIR" ]; then
  echo "Removing existing fixture at $FIXTURE_DIR..."
  rm -rf "$FIXTURE_DIR"
fi

mkdir -p "$FIXTURE_DIR"

# Isolated git config
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME="Test User"
export GIT_AUTHOR_EMAIL="test@test.com"
export GIT_COMMITTER_NAME="Test User"
export GIT_COMMITTER_EMAIL="test@test.com"

# Create repo
mkdir -p "$REPO_DIR"
cd "$REPO_DIR"
git init -b main
git config commit.gpgsign false

# Directory structure
mkdir -p app/models app/controllers config/environments db/migrate lib test .grove/hooks

# Files
cat > .gitignore <<'GITEOF'
.env
config/master.key
tmp/
log/
node_modules/
GITEOF

cat > Gemfile <<'GEMEOF'
source 'https://rubygems.org'
gem 'rails', '~> 8.0'
GEMEOF

cat > README.md <<'MDEOF'
# Rails App
Test fixture for grove TUI testing.
MDEOF

cat > app/models/user.rb <<'RBEOF'
class User < ApplicationRecord
  validates :email, presence: true
end
RBEOF

cat > app/controllers/application_controller.rb <<'RBEOF'
class ApplicationController < ActionController::Base
end
RBEOF

cat > config/routes.rb <<'RBEOF'
Rails.application.routes.draw do
  root 'home#index'
end
RBEOF

cat > .grove/config.toml <<'TOMLEOF'
project_name = "rails-app"

[plugins.docker]
enabled = true
auto_start = false
auto_stop = false
TOMLEOF

echo '{"version":1,"project":"rails-app","worktrees":{}}' > .grove/state.json

git add -A
git commit -m "Initial commit: Rails 8 app scaffold"

# Add changelog
cat > CHANGELOG.md <<'MDEOF'
# Changelog
## 0.1.0
- Initial release
MDEOF
git add CHANGELOG.md
git commit -m "Add changelog"

# Create worktrees with varied states
cd "$FIXTURE_DIR"

# Clean worktree
git -C "$REPO_DIR" worktree add -b testing "$FIXTURE_DIR/rails-app-testing"

# Dirty worktree
git -C "$REPO_DIR" worktree add -b staging "$FIXTURE_DIR/rails-app-staging"
echo "DIRTY_FILE=true" > "$FIXTURE_DIR/rails-app-staging/.env"
cat > "$FIXTURE_DIR/rails-app-staging/config/routes.rb" <<'RBEOF'
Rails.application.routes.draw do
  root 'home#index'
  get '/health', to: 'health#show'
end
RBEOF

# Feature worktree with extra commit (ahead)
git -C "$REPO_DIR" worktree add -b feature/auth "$FIXTURE_DIR/rails-app-feature-auth"
mkdir -p "$FIXTURE_DIR/rails-app-feature-auth/lib"
cat > "$FIXTURE_DIR/rails-app-feature-auth/lib/auth.rb" <<'RBEOF'
module Auth
  def self.authenticate(user, password)
    # TODO: implement
  end
end
RBEOF
git -C "$FIXTURE_DIR/rails-app-feature-auth" add lib/auth.rb
git -C "$FIXTURE_DIR/rails-app-feature-auth" commit -m "Add auth module"

echo ""
echo "Fixture created at: $FIXTURE_DIR"
echo "Main repo:   $REPO_DIR"
echo "Worktrees:"
git -C "$REPO_DIR" worktree list
echo ""
echo "States:"
echo "  rails-app          — main (clean)"
echo "  rails-app-testing  — clean"
echo "  rails-app-staging  — dirty (modified routes, untracked .env)"
echo "  rails-app-feature-auth — ahead by 1 commit"
