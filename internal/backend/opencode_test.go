package backend

import (
	"os"
	"strings"
	"testing"
)

// writeFakeScript creates a temporary shell script with the given content and
// returns the path to the executable file.
func writeFakeScript(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fake-opencode-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("#!/bin/sh\n" + content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0755); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestNewOpenCodeBackend_WithPath(t *testing.T) {
	path := "/usr/local/bin/opencode"
	b, err := NewOpenCodeBackend(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.binaryPath != path {
		t.Errorf("binaryPath = %q, want %q", b.binaryPath, path)
	}
}

func TestNewOpenCodeBackend_EmptyPath_NotFound(t *testing.T) {
	// Ensure "opencode" is not in PATH by using a minimal PATH.
	t.Setenv("PATH", t.TempDir())

	_, err := NewOpenCodeBackend("")
	if err == nil {
		t.Fatal("expected error when opencode is not in PATH, got nil")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not found in PATH")
	}
}

func TestOpenCodeBackend_Query_NoUserMessage(t *testing.T) {
	b := &OpenCodeBackend{binaryPath: "/bin/echo"}

	// Only assistant messages, no user message at the end.
	messages := []Message{
		{Role: "assistant", Content: "I can help with that."},
		{Role: "assistant", Content: "Here is more info."},
	}

	_, err := b.Query(messages, "test-model", nil)
	if err == nil {
		t.Fatal("expected error for no user message, got nil")
	}
	if !strings.Contains(err.Error(), "no user message found") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "no user message found")
	}
}

func TestOpenCodeBackend_Query_SingleMessage(t *testing.T) {
	// Fake script that echoes a known response.
	script := writeFakeScript(t, `printf "Hello from opencode"`)

	b := &OpenCodeBackend{binaryPath: script}

	messages := []Message{
		{Role: "user", Content: "Say hello"},
	}

	var chunks []string
	result, err := b.Query(messages, "test-model", func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello from opencode" {
		t.Errorf("result = %q, want %q", result, "Hello from opencode")
	}
	// onChunk should have been called at least once, and the joined chunks
	// should reconstruct the full response.
	joined := strings.Join(chunks, "")
	if joined != "Hello from opencode" {
		t.Errorf("joined chunks = %q, want %q", joined, "Hello from opencode")
	}
}

func TestOpenCodeBackend_Query_WithHistory(t *testing.T) {
	// Fake script that prints the second arg (the prompt in `opencode run <prompt>`).
	script := writeFakeScript(t, `printf "%s" "$2"`)

	b := &OpenCodeBackend{binaryPath: script}

	messages := []Message{
		{Role: "user", Content: "What is Go?"},
		{Role: "assistant", Content: "Go is a programming language."},
		{Role: "user", Content: "Tell me more"},
	}

	var chunks []string
	result, err := b.Query(messages, "test-model", func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The prompt should contain the history prefix and the current request.
	if !strings.Contains(result, "Previous conversation:") {
		t.Errorf("result should contain 'Previous conversation:', got %q", result)
	}
	if !strings.Contains(result, "User: What is Go?") {
		t.Errorf("result should contain 'User: What is Go?', got %q", result)
	}
	if !strings.Contains(result, "Assistant: Go is a programming language.") {
		t.Errorf("result should contain 'Assistant: Go is a programming language.', got %q", result)
	}
	if !strings.Contains(result, "Current request:") {
		t.Errorf("result should contain 'Current request:', got %q", result)
	}
	if !strings.Contains(result, "Tell me more") {
		t.Errorf("result should contain 'Tell me more', got %q", result)
	}
}

func TestOpenCodeBackend_Query_BinaryFails(t *testing.T) {
	// Fake script that exits with non-zero status and no output.
	script := writeFakeScript(t, `exit 1`)

	b := &OpenCodeBackend{binaryPath: script}

	messages := []Message{
		{Role: "user", Content: "hello"},
	}

	_, err := b.Query(messages, "test-model", nil)
	if err == nil {
		t.Fatal("expected error when binary fails, got nil")
	}
	if !strings.Contains(err.Error(), "opencode CLI failed") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "opencode CLI failed")
	}
}

func TestOpenCodeBackend_Query_PartialOutputThenFail(t *testing.T) {
	// Fake script that writes partial output then exits with error.
	script := writeFakeScript(t, `
printf "partial output"
exit 1
`)

	b := &OpenCodeBackend{binaryPath: script}

	messages := []Message{
		{Role: "user", Content: "hello"},
	}

	var chunks []string
	result, err := b.Query(messages, "test-model", func(text string) {
		chunks = append(chunks, text)
	})

	// Both partial output and error should be returned.
	if err == nil {
		t.Fatal("expected error when binary exits non-zero, got nil")
	}
	if !strings.Contains(err.Error(), "opencode CLI exited with error") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "opencode CLI exited with error")
	}
	if result != "partial output" {
		t.Errorf("result = %q, want %q", result, "partial output")
	}
	joined := strings.Join(chunks, "")
	if joined != "partial output" {
		t.Errorf("joined chunks = %q, want %q", joined, "partial output")
	}
}
