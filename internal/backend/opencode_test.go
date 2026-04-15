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

// fakeOpenCodeJSON returns a shell script body that emits opencode-style
// newline-delimited JSON events to stdout, producing the given text.
func fakeOpenCodeJSON(text string) string {
	// Emit step_start, text event(s), step_finish — matching real opencode output.
	return `printf '{"type":"step_start","part":{}}\n'
printf '{"type":"text","part":{"text":"` + text + `"}}\n'
printf '{"type":"step_finish","part":{}}\n'
`
}

func TestNewOpenCodeBackend_WithPath(t *testing.T) {
	path := "/usr/local/bin/opencode"
	b, err := NewOpenCodeBackend(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.configuredPath != path {
		t.Errorf("configuredPath = %q, want %q", b.configuredPath, path)
	}
}

func TestOpenCodeBackend_Query_NotInPath(t *testing.T) {
	// With empty path and opencode not in PATH, Query should fail.
	t.Setenv("PATH", t.TempDir())

	b, err := NewOpenCodeBackend("")
	if err != nil {
		t.Fatalf("unexpected error creating backend: %v", err)
	}

	messages := []Message{{Role: "user", Content: "hello"}}
	_, err = b.Query(messages, "", nil)
	if err == nil {
		t.Fatal("expected error when opencode is not in PATH, got nil")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not found in PATH")
	}
}

func TestOpenCodeBackend_Query_NoUserMessage(t *testing.T) {
	b := &OpenCodeBackend{configuredPath: "/bin/echo"}

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
	// Fake script that emits a JSON text event.
	script := writeFakeScript(t, fakeOpenCodeJSON("Hello from opencode"))

	b := &OpenCodeBackend{configuredPath: script}

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
	joined := strings.Join(chunks, "")
	if joined != "Hello from opencode" {
		t.Errorf("joined chunks = %q, want %q", joined, "Hello from opencode")
	}
}

func TestOpenCodeBackend_Query_MultipleTextEvents(t *testing.T) {
	// Fake script that emits multiple text events (like streaming chunks).
	script := writeFakeScript(t, `
printf '{"type":"step_start","part":{}}\n'
printf '{"type":"text","part":{"text":"Hello"}}\n'
printf '{"type":"text","part":{"text":" world"}}\n'
printf '{"type":"step_finish","part":{}}\n'
`)

	b := &OpenCodeBackend{configuredPath: script}

	messages := []Message{
		{Role: "user", Content: "Say hello world"},
	}

	var chunks []string
	result, err := b.Query(messages, "", func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello world" {
		t.Errorf("result = %q, want %q", result, "Hello world")
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "Hello" || chunks[1] != " world" {
		t.Errorf("chunks = %v, want [Hello, ' world']", chunks)
	}
}

func TestOpenCodeBackend_Query_WithHistory(t *testing.T) {
	// Fake script that writes the last positional arg (the prompt) to a temp file
	// so we can inspect it, then emits a simple JSON text event.
	argFile := t.TempDir() + "/prompt_arg.txt"
	script := writeFakeScript(t, `
# Get the last argument (the prompt).
for arg; do :; done
printf '%s' "$arg" > `+argFile+`
printf '{"type":"text","part":{"text":"ok"}}\n'
`)

	b := &OpenCodeBackend{configuredPath: script}

	messages := []Message{
		{Role: "user", Content: "What is Go?"},
		{Role: "assistant", Content: "Go is a programming language."},
		{Role: "user", Content: "Tell me more"},
	}

	result, err := b.Query(messages, "test-model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}

	// Read what was actually passed as the prompt argument.
	data, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("failed to read prompt arg file: %v", err)
	}
	prompt := string(data)

	// The prompt should contain the history prefix and the current request.
	if !strings.Contains(prompt, "Previous conversation:") {
		t.Errorf("prompt should contain 'Previous conversation:', got %q", prompt)
	}
	if !strings.Contains(prompt, "User: What is Go?") {
		t.Errorf("prompt should contain 'User: What is Go?', got %q", prompt)
	}
	if !strings.Contains(prompt, "Assistant: Go is a programming language.") {
		t.Errorf("prompt should contain 'Assistant: Go is a programming language.', got %q", prompt)
	}
	if !strings.Contains(prompt, "Current request:") {
		t.Errorf("prompt should contain 'Current request:', got %q", prompt)
	}
	if !strings.Contains(prompt, "Tell me more") {
		t.Errorf("prompt should contain 'Tell me more', got %q", prompt)
	}
}

func TestOpenCodeBackend_Query_IgnoresNonTextEvents(t *testing.T) {
	// Emit various event types — only "text" events should produce output.
	script := writeFakeScript(t, `
printf '{"type":"step_start","part":{}}\n'
printf '{"type":"tool_call","part":{"name":"bash","input":"ls"}}\n'
printf '{"type":"text","part":{"text":"actual output"}}\n'
printf '{"type":"step_finish","part":{}}\n'
`)

	b := &OpenCodeBackend{configuredPath: script}

	messages := []Message{{Role: "user", Content: "hello"}}

	result, err := b.Query(messages, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "actual output" {
		t.Errorf("result = %q, want %q", result, "actual output")
	}
}

func TestOpenCodeBackend_Query_BinaryFails(t *testing.T) {
	// Fake script that exits with non-zero status and no JSON output.
	script := writeFakeScript(t, `exit 1`)

	b := &OpenCodeBackend{configuredPath: script}

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
	// Fake script that writes a text event then exits with error.
	script := writeFakeScript(t, `
printf '{"type":"text","part":{"text":"partial output"}}\n'
exit 1
`)

	b := &OpenCodeBackend{configuredPath: script}

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

func TestOpenCodeBackend_Query_ErrorEvent(t *testing.T) {
	// Fake script that emits a JSON error event (e.g. invalid model).
	script := writeFakeScript(t, `
printf '{"type":"error","error":{"name":"UnknownError","data":{"message":"Model not found: bad-model"}}}\n'
`)

	b := &OpenCodeBackend{configuredPath: script}

	messages := []Message{
		{Role: "user", Content: "hello"},
	}

	_, err := b.Query(messages, "", nil)
	if err == nil {
		t.Fatal("expected error from error event, got nil")
	}
	if !strings.Contains(err.Error(), "Model not found") {
		t.Errorf("error = %q, want it to contain 'Model not found'", err.Error())
	}
}

func TestOpenCodeBackend_Query_ErrorEventWithName(t *testing.T) {
	// Error event with name but no message falls back to name.
	script := writeFakeScript(t, `
printf '{"type":"error","error":{"name":"ProviderError","data":{}}}\n'
`)

	b := &OpenCodeBackend{configuredPath: script}
	messages := []Message{{Role: "user", Content: "hello"}}

	_, err := b.Query(messages, "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ProviderError") {
		t.Errorf("error = %q, want it to contain 'ProviderError'", err.Error())
	}
}
