package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ParseSSEStream reads an SSE stream from r and sends parsed events to the
// returned channel. The channel is closed when the stream ends or on error.
func ParseSSEStream(r io.Reader) <-chan StreamEvent {
	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)

		var eventType string
		var dataLines []string

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				// Empty line = end of event. Dispatch if we have data.
				if len(dataLines) > 0 {
					data := strings.Join(dataLines, "\n")
					if event, ok := parseEventData(eventType, data); ok {
						ch <- event
						if event.Type == "error" {
							return
						}
					}
				}
				eventType = ""
				dataLines = nil
				continue
			}

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
			} else if line == "data:" {
				dataLines = append(dataLines, "")
			}
			// Ignore comments (lines starting with ':') and other fields.
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{
				Type:  "error",
				Error: &APIError{Type: "stream_error", Message: fmt.Sprintf("stream read error: %v", err)},
			}
		}
	}()

	return ch
}

func parseEventData(eventType, data string) (StreamEvent, bool) {
	switch eventType {
	case "message_start":
		var msg sseMessageStart
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			return StreamEvent{}, false
		}
		return StreamEvent{
			Type: "message_start",
			Usage: &StreamUsage{
				InputTokens: msg.Message.Usage.InputTokens,
			},
		}, true

	case "content_block_delta":
		var delta sseContentBlockDelta
		if err := json.Unmarshal([]byte(data), &delta); err != nil {
			return StreamEvent{}, false
		}
		return StreamEvent{
			Type:  "content_block_delta",
			Delta: &delta.Delta,
		}, true

	case "message_delta":
		var msg sseMessageDelta
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			return StreamEvent{}, false
		}
		return StreamEvent{
			Type: "message_delta",
			Usage: &StreamUsage{
				OutputTokens: msg.Usage.OutputTokens,
			},
		}, true

	case "message_stop":
		return StreamEvent{Type: "message_stop"}, true

	case "error":
		var errMsg sseError
		if err := json.Unmarshal([]byte(data), &errMsg); err != nil {
			return StreamEvent{
				Type:  "error",
				Error: &APIError{Type: "parse_error", Message: data},
			}, true
		}
		return StreamEvent{
			Type:  "error",
			Error: &errMsg.Error,
		}, true

	case "content_block_start", "content_block_stop", "ping":
		// These events don't carry useful data for our purposes.
		return StreamEvent{}, false

	default:
		return StreamEvent{}, false
	}
}
