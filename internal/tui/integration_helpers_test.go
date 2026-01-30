//go:build integration

package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitEnv returns environment variables for isolated git operations.
func gitEnv() []string {
	return append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
}

// runGit executes a git command in the given directory with isolated config.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %s\n%s", args, dir, err, out)
	}
	return string(out)
}

// setupRailsFixture creates a realistic Rails 8 repo with grove config.
// Returns the repo path (symlink-resolved).
func setupRailsFixture(t *testing.T) string {
	t.Helper()

	raw := t.TempDir()
	base, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatal(err)
	}

	repo := filepath.Join(base, "rails-app")
	must(t, os.MkdirAll(repo, 0755))

	// git init
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "commit.gpgsign", "false")

	// Directory structure
	dirs := []string{
		"app/models",
		"app/controllers",
		"config/environments",
		"db/migrate",
		"lib",
		"test",
		".grove/hooks",
	}
	for _, d := range dirs {
		must(t, os.MkdirAll(filepath.Join(repo, d), 0755))
	}

	// Files
	files := map[string]string{
		".gitignore": ".env\nconfig/master.key\ntmp/\nlog/\nnode_modules/\n",
		"Gemfile":    "source 'https://rubygems.org'\ngem 'rails', '~> 8.0'\n",
		"Gemfile.lock": "GEM\n  remote: https://rubygems.org/\n  specs:\n    rails (8.0.0)\n\n" +
			"PLATFORMS\n  ruby\n\nDEPENDENCIES\n  rails (~> 8.0)\n",
		"README.md": "# Rails App\nTest fixture for grove integration tests.\n",
		"app/models/user.rb": "class User < ApplicationRecord\n  validates :email, presence: true\nend\n",
		"app/controllers/application_controller.rb": "class ApplicationController < ActionController::Base\nend\n",
		"config/database.yml":                       "default: &default\n  adapter: postgresql\n  pool: 5\n\ndevelopment:\n  <<: *default\n  database: app_development\n",
		"config/routes.rb":                          "Rails.application.routes.draw do\n  root 'home#index'\nend\n",
		"config/environments/production.rb":         "Rails.application.configure do\n  config.eager_load = true\nend\n",
		"config/credentials.yml.enc":                "encrypted-content-placeholder",
		"db/migrate/001_create_users.rb":            "class CreateUsers < ActiveRecord::Migration[8.0]\n  def change\n    create_table :users do |t|\n      t.string :email\n      t.timestamps\n    end\n  end\nend\n",
		".env.example":       "DATABASE_URL=postgres://localhost/app_development\nSECRET_KEY_BASE=changeme\n",
		"docker-compose.yml": "services:\n  web:\n    build:\n      context: .\n      dockerfile: Dockerfile\n    ports:\n      - \"3000:3000\"\n    depends_on:\n      - db\n    environment:\n      DATABASE_URL: postgres://postgres:password@db:5432/app_development\n\n  db:\n    image: postgres:16-alpine\n    environment:\n      POSTGRES_PASSWORD: password\n    volumes:\n      - pgdata:/var/lib/postgresql/data\n\nvolumes:\n  pgdata:\n",
		"Dockerfile":         "FROM ruby:3.3-slim\nWORKDIR /app\nCOPY Gemfile* ./\nRUN echo \"gem install skipped in test fixture\"\nCOPY . .\nCMD [\"echo\", \"Rails test fixture\"]\n",
		".grove/config.toml": "project_name = \"rails-app\"\n\n[plugins.docker]\nenabled = true\nauto_start = false\nauto_stop = false\n",
		".grove/state.json":  `{"version":1,"project":"rails-app","worktrees":{}}`,
	}

	for name, content := range files {
		writeFile(t, filepath.Join(repo, name), content)
	}

	// Initial commit
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "Initial commit: Rails 8 app scaffold")

	// Second commit so worktrees diverge
	writeFile(t, filepath.Join(repo, "CHANGELOG.md"), "# Changelog\n## 0.1.0\n- Initial release\n")
	runGit(t, repo, "add", "CHANGELOG.md")
	runGit(t, repo, "commit", "-m", "Add changelog")

	// Create branches (not worktrees — just branches)
	runGit(t, repo, "branch", "feature/auth")
	runGit(t, repo, "branch", "feature/api")
	runGit(t, repo, "branch", "hotfix/login")

	return repo
}

// setupRailsFixtureWithWorktrees creates a fixture and adds worktrees.
func setupRailsFixtureWithWorktrees(t *testing.T, names ...string) string {
	t.Helper()
	repo := setupRailsFixture(t)

	for _, name := range names {
		fullName := "rails-app-" + name
		wtPath := filepath.Join(filepath.Dir(repo), fullName)
		runGit(t, repo, "worktree", "add", "-b", name, wtPath)
	}

	return repo
}

// setupRailsFixtureWithDirtyWorktree creates a fixture with a dirty worktree.
func setupRailsFixtureWithDirtyWorktree(t *testing.T) string {
	t.Helper()
	repo := setupRailsFixtureWithWorktrees(t, "dirty-wt")

	wtPath := filepath.Join(filepath.Dir(repo), "rails-app-dirty-wt")

	// Untracked file (gitignored — .env)
	writeFile(t, filepath.Join(wtPath, ".env"), "SECRET_KEY_BASE=realvalue\n")

	// Modify tracked file
	writeFile(t, filepath.Join(wtPath, "config/routes.rb"),
		"Rails.application.routes.draw do\n  root 'home#index'\n  get '/health', to: 'health#show'\nend\n")

	return repo
}

// setupRailsFixtureWithUpstream creates a fixture with a bare remote and upstream tracking.
func setupRailsFixtureWithUpstream(t *testing.T) string {
	t.Helper()
	repo := setupRailsFixture(t)

	base := filepath.Dir(repo)
	bareRepo := filepath.Join(base, "rails-app.git")

	// Create bare repo
	runGit(t, base, "clone", "--bare", repo, bareRepo)

	// Add remote + push
	runGit(t, repo, "remote", "add", "origin", bareRepo)
	runGit(t, repo, "push", "-u", "origin", "main")

	// Add local commits ahead of upstream
	writeFile(t, filepath.Join(repo, "lib/utils.rb"), "module Utils\nend\n")
	runGit(t, repo, "add", "lib/utils.rb")
	runGit(t, repo, "commit", "-m", "Add utils module")

	writeFile(t, filepath.Join(repo, "lib/helpers.rb"), "module Helpers\nend\n")
	runGit(t, repo, "add", "lib/helpers.rb")
	runGit(t, repo, "commit", "-m", "Add helpers module")

	return repo
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
