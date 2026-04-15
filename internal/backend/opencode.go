package backend

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// OpenCodeBackend uses the `opencode` CLI tool as the backend.
// It invokes `opencode run --format json <message>` and parses the
// newline-delimited JSON event stream from stdout.
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

// openCodeEvent represents a single JSON event from `opencode run --format json`.
type openCodeEvent struct {
	Type  string `json:"type"`
	Part  struct {
		Text string `json:"text"`
	} `json:"part"`
	Error struct {
		Name string `json:"name"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	} `json:"error"`
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

	// opencode uses `opencode run --format json <message>` for non-interactive mode.
	// Default format writes to stderr with ANSI codes; JSON format writes
	// newline-delimited JSON events to stdout which we can parse.
	// Don't pass --model: opencode uses its own provider/model format
	// (e.g. "anthropic/claude-sonnet") which differs from the inline-cli
	// model config. Let opencode use its own configured default.
	args := []string{"run", "--format", "json", prompt}

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
	// Each JSON event is a single line.
	for scanner.Scan() {
		line := scanner.Bytes()
		var event openCodeEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		switch event.Type {
		case "text":
			if event.Part.Text != "" {
				fullResponse.WriteString(event.Part.Text)
				if onChunk != nil {
					onChunk(event.Part.Text)
				}
			}
		case "error":
			msg := event.Error.Data.Message
			if msg == "" {
				msg = event.Error.Name
			}
			if msg == "" {
				msg = "unknown error"
			}
			return fullResponse.String(), fmt.Errorf("opencode error: %s", msg)
		}
	}

	if err := cmd.Wait(); err != nil {
		if fullResponse.Len() > 0 {
			return fullResponse.String(), fmt.Errorf("opencode CLI exited with error: %w", err)
		}
		return "", fmt.Errorf("opencode CLI failed: %w", err)
	}

	return fullResponse.String(), nil
}
