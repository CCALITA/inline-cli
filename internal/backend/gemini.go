package backend

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// GeminiBackend uses the Google `gemini` CLI tool as the backend.
// It execs `gemini -p <prompt> -o text` and streams stdout.
type GeminiBackend struct {
	// Configured path to the gemini binary. Empty means auto-detect via PATH.
	configuredPath string
}

// NewGeminiBackend creates a backend that delegates to the gemini CLI.
// If binaryPath is empty, it will look for "gemini" in PATH on each query.
func NewGeminiBackend(binaryPath string) (*GeminiBackend, error) {
	return &GeminiBackend{configuredPath: binaryPath}, nil
}

// resolveBinary finds the gemini binary, resolving PATH on each call so it
// picks up installs, upgrades, and PATH changes without a daemon restart.
func (b *GeminiBackend) resolveBinary() (string, error) {
	if b.configuredPath != "" {
		return b.configuredPath, nil
	}
	path, err := exec.LookPath("gemini")
	if err != nil {
		return "", fmt.Errorf("gemini CLI not found in PATH: %w", err)
	}
	return path, nil
}

func (b *GeminiBackend) Query(messages []Message, model string, onChunk func(text string)) (string, error) {
	binaryPath, err := b.resolveBinary()
	if err != nil {
		return "", err
	}

	prompt, history := extractPromptAndHistory(messages)
	if prompt == "" {
		return "", fmt.Errorf("no user message found")
	}

	if len(history) > 0 {
		prompt = formatHistory(history) + "\n\n" + prompt
	}

	// gemini CLI supports -p for non-interactive mode and -o for output format.
	// Use -o text to get plain text on stdout (default may include ANSI or TUI).
	// Don't pass --model: gemini uses its own model config and the inline-cli
	// default (e.g. claude-sonnet) would be invalid for gemini.
	args := []string{"-p", prompt, "-o", "text"}

	cmd := exec.Command(binaryPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr so we can surface the actual error message from gemini.
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start gemini CLI: %w", err)
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
		// Extract the first meaningful line from stderr for the error message.
		errMsg := firstLine(stderrBuf.String())
		if fullResponse.Len() > 0 {
			if errMsg != "" {
				return fullResponse.String(), fmt.Errorf("gemini CLI error: %s", errMsg)
			}
			return fullResponse.String(), fmt.Errorf("gemini CLI exited with error: %w", err)
		}
		if errMsg != "" {
			return "", fmt.Errorf("gemini CLI error: %s", errMsg)
		}
		return "", fmt.Errorf("gemini CLI failed: %w", err)
	}

	return fullResponse.String(), nil
}

// firstLine returns the first non-empty line from s, trimmed.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
