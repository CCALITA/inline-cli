package render

import (
	"bytes"
	"strings"

	"github.com/charmbracelet/glamour"
)

// Markdown renders markdown text to styled terminal output.
type Markdown struct {
	renderer *glamour.TermRenderer
	buffer   strings.Builder
}

// NewMarkdown creates a new markdown renderer.
func NewMarkdown(width int) *Markdown {
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	return &Markdown{
		renderer: r,
	}
}

// RenderFull renders a complete markdown string.
func (m *Markdown) RenderFull(text string) string {
	if m.renderer == nil {
		return text
	}
	out, err := m.renderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(out, "\n")
}

// RenderStreaming buffers streaming chunks and renders complete lines.
// Returns the rendered output ready to display (may be empty if buffering).
func (m *Markdown) RenderStreaming(chunk string) string {
	m.buffer.WriteString(chunk)
	content := m.buffer.String()

	// If we have a complete line (ends with newline), render up to that point.
	lastNewline := strings.LastIndex(content, "\n")
	if lastNewline < 0 {
		return ""
	}

	// Check if we're in a code block — if so, wait for it to close.
	toRender := content[:lastNewline+1]
	if isInsideCodeBlock(toRender) {
		return ""
	}

	m.buffer.Reset()
	m.buffer.WriteString(content[lastNewline+1:])

	return m.RenderFull(toRender)
}

// Flush renders any remaining buffered content.
func (m *Markdown) Flush() string {
	if m.buffer.Len() == 0 {
		return ""
	}
	content := m.buffer.String()
	m.buffer.Reset()
	return m.RenderFull(content)
}

// isInsideCodeBlock checks if the text has an unclosed code fence.
func isInsideCodeBlock(text string) bool {
	lines := bytes.Split([]byte(text), []byte("\n"))
	inBlock := false
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("```")) {
			inBlock = !inBlock
		}
	}
	return inBlock
}
