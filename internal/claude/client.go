package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL   = "https://api.anthropic.com"
	anthropicVersion = "2023-06-01"
)

// Client communicates with the Claude Messages API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Claude API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			// No overall timeout — streaming responses can be long.
			// Connect timeout is handled by the transport.
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
				MaxIdleConns:          5,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}
}

// SetBaseURL overrides the API base URL (useful for testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// CreateMessageStream sends a streaming message request and returns a channel
// of parsed SSE events.
func (c *Client) CreateMessageStream(req MessageRequest) (<-chan StreamEvent, error) {
	req.Stream = true
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, c.handleErrorResponse(resp)
	}

	// Parse SSE stream. The body is kept open — ParseSSEStream closes it when done.
	ch := ParseSSEStream(resp.Body)

	// Wrap to ensure body is closed when the channel drains.
	wrapped := make(chan StreamEvent, 64)
	go func() {
		defer resp.Body.Close()
		defer close(wrapped)
		for event := range ch {
			wrapped <- event
		}
	}()

	return wrapped, nil
}

func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var apiErr struct {
		Error APIError `json:"error"`
	}
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, apiErr.Error.Message)
	}

	return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
}
