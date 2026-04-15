package backend

import (
	"fmt"
	"strings"
)

// Backend defines the interface for communicating with Claude.
// Implementations handle prompt delivery and streaming response capture.
type Backend interface {
	// Query sends a prompt (with conversation history) and streams the response.
	// onChunk is called for each text chunk as it arrives.
	// Returns the full response text and any error.
	Query(messages []Message, model string, onChunk func(text string)) (string, error)
}

// Message represents a single message in a conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// extractPromptAndHistory splits a message list into the last user message
// (prompt) and a formatted history of preceding messages.
func extractPromptAndHistory(messages []Message) (prompt string, history []string) {
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
	return
}

// formatHistory joins history lines with a header, suitable for prepending to a prompt.
func formatHistory(history []string) string {
	return "Previous conversation:\n" + strings.Join(history, "\n")
}
