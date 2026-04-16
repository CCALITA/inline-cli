package main

import (
	_ "embed"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CCALITA/inline-cli/internal/config"
	"github.com/CCALITA/inline-cli/internal/daemon"
	"github.com/CCALITA/inline-cli/internal/ipc"
	"github.com/CCALITA/inline-cli/internal/render"
)

var version = "dev"

// errAlreadyDisplayed signals that the error was already shown to the user
// via the renderer — cobra should not print it again.
var errAlreadyDisplayed = fmt.Errorf("")

//go:embed shell_zsh.sh
var zshScript string

//go:embed shell_bash.sh
var bashScript string

func main() {
	rootCmd := &cobra.Command{
		Use:   "inline-cli",
		Short: "Inline Claude assistant for your terminal",
		Long: `Inline Claude assistant for your terminal.

Type a question directly in your shell prompt and press Ctrl+J (or Shift+Enter
in supported terminals) to get an AI response streamed inline — no context
switching needed.

Quick start:
  1. Run 'inline-cli setup' to choose a backend
  2. Add shell integration: eval "$(inline-cli init zsh)"  (or bash)
  3. Restart your shell and start asking questions with Ctrl+J

Shift+Enter requires terminal configuration — see 'inline-cli init --help'.`,
		Version:       version,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(
		newDaemonCmd(),
		newQueryCmd(),
		newStatusCmd(),
		newStopSessionCmd(),
		newInitCmd(),
		newBackendCmd(),
		newSetupCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		if err != errAlreadyDisplayed {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the background daemon",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Start the background daemon",
			RunE:  runDaemonStart,
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the background daemon",
			RunE:  runDaemonStop,
		},
		&cobra.Command{
			Use:    "run",
			Short:  "Run the daemon in the foreground (internal use)",
			RunE:   runDaemonRun,
			Hidden: true,
		},
	)

	return cmd
}

func newQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Send a prompt to Claude",
		RunE:  runQuery,
	}

	cmd.Flags().StringP("dir", "d", "", "Working directory (defaults to current dir)")
	cmd.Flags().StringP("prompt", "p", "", "Prompt text")
	cmd.Flags().Bool("raw", false, "Skip markdown rendering")

	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon and session status",
		RunE:  runStatus,
	}
}

func newStopSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "stop-session",
		Short:  "Stop a session for a directory",
		Hidden: true,
		RunE:   runStopSession,
	}
	cmd.Flags().StringP("dir", "d", "", "Directory whose session to stop")
	return cmd
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "init [shell]",
		Short:     "Output shell integration script",
		Long:      "Output shell integration script to stdout.\n  zsh:  eval \"$(inline-cli init zsh)\"\n  bash: eval \"$(inline-cli init bash)\"",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"zsh", "bash"},
		RunE:      runInit,
	}
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	d := daemon.NewDaemon(cfg.PIDFile, cfg.SocketPath)
	if err := d.Start(); err != nil {
		return err
	}

	fmt.Printf("%s daemon started\n", render.Green("✓"))
	return nil
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	d := daemon.NewDaemon(cfg.PIDFile, cfg.SocketPath)
	if err := d.Stop(); err != nil {
		return err
	}

	fmt.Printf("%s daemon stopped\n", render.Green("✓"))
	return nil
}

func runDaemonRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Override socket path from env if set (passed by the Start command).
	if v := os.Getenv("INLINE_CLI_SOCKET"); v != "" {
		cfg.SocketPath = v
	}

	srv, err := daemon.NewServer(cfg)
	if err != nil {
		return err
	}

	return srv.Run()
}

func runQuery(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		dir, _ = os.Getwd()
	}

	prompt, _ := cmd.Flags().GetString("prompt")
	if prompt == "" && len(args) > 0 {
		prompt = strings.Join(args, " ")
	}
	if prompt == "" {
		return fmt.Errorf("prompt is required (use --prompt or pass as argument)")
	}

	raw, _ := cmd.Flags().GetBool("raw")

	// Auto-start daemon if not running.
	d := daemon.NewDaemon(cfg.PIDFile, cfg.SocketPath)
	if err := d.EnsureRunning(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for the daemon socket to become available.
	if err := waitForSocket(cfg.SocketPath, 3*time.Second); err != nil {
		return fmt.Errorf("daemon started but socket not ready: %w", err)
	}

	client := ipc.NewClient(cfg.SocketPath)
	requestID := uuid.New().String()

	renderer := render.NewRenderer(os.Stdout)
	var md *render.Markdown
	if !raw {
		md = render.NewMarkdown(renderer.Width())
	}

	ch, err := client.Query(dir, prompt, requestID)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	firstChunk := true
	var fullText strings.Builder

	for resp := range ch {
		switch resp.Type {
		case ipc.TypeStart:
			renderer.ShowThinking()

		case ipc.TypeChunk:
			if firstChunk {
				renderer.ClearThinking()
				renderer.StartResponse()
				firstChunk = false
			}
			fullText.WriteString(resp.Text)
			if raw || md == nil {
				renderer.WriteChunk(resp.Text)
			} else {
				if rendered := md.RenderStreaming(resp.Text); rendered != "" {
					renderer.WriteChunk(rendered)
				}
			}

		case ipc.TypeDone:
			// Flush remaining markdown.
			if md != nil && !raw {
				if flushed := md.Flush(); flushed != "" {
					renderer.WriteChunk(flushed)
				}
			}
			if !firstChunk {
				renderer.EndResponse()
			}

		case ipc.TypeError:
			if firstChunk {
				renderer.ClearThinking()
			}
			renderer.ShowError(resp.Message)
			return errAlreadyDisplayed
		}
	}

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	d := daemon.NewDaemon(cfg.PIDFile, cfg.SocketPath)
	if !d.IsRunning() {
		fmt.Println("daemon is not running")
		return nil
	}

	client := ipc.NewClient(cfg.SocketPath)
	status, err := client.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	fmt.Printf("daemon: running (PID %d)\n", status.PID)
	if len(status.Sessions) == 0 {
		fmt.Println("sessions: none")
	} else {
		fmt.Printf("sessions: %d active\n", len(status.Sessions))
		for _, s := range status.Sessions {
			fmt.Printf("  %s (%d messages, last used: %s)\n", s.Dir, s.MessageCount, s.LastUsed)
		}
	}

	return nil
}

func runStopSession(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		return fmt.Errorf("--dir is required")
	}

	client := ipc.NewClient(cfg.SocketPath)
	return client.StopSession(dir)
}

func runInit(cmd *cobra.Command, args []string) error {
	shell := args[0]

	var script string
	switch shell {
	case "zsh":
		script = zshScript
	case "bash":
		script = bashScript
	default:
		return fmt.Errorf("unsupported shell: %s (supported: zsh, bash)", shell)
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine binary path: %w", err)
	}

	// Output the shell script with the binary path substituted.
	script = strings.ReplaceAll(script, "{{INLINE_CLI_BIN}}", binaryPath)
	fmt.Print(script)
	return nil
}

// waitForSocket polls until the Unix socket is accepting connections or the timeout expires.
func waitForSocket(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	delay := 50 * time.Millisecond
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(delay)
		if delay < 400*time.Millisecond {
			delay *= 2
		}
	}
	return fmt.Errorf("timed out waiting for socket %s", socketPath)
}
