package backend

import (
	"fmt"
	"strings"

	"github.com/CCALITA/inline-cli/internal/claude"
)

// APIBackend connects directly to the Anthropic Messages API.
type APIBackend struct {
	client *claude.Client
}

// NewAPIBackend creates a backend that talks to the Anthropic API directly.
func NewAPIBackend(apiKey string, baseURL string) (*APIBackend, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for api backend")
	}
	c := claude.NewClient(apiKey)
	if baseURL != "" {
		c.SetBaseURL(baseURL)
	}
	return &APIBackend{client: c}, nil
}

func (b *APIBackend) Query(messages []Message, model string, onChunk func(text string)) (string, error) {
	// Convert to claude.Message.
	claudeMsgs := make([]claude.Message, len(messages))
	for i, m := range messages {
		claudeMsgs[i] = claude.Message{Role: m.Role, Content: m.Content}
	}

	req := claude.MessageRequest{
		Model:    model,
		Messages: claudeMsgs,
	}

	ch, err := b.client.CreateMessageStream(req)
	if err != nil {
		return "", fmt.Errorf("API stream failed: %w", err)
	}

	var fullResponse strings.Builder
	for event := range ch {
		switch event.Type {
		case "content_block_delta":
			if event.Delta != nil {
				fullResponse.WriteString(event.Delta.Text)
				if onChunk != nil {
					onChunk(event.Delta.Text)
				}
			}
		case "error":
			if event.Error != nil {
				return fullResponse.String(), event.Error
			}
		}
	}

	return fullResponse.String(), nil
}
