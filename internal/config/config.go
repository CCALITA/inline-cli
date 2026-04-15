package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for inline-cli.
type Config struct {
	// Backend selection: "api" (default), "cli", "acp"
	Backend string `toml:"backend"`

	// API backend settings
	APIKey     string `toml:"api_key"`
	APIBaseURL string `toml:"api_base_url"`
	Model      string `toml:"model"`

	// CLI backend settings
	CLIPath string `toml:"cli_path"` // Path to claude binary (default: auto-detect)

	// Gemini backend settings
	GeminiPath string `toml:"gemini_path"` // Path to gemini binary (default: auto-detect)

	// OpenCode backend settings
	OpenCodePath string `toml:"opencode_path"` // Path to opencode binary (default: auto-detect)

	// General settings
	SocketPath            string `toml:"socket_path"`
	PIDFile               string `toml:"pid_file"`
	MaxSessionIdleMinutes int    `toml:"max_session_idle_minutes"`
	MaxMessages           int    `toml:"max_messages"`
	FallbackKeybinding    string `toml:"fallback_keybinding"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	uid := os.Getuid()
	homeDir, _ := os.UserHomeDir()
	return Config{
		Backend:               "api",
		Model:                 "claude-sonnet-4-20250514",
		SocketPath:            fmt.Sprintf("/tmp/inline-cli-%d.sock", uid),
		PIDFile:               filepath.Join(homeDir, ".inline-cli", "pid"),
		MaxSessionIdleMinutes: 30,
		MaxMessages:           50,
		FallbackKeybinding:    "^J",
	}
}

// BackendName returns a display name for the configured backend.
func (c Config) BackendName() string {
	if c.Backend == "" {
		return "api"
	}
	return c.Backend
}

// Load reads configuration from the config file, environment variables, and
// applies defaults. Precedence: env vars > config file > defaults.
func Load() (Config, error) {
	cfg := DefaultConfig()

	// Load from config file if it exists.
	configPath := configFilePath()
	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
			return cfg, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}
	}

	// Environment variables override config file values.
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("INLINE_CLI_BACKEND"); v != "" {
		cfg.Backend = v
	}
	if v := os.Getenv("INLINE_CLI_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("INLINE_CLI_SOCKET"); v != "" {
		cfg.SocketPath = v
	}
	if v := os.Getenv("INLINE_CLI_API_BASE_URL"); v != "" {
		cfg.APIBaseURL = v
	}
	if v := os.Getenv("INLINE_CLI_CLI_PATH"); v != "" {
		cfg.CLIPath = v
	}
	if v := os.Getenv("INLINE_CLI_GEMINI_PATH"); v != "" {
		cfg.GeminiPath = v
	}
	if v := os.Getenv("INLINE_CLI_OPENCODE_PATH"); v != "" {
		cfg.OpenCodePath = v
	}
	if v := os.Getenv("INLINE_CLI_MAX_IDLE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSessionIdleMinutes = n
		}
	}

	return cfg, nil
}

func configFilePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".inline-cli", "config.toml")
}

// ConfigFilePath returns the path to the config file.
func ConfigFilePath() string {
	return configFilePath()
}

// SaveBackend updates the backend field in the config file, preserving other settings.
// Creates the config directory and file if they don't exist.
func SaveBackend(backend string) error {
	path := configFilePath()

	// Ensure directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load existing config to preserve other fields.
	cfg := DefaultConfig()
	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return fmt.Errorf("failed to read existing config: %w", err)
		}
	}

	cfg.Backend = backend

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}
