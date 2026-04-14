package backend

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

// CodexBackend uses the OpenAI Codex CLI tool as the backend.
// It execs `codex -q <prompt>` and streams stdout.
type CodexBackend struct {
	binaryPath string
}

// NewCodexBackend creates a backend that delegates to the Codex CLI.
// If binaryPath is empty, it looks for "codex" in PATH.
func NewCodexBackend(binaryPath string) (*CodexBackend, error) {
	if binaryPath == "" {
		path, err := exec.LookPath("codex")
		if err != nil {
			return nil, fmt.Errorf("codex CLI not found in PATH: %w", err)
		}
		binaryPath = path
	}
	return &CodexBackend{binaryPath: binaryPath}, nil
}

func (b *CodexBackend) Query(messages []Message, model string, onChunk func(text string)) (string, error) {
	// Build the prompt: use the last user message as the primary prompt.
	// Pass conversation history as context prepended to the prompt.
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

	// Codex CLI uses -q (--quiet) for non-interactive mode that streams to stdout,
	// and --approval-mode suggest for safe operation (suggest changes, don't auto-apply).
	args := []string{"-q", "--approval-mode", "suggest"}

	// Pass model if specified.
	if model != "" {
		args = append(args, "--model", model)
	}

	// Prepend conversation history to the prompt for context.
	fullPrompt := prompt
	if len(history) > 0 {
		fullPrompt = "Previous conversation:\n" + strings.Join(history, "\n") + "\n\nCurrent request:\n" + prompt
	}

	args = append(args, fullPrompt)

	cmd := exec.Command(b.binaryPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start codex CLI: %w", err)
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
			return fullResponse.String(), fmt.Errorf("codex CLI exited with error: %w", err)
		}
		return "", fmt.Errorf("codex CLI failed: %w", err)
	}

	return fullResponse.String(), nil
}
