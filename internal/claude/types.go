package claude

// MessageRequest is the request body for the Claude Messages API.
type MessageRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream"`
	System    string    `json:"system,omitempty"`
}

// Message represents a single message in a conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamEvent represents a parsed SSE event from the Claude streaming API.
type StreamEvent struct {
	Type  string
	Delta *Delta
	Error *APIError
	Usage *StreamUsage
}

// Delta contains the incremental text content from a content_block_delta event.
type Delta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// StreamUsage contains token usage from a message_delta event.
type StreamUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// APIError represents an error response from the Claude API.
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Type + ": " + e.Message
}

// Internal types for JSON parsing of SSE data payloads.

type sseMessageStart struct {
	Type    string `json:"type"`
	Message struct {
		ID    string `json:"id"`
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type sseContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta Delta  `json:"delta"`
}

type sseMessageDelta struct {
	Type  string `json:"type"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type sseError struct {
	Type  string   `json:"type"`
	Error APIError `json:"error"`
}
