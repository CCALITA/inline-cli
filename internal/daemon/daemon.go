package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Daemon manages the background daemon process lifecycle.
type Daemon struct {
	pidFile    string
	socketPath string
	binary     string
}

// NewDaemon creates a new daemon manager.
func NewDaemon(pidFile, socketPath string) (*Daemon, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to determine binary path: %w", err)
	}
	return &Daemon{
		pidFile:    pidFile,
		socketPath: socketPath,
		binary:     binary,
	}, nil
}

// Start launches the daemon as a background process.
func (d *Daemon) Start() error {
	if d.IsRunning() {
		return fmt.Errorf("daemon is already running (PID %d)", d.readPID())
	}

	// Clean up stale socket if it exists.
	d.cleanStaleSocket()

	// Ensure PID file directory exists.
	if err := os.MkdirAll(filepath.Dir(d.pidFile), 0700); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	// Re-exec ourselves with the hidden "daemon run" subcommand.
	cmd := exec.Command(d.binary, "daemon", "run")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create a new session so the daemon survives shell exit.
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Pass config via environment so the child uses the same settings.
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("INLINE_CLI_SOCKET=%s", d.socketPath),
		fmt.Sprintf("INLINE_CLI_PID_FILE=%s", d.pidFile),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Write PID file.
	pid := cmd.Process.Pid
	if err := os.WriteFile(d.pidFile, []byte(strconv.Itoa(pid)), 0600); err != nil {
		// Try to kill the orphaned process.
		cmd.Process.Kill()
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Detach from the child — don't wait for it.
	cmd.Process.Release()

	return nil
}

// Stop sends SIGTERM to the daemon process.
func (d *Daemon) Stop() error {
	pid := d.readPID()
	if pid <= 0 {
		return fmt.Errorf("daemon is not running (no PID file)")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		d.cleanup()
		return fmt.Errorf("process %d not found", pid)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		d.cleanup()
		return fmt.Errorf("failed to stop daemon (PID %d): %w", pid, err)
	}

	d.cleanup()
	return nil
}

// IsRunning checks if the daemon process is alive.
func (d *Daemon) IsRunning() bool {
	pid := d.readPID()
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, kill(pid, 0) checks if process exists.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// PID returns the daemon's PID, or 0 if not running.
func (d *Daemon) PID() int {
	if d.IsRunning() {
		return d.readPID()
	}
	return 0
}

// EnsureRunning starts the daemon if it's not already running.
func (d *Daemon) EnsureRunning() error {
	if d.IsRunning() {
		return nil
	}
	return d.Start()
}

func (d *Daemon) readPID() int {
	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func (d *Daemon) cleanup() {
	os.Remove(d.pidFile)
	os.Remove(d.socketPath)
}

func (d *Daemon) cleanStaleSocket() {
	if _, err := os.Stat(d.socketPath); err == nil {
		os.Remove(d.socketPath)
	}
}
