package backend

import (
	"os"
	"strings"
	"testing"
)

func writeFakeGeminiScript(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fake-gemini-*")
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

func TestNewGeminiBackend(t *testing.T) {
	b, err := NewGeminiBackend("/usr/local/bin/gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.configuredPath != "/usr/local/bin/gemini" {
		t.Errorf("configuredPath = %q, want %q", b.configuredPath, "/usr/local/bin/gemini")
	}
}

func TestNewGeminiBackend_EmptyPath(t *testing.T) {
	// Empty path is allowed — resolution happens at query time.
	b, err := NewGeminiBackend("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.configuredPath != "" {
		t.Errorf("configuredPath = %q, want empty", b.configuredPath)
	}
}

func TestGeminiBackend_ResolveBinary_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	b := &GeminiBackend{}

	_, err := b.resolveBinary()
	if err == nil {
		t.Fatal("expected error when gemini not in PATH, got nil")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error = %q, want it to contain 'not found in PATH'", err.Error())
	}
}

func TestGeminiBackend_Query_NoUserMessage(t *testing.T) {
	b := &GeminiBackend{configuredPath: "/bin/echo"}

	messages := []Message{
		{Role: "assistant", Content: "I can help."},
	}

	_, err := b.Query(messages, "", nil)
	if err == nil {
		t.Fatal("expected error for no user message, got nil")
	}
	if !strings.Contains(err.Error(), "no user message found") {
		t.Errorf("error = %q, want 'no user message found'", err.Error())
	}
}

func TestGeminiBackend_Query_SingleMessage(t *testing.T) {
	script := writeFakeGeminiScript(t, `printf "Hello from gemini"`)
	b := &GeminiBackend{configuredPath: script}

	messages := []Message{
		{Role: "user", Content: "Say hello"},
	}

	var chunks []string
	result, err := b.Query(messages, "", func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello from gemini" {
		t.Errorf("result = %q, want %q", result, "Hello from gemini")
	}
	joined := strings.Join(chunks, "")
	if joined != "Hello from gemini" {
		t.Errorf("joined chunks = %q, want %q", joined, "Hello from gemini")
	}
}

func TestGeminiBackend_Query_WithHistory(t *testing.T) {
	// Fake script writes all args to a file so we can inspect them.
	argFile := t.TempDir() + "/args.txt"
	script := writeFakeGeminiScript(t, `
printf '%s\n' "$@" > `+argFile+`
printf 'ok'
`)
	b := &GeminiBackend{configuredPath: script}

	messages := []Message{
		{Role: "user", Content: "What is Go?"},
		{Role: "assistant", Content: "A programming language."},
		{Role: "user", Content: "Tell me more"},
	}

	result, err := b.Query(messages, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}

	data, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("failed to read args file: %v", err)
	}
	args := string(data)

	// Should have -p flag with prompt containing history.
	if !strings.Contains(args, "-p") {
		t.Errorf("args should contain '-p', got %q", args)
	}
	if !strings.Contains(args, "Previous conversation:") {
		t.Errorf("args should contain history, got %q", args)
	}
	if !strings.Contains(args, "User: What is Go?") {
		t.Errorf("args should contain history entry, got %q", args)
	}
	// Should have -o text flags.
	if !strings.Contains(args, "-o") || !strings.Contains(args, "text") {
		t.Errorf("args should contain '-o text', got %q", args)
	}
}

func TestGeminiBackend_Query_BinaryFails(t *testing.T) {
	script := writeFakeGeminiScript(t, `echo "API error: invalid request" >&2; exit 1`)
	b := &GeminiBackend{configuredPath: script}

	messages := []Message{{Role: "user", Content: "hello"}}

	_, err := b.Query(messages, "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "API error: invalid request") {
		t.Errorf("error = %q, want stderr content surfaced", err.Error())
	}
}

func TestExtractError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{
			name:   "empty",
			stderr: "",
			want:   "",
		},
		{
			name:   "skill conflict only",
			stderr: "Skill conflict detected: foo vs bar\n",
			want:   "",
		},
		{
			name:   "skill conflict with real error",
			stderr: "Skill conflict detected: foo vs bar\nError when talking to Gemini API: 400\n",
			want:   "Error when talking to Gemini API: 400",
		},
		{
			name:   "stack trace skipped",
			stderr: "Something failed\nat /usr/lib/node.js:10:5\nat main.go:20\n",
			want:   "Something failed",
		},
		{
			name:   "prefers API error over fallback",
			stderr: "some warning\nINVALID_ARGUMENT: bad field\n",
			want:   "INVALID_ARGUMENT: bad field",
		},
		{
			name:   "PERMISSION_DENIED",
			stderr: "PERMISSION_DENIED: not authorized\n",
			want:   "PERMISSION_DENIED: not authorized",
		},
		{
			name:   "UNAUTHENTICATED",
			stderr: "UNAUTHENTICATED: missing credentials\n",
			want:   "UNAUTHENTICATED: missing credentials",
		},
		{
			name:   "fallback to first non-skipped line",
			stderr: "Skill conflict detected: x\nat foo.js:1\nactual problem here\n",
			want:   "actual problem here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractError(tt.stderr)
			if got != tt.want {
				t.Errorf("extractError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeminiBackend_Query_BinaryFailsNoStderr(t *testing.T) {
	script := writeFakeGeminiScript(t, `exit 1`)
	b := &GeminiBackend{configuredPath: script}

	messages := []Message{{Role: "user", Content: "hello"}}

	_, err := b.Query(messages, "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gemini CLI failed") {
		t.Errorf("error = %q, want 'gemini CLI failed'", err.Error())
	}
}

func TestGeminiBackend_Query_PartialOutputThenFail(t *testing.T) {
	// When gemini produces output but exits non-zero (e.g. skill conflict
	// warning), the response should be returned without an error.
	script := writeFakeGeminiScript(t, `
printf "partial"
echo "something went wrong" >&2
exit 1
`)
	b := &GeminiBackend{configuredPath: script}

	messages := []Message{{Role: "user", Content: "hello"}}

	var chunks []string
	result, err := b.Query(messages, "", func(text string) {
		chunks = append(chunks, text)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v (should succeed when output exists)", err)
	}
	if result != "partial" {
		t.Errorf("result = %q, want %q", result, "partial")
	}
}
