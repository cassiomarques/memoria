package ipc_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cassiomarques/memoria/internal/ipc"
	"github.com/cassiomarques/memoria/internal/search"
	"github.com/cassiomarques/memoria/internal/service"
	"github.com/cassiomarques/memoria/internal/storage"
)

// testEnv sets up a full NoteService backed by real stores in a temp dir.
type testEnv struct {
	svc      *service.NoteService
	dir      string
	sockPath string
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	notesDir := filepath.Join(dir, "notes")
	os.MkdirAll(notesDir, 0755)

	files, err := storage.NewFileStore(notesDir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	meta, err := storage.NewMetaStore(filepath.Join(dir, "meta.db"))
	if err != nil {
		t.Fatalf("NewMetaStore: %v", err)
	}

	idx, err := search.NewSearchIndex(filepath.Join(dir, "search.bleve"))
	if err != nil {
		t.Fatalf("NewSearchIndex: %v", err)
	}

	svc := service.New(files, meta, idx, nil, nil)
	t.Cleanup(func() { svc.Close() })

	// Use /tmp for socket to avoid macOS 104-byte Unix socket path limit
	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("memoria-test-%d.sock", os.Getpid()))
	t.Cleanup(func() { os.Remove(sockPath) })

	return &testEnv{
		svc:      svc,
		dir:      dir,
		sockPath: sockPath,
	}
}

func (e *testEnv) startServer(t *testing.T) {
	t.Helper()
	handler := ipc.NewHandler(e.svc)
	srv, err := ipc.NewServer(e.sockPath, handler)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { srv.Close() })
}

func (e *testEnv) dial(t *testing.T) *ipc.Client {
	t.Helper()
	client, err := ipc.NewClient(e.sockPath)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestRoundTrip_Search(t *testing.T) {
	env := setupTestEnv(t)

	// Seed a note so we can search for it
	_, err := env.svc.Create("hello.md", "Hello world, this is a test note", []string{"demo"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{
		Command: ipc.CmdSearch,
		Args:    map[string]string{"query": "hello", "limit": "10"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	var results []search.SearchResult
	if err := json.Unmarshal(resp.Data, &results); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
	if results[0].Path != "hello.md" {
		t.Errorf("expected path hello.md, got %s", results[0].Path)
	}
}

func TestSearch_MultiWordIsAND(t *testing.T) {
	env := setupTestEnv(t)

	// "apple" matches both notes, but "apple banana" should only match the one with both words
	env.svc.Create("fruit.md", "I like apple and banana smoothies", nil)
	env.svc.Create("tech.md", "apple released a new laptop", nil)

	env.startServer(t)

	// Search for both words — should only match fruit.md
	client := env.dial(t)
	resp, err := client.Send(ipc.Request{
		Command: ipc.CmdSearch,
		Args:    map[string]string{"query": "apple banana"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	var results []search.SearchResult
	json.Unmarshal(resp.Data, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (AND), got %d", len(results))
	}
	if results[0].Path != "fruit.md" {
		t.Errorf("expected fruit.md, got %s", results[0].Path)
	}
}

func TestSearch_ExactPhrase(t *testing.T) {
	env := setupTestEnv(t)

	// Both notes contain "apple" and "pie", but only one has the exact phrase "apple pie"
	env.svc.Create("recipe.md", "This is my famous apple pie recipe", nil)
	env.svc.Create("random.md", "I ate an apple then later had some pie", nil)

	env.startServer(t)

	// Quoted phrase — should only match the note with "apple pie" adjacent
	client := env.dial(t)
	resp, err := client.Send(ipc.Request{
		Command: ipc.CmdSearch,
		Args:    map[string]string{"query": `"apple pie"`},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	var results []search.SearchResult
	json.Unmarshal(resp.Data, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (exact phrase), got %d", len(results))
	}
	if results[0].Path != "recipe.md" {
		t.Errorf("expected recipe.md, got %s", results[0].Path)
	}
}

func TestRoundTrip_Tags(t *testing.T) {
	env := setupTestEnv(t)
	env.svc.Create("tagged.md", "content", []string{"golang", "tui"})

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{Command: ipc.CmdTags})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	var tags []storage.TagInfo
	if err := json.Unmarshal(resp.Data, &tags); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
}

func TestRoundTrip_List(t *testing.T) {
	env := setupTestEnv(t)
	env.svc.Create("notes/a.md", "aaa", nil)
	env.svc.Create("notes/b.md", "bbb", nil)
	env.svc.Create("other/c.md", "ccc", nil)

	env.startServer(t)

	// List specific folder
	c1 := env.dial(t)
	resp, err := c1.Send(ipc.Request{
		Command: ipc.CmdList,
		Args:    map[string]string{"folder": "notes"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	var paths []string
	if err := json.Unmarshal(resp.Data, &paths); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 notes in 'notes' folder, got %d", len(paths))
	}

	// List all (new connection since server handles one request per connection)
	c2 := env.dial(t)
	resp, err = c2.Send(ipc.Request{Command: ipc.CmdList})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	var allPaths []string
	json.Unmarshal(resp.Data, &allPaths)
	if len(allPaths) != 3 {
		t.Errorf("expected 3 total notes, got %d", len(allPaths))
	}
}

func TestRoundTrip_Cat(t *testing.T) {
	env := setupTestEnv(t)
	env.svc.Create("readme.md", "# README\nHello", nil)

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{
		Command: ipc.CmdCat,
		Args:    map[string]string{"path": "readme.md"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	var content string
	json.Unmarshal(resp.Data, &content)
	if content == "" {
		t.Error("expected non-empty content")
	}
}

func TestRoundTrip_Sync(t *testing.T) {
	env := setupTestEnv(t)

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{Command: ipc.CmdSync})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}
}

func TestRoundTrip_New(t *testing.T) {
	env := setupTestEnv(t)

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{
		Command: ipc.CmdNew,
		Args: map[string]string{
			"path": "cli-created.md",
			"tags": "cli,test",
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	// Verify the note was actually created
	n, err := env.svc.Get("cli-created.md")
	if err != nil {
		t.Fatalf("note not found after create: %v", err)
	}
	if len(n.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(n.Tags))
	}
}

func TestRoundTrip_NewWithContent(t *testing.T) {
	env := setupTestEnv(t)

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{
		Command: ipc.CmdNew,
		Args: map[string]string{
			"path":    "with-content.md",
			"content": "# Hello\nThis has body text",
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	n, err := env.svc.Get("with-content.md")
	if err != nil {
		t.Fatalf("note not found: %v", err)
	}
	if !strings.Contains(n.Content, "This has body text") {
		t.Errorf("expected content to contain body text, got: %s", n.Content)
	}
}

func TestRoundTrip_Todos(t *testing.T) {
	env := setupTestEnv(t)
	env.svc.CreateTodo(service.CreateTodoOptions{
		Title:  "Buy groceries",
		Folder: "TODO",
	})

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{Command: ipc.CmdTodos})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	var todos []*storage.NoteMeta
	json.Unmarshal(resp.Data, &todos)
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0].Path != "TODO/buy-groceries.md" {
		t.Errorf("unexpected path: %s", todos[0].Path)
	}
}

func TestRoundTrip_UnknownCommand(t *testing.T) {
	env := setupTestEnv(t)
	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{Command: "nope"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatal("expected error for unknown command")
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestClient_ConnectFailure(t *testing.T) {
	// Connecting to a non-existent socket should return an error
	_, err := ipc.NewClient("/tmp/memoria-test-nonexistent.sock")
	if err == nil {
		t.Fatal("expected error connecting to non-existent socket")
	}
}

func TestOnWriteCallback(t *testing.T) {
	env := setupTestEnv(t)

	var called int32
	handler := ipc.NewHandler(env.svc)
	handler.SetOnWrite(func() {
		atomic.AddInt32(&called, 1)
	})
	srv, err := ipc.NewServer(env.sockPath, handler)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { srv.Close() })

	// sync is a write command — should trigger OnWrite
	client := env.dial(t)
	resp, err := client.Send(ipc.Request{Command: ipc.CmdSync})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("expected OnWrite called once, got %d", atomic.LoadInt32(&called))
	}

	// new is a write command — should trigger OnWrite again
	client2 := env.dial(t)
	client2.Send(ipc.Request{
		Command: ipc.CmdNew,
		Args:    map[string]string{"path": "test.md"},
	})
	if atomic.LoadInt32(&called) != 2 {
		t.Errorf("expected OnWrite called twice, got %d", atomic.LoadInt32(&called))
	}
}

func TestClientMultipleRequests(t *testing.T) {
	env := setupTestEnv(t)
	env.startServer(t)

	// With the dial-per-Send client, a single Client can send multiple requests
	client := env.dial(t)

	resp1, err := client.Send(ipc.Request{Command: ipc.CmdSync})
	if err != nil {
		t.Fatalf("first Send: %v", err)
	}
	if !resp1.OK {
		t.Fatalf("first: expected OK, got error: %s", resp1.Error)
	}

	resp2, err := client.Send(ipc.Request{Command: ipc.CmdTags})
	if err != nil {
		t.Fatalf("second Send: %v", err)
	}
	if !resp2.OK {
		t.Fatalf("second: expected OK, got error: %s", resp2.Error)
	}
}

func TestPathValidation(t *testing.T) {
	env := setupTestEnv(t)
	env.startServer(t)

	tests := []struct {
		name    string
		command string
		args    map[string]string
	}{
		{"cat absolute path", ipc.CmdCat, map[string]string{"path": "/etc/passwd"}},
		{"cat traversal", ipc.CmdCat, map[string]string{"path": "../../../etc/passwd"}},
		{"cat empty path", ipc.CmdCat, map[string]string{}},
		{"new absolute path", ipc.CmdNew, map[string]string{"path": "/tmp/evil.md"}},
		{"new traversal", ipc.CmdNew, map[string]string{"path": "../../evil.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := env.dial(t)
			resp, err := client.Send(ipc.Request{Command: tt.command, Args: tt.args})
			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if resp.OK {
				t.Fatal("expected error for invalid path")
			}
		})
	}
}

func TestRoundTrip_Edit(t *testing.T) {
	env := setupTestEnv(t)

	// Create a note first
	_, err := env.svc.Create("editable.md", "original content", []string{"test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{
		Command: ipc.CmdEdit,
		Args: map[string]string{
			"path":    "editable.md",
			"content": "updated via IPC",
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got error: %s", resp.Error)
	}

	// Verify content was updated
	n, err := env.svc.Get("editable.md")
	if err != nil {
		t.Fatalf("Get after edit: %v", err)
	}
	if !strings.Contains(n.Content, "updated via IPC") {
		t.Errorf("expected updated content, got: %s", n.Content)
	}
}

func TestRoundTrip_Edit_Nonexistent(t *testing.T) {
	env := setupTestEnv(t)

	env.startServer(t)
	client := env.dial(t)

	resp, err := client.Send(ipc.Request{
		Command: ipc.CmdEdit,
		Args: map[string]string{
			"path":    "does-not-exist.md",
			"content": "some content",
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatal("expected error when editing nonexistent note")
	}
}
