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

	// Build the prompt: use the last user message as the primary prompt.
	// Prepend conversation history as context directly in the prompt.
	var prompt string
	var history []string

	for i, m := range messages {
		if i == len(messages)-1 && m.Role == "user" {
			prompt = m.Content
		} else {
			prefix := "User"
			if m.Role == "assistant" {
				prefix = "Assistant"
			}
			history = append(history, fmt.Sprintf("%s: %s", prefix, m.Content))
		}
	}

	if prompt == "" {
		return "", fmt.Errorf("no user message found")
	}

	// If there is conversation history, prepend it to the prompt since the
	// gemini CLI does not have a dedicated system-prompt flag.
	if len(history) > 0 {
		historyCtx := "Previous conversation:\n" + strings.Join(history, "\n") + "\n\n"
		prompt = historyCtx + prompt
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
		// If we already got some output, return it with the error.
		if fullResponse.Len() > 0 {
			return fullResponse.String(), fmt.Errorf("gemini CLI exited with error: %w", err)
		}
		return "", fmt.Errorf("gemini CLI failed: %w", err)
	}

	return fullResponse.String(), nil
}
