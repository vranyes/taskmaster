package taskmaster

import (
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("TASKMASTER_BASE_URL", "")
	t.Setenv("TASKMASTER_MODEL", "")
	t.Setenv("TASKMASTER_API_KEY", "")
	t.Setenv("TASKMASTER_ADDR", "")
	cfg := LoadConfig()
	if cfg.Addr != ":8080" {
		t.Fatalf("expected default addr :8080, got %q", cfg.Addr)
	}
}

func TestLoadConfig_ReadsEnv(t *testing.T) {
	t.Setenv("TASKMASTER_BASE_URL", "https://home.example.com/v1")
	t.Setenv("TASKMASTER_MODEL", "home-model")
	t.Setenv("TASKMASTER_API_KEY", "secret")
	t.Setenv("TASKMASTER_ADDR", ":9090")
	t.Setenv("TASKMASTER_EDGE_SECRET", "edge-hmac")
	t.Setenv("TASKMASTER_DIRECTORY_URL", "http://dir:8082")
	t.Setenv("TASKMASTER_RESOLVE_SECRET", "dir-secret")
	cfg := LoadConfig()
	if cfg.BaseURL != "https://home.example.com/v1" {
		t.Fatalf("unexpected base url: %q", cfg.BaseURL)
	}
	if cfg.Model != "home-model" {
		t.Fatalf("unexpected model: %q", cfg.Model)
	}
	if cfg.APIKey != "secret" {
		t.Fatalf("unexpected api key: %q", cfg.APIKey)
	}
	if cfg.Addr != ":9090" {
		t.Fatalf("unexpected addr: %q", cfg.Addr)
	}
	if cfg.EdgeSecret != "edge-hmac" {
		t.Fatalf("unexpected edge secret: %q", cfg.EdgeSecret)
	}
	if cfg.DirectoryURL != "http://dir:8082" {
		t.Fatalf("unexpected directory url: %q", cfg.DirectoryURL)
	}
	if cfg.ResolveSecret != "dir-secret" {
		t.Fatalf("unexpected resolve secret: %q", cfg.ResolveSecret)
	}
}
