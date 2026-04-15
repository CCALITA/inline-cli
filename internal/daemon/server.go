package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CCALITA/inline-cli/internal/backend"
	"github.com/CCALITA/inline-cli/internal/config"
	"github.com/CCALITA/inline-cli/internal/ipc"
	"github.com/CCALITA/inline-cli/internal/session"
)

// Server listens on a Unix domain socket and handles IPC requests.
type Server struct {
	cfg      config.Config
	listener net.Listener
	manager  *session.Manager
	backend  backend.Backend
}

// NewServer creates a new daemon server.
func NewServer(cfg config.Config) (*Server, error) {
	b, err := createBackend(cfg)
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:     cfg,
		manager: session.NewManager(cfg.MaxMessages, time.Duration(cfg.MaxSessionIdleMinutes)*time.Minute),
		backend: b,
	}, nil
}

// createBackend creates the appropriate backend based on config.
func createBackend(cfg config.Config) (backend.Backend, error) {
	switch cfg.Backend {
	case "cli":
		return backend.NewCLIBackend(cfg.CLIPath)
	case "gemini":
		return backend.NewGeminiBackend(cfg.GeminiPath)
	case "opencode":
		return backend.NewOpenCodeBackend(cfg.OpenCodePath)
	case "acp":
		return backend.NewACPBackend()
	case "api", "":
		return backend.NewAPIBackend(cfg.APIKey, cfg.APIBaseURL)
	default:
		return nil, fmt.Errorf("unknown backend: %q (supported: api, cli, gemini, opencode, acp)", cfg.Backend)
	}
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run() error {
	os.Remove(s.cfg.SocketPath)

	var err error
	s.listener, err = net.Listen("unix", s.cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.cfg.SocketPath, err)
	}

	os.Chmod(s.cfg.SocketPath, 0600)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down daemon...")
		s.Shutdown()
	}()

	log.Printf("daemon listening on %s (backend: %s)", s.cfg.SocketPath, s.cfg.BackendName())

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			break
		}
		go s.handleConnection(conn)
	}

	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() {
	s.manager.Close()
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.cfg.SocketPath)
	if pidFile := os.Getenv("INLINE_CLI_PID_FILE"); pidFile != "" {
		os.Remove(pidFile)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	if !scanner.Scan() {
		return
	}

	req, err := ipc.DecodeRequest(scanner.Bytes())
	if err != nil {
		s.writeError(conn, "invalid_request", fmt.Sprintf("failed to decode request: %v", err))
		return
	}

	switch req.Type {
	case ipc.TypeQuery:
		s.handleQuery(conn, req)
	case ipc.TypeStatus:
		s.handleStatus(conn)
	case ipc.TypeStopSession:
		s.handleStopSession(conn, req)
	case ipc.TypeStopDaemon:
		s.writeResponse(conn, ipc.Response{Type: ipc.TypeDone})
		go func() {
			time.Sleep(100 * time.Millisecond)
			s.Shutdown()
			os.Exit(0)
		}()
	default:
		s.writeError(conn, "unknown_request", fmt.Sprintf("unknown request type: %s", req.Type))
	}
}

func (s *Server) handleQuery(conn net.Conn, req ipc.Request) {
	if req.Dir == "" {
		s.writeError(conn, "invalid_request", "dir is required")
		return
	}
	if req.Prompt == "" {
		s.writeError(conn, "invalid_request", "prompt is required")
		return
	}

	sess := s.manager.GetOrCreate(req.Dir)

	s.writeResponse(conn, ipc.Response{
		Type:      ipc.TypeStart,
		RequestID: req.RequestID,
	})

	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	_, err := sess.Query(s.backend, s.cfg.Model, req.Prompt, func(text string) {
		s.writeResponse(conn, ipc.Response{
			Type: ipc.TypeChunk,
			Text: text,
		})
	})

	if err != nil {
		s.writeError(conn, "query_error", err.Error())
		return
	}

	s.writeResponse(conn, ipc.Response{Type: ipc.TypeDone})
}

func (s *Server) handleStatus(conn net.Conn) {
	sessions := s.manager.List()
	statusSessions := make([]ipc.SessionStatus, len(sessions))
	for i, sess := range sessions {
		statusSessions[i] = ipc.SessionStatus{
			Dir:          sess.Dir,
			MessageCount: sess.MessageCount,
			LastUsed:     sess.LastUsed.Format(time.RFC3339),
		}
	}

	status := ipc.DaemonStatus{
		Running:  true,
		PID:      os.Getpid(),
		Sessions: statusSessions,
	}

	data, err := json.Marshal(status)
	if err != nil {
		s.writeError(conn, "internal_error", fmt.Sprintf("failed to marshal status: %v", err))
		return
	}
	conn.Write(append(data, '\n'))
}

func (s *Server) handleStopSession(conn net.Conn, req ipc.Request) {
	if req.Dir == "" {
		s.writeError(conn, "invalid_request", "dir is required")
		return
	}
	s.manager.Stop(req.Dir)
	s.writeResponse(conn, ipc.Response{Type: ipc.TypeDone})
}

func (s *Server) writeResponse(conn net.Conn, resp ipc.Response) {
	data, err := ipc.EncodeResponse(resp)
	if err != nil {
		return
	}
	conn.Write(data)
}

func (s *Server) writeError(conn net.Conn, code, message string) {
	s.writeResponse(conn, ipc.Response{
		Type:    ipc.TypeError,
		Code:    code,
		Message: message,
	})
}
