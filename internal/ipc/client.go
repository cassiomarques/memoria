package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

// Client sends commands to a running TUI's Unix socket.
// Each Send call opens a new connection (the server handles one request per connection).
type Client struct {
	sockPath string
}

// NewClient verifies that the socket is connectable and returns a Client.
// Returns an error if the socket doesn't exist or the TUI isn't running.
func NewClient(sockPath string) (*Client, error) {
	// Probe connectivity so callers can detect stale sockets early
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("connecting to memoria: %w", err)
	}
	_ = conn.Close()
	return &Client{sockPath: sockPath}, nil
}

// Send sends a request and reads the response. Opens a fresh connection each time.
func (c *Client) Send(req Request) (*Response, error) {
	conn, err := net.Dial("unix", c.sockPath)
	if err != nil {
		return nil, fmt.Errorf("connecting to memoria: %w", err)
	}
	defer conn.Close()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		return nil, fmt.Errorf("server closed connection")
	}

	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &resp, nil
}

// Close is a no-op (connections are opened and closed per Send call).
// Retained for interface compatibility.
func (c *Client) Close() error {
	return nil
}

// SocketPath returns the default socket path for the given config directory.
func SocketPath(configDir string) string {
	return configDir + "/memoria.sock"
}
