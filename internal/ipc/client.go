package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client connects to the daemon's Unix domain socket.
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new IPC client.
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		timeout:    5 * time.Second,
	}
}

// Query sends a prompt to the daemon and returns a channel of streaming responses.
func (c *Client) Query(dir, prompt, requestID string) (<-chan Response, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, err
	}

	req := Request{
		Type:      TypeQuery,
		Dir:       dir,
		Prompt:    prompt,
		RequestID: requestID,
	}

	if err := c.sendRequest(conn, req); err != nil {
		conn.Close()
		return nil, err
	}

	ch := make(chan Response, 64)
	go func() {
		defer conn.Close()
		defer close(ch)
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			var resp Response
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				ch <- Response{Type: TypeError, Message: fmt.Sprintf("failed to decode response: %v", err)}
				return
			}
			ch <- resp
			if resp.Type == TypeDone || resp.Type == TypeError {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- Response{Type: TypeError, Message: fmt.Sprintf("connection error: %v", err)}
		}
	}()

	return ch, nil
}

// Status retrieves the daemon's current status.
func (c *Client) Status() (DaemonStatus, error) {
	conn, err := c.connect()
	if err != nil {
		return DaemonStatus{}, err
	}
	defer conn.Close()

	req := Request{Type: TypeStatus}
	if err := c.sendRequest(conn, req); err != nil {
		return DaemonStatus{}, err
	}

	conn.SetReadDeadline(time.Now().Add(c.timeout))
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		var status DaemonStatus
		if err := json.Unmarshal(scanner.Bytes(), &status); err != nil {
			return DaemonStatus{}, fmt.Errorf("failed to decode status: %w", err)
		}
		return status, nil
	}
	if err := scanner.Err(); err != nil {
		return DaemonStatus{}, fmt.Errorf("failed to read status: %w", err)
	}
	return DaemonStatus{}, fmt.Errorf("no response from daemon")
}

// StopSession tells the daemon to stop a session for the given directory.
func (c *Client) StopSession(dir string) error {
	conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()

	req := Request{Type: TypeStopSession, Dir: dir}
	return c.sendRequest(conn, req)
}

func (c *Client) connect() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon at %s: %w", c.socketPath, err)
	}
	return conn, nil
}

func (c *Client) sendRequest(conn net.Conn, req Request) error {
	data, err := EncodeRequest(req)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}
	conn.SetWriteDeadline(time.Now().Add(c.timeout))
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	return nil
}
