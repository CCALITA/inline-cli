package backend

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
