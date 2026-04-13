package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiCyan    = "\033[36m"
	ansiYellow  = "\033[33m"
	ansiGreen   = "\033[32m"
)

// Renderer handles streaming terminal output.
type Renderer struct {
	out      io.Writer
	width    int
	mu       sync.Mutex
	started  bool
}

// NewRenderer creates a new terminal renderer.
func NewRenderer(out io.Writer) *Renderer {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}
	return &Renderer{
		out:   out,
		width: width,
	}
}

// StartResponse prints the response header/separator.
func (r *Renderer) StartResponse() {
	r.mu.Lock()
	defer r.mu.Unlock()

	sep := strings.Repeat("─", min(r.width, 60))
	fmt.Fprintf(r.out, "\n%s%s%s\n", ansiDim, sep, ansiReset)
	r.started = true
}

// WriteChunk writes a streaming text chunk to the terminal.
func (r *Renderer) WriteChunk(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		sep := strings.Repeat("─", min(r.width, 60))
		fmt.Fprintf(r.out, "\n%s%s%s\n", ansiDim, sep, ansiReset)
		r.started = true
	}

	fmt.Fprint(r.out, text)
}

// EndResponse prints the response footer.
func (r *Renderer) EndResponse() {
	r.mu.Lock()
	defer r.mu.Unlock()

	sep := strings.Repeat("─", min(r.width, 60))
	fmt.Fprintf(r.out, "\n%s%s%s\n", ansiDim, sep, ansiReset)
	r.started = false
}

// ShowThinking displays a thinking indicator.
func (r *Renderer) ShowThinking() {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.out, "%s%s thinking...%s", ansiDim, ansiCyan, ansiReset)
}

// ClearThinking removes the thinking indicator.
func (r *Renderer) ClearThinking() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Move cursor to start of line and clear.
	fmt.Fprint(r.out, "\r\033[K")
}

// ShowError displays an error message.
func (r *Renderer) ShowError(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.out, "\n%s%serror: %s%s\n", ansiBold, ansiYellow, msg, ansiReset)
}
