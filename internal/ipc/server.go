package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cassiomarques/memoria/internal/service"
	"github.com/cassiomarques/memoria/internal/storage"
)

// Handler dispatches IPC requests to a NoteService. It is used by both the
// socket Server (Mode A) and the direct CLI execution path (Mode B).
type Handler struct {
	svc     *service.NoteService
	onWrite atomic.Value // stores func()
}

// NewHandler creates a handler that dispatches to the given NoteService.
func NewHandler(svc *service.NoteService) *Handler {
	return &Handler{svc: svc}
}

// SetOnWrite sets the callback invoked after a write command completes.
// Safe to call from any goroutine.
func (h *Handler) SetOnWrite(fn func()) {
	h.onWrite.Store(fn)
}

func (h *Handler) callOnWrite() {
	if fn, ok := h.onWrite.Load().(func()); ok && fn != nil {
		fn()
	}
}

// Server listens on a Unix socket and dispatches CLI commands via a Handler.
type Server struct {
	listener net.Listener
	handler  *Handler
	sockPath string
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewServer starts listening on the given Unix socket path.
// It removes any stale socket file before binding.
func NewServer(sockPath string, handler *Handler) (*Server, error) {
	// Remove stale socket from a previous crash
	if _, err := os.Stat(sockPath); err == nil {
		_ = os.Remove(sockPath)
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", sockPath, err)
	}

	// Restrict socket to owner only (prevents other local users from connecting)
	if err := os.Chmod(sockPath, 0600); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("chmod socket: %w", err)
	}

	s := &Server{
		listener: listener,
		handler:  handler,
		sockPath: sockPath,
		done:     make(chan struct{}),
	}
	go s.acceptLoop()
	return s, nil
}

// Handler returns the server's handler, useful for setting callbacks after construction.
func (s *Server) Handler() *Handler {
	return s.handler
}

// Close stops the server, waits for active connections to finish, and removes the socket file.
func (s *Server) Close() error {
	close(s.done)
	err := s.listener.Close()
	s.wg.Wait()
	_ = os.Remove(s.sockPath)
	return err
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				continue
			}
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	// Prevent stalled clients from holding goroutines indefinitely
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	scanner := bufio.NewScanner(conn)
	// Allow up to 1MB messages
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	if !scanner.Scan() {
		return
	}
	line := scanner.Bytes()

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		writeResponse(conn, ErrResponse("invalid request: "+err.Error()))
		return
	}

	resp := s.handler.Dispatch(req)
	writeResponse(conn, resp)
}

func writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = conn.Write(data)
}

// Dispatch handles a single request using this handler's NoteService.
// Exported so it can be used for both socket-based (Mode A) and direct (Mode B) execution.
func (h *Handler) Dispatch(req Request) Response {
	switch req.Command {
	case CmdSearch:
		return h.handleSearch(req)
	case CmdList:
		return h.handleList(req)
	case CmdTags:
		return h.handleTags(req)
	case CmdTodos:
		return h.handleTodos(req)
	case CmdCat:
		return h.handleCat(req)
	case CmdSync:
		return h.handleSync(req)
	case CmdNew:
		return h.handleNew(req)
	case CmdTodo:
		return h.handleTodo(req)
	default:
		return ErrResponse(fmt.Sprintf("unknown command: %q", req.Command))
	}
}

func (h *Handler) handleSearch(req Request) Response {
	query := req.Args["query"]
	if query == "" {
		return ErrResponse("search requires a 'query' argument")
	}
	limit := 20
	if l, err := strconv.Atoi(req.Args["limit"]); err == nil && l > 0 && l <= 1000 {
		limit = l
	}
	results, err := h.svc.SearchFuzzy(query, limit)
	if err != nil {
		return ErrResponse(err.Error())
	}
	return OKResponse(results)
}

func (h *Handler) handleList(req Request) Response {
	folder := req.Args["folder"]
	if folder != "" {
		notes, err := h.svc.List(folder)
		if err != nil {
			return ErrResponse(err.Error())
		}
		paths := make([]string, len(notes))
		for i, n := range notes {
			paths[i] = n.Path
		}
		return OKResponse(paths)
	}

	notes, err := h.svc.ListAll()
	if err != nil {
		return ErrResponse(err.Error())
	}
	paths := make([]string, len(notes))
	for i, n := range notes {
		paths[i] = n.Path
	}
	return OKResponse(paths)
}

func (h *Handler) handleTags(_ Request) Response {
	tags, err := h.svc.ListTags()
	if err != nil {
		return ErrResponse(err.Error())
	}
	return OKResponse(tags)
}

func (h *Handler) handleTodos(req Request) Response {
	todos, err := h.svc.ListTodos()
	if err != nil {
		return ErrResponse(err.Error())
	}

	filter := req.Args["filter"]
	if filter != "" {
		todos = filterTodos(todos, filter)
	}
	return OKResponse(todos)
}

func (h *Handler) handleCat(req Request) Response {
	path := req.Args["path"]
	if err := validatePath(path); err != nil {
		return ErrResponse("cat: " + err.Error())
	}
	n, err := h.svc.Get(path)
	if err != nil {
		return ErrResponse(err.Error())
	}
	return OKResponse(n.Content)
}

func (h *Handler) handleSync(_ Request) Response {
	if err := h.svc.Sync(); err != nil {
		return ErrResponse(err.Error())
	}
	h.callOnWrite()
	return OKResponse("synced")
}

func (h *Handler) handleNew(req Request) Response {
	path := req.Args["path"]
	if err := validatePath(path); err != nil {
		return ErrResponse("new: " + err.Error())
	}
	content := req.Args["content"]
	var tags []string
	if t := req.Args["tags"]; t != "" {
		tags = strings.Split(t, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}
	n, err := h.svc.Create(path, content, tags)
	if err != nil {
		return ErrResponse(err.Error())
	}
	h.callOnWrite()
	return OKResponse(n.Path)
}

func (h *Handler) handleTodo(req Request) Response {
	title := req.Args["title"]
	if title == "" {
		return ErrResponse("todo requires a 'title' argument")
	}
	folder := req.Args["folder"]
	if folder == "" {
		folder = "TODO"
	}
	var tags []string
	if t := req.Args["tags"]; t != "" {
		tags = strings.Split(t, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}
	n, err := h.svc.CreateTodo(service.CreateTodoOptions{
		Title:  title,
		Folder: folder,
		Tags:   tags,
	})
	if err != nil {
		return ErrResponse(err.Error())
	}
	h.callOnWrite()
	return OKResponse(n.Path)
}

// validatePath rejects absolute paths and directory traversal attempts.
func validatePath(p string) error {
	if p == "" {
		return fmt.Errorf("path is required")
	}
	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute paths not allowed")
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	return nil
}

// filterTodos applies a named filter to a list of todo metadata.
func filterTodos(todos []*storage.NoteMeta, filter string) []*storage.NoteMeta {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)

	var out []*storage.NoteMeta
	for _, t := range todos {
		switch filter {
		case "overdue":
			if t.Due != nil && t.Due.Before(today) && !t.Done {
				out = append(out, t)
			}
		case "today":
			if t.Due != nil && !t.Due.Before(today) && t.Due.Before(tomorrow) && !t.Done {
				out = append(out, t)
			}
		case "pending":
			if !t.Done {
				out = append(out, t)
			}
		case "done":
			if t.Done {
				out = append(out, t)
			}
		default:
			out = append(out, t)
		}
	}
	return out
}
