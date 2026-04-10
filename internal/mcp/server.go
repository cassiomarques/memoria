// Package mcp implements an MCP (Model Context Protocol) server for memoria.
// It exposes memoria's note-taking capabilities as MCP tools, allowing AI
// assistants to search, read, and create notes.
//
// Each tool call delegates to the memoria CLI binary, which handles IPC with
// a running TUI instance or opens stores directly as needed.
package mcp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultTimeout = 30 * time.Second
	syncTimeout    = 120 * time.Second
)

// Server wraps an MCP server that exposes memoria tools.
type Server struct {
	server  *mcp.Server
	binPath string
	homeDir string
	writeMu sync.Mutex // serializes write operations (new, todo, sync)
}

// NewServer creates a new MCP server with all memoria tools registered.
// binPath is the path to the memoria binary. homeDir, if non-empty, is
// forwarded as --home to every subprocess.
func NewServer(binPath, homeDir, version string) *Server {
	s := &Server{
		server: mcp.NewServer(&mcp.Implementation{
			Name:    "memoria",
			Version: version,
		}, nil),
		binPath: binPath,
		homeDir: homeDir,
	}
	s.registerTools()
	return s
}

// Run starts the MCP server on stdio, blocking until the client disconnects.
func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// MCPServer returns the underlying MCP server, useful for testing with
// in-memory transports.
func (s *Server) MCPServer() *mcp.Server {
	return s.server
}

// registerTools adds all memoria tools to the MCP server.
func (s *Server) registerTools() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search",
		Description: "Full-text search across all notes. Returns matching notes ranked by relevance with highlighted fragments.",
	}, s.handleSearch)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list",
		Description: "List all notes, or notes within a specific folder.",
	}, s.handleList)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "cat",
		Description: "Read the full content of a note by its path.",
	}, s.handleCat)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "tags",
		Description: "List all tags with the number of notes using each tag.",
	}, s.handleTags)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "todos",
		Description: "List todos, optionally filtered by status: overdue, today, pending, or done.",
	}, s.handleTodos)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "new",
		Description: "Create a new note at the given path with optional content and tags.",
	}, s.handleNew)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "edit",
		Description: "Update the content of an existing note. Fails if the note does not exist.",
	}, s.handleEdit)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "todo",
		Description: "Create a new todo with a title, optional folder, and optional tags.",
	}, s.handleTodo)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "sync",
		Description: "Sync notes with the git remote: pull, reindex, commit, and push.",
	}, s.handleSync)
}

// --- Input types for each tool ---

type searchInput struct {
	Query string `json:"query" jsonschema:"The search query. Words are AND-ed together. Wrap in quotes for exact phrase match."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default 20, max 1000)"`
}

type listInput struct {
	Folder string `json:"folder,omitempty" jsonschema:"Folder to list notes from. Omit to list all notes."`
}

type catInput struct {
	Path string `json:"path" jsonschema:"Relative path of the note to read (e.g. Projects/ideas.md)."`
}

type todosInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"Filter todos: overdue, today, pending, or done. Omit for all todos."`
}

type newInput struct {
	Path    string `json:"path" jsonschema:"Relative path for the new note (e.g. Projects/new-idea.md)."`
	Content string `json:"content,omitempty" jsonschema:"Markdown content for the note body."`
	Tags    string `json:"tags,omitempty" jsonschema:"Comma-separated tags (e.g. golang,tui)."`
}

type editInput struct {
	Path    string `json:"path" jsonschema:"Relative path of the note to edit (e.g. Projects/ideas.md)."`
	Content string `json:"content" jsonschema:"New markdown content for the note body."`
}

type todoInput struct {
	Title  string `json:"title" jsonschema:"Title of the todo item."`
	Folder string `json:"folder,omitempty" jsonschema:"Folder to create the todo in (default: TODO)."`
	Tags   string `json:"tags,omitempty" jsonschema:"Comma-separated tags."`
	Due    string `json:"due,omitempty" jsonschema:"Due date in YYYY-MM-DD format."`
}

// --- Tool handlers ---

func (s *Server) handleSearch(ctx context.Context, _ *mcp.CallToolRequest, input searchInput) (*mcp.CallToolResult, any, error) {
	if input.Query == "" {
		return errResult("search requires a query"), nil, nil
	}
	args := []string{"search", input.Query}
	if input.Limit > 0 {
		args = append(args, fmt.Sprintf("--limit=%d", input.Limit))
	}
	return s.run(ctx, defaultTimeout, args...)
}

func (s *Server) handleList(ctx context.Context, _ *mcp.CallToolRequest, input listInput) (*mcp.CallToolResult, any, error) {
	args := []string{"list"}
	if input.Folder != "" {
		args = append(args, input.Folder)
	}
	return s.run(ctx, defaultTimeout, args...)
}

func (s *Server) handleCat(ctx context.Context, _ *mcp.CallToolRequest, input catInput) (*mcp.CallToolResult, any, error) {
	if input.Path == "" {
		return errResult("cat requires a path"), nil, nil
	}
	return s.run(ctx, defaultTimeout, "cat", input.Path)
}

func (s *Server) handleTags(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return s.run(ctx, defaultTimeout, "tags")
}

func (s *Server) handleTodos(ctx context.Context, _ *mcp.CallToolRequest, input todosInput) (*mcp.CallToolResult, any, error) {
	args := []string{"todos"}
	if input.Filter != "" {
		args = append(args, input.Filter)
	}
	return s.run(ctx, defaultTimeout, args...)
}

func (s *Server) handleNew(ctx context.Context, _ *mcp.CallToolRequest, input newInput) (*mcp.CallToolResult, any, error) {
	if input.Path == "" {
		return errResult("new requires a path"), nil, nil
	}
	args := []string{"new", input.Path}
	if input.Tags != "" {
		args = append(args, "--tags", input.Tags)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	return s.runWithStdin(ctx, defaultTimeout, input.Content, args...)
}

func (s *Server) handleEdit(ctx context.Context, _ *mcp.CallToolRequest, input editInput) (*mcp.CallToolResult, any, error) {
	if input.Path == "" {
		return errResult("edit requires a path"), nil, nil
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	return s.runWithStdin(ctx, defaultTimeout, input.Content, "edit", input.Path)
}

func (s *Server) handleTodo(ctx context.Context, _ *mcp.CallToolRequest, input todoInput) (*mcp.CallToolResult, any, error) {
	if input.Title == "" {
		return errResult("todo requires a title"), nil, nil
	}
	args := []string{"todo", input.Title}
	if input.Folder != "" {
		args = append(args, "--folder", input.Folder)
	}
	if input.Tags != "" {
		args = append(args, "--tags", input.Tags)
	}
	if input.Due != "" {
		args = append(args, "--due", input.Due)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	return s.run(ctx, defaultTimeout, args...)
}

func (s *Server) handleSync(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	return s.run(ctx, syncTimeout, "sync")
}

// --- Subprocess execution ---

// run executes a memoria CLI command and returns the output as an MCP result.
func (s *Server) run(ctx context.Context, timeout time.Duration, args ...string) (*mcp.CallToolResult, any, error) {
	return s.runWithStdin(ctx, timeout, "", args...)
}

// runWithStdin executes a memoria CLI command, optionally piping content to stdin.
func (s *Server) runWithStdin(ctx context.Context, timeout time.Duration, stdin string, args ...string) (*mcp.CallToolResult, any, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fullArgs := []string{"--json"}
	if s.homeDir != "" {
		fullArgs = append(fullArgs, "--home", s.homeDir)
	}
	fullArgs = append(fullArgs, args...)

	cmd := exec.CommandContext(ctx, s.binPath, fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if stdin != "" {
		cmd.Stdin = bytes.NewBufferString(stdin)
	}

	if err := cmd.Run(); err != nil {
		// Include stderr in the error for debugging
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return errResult(msg), nil, nil
	}

	return textResult(stdout.String()), nil, nil
}

// textResult creates a successful MCP result with text content.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// errResult creates an MCP result that indicates a tool-level error.
func errResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

// ResolveBinPath returns the path to the memoria binary, checking the
// MEMORIA_BIN environment variable first, then falling back to os.Executable().
func ResolveBinPath() (string, error) {
	if bin := os.Getenv("MEMORIA_BIN"); bin != "" {
		if _, err := os.Stat(bin); err != nil {
			return "", fmt.Errorf("MEMORIA_BIN=%q: %w", bin, err)
		}
		return bin, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable path: %w", err)
	}
	return exe, nil
}
