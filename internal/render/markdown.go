package render

import (
	"bytes"
	"os"
	"strconv"
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
		resolveGlamourStyle(),
		glamour.WithWordWrap(width),
	)
	return &Markdown{
		renderer: r,
	}
}

// resolveGlamourStyle picks a glamour style without sending OSC queries
// to the terminal. glamour.WithAutoStyle() triggers a termenv OSC 11
// background-color query that blocks for up to 5 seconds if the terminal
// doesn't respond. This function avoids that by using only env vars.
//
// Detection order:
//  1. GLAMOUR_STYLE env var (explicit user override)
//  2. COLORFGBG env var (passive terminal hint, no I/O)
//  3. Default to "dark" (safest assumption for developer terminals)
func resolveGlamourStyle() glamour.TermRendererOption {
	if style := os.Getenv("GLAMOUR_STYLE"); style != "" {
		return glamour.WithStylePath(style)
	}

	if colorFGBG := os.Getenv("COLORFGBG"); colorFGBG != "" {
		if isDarkFromCOLORFGBG(colorFGBG) {
			return glamour.WithStandardStyle("dark")
		}
		return glamour.WithStandardStyle("light")
	}

	return glamour.WithStandardStyle("dark")
}

// isDarkFromCOLORFGBG parses the COLORFGBG environment variable (format: "fg;bg")
// and returns true if the background indicates a dark terminal.
// Uses vim's heuristic: ANSI colors 0-6 are dark backgrounds, 7+ are light.
func isDarkFromCOLORFGBG(value string) bool {
	parts := strings.Split(value, ";")
	if len(parts) < 2 {
		return true
	}
	bg, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return true
	}
	return bg >= 0 && bg <= 6
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
