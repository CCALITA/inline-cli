package daemon

import (
	"testing"

	"github.com/CCALITA/inline-cli/internal/backend"
	"github.com/CCALITA/inline-cli/internal/config"
)

func TestCreateBackend_OpenCode(t *testing.T) {
	cfg := config.Config{
		Backend:      "opencode",
		OpenCodePath: "/usr/local/bin/opencode",
	}

	b, err := createBackend(cfg)
	if err != nil {
		t.Fatalf("createBackend() error: %v", err)
	}

	if _, ok := b.(*backend.OpenCodeBackend); !ok {
		t.Errorf("createBackend() returned %T, want *backend.OpenCodeBackend", b)
	}
}

func TestCreateBackend_UnknownBackend(t *testing.T) {
	cfg := config.Config{
		Backend: "nonexistent",
	}

	_, err := createBackend(cfg)
	if err == nil {
		t.Fatal("expected error for unknown backend, got nil")
	}
}

func TestCreateBackend_Claude(t *testing.T) {
	cfg := config.Config{
		Backend: "claude",
		CLIPath: "/usr/local/bin/claude",
	}

	b, err := createBackend(cfg)
	if err != nil {
		t.Fatalf("createBackend() error: %v", err)
	}

	if _, ok := b.(*backend.CLIBackend); !ok {
		t.Errorf("createBackend() returned %T, want *backend.CLIBackend", b)
	}
}
