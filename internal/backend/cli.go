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
	binaryPath string
}

// NewCLIBackend creates a backend that delegates to the claude CLI.
// If binaryPath is empty, it looks for "claude" in PATH.
func NewCLIBackend(binaryPath string) (*CLIBackend, error) {
	if binaryPath == "" {
		path, err := exec.LookPath("claude")
		if err != nil {
			return nil, fmt.Errorf("claude CLI not found in PATH: %w", err)
		}
		binaryPath = path
	}
	return &CLIBackend{binaryPath: binaryPath}, nil
}

func (b *CLIBackend) Query(messages []Message, model string, onChunk func(text string)) (string, error) {
	// Build the prompt: use the last user message as the primary prompt.
	// Pass conversation history as context via --system-prompt.
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

	args := []string{"-p", prompt, "--output-format", "text"}

	// Don't pass --model to the CLI — let it use its own default.
	// The CLI has its own model configuration and may not support
	// the same models as the direct API.

	// Pass conversation history as system prompt for context.
	if len(history) > 0 {
		systemCtx := "Previous conversation:\n" + strings.Join(history, "\n")
		args = append(args, "--system-prompt", systemCtx)
	}

	cmd := exec.Command(b.binaryPath, args...)

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
