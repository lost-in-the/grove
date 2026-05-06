package detect

import (
	"path/filepath"
	"testing"
)

func TestInferAppService_SingleService(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", `services:
  app:
    image: ruby:3
`)
	got, ok := InferAppService(filepath.Join(dir, "docker-compose.yml"))
	if !ok || got != "app" {
		t.Fatalf("got %q,%v; want app,true", got, ok)
	}
}

func TestInferAppService_SkipsInfra(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", `version: "3"
services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7
  web:
    image: ruby:3
    depends_on:
      - postgres
`)
	got, ok := InferAppService(filepath.Join(dir, "docker-compose.yml"))
	if !ok || got != "web" {
		t.Fatalf("got %q,%v; want web,true", got, ok)
	}
}

func TestInferAppService_PrefersCanonicalNameOverDeclarationOrder(t *testing.T) {
	// Regression for the acupoll case (issue #56): a Rails+webpack stack
	// declares `webpack` before `web`. The previous heuristic ("first
	// non-infra in declaration order") picked `webpack` and routed
	// `bundle install` to the asset pipeline. The priority list pulls
	// `web` ahead.
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", `services:
  postgres:
    image: postgres:16
  redis:
    image: redis:7
  webpack:
    image: node:20
  worker:
    build: .
  web:
    build: .
    depends_on:
      - postgres
`)
	got, ok := InferAppService(filepath.Join(dir, "docker-compose.yml"))
	if !ok || got != "web" {
		t.Fatalf("got %q,%v; want web,true (priority list should pull web ahead of webpack)", got, ok)
	}
}

func TestInferAppService_FallsBackToDeclarationOrderWhenNoCanonicalName(t *testing.T) {
	// When no service matches the priority list, fall back to the original
	// "first non-infra" heuristic so existing behavior is preserved.
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", `services:
  postgres:
    image: postgres:16
  webpack:
    image: node:20
  worker:
    build: .
`)
	got, ok := InferAppService(filepath.Join(dir, "docker-compose.yml"))
	if !ok || got != "webpack" {
		t.Fatalf("got %q,%v; want webpack,true (no canonical name → first non-infra)", got, ok)
	}
}

func TestInferAppService_PriorityNameIsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", `services:
  Postgres:
    image: postgres:16
  Worker:
    build: .
  Web:
    build: .
`)
	got, ok := InferAppService(filepath.Join(dir, "docker-compose.yml"))
	if !ok || got != "Web" {
		t.Fatalf("got %q,%v; want Web,true (case-insensitive priority match should preserve original casing)", got, ok)
	}
}

func TestInferAppService_AllInfraReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", `services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7
`)
	_, ok := InferAppService(filepath.Join(dir, "docker-compose.yml"))
	if ok {
		t.Fatal("expected ok=false when all services are infra")
	}
}

func TestInferAppService_MissingFile(t *testing.T) {
	_, ok := InferAppService(filepath.Join(t.TempDir(), "nope.yml"))
	if ok {
		t.Fatal("expected ok=false for missing file")
	}
}

func TestInferAppService_IgnoresNestedKeys(t *testing.T) {
	// Nested keys like `image:`, `ports:` under a service must not be treated
	// as new services.
	dir := t.TempDir()
	writeFile(t, dir, "compose.yml", `services:
  app:
    image: ruby
    ports:
      - "3000:3000"
    environment:
      RAILS_ENV: development
`)
	got, ok := InferAppService(filepath.Join(dir, "compose.yml"))
	if !ok || got != "app" {
		t.Fatalf("got %q,%v; want app,true", got, ok)
	}
}

func TestFindComposeFile(t *testing.T) {
	dir := t.TempDir()
	if FindComposeFile(dir) != "" {
		t.Fatal("expected empty for no compose file")
	}
	writeFile(t, dir, "compose.yaml", "services: {}")
	if got := FindComposeFile(dir); got == "" {
		t.Fatal("expected non-empty after creating compose.yaml")
	}
}

func TestHasDocker(t *testing.T) {
	dir := t.TempDir()
	if HasDocker(dir) {
		t.Fatal("empty dir should not have docker")
	}
	writeFile(t, dir, "Dockerfile", "FROM scratch")
	if !HasDocker(dir) {
		t.Fatal("Dockerfile should be detected")
	}
}
