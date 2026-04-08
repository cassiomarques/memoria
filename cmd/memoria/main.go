package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/cassiomarques/memoria/internal/config"
	"github.com/cassiomarques/memoria/internal/editor"
	"github.com/cassiomarques/memoria/internal/git"
	"github.com/cassiomarques/memoria/internal/ipc"
	mcpserver "github.com/cassiomarques/memoria/internal/mcp"
	"github.com/cassiomarques/memoria/internal/search"
	"github.com/cassiomarques/memoria/internal/service"
	"github.com/cassiomarques/memoria/internal/storage"
	"github.com/cassiomarques/memoria/internal/tui"
	"github.com/cassiomarques/memoria/internal/tui/theme"
)

var version = "dev"

// knownSubcommands lists the CLI commands that bypass the TUI.
var knownSubcommands = map[string]bool{
	"search": true, "list": true, "tags": true, "todos": true,
	"cat": true, "sync": true, "new": true, "todo": true,
	"mcp": true,
}

func main() {
	var homeDir string
	var jsonOutput bool
	args := os.Args[1:]

	// Extract global flags first
	filtered := args[:0]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v":
			fmt.Printf("memoria %s\n", version)
			os.Exit(0)
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		case "--home":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --home requires a directory path")
				os.Exit(1)
			}
			homeDir = args[i+1]
			i++
		case "--json":
			jsonOutput = true
		default:
			filtered = append(filtered, args[i])
		}
	}

	// Determine if we're running a CLI subcommand or the TUI
	if len(filtered) > 0 {
		if filtered[0] == "help" {
			printUsage()
			os.Exit(0)
		}
		if filtered[0] == "mcp" {
			if err := runMCP(homeDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if knownSubcommands[filtered[0]] {
			if err := runCLI(homeDir, jsonOutput, filtered[0], filtered[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'memoria help' for usage.\n", filtered[0])
		os.Exit(1)
	}

	if err := runTUI(homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Usage: memoria [command] [options]

Run without arguments to start the interactive TUI.

Commands:
  search <query>          Full-text search (AND across words; --exact for phrase match)
  list [folder]           List notes (optionally filtered by folder)
  tags                    List all tags with note counts
  todos [filter]          List todos (filter: overdue, today, pending, done)
  cat <path>              Print note content to stdout
  sync                    Sync notes (git pull + reindex + commit + push)
  new <path> [--tags t]   Create a new note
  todo <title> [opts]     Create a new todo (--folder F, --tags t1,t2)
  mcp                     Start the MCP server (stdio transport)
  help                    Show this help

Global flags:
  --json                  Output as JSON (for scripting / Copilot CLI)
  --home <dir>            Use a custom config/data directory
  --version, -v           Print version

When the TUI is running, CLI commands communicate with it via a local socket
for full search capabilities and automatic TUI refresh. When no TUI is running,
commands open the stores directly.
`)
}

// runTUI starts the interactive TUI with the IPC server alongside it.
func runTUI(homeDir string) error {
	if homeDir != "" {
		if err := config.SetConfigDir(homeDir); err != nil {
			return fmt.Errorf("invalid --home path: %w", err)
		}
	}

	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	theme.Init(cfg.ResolveTheme())

	if err := config.EnsureDirs(cfg); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	releaseLock, err := config.AcquireLock(config.DefaultConfigDir())
	if err != nil {
		return err
	}
	defer releaseLock()

	cfgPath := config.DefaultConfigPath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := cfg.Save(cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save default config: %v\n", err)
		}
	}

	notesDir, err := cfg.ResolveNotesDir()
	if err != nil {
		return fmt.Errorf("resolving notes dir: %w", err)
	}

	files, err := storage.NewFileStore(notesDir)
	if err != nil {
		return fmt.Errorf("initializing file store: %w", err)
	}

	dbPath := filepath.Join(config.DefaultConfigDir(), "meta.db")
	meta, err := storage.NewMetaStore(dbPath)
	if err != nil {
		return fmt.Errorf("initializing meta store: %w", err)
	}

	indexPath := filepath.Join(config.DefaultConfigDir(), "search.bleve")
	idx, err := search.NewSearchIndex(indexPath)
	if err != nil {
		return fmt.Errorf("initializing search index: %w", err)
	}

	repo, err := git.InitOrOpen(notesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: git init failed: %v\n", err)
		repo = nil
	}

	ed := editor.New(cfg.ResolveEditor())

	svc := service.New(files, meta, idx, repo, ed)
	defer svc.Close()

	if err := svc.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: initial sync failed: %v\n", err)
	}

	if count, err := svc.EnsureFrontmatter(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: frontmatter migration failed: %v\n", err)
	} else if count > 0 {
		fmt.Fprintf(os.Stderr, "Added frontmatter to %d notes\n", count)
	}

	// Start IPC server so CLI commands can talk to this running instance
	sockPath := ipc.SocketPath(config.DefaultConfigDir())
	handler := ipc.NewHandler(svc)
	ipcServer, err := ipc.NewServer(sockPath, handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not start IPC server: %v\n", err)
	}
	if ipcServer != nil {
		defer ipcServer.Close()
	}

	app := tui.NewAppWithService(svc, tui.AppOptions{
		ExpandFolders:     cfg.ResolveExpandFolders(),
		ShowPinnedNotes:   cfg.ResolveShowPinnedNotes(),
		ShowTimestamps:    cfg.ResolveShowTimestamps(),
		Version:           version,
		DefaultTodoFolder: cfg.ResolveDefaultTodoFolder(),
	})

	p := tea.NewProgram(app)

	// Wire IPC write callback to inject a refresh message into the TUI.
	// tea.Program.Send() is goroutine-safe — it's designed for exactly this
	// kind of external event injection into the Elm Architecture update loop.
	if ipcServer != nil {
		ipcServer.Handler().SetOnWrite(func() {
			p.Send(tui.ExternalRefreshMsg{})
		})
	}

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}

// runCLI executes a single CLI subcommand. It first tries to connect to a
// running TUI instance via the Unix socket (Mode A). If no TUI is running,
// it opens the stores directly (Mode B).
func runCLI(homeDir string, jsonOutput bool, command string, args []string) error {
	if homeDir != "" {
		if err := config.SetConfigDir(homeDir); err != nil {
			return fmt.Errorf("invalid --home path: %w", err)
		}
	}

	sockPath := ipc.SocketPath(config.DefaultConfigDir())

	// Build the IPC request from CLI args
	req := buildRequest(command, args)

	// Try Mode A: connect to running TUI
	client, err := ipc.NewClient(sockPath)
	if err == nil {
		defer client.Close()
		resp, err := client.Send(req)
		if err != nil {
			return fmt.Errorf("communicating with TUI: %w", err)
		}
		return printResponse(resp, jsonOutput)
	}

	// Mode B: no TUI running, open stores directly
	return runDirect(homeDir, jsonOutput, req)
}

// buildRequest converts CLI subcommand + args into an IPC request.
func buildRequest(command string, args []string) ipc.Request {
	req := ipc.Request{
		Command: command,
		Args:    make(map[string]string),
	}

	switch command {
	case "search":
		var exact bool
		var queryParts []string
		for _, a := range args {
			if a == "--exact" {
				exact = true
			} else {
				queryParts = append(queryParts, a)
			}
		}
		if len(queryParts) > 0 {
			q := strings.Join(queryParts, " ")
			if exact {
				q = `"` + q + `"`
			}
			req.Args["query"] = q
		}
	case "list":
		if len(args) > 0 {
			req.Args["folder"] = args[0]
		}
	case "todos":
		if len(args) > 0 {
			req.Args["filter"] = args[0]
		}
	case "cat":
		if len(args) > 0 {
			req.Args["path"] = args[0]
		}
	case "new":
		// memoria new <path> [--tags tag1,tag2]
		// Content can be piped via stdin
		if len(args) > 0 {
			req.Args["path"] = args[0]
		}
		for i := 1; i < len(args); i++ {
			if args[i] == "--tags" && i+1 < len(args) {
				req.Args["tags"] = args[i+1]
				i++
			}
		}
		if content := readStdin(); content != "" {
			req.Args["content"] = content
		}
	case "todo":
		// memoria todo <title> [--folder F] [--tags t1,t2]
		var titleParts []string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--folder":
				if i+1 < len(args) {
					req.Args["folder"] = args[i+1]
					i++
				}
			case "--tags":
				if i+1 < len(args) {
					req.Args["tags"] = args[i+1]
					i++
				}
			default:
				titleParts = append(titleParts, args[i])
			}
		}
		if len(titleParts) > 0 {
			req.Args["title"] = strings.Join(titleParts, " ")
		}
	}

	return req
}

// readStdin returns stdin content if it's being piped (not a terminal).
// Returns empty string if stdin is a terminal or on read error.
func readStdin() string {
	info, err := os.Stdin.Stat()
	if err != nil {
		return ""
	}
	// Check if stdin is a pipe or redirect (not a terminal)
	if info.Mode()&os.ModeCharDevice != 0 {
		return ""
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return ""
	}
	return string(data)
}

// printResponse outputs the response to stdout.
func printResponse(resp *ipc.Response, jsonOutput bool) error {
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}

	if jsonOutput {
		// Raw JSON output for machine consumption
		_, _ = os.Stdout.Write(resp.Data)
		fmt.Println()
		return nil
	}

	// Human-readable output depends on the data type
	var raw any
	if err := json.Unmarshal(resp.Data, &raw); err != nil {
		// Not valid JSON, print as-is
		fmt.Println(string(resp.Data))
		return nil //nolint:nilerr // intentional: raw text fallback
	}

	switch v := raw.(type) {
	case string:
		fmt.Println(v)
	case []any:
		for _, item := range v {
			switch entry := item.(type) {
			case string:
				fmt.Println(entry)
			case map[string]any:
				// Search results, tags, todos, etc.
				printMapEntry(entry)
			default:
				data, _ := json.Marshal(entry)
				fmt.Println(string(data))
			}
		}
	default:
		data, _ := json.MarshalIndent(raw, "", "  ")
		fmt.Println(string(data))
	}
	return nil
}

func printMapEntry(m map[string]any) {
	// Try common field patterns for human-readable output
	if path, ok := m["Path"].(string); ok {
		line := path
		if score, ok := m["Score"].(float64); ok && score > 0 {
			line = fmt.Sprintf("%.2f  %s", score, path)
		}
		if title, ok := m["Title"].(string); ok && title != "" {
			line += fmt.Sprintf("  (%s)", title)
		}
		fmt.Println(line)
		return
	}
	if tag, ok := m["Tag"].(string); ok {
		count := m["Count"]
		fmt.Printf("%-20s %v\n", tag, count)
		return
	}
	data, _ := json.Marshal(m)
	fmt.Println(string(data))
}

// runDirect opens stores directly and executes the command (Mode B).
func runDirect(homeDir string, jsonOutput bool, req ipc.Request) error {
	if homeDir != "" {
		if err := config.SetConfigDir(homeDir); err != nil {
			return fmt.Errorf("invalid --home path: %w", err)
		}
	}

	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := config.EnsureDirs(cfg); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	notesDir, err := cfg.ResolveNotesDir()
	if err != nil {
		return fmt.Errorf("resolving notes dir: %w", err)
	}

	files, err := storage.NewFileStore(notesDir)
	if err != nil {
		return fmt.Errorf("initializing file store: %w", err)
	}

	dbPath := filepath.Join(config.DefaultConfigDir(), "meta.db")
	meta, err := storage.NewMetaStore(dbPath)
	if err != nil {
		return fmt.Errorf("initializing meta store: %w", err)
	}

	indexPath := filepath.Join(config.DefaultConfigDir(), "search.bleve")
	idx, err := search.NewSearchIndex(indexPath)
	if err != nil {
		return fmt.Errorf("initializing search index: %w", err)
	}

	repo, err := git.InitOrOpen(notesDir)
	if err != nil {
		repo = nil
	}

	svc := service.New(files, meta, idx, repo, nil)
	defer svc.Close()

	handler := ipc.NewHandler(svc)
	resp := handler.Dispatch(req)
	return printResponse(&resp, jsonOutput)
}

// runMCP starts the MCP stdio server.
func runMCP(homeDir string) error {
	binPath, err := mcpserver.ResolveBinPath()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	srv := mcpserver.NewServer(binPath, homeDir, version)
	return srv.Run(context.Background())
}
