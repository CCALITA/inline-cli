package backend

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// CLIBackend uses the `claude` CLI tool as the backend.
// It execs `claude -p <prompt>` and streams stdout.
type CLIBackend struct {
	// Configured path to the claude binary. Empty means auto-detect via PATH.
	configuredPath string
}

// NewCLIBackend creates a backend that delegates to the claude CLI.
// If binaryPath is empty, it will look for "claude" in PATH on each query.
func NewCLIBackend(binaryPath string) (*CLIBackend, error) {
	return &CLIBackend{configuredPath: binaryPath}, nil
}

// resolveBinary finds the claude binary, resolving PATH on each call so it
// picks up installs, upgrades, and PATH changes without a daemon restart.
func (b *CLIBackend) resolveBinary() (string, error) {
	if b.configuredPath != "" {
		return b.configuredPath, nil
	}
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude CLI not found in PATH: %w", err)
	}
	return path, nil
}

func (b *CLIBackend) Query(messages []Message, model string, onChunk func(text string)) (string, error) {
	binaryPath, err := b.resolveBinary()
	if err != nil {
		return "", err
	}

	prompt, history := extractPromptAndHistory(messages)
	if prompt == "" {
		return "", fmt.Errorf("no user message found")
	}

	args := []string{"-p", prompt, "--output-format", "text"}

	if len(history) > 0 {
		args = append(args, "--system-prompt", formatHistory(history))
	}

	cmd := exec.Command(binaryPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude CLI: %w", err)
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Split(scanChunks)

	for scanner.Scan() {
		chunk := scanner.Text()
		fullResponse.WriteString(chunk)
		if onChunk != nil {
			onChunk(chunk)
		}
	}

	if err := cmd.Wait(); err != nil {
		// If we already got some output, return it with the error.
		if fullResponse.Len() > 0 {
			return fullResponse.String(), fmt.Errorf("claude CLI exited with error: %w", err)
		}
		return "", fmt.Errorf("claude CLI failed: %w", err)
	}

	return fullResponse.String(), nil
}

// scanChunks is a bufio.SplitFunc that splits on any available data,
// delivering output as soon as it arrives rather than waiting for newlines.
func scanChunks(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// Return all available data immediately for streaming feel.
	return len(data), data, nil
}
