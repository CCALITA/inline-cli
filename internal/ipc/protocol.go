package ipc

import "encoding/json"

// Request types
const (
	TypeQuery       = "query"
	TypeStatus      = "status"
	TypeStopSession = "stop_session"
	TypeStopDaemon  = "stop_daemon"
)

// Response types
const (
	TypeStart = "start"
	TypeChunk = "chunk"
	TypeDone  = "done"
	TypeError = "error"
)

// Request is sent from the CLI client to the daemon over the Unix socket.
type Request struct {
	Type      string `json:"type"`
	Dir       string `json:"dir,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Response is streamed from the daemon back to the CLI client as NDJSON.
type Response struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Usage     *Usage `json:"usage,omitempty"`
}

// Usage contains token usage information from the Claude API.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// DaemonStatus is returned by the status command.
type DaemonStatus struct {
	Running  bool            `json:"running"`
	PID      int             `json:"pid,omitempty"`
	Sessions []SessionStatus `json:"sessions,omitempty"`
}

// SessionStatus describes an active session.
type SessionStatus struct {
	Dir          string `json:"dir"`
	MessageCount int    `json:"message_count"`
	LastUsed     string `json:"last_used"`
}

// EncodeRequest serializes a request to JSON bytes with a trailing newline.
func EncodeRequest(r Request) ([]byte, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodeRequest deserializes a request from JSON bytes.
func DecodeRequest(data []byte) (Request, error) {
	var r Request
	err := json.Unmarshal(data, &r)
	return r, err
}

// EncodeResponse serializes a response to JSON bytes with a trailing newline.
func EncodeResponse(r Response) ([]byte, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodeResponse deserializes a response from JSON bytes.
func DecodeResponse(data []byte) (Response, error) {
	var r Response
	err := json.Unmarshal(data, &r)
	return r, err
}
