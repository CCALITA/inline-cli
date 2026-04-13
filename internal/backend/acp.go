package backend

import "fmt"

// TODO: Implement ACP (Agent Communication Protocol) backend.
// ACP is a protocol for agent-to-agent communication.
// This is a placeholder — implementation will follow once the ACP spec is available.

// ACPBackend connects to Claude via the Agent Communication Protocol.
type ACPBackend struct{}

func NewACPBackend() (*ACPBackend, error) {
	return nil, fmt.Errorf("ACP backend is not yet implemented")
}

func (b *ACPBackend) Query(messages []Message, model string, onChunk func(text string)) (string, error) {
	return "", fmt.Errorf("ACP backend is not yet implemented")
}
