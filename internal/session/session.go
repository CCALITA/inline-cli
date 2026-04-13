package session

import (
	"fmt"
	"sync"

	"github.com/CCALITA/inline-cli/internal/backend"
)

// Session maintains a Claude conversation scoped to a directory.
type Session struct {
	Dir         string
	Messages    []backend.Message
	MaxMessages int

	mu sync.Mutex
}

// NewSession creates a new session for the given directory.
func NewSession(dir string, maxMessages int) *Session {
	return &Session{
		Dir:         dir,
		MaxMessages: maxMessages,
	}
}

// Query sends a prompt to the backend and streams the response via the callback.
// Returns the full assistant response text and any error.
func (s *Session) Query(b backend.Backend, model, prompt string, onChunk func(text string)) (string, error) {
	s.mu.Lock()
	s.Messages = append(s.Messages, backend.Message{Role: "user", Content: prompt})
	msgs := make([]backend.Message, len(s.Messages))
	copy(msgs, s.Messages)
	s.mu.Unlock()

	fullResponse, err := b.Query(msgs, model, onChunk)
	if err != nil {
		// Remove the user message we just added since the request failed.
		s.mu.Lock()
		if len(s.Messages) > 0 {
			s.Messages = s.Messages[:len(s.Messages)-1]
		}
		s.mu.Unlock()
		return "", fmt.Errorf("query failed: %w", err)
	}

	// Append assistant response and enforce sliding window.
	s.mu.Lock()
	s.Messages = append(s.Messages, backend.Message{Role: "assistant", Content: fullResponse})
	s.trimMessages()
	s.mu.Unlock()

	return fullResponse, nil
}

// Reset clears the conversation history.
func (s *Session) Reset() {
	s.mu.Lock()
	s.Messages = nil
	s.mu.Unlock()
}

// MessageCount returns the number of messages in the conversation.
func (s *Session) MessageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Messages)
}

// trimMessages enforces the sliding window by dropping oldest messages
// when the count exceeds MaxMessages. Always keeps messages in pairs
// (user + assistant) to maintain conversation coherence.
func (s *Session) trimMessages() {
	if s.MaxMessages <= 0 || len(s.Messages) <= s.MaxMessages {
		return
	}
	excess := len(s.Messages) - s.MaxMessages
	if excess%2 != 0 {
		excess++
	}
	if excess >= len(s.Messages) {
		excess = len(s.Messages) - 2
	}
	if excess > 0 {
		s.Messages = s.Messages[excess:]
	}
}
