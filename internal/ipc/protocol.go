package ipc

import "encoding/json"

// Command constants for CLI → TUI communication.
const (
	CmdSearch   = "search"
	CmdList     = "list"
	CmdTags     = "tags"
	CmdTodos    = "todos"
	CmdCat      = "cat"
	CmdSync     = "sync"
	CmdNew      = "new"
	CmdEdit     = "edit"
	CmdTodo     = "todo"
	CmdNavigate = "navigate"
	CmdRecent   = "recent"
)

// Request is sent by the CLI to the TUI over the Unix socket.
type Request struct {
	Command string            `json:"command"`
	Args    map[string]string `json:"args,omitempty"`
}

// Response is sent back from the TUI to the CLI.
type Response struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// OKResponse creates a success response with JSON-encoded data.
func OKResponse(data any) Response {
	raw, err := json.Marshal(data)
	if err != nil {
		return ErrResponse("encoding response: " + err.Error())
	}
	return Response{OK: true, Data: raw}
}

// ErrResponse creates an error response.
func ErrResponse(msg string) Response {
	return Response{OK: false, Error: msg}
}
