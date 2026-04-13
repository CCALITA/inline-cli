package claude

import (
	"strings"
	"testing"
)

func TestParseSSEStream_ContentDelta(t *testing.T) {
	input := `event: message_start
data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":25}}}

event: content_block_start
data: {"type":"content_block_start","index":0}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","usage":{"output_tokens":12}}

event: message_stop
data: {"type":"message_stop"}

`

	ch := ParseSSEStream(strings.NewReader(input))

	var events []StreamEvent
	for event := range ch {
		events = append(events, event)
	}

	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	// message_start
	if events[0].Type != "message_start" {
		t.Errorf("event[0].Type = %q, want message_start", events[0].Type)
	}
	if events[0].Usage == nil || events[0].Usage.InputTokens != 25 {
		t.Errorf("event[0].Usage.InputTokens = %v, want 25", events[0].Usage)
	}

	// content_block_delta "Hello"
	if events[1].Type != "content_block_delta" {
		t.Errorf("event[1].Type = %q, want content_block_delta", events[1].Type)
	}
	if events[1].Delta == nil || events[1].Delta.Text != "Hello" {
		t.Errorf("event[1].Delta.Text = %q, want Hello", events[1].Delta.Text)
	}

	// content_block_delta " world"
	if events[2].Delta == nil || events[2].Delta.Text != " world" {
		t.Errorf("event[2].Delta.Text = %q, want ' world'", events[2].Delta.Text)
	}

	// message_delta
	if events[3].Type != "message_delta" {
		t.Errorf("event[3].Type = %q, want message_delta", events[3].Type)
	}
	if events[3].Usage == nil || events[3].Usage.OutputTokens != 12 {
		t.Errorf("event[3].Usage.OutputTokens = %v, want 12", events[3].Usage)
	}

	// message_stop
	if events[4].Type != "message_stop" {
		t.Errorf("event[4].Type = %q, want message_stop", events[4].Type)
	}
}

func TestParseSSEStream_Error(t *testing.T) {
	input := `event: error
data: {"type":"error","error":{"type":"overloaded_error","message":"API is overloaded"}}

`

	ch := ParseSSEStream(strings.NewReader(input))

	events := drain(ch)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "error" {
		t.Errorf("event.Type = %q, want error", events[0].Type)
	}
	if events[0].Error == nil || events[0].Error.Message != "API is overloaded" {
		t.Errorf("event.Error = %v, want 'API is overloaded'", events[0].Error)
	}
}

func TestParseSSEStream_EmptyStream(t *testing.T) {
	ch := ParseSSEStream(strings.NewReader(""))
	events := drain(ch)
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestParseSSEStream_PingIgnored(t *testing.T) {
	input := `event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

event: message_stop
data: {"type":"message_stop"}

`

	ch := ParseSSEStream(strings.NewReader(input))
	events := drain(ch)
	if len(events) != 2 {
		t.Fatalf("expected 2 events (delta + stop), got %d", len(events))
	}
	if events[0].Delta.Text != "hi" {
		t.Errorf("got text %q, want hi", events[0].Delta.Text)
	}
	if events[1].Type != "message_stop" {
		t.Errorf("got type %q, want message_stop", events[1].Type)
	}
}

func drain(ch <-chan StreamEvent) []StreamEvent {
	var out []StreamEvent
	for e := range ch {
		out = append(out, e)
	}
	return out
}
