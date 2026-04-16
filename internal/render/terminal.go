package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// Renderer handles streaming terminal output.
type Renderer struct {
	out     io.Writer
	width   int
	color   bool
	mu      sync.Mutex
	started bool
}

// NewRenderer creates a new terminal renderer.
func NewRenderer(out io.Writer) *Renderer {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}
	color := shouldUseColor()
	return &Renderer{
		out:   out,
		width: width,
		color: color,
	}
}

// Width returns the detected terminal width.
func (r *Renderer) Width() int {
	return r.width
}

// shouldUseColor checks whether ANSI color output should be used.
func shouldUseColor() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// ColorEnabled returns whether ANSI color output should be used.
// Checks NO_COLOR env var, TERM=dumb, and whether stdout is a terminal.
func ColorEnabled() bool {
	return shouldUseColor()
}

// Green wraps text in green ANSI codes if color is enabled.
func Green(text string) string {
	if !shouldUseColor() {
		return text
	}
	return "\033[32m" + text + "\033[0m"
}

// Red wraps text in red ANSI codes if color is enabled.
func Red(text string) string {
	if !shouldUseColor() {
		return text
	}
	return "\033[31m" + text + "\033[0m"
}

func (r *Renderer) ansi(code string) string {
	if !r.color {
		return ""
	}
	return code
}

// StartResponse prints the response header/separator.
func (r *Renderer) StartResponse() {
	r.mu.Lock()
	defer r.mu.Unlock()

	sep := strings.Repeat("─", min(r.width, 60))
	fmt.Fprintf(r.out, "\n%s%s%s\n", r.ansi("\033[2m"), sep, r.ansi("\033[0m"))
	r.started = true
}

// WriteChunk writes a streaming text chunk to the terminal.
func (r *Renderer) WriteChunk(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		sep := strings.Repeat("─", min(r.width, 60))
		fmt.Fprintf(r.out, "\n%s%s%s\n", r.ansi("\033[2m"), sep, r.ansi("\033[0m"))
		r.started = true
	}

	fmt.Fprint(r.out, text)
}

// EndResponse prints the response footer.
func (r *Renderer) EndResponse() {
	r.mu.Lock()
	defer r.mu.Unlock()

	sep := strings.Repeat("─", min(r.width, 60))
	fmt.Fprintf(r.out, "\n%s%s%s\n", r.ansi("\033[2m"), sep, r.ansi("\033[0m"))
	r.started = false
}

// ShowThinking displays a thinking indicator.
func (r *Renderer) ShowThinking() {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.out, "%s%s thinking...%s", r.ansi("\033[2m"), r.ansi("\033[36m"), r.ansi("\033[0m"))
}

// ClearThinking removes the thinking indicator.
func (r *Renderer) ClearThinking() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.color {
		// Move cursor to start of line and clear (terminal only).
		fmt.Fprint(r.out, "\r\033[K")
	} else {
		fmt.Fprintln(r.out)
	}
}

// ShowError displays an error message.
func (r *Renderer) ShowError(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.out, "\n%s%serror: %s%s\n", r.ansi("\033[1m"), r.ansi("\033[31m"), msg, r.ansi("\033[0m"))
}
