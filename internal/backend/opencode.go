package backend

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// OpenCodeBackend uses the `opencode` CLI tool as the backend.
// It execs `opencode -p <prompt>` and streams stdout.
type OpenCodeBackend struct {
	binaryPath string
}

// NewOpenCodeBackend creates a backend that delegates to the opencode CLI.
// If binaryPath is empty, it looks for "opencode" in PATH.
func NewOpenCodeBackend(binaryPath string) (*OpenCodeBackend, error) {
	if binaryPath == "" {
		path, err := exec.LookPath("opencode")
		if err != nil {
			return nil, fmt.Errorf("opencode CLI not found in PATH: %w", err)
		}
		binaryPath = path
	}
	return &OpenCodeBackend{binaryPath: binaryPath}, nil
}

func (b *OpenCodeBackend) Query(messages []Message, model string, onChunk func(text string)) (string, error) {
	// Build the prompt: use the last user message as the primary prompt.
	// Pass conversation history as context in the prompt itself,
	// since opencode does not support a --system-prompt flag.
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

	// Prepend conversation history to the prompt for context.
	if len(history) > 0 {
		historyCtx := "Previous conversation:\n" + strings.Join(history, "\n") + "\n\nCurrent request:\n"
		prompt = historyCtx + prompt
	}

	args := []string{"-p", prompt, "--output-format", "text", "--quiet"}

	cmd := exec.Command(b.binaryPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start opencode CLI: %w", err)
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
			return fullResponse.String(), fmt.Errorf("opencode CLI exited with error: %w", err)
		}
		return "", fmt.Errorf("opencode CLI failed: %w", err)
	}

	return fullResponse.String(), nil
}
