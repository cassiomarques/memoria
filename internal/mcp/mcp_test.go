package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpserver "github.com/cassiomarques/memoria/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var testBinary string

func TestMain(m *testing.M) {
	// Build the memoria binary once for all tests.
	dir, err := os.MkdirTemp("", "memoria-mcp-test-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}
	testBinary = filepath.Join(dir, "memoria")

	cmd := exec.Command("go", "build", "-o", testBinary, "./cmd/memoria")
	cmd.Dir = findRepoRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building test binary: %v\n", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// findRepoRoot walks up from the current directory to find the go.mod file.
func findRepoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find repo root (go.mod)")
		}
		dir = parent
	}
}

// testSetup creates a temp home dir and returns a connected MCP client session.
func testSetup(t *testing.T) *mcp.ClientSession {
	t.Helper()

	homeDir := t.TempDir()

	srv := mcpserver.NewServer(testBinary, homeDir, "test")
	ct, st := mcp.NewInMemoryTransports()

	ctx := context.Background()
	ss, err := srv.MCPServer().Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })

	return cs
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// --- Tests ---

func TestSearch(t *testing.T) {
	cs := testSetup(t)

	// Create a note to search for
	result := callTool(t, cs, "new", map[string]any{
		"path":    "test/searchable.md",
		"content": "Golang is a fantastic programming language",
		"tags":    "golang,programming",
	})
	if result.IsError {
		t.Fatalf("new failed: %s", resultText(t, result))
	}

	// Search for it
	result = callTool(t, cs, "search", map[string]any{
		"query": "fantastic programming",
	})
	if result.IsError {
		t.Fatalf("search failed: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "searchable.md") {
		t.Errorf("expected search result to contain 'searchable.md', got: %s", text)
	}
}

func TestSearch_NoQuery(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "search", map[string]any{})
	if !result.IsError {
		t.Fatal("expected error for empty query")
	}
}

func TestList(t *testing.T) {
	cs := testSetup(t)

	// Create notes in different folders
	callTool(t, cs, "new", map[string]any{"path": "notes/a.md", "content": "aaa"})
	callTool(t, cs, "new", map[string]any{"path": "notes/b.md", "content": "bbb"})
	callTool(t, cs, "new", map[string]any{"path": "other/c.md", "content": "ccc"})

	// List specific folder
	result := callTool(t, cs, "list", map[string]any{"folder": "notes"})
	if result.IsError {
		t.Fatalf("list failed: %s", resultText(t, result))
	}
	text := resultText(t, result)

	var paths []string
	if err := json.Unmarshal([]byte(text), &paths); err != nil {
		t.Fatalf("unmarshal list: %v (text: %q)", err, text)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 notes in 'notes' folder, got %d: %v", len(paths), paths)
	}

	// List all
	result = callTool(t, cs, "list", map[string]any{})
	text = resultText(t, result)
	var allPaths []string
	if err := json.Unmarshal([]byte(text), &allPaths); err != nil {
		t.Fatalf("unmarshal list all: %v (text: %q)", err, text)
	}
	if len(allPaths) != 3 {
		t.Errorf("expected 3 total notes, got %d: %v", len(allPaths), allPaths)
	}
}

func TestCat(t *testing.T) {
	cs := testSetup(t)

	callTool(t, cs, "new", map[string]any{
		"path":    "readme.md",
		"content": "# Hello World\nThis is the body",
	})

	result := callTool(t, cs, "cat", map[string]any{"path": "readme.md"})
	if result.IsError {
		t.Fatalf("cat failed: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected content with 'Hello World', got: %s", text)
	}
}

func TestCat_NoPath(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "cat", map[string]any{})
	if !result.IsError {
		t.Fatal("expected error for empty path")
	}
}

func TestCat_PathTraversal(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "cat", map[string]any{"path": "../../etc/passwd"})
	if !result.IsError {
		t.Fatal("expected error for path traversal")
	}
}

func TestCat_AbsolutePath(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "cat", map[string]any{"path": "/etc/passwd"})
	if !result.IsError {
		t.Fatal("expected error for absolute path")
	}
}

func TestTags(t *testing.T) {
	cs := testSetup(t)

	callTool(t, cs, "new", map[string]any{
		"path": "tagged.md",
		"tags": "golang,tui",
	})

	result := callTool(t, cs, "tags", map[string]any{})
	if result.IsError {
		t.Fatalf("tags failed: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "golang") || !strings.Contains(text, "tui") {
		t.Errorf("expected tags to include 'golang' and 'tui', got: %s", text)
	}
}

func TestTodos(t *testing.T) {
	cs := testSetup(t)

	// Create a todo
	result := callTool(t, cs, "todo", map[string]any{
		"title": "Buy groceries",
	})
	if result.IsError {
		t.Fatalf("todo create failed: %s", resultText(t, result))
	}

	// List todos
	result = callTool(t, cs, "todos", map[string]any{})
	if result.IsError {
		t.Fatalf("todos list failed: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "buy-groceries") {
		t.Errorf("expected todo in list, got: %s", text)
	}
}

func TestTodos_WithFilter(t *testing.T) {
	cs := testSetup(t)

	callTool(t, cs, "todo", map[string]any{"title": "Pending task"})

	result := callTool(t, cs, "todos", map[string]any{"filter": "pending"})
	if result.IsError {
		t.Fatalf("todos pending failed: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "pending-task") {
		t.Errorf("expected pending todo, got: %s", text)
	}

	// Done filter should be empty
	result = callTool(t, cs, "todos", map[string]any{"filter": "done"})
	text = resultText(t, result)
	if strings.Contains(text, "pending-task") {
		t.Errorf("done filter should not contain pending todo, got: %s", text)
	}
}

func TestNew(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "new", map[string]any{
		"path":    "projects/idea.md",
		"content": "# Big Idea\nThis will change everything",
		"tags":    "project,idea",
	})
	if result.IsError {
		t.Fatalf("new failed: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "projects/idea.md") {
		t.Errorf("expected created path in result, got: %s", text)
	}

	// Verify via cat
	result = callTool(t, cs, "cat", map[string]any{"path": "projects/idea.md"})
	if result.IsError {
		t.Fatalf("cat after new failed: %s", resultText(t, result))
	}
	text = resultText(t, result)
	if !strings.Contains(text, "Big Idea") {
		t.Errorf("expected note content, got: %s", text)
	}
}

func TestNew_NoPath(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "new", map[string]any{})
	if !result.IsError {
		t.Fatal("expected error for missing path")
	}
}

func TestNew_PathTraversal(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "new", map[string]any{
		"path": "../../evil.md",
	})
	if !result.IsError {
		t.Fatal("expected error for path traversal")
	}
}

func TestTodo(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "todo", map[string]any{
		"title":  "Write tests",
		"folder": "Work",
		"tags":   "dev,testing",
	})
	if result.IsError {
		t.Fatalf("todo failed: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Work/") {
		t.Errorf("expected Work folder in path, got: %s", text)
	}
}

func TestTodo_NoTitle(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "todo", map[string]any{})
	if !result.IsError {
		t.Fatal("expected error for missing title")
	}
}

func TestSync(t *testing.T) {
	cs := testSetup(t)

	result := callTool(t, cs, "sync", map[string]any{})
	if result.IsError {
		t.Fatalf("sync failed: %s", resultText(t, result))
	}
}

func TestToolsList(t *testing.T) {
	cs := testSetup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	expectedTools := map[string]bool{
		"search": false,
		"list":   false,
		"cat":    false,
		"tags":   false,
		"todos":  false,
		"new":    false,
		"todo":   false,
		"sync":   false,
	}

	for _, tool := range result.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found in list", name)
		}
	}
}
