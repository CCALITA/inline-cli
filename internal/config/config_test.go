package config

import (
	"testing"
)

func TestConfig_OpenCodePath_EnvVar(t *testing.T) {
	expectedPath := "/custom/path/to/opencode"
	t.Setenv("INLINE_CLI_OPENCODE_PATH", expectedPath)

	// Clear any other env vars that might interfere.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("INLINE_CLI_BACKEND", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.OpenCodePath != expectedPath {
		t.Errorf("OpenCodePath = %q, want %q", cfg.OpenCodePath, expectedPath)
	}
}

func TestDefaultConfig_BackendName(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		want    string
	}{
		{name: "empty backend defaults to api", backend: "", want: "api"},
		{name: "api backend", backend: "api", want: "api"},
		{name: "claude backend", backend: "claude", want: "claude"},
		{name: "opencode backend", backend: "opencode", want: "opencode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Backend: tt.backend}
			got := cfg.BackendName()
			if got != tt.want {
				t.Errorf("BackendName() = %q, want %q", got, tt.want)
			}
		})
	}
}
