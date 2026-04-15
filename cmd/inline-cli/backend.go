package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CCALITA/inline-cli/internal/config"
	"github.com/CCALITA/inline-cli/internal/daemon"
)

// backendInfo describes a supported backend.
type backendInfo struct {
	Name   string // config value
	Desc   string // human-readable description
	Binary string // CLI binary name to check (empty for API-based)
}

var backends = []backendInfo{
	{Name: "api", Desc: "Anthropic API (requires API key)", Binary: ""},
	{Name: "claude", Desc: "Claude CLI", Binary: "claude"},
	{Name: "gemini", Desc: "Gemini CLI", Binary: "gemini"},
	{Name: "opencode", Desc: "OpenCode CLI", Binary: "opencode"},
	{Name: "codex", Desc: "Codex CLI", Binary: "codex"},
}

// isInstalled checks if a CLI binary is available in PATH.
func (b backendInfo) isInstalled() bool {
	if b.Binary == "" {
		return true // API backend is always "available"
	}
	_, err := exec.LookPath(b.Binary)
	return err == nil
}

// validBackendNames returns the list of valid backend name strings.
func validBackendNames() []string {
	names := make([]string, len(backends))
	for i, b := range backends {
		names[i] = b.Name
	}
	return names
}

func newBackendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backend",
		Short: "Manage backends",
	}

	cmd.AddCommand(
		newBackendListCmd(),
		newBackendShowCmd(),
		newBackendSetCmd(),
	)

	return cmd
}

func newBackendListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available backends",
		RunE:  runBackendList,
	}
}

func newBackendShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current backend",
		RunE:  runBackendShow,
	}
}

func newBackendSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "set <backend>",
		Short:     "Switch to a different backend",
		Args:      cobra.ExactArgs(1),
		ValidArgs: validBackendNames(),
		RunE:      runBackendSet,
	}
}

func runBackendList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	active := cfg.BackendName()

	for _, b := range backends {
		marker := "  "
		if b.Name == active {
			marker = "* "
		}

		status := ""
		if b.Binary != "" {
			if b.isInstalled() {
				status = "  \033[32m✓ installed\033[0m"
			} else {
				status = "  \033[31m✗ not found\033[0m"
			}
		}

		activeSuffix := ""
		if b.Name == active {
			activeSuffix = "  (active)"
		}

		fmt.Printf("%s%-10s — %s%s%s\n", marker, b.Name, b.Desc, status, activeSuffix)
	}

	// Warn about env var override.
	if v := os.Getenv("INLINE_CLI_BACKEND"); v != "" {
		fmt.Printf("\n\033[33mNote: INLINE_CLI_BACKEND=%s is set and overrides the config file.\033[0m\n", v)
	}

	return nil
}

func runBackendShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println(cfg.BackendName())

	if v := os.Getenv("INLINE_CLI_BACKEND"); v != "" {
		fmt.Fprintf(os.Stderr, "Note: overridden by INLINE_CLI_BACKEND=%s\n", v)
	}

	return nil
}

func runBackendSet(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate.
	valid := false
	for _, b := range backends {
		if b.Name == name {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unknown backend %q (supported: %s)", name, strings.Join(validBackendNames(), ", "))
	}

	// Warn about env var override.
	if v := os.Getenv("INLINE_CLI_BACKEND"); v != "" {
		fmt.Fprintf(os.Stderr, "\033[33mWarning: INLINE_CLI_BACKEND=%s is set and will override this config.\033[0m\n", v)
		fmt.Fprintf(os.Stderr, "Run: unset INLINE_CLI_BACKEND\n\n")
	}

	// Write config.
	if err := config.SaveBackend(name); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Backend set to %q\n", name)

	// Auto-restart daemon.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	d := daemon.NewDaemon(cfg.PIDFile, cfg.SocketPath)
	if d.IsRunning() {
		if err := d.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to stop daemon: %v\n", err)
		} else {
			fmt.Println("Daemon restarted. Next query will use the new backend.")
		}
	}

	return nil
}
