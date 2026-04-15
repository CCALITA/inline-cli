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
}

func (b backendInfo) isInstalled() bool {
	if b.Binary == "" {
		return true
	}
	_, err := exec.LookPath(b.Binary)
	return err == nil
}

func (b backendInfo) installStatus() string {
	if b.Binary == "" {
		return ""
	}
	if b.isInstalled() {
		return "  \033[32m✓ installed\033[0m"
	}
	return "  \033[31m✗ not found\033[0m"
}

func findBackend(name string) (backendInfo, bool) {
	for _, b := range backends {
		if b.Name == name {
			return b, true
		}
	}
	return backendInfo{}, false
}

func validBackendNames() []string {
	names := make([]string, len(backends))
	for i, b := range backends {
		names[i] = b.Name
	}
	return names
}

func restartDaemonIfRunning() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	d := daemon.NewDaemon(cfg.PIDFile, cfg.SocketPath)
	if d.IsRunning() {
		if err := d.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to stop daemon: %v\n", err)
		} else {
			fmt.Println("Daemon stopped. Next query will use the new backend.")
		}
	}
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
		activeSuffix := ""
		if b.Name == active {
			marker = "* "
			activeSuffix = "  (active)"
		}
		fmt.Printf("%s%-10s — %s%s%s\n", marker, b.Name, b.Desc, b.installStatus(), activeSuffix)
	}

	return nil
}

func runBackendShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println(cfg.BackendName())
	return nil
}

func runBackendSet(cmd *cobra.Command, args []string) error {
	name := args[0]

	if _, ok := findBackend(name); !ok {
		return fmt.Errorf("unknown backend %q (supported: %s)", name, strings.Join(validBackendNames(), ", "))
	}

	if err := config.SaveBackend(name); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Backend set to %q\n", name)
	restartDaemonIfRunning()

	return nil
}
