# memoria

[![CI](https://github.com/cassiomarques/memoria/actions/workflows/ci.yaml/badge.svg)](https://github.com/cassiomarques/memoria/actions/workflows/ci.yaml)
[![Release](https://github.com/cassiomarques/memoria/actions/workflows/release.yaml/badge.svg)](https://github.com/cassiomarques/memoria/actions/workflows/release.yaml)

A terminal-based note-taking app with full-text search, automatic git sync, editor integration and other niceties. 

Memoria keeps your notes as plain Markdown files organized in folders, indexes them for instant search, and automatically syncs everything to a git remote. Edit with your favourite editor, navigate with vim keybindings.

![Memoria - Notes tree with preview](assets/notes-with-preview.jpg)

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Keybindings](#keybindings)
- [Commands](#commands) (TUI)
- [CLI Subcommands](#cli-subcommands)
  - [Commands Reference](#commands-reference)
  - [Global Flags](#global-flags)
  - [How It Works: TUI + CLI Coordination](#how-it-works-tui--cli-coordination)
  - [External Integration Examples](#external-integration-examples)
- [MCP Server](#mcp-server)
- [Configuration](#configuration)
- [How notes are stored](#how-notes-are-stored)
- [Architecture](#architecture)
- [Tech stack](#tech-stack)
- [Development](#development)
- [Releasing](#releasing)
- [License](#license)

## Features

- **Plain Markdown** — Notes are `.md` files with YAML frontmatter. 
- **Full-text search** — Powered by [Bleve](https://blevesearch.com/). Fuzzy, typo-tolerant, instant.
- **Automatic git sync** — Every create, edit, delete, move, and tag change is committed and pushed.
- **Folder hierarchy** — Organize notes in nested folders. Collapsible tree view in the TUI.
- **Tagging** — Add and remove tags at any time. Search and filter by tag.
- **Your editor** — Opens `$EDITOR` (or vim) for editing. Memoria handles the rest.
- **Beautiful TUI** — Catppuccin theme (dark/light), markdown preview, vim-style navigation.
- **CLI subcommands** — `memoria search`, `memoria list`, `memoria tags`, etc. for scripting and AI assistant integration.
- **Single binary** — Pure Go, no CGO, no external dependencies.

## Installation

### Homebrew (macOS & Linux)

```bash
brew install cassiomarques/tap/memoria
```

### Go install

```bash
go install github.com/cassiomarques/memoria/cmd/memoria@latest
```

### From source

```bash
git clone https://github.com/cassiomarques/memoria.git
cd memoria
make build
# Binary is in bin/memoria
```

### GitHub Releases

Pre-built binaries for macOS and Linux (amd64 & arm64) are available on the [Releases](https://github.com/cassiomarques/memoria/releases) page.

## Quick start

```bash
# Launch the TUI
memoria

# On first run, memoria creates:
#   ~/.memoria/config.yaml    — configuration
#   ~/.memoria/notes/          — your notes (git repo)
#   ~/.memoria/meta.db         — metadata (SQLite)
#   ~/.memoria/search.bleve/   — search index
```

### Using a custom home directory

Use `--home` to run memoria with a completely isolated data directory:

```bash
# Great for demos, testing, or separate note collections
memoria --home ~/demo-notes
```

This creates config, notes, database, and search index all under `~/demo-notes/` instead of `~/.memoria/`.

### Set up git sync (optional but recommended)

Create a private repository on GitHub (or any git host), then inside memoria:

```
:remote git@github.com:youruser/my-notes.git
```

From this point on, every change is automatically committed and pushed.

### Create your first note

```
:new ideas/my-first-note
```

This creates `ideas/my-first-note.md` and opens it in your editor. When you save and close, memoria indexes it and syncs to git.

## Keybindings

| Key | Action |
|-----|--------|
| **j / k** or **↑ / ↓** | Navigate the note tree |
| **h / l** or **← / →** | Collapse / expand folder |
| **Enter** | Open selected note in editor (or toggle folder) |
| **p** | Toggle markdown preview for selected note |
| **e** | Edit note from preview (when preview is focused) |
| **Tab** | Switch focus between tree and preview |
| **n** | Create a new note in the focused folder |
| **d** | Delete selected note or folder (with confirmation) |
| **:** | Open command bar |
| **/** | Search / filter notes |
| **Ctrl+f** | Fuzzy finder (search all notes by name) |
| **Tab** / **Shift+Tab** | Cycle through finder results |
| **?** | Show help |
| **Esc** | Close preview / help / command bar |
| **q** | Close preview/help if open, otherwise quit |
| **Ctrl+C** | Quit immediately |

### Search syntax

Press `/` to start searching. Type your query, press **Enter** to lock results and browse them with normal keybindings, press **Esc** to clear.

| Pattern | Meaning |
|---------|---------|
| `foo` | Fuzzy match "foo" in title, path, folder, tags, and content |
| `foo bar` | Match "foo" **AND** "bar" (both must match) |
| `"exact phrase"` | Match exact phrase (substring) |
| `#tag` | Filter by tag name |
| `foo #work` | Match "foo" AND notes tagged "work" |

### Navigation extras

| Key | Action |
|-----|--------|
| **g g** | Jump to top of list |
| **G** | Jump to bottom of list |
| **Ctrl+d** | Page down |
| **Ctrl+u** | Page up |

## Commands

Type `:` to open the command bar. Tab completion is available for paths, folders, and tags.

| Command | Usage | Description |
|---------|-------|-------------|
| `new` | `:new <path> [tag1 tag2...]` | Create a note and open in editor |
| `open` | `:open <path>` | Open an existing note |
| `search` | `:search <query>` | Full-text search across all notes |
| `tag` | `:tag <path> <tag1> [tag2...]` | Add tags to a note |
| `untag` | `:untag <path> <tag1> [tag2...]` | Remove tags from a note |
| `ls` | `:ls [folder]` | List notes (optionally in a folder) |
| `cd` | `:cd [folder]` | Change folder context (`/` for root) |
| `mv` | `:mv <old> <new>` | Move or rename a note |
| `rename` | `:rename <new-name>` | Rename selected note (stays in same folder) |
| `rm` | `:rm <path>` | Delete a note |
| `tags` | `:tags` | Show all tags with note counts |
| `todo` | `:todo <title> [#tag] [@due(YYYY-MM-DD)] [--folder <path>]` | Create a todo note |
| `todo-due` | `:todo-due <YYYY-MM-DD>` or `:todo-due clear` | Set or clear due date on selected todo |
| `todos` | `:todos` | Show all todos sorted by due date |
| `sync` | `:sync` | Pull from remote and reload all notes |
| `remote` | `:remote <git-url>` | Configure git remote |
| `fixfm` | `:fixfm` | Add frontmatter to notes missing it |
| `help` | `:help` | Show help |
| `quit` | `:quit` | Exit memoria |

## CLI Subcommands

Memoria can be used from the command line without opening the TUI. Every command works both when the TUI is running and when it's not.

### Commands Reference

#### `memoria search <query> [--exact]`

Full-text search across all notes using the Bleve index. Results are ranked by relevance.

Multiple words are AND'd — a note must contain all words to match. Use `--exact` for exact phrase matching (words must appear in that exact order).

```bash
memoria search "meeting notes"                # Notes containing both "meeting" AND "notes"
memoria search "database migration" --json     # JSON output with highlights
memoria search --exact "approved and built"    # Exact phrase match
```

Output includes the note path and relevance score. With `--json`, also includes matched text fragments with `<mark>` highlights.

#### `memoria list [folder]`

List note paths. Without arguments, lists all notes recursively. With a folder name, lists only notes in that folder.

```bash
memoria list                    # All notes
memoria list Projects           # Notes in Projects/
memoria list Projects --json    # As JSON array
```

#### `memoria tags`

List all tags with note counts.

```bash
memoria tags
memoria tags --json
```

#### `memoria todos [filter]`

List todo notes. Optional filter: `overdue`, `today`, `pending`, `done`.

```bash
memoria todos                   # All todos
memoria todos overdue           # Past due and not done
memoria todos today             # Due today
memoria todos pending           # Not yet done
memoria todos done              # Completed
memoria todos --json            # JSON output
```

#### `memoria cat <path>`

Print a note's markdown content to stdout. The path is relative to your notes directory.

```bash
memoria cat daily.md
memoria cat Projects/roadmap.md
memoria cat Projects/roadmap.md --json   # Content as JSON string
```

#### `memoria sync`

Pull from the git remote, reindex all notes, then commit and push local changes. Equivalent to the TUI's `:sync` command.

```bash
memoria sync
```

#### `memoria new <path> [--tags tag1,tag2]`

Create a new note. The path is relative to the notes directory; folders are created automatically. Tags are comma-separated.

Content can be piped via stdin. Without stdin, an empty note (frontmatter only) is created.

```bash
memoria new ideas/cool-project.md
memoria new logs/deploy.md --tags deploy,ops

# Pipe content from stdin
echo "# Meeting Notes" | memoria new meetings/standup.md

# Here-doc
memoria new logs/deploy.md --tags ops <<EOF
# Deploy Log
Deployed v2.3.1 to production
EOF

# From a file
cat draft.md | memoria new final/report.md
```

#### `memoria edit <path>`

Update the content of an existing note. The new content is read from stdin. Frontmatter metadata (tags, dates) is preserved. Fails if the note does not exist.

```bash
echo "# Updated Content" | memoria edit ideas/cool-project.md

# Rewrite a note from a file
cat revised.md | memoria edit reports/quarterly.md

# Using a here-doc
memoria edit logs/deploy.md <<EOF
# Deploy Log (Updated)
Rolled back v2.3.1 due to errors
EOF
```

#### `memoria todo <title> [--folder F] [--tags t1,t2] [--due YYYY-MM-DD]`

Create a new todo note. The title is slugified into a filename (e.g. "Buy groceries" → `buy-groceries.md`). Defaults to the `TODO/` folder.

```bash
memoria todo "Buy groceries"
memoria todo "Review PR" --folder "Work/tasks" --tags review,urgent
memoria todo "Submit report" --due 2026-04-15
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output structured JSON instead of human-readable text |
| `--home <dir>` | Use a custom config/data directory instead of `~/.memoria` |
| `--version`, `-v` | Print version and exit |
| `--help`, `-h` | Print usage and exit |

### How It Works: TUI + CLI Coordination

When you run a CLI command while the TUI is open in another terminal, the two coordinate automatically:

1. **The TUI starts a local Unix socket** at `~/.memoria/memoria.sock` on launch
2. **CLI commands connect to that socket**, sending the request to the running TUI process
3. **The TUI executes the command** using its already-open services (including the Bleve full-text index)
4. **After write commands** (`sync`, `new`, `todo`), the TUI automatically refreshes its note list and tags — you'll see the changes appear immediately

When no TUI is running, CLI commands open the stores (filesystem, SQLite, Bleve, git) directly. Everything works the same, there's just no live TUI to refresh.

This design means:
- **No lock conflicts** — the CLI never fights the TUI for file locks
- **Full search power** — CLI search uses the same Bleve index as the TUI, not a degraded fallback
- **Instant feedback** — create a note via CLI and see it appear in the TUI within a second

### External Integration Examples

#### GitHub Copilot CLI / AI Assistants

The `--json` flag makes output machine-parseable, which is ideal for AI tools operating on your notes:

```bash
# AI assistant searches your notes
memoria search "authentication flow" --json | jq '.[].Path'

# AI creates a note with content piped in
memoria new "summaries/meeting-2024-03-15.md" --tags meeting,summary

# AI checks what's overdue
memoria todos overdue --json
```

#### Shell Scripts and Automation

```bash
# Daily backup of note list
memoria list --json > ~/backups/memoria-notes-$(date +%F).json

# Create a daily journal entry
memoria new "journal/$(date +%Y-%m-%d).md" --tags journal,daily

# Sync from a cron job (works even without TUI)
memoria sync

# Count notes per tag
memoria tags --json | jq -r '.[] | "\(.Tag): \(.Count)"'

# Find all notes about a topic and open them
memoria search "kubernetes" --json | jq -r '.[].Path' | xargs -I{} memoria cat {}
```

#### Pipe-Friendly

All commands write to stdout and errors to stderr, so they compose naturally with Unix pipes:

```bash
# Search and preview matches
memoria search "TODO" --json | jq -r '.[].Path' | while read p; do
  echo "=== $p ==="
  memoria cat "$p" | head -5
  echo
done
```

## MCP Server

Memoria includes a built-in [MCP](https://modelcontextprotocol.io/) (Model Context Protocol) server, allowing AI assistants like GitHub Copilot CLI, VS Code Copilot, Claude Desktop, and other MCP clients to interact with your notes directly.

### Starting the MCP server

```bash
memoria mcp
```

This starts the MCP server on stdio (stdin/stdout). You don't typically run this manually — it's launched by your MCP client.

### Configuring with GitHub Copilot CLI

Add to your [MCP configuration](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers):

```json
{
  "mcpServers": {
    "memoria": {
      "command": "memoria",
      "args": ["mcp"]
    }
  }
}
```

To use a custom notes directory:

```json
{
  "mcpServers": {
    "memoria": {
      "command": "memoria",
      "args": ["mcp", "--home", "/path/to/notes"]
    }
  }
}
```

### Available tools

| Tool | Description |
|------|-------------|
| `search` | Full-text search across all notes with fuzzy matching |
| `list` | List notes, optionally filtered by folder |
| `cat` | Read a note's full content |
| `tags` | List all tags with note counts |
| `todos` | List todos, optionally filtered (overdue/today/pending/done) |
| `new` | Create a new note with optional content and tags |
| `edit` | Update the content of an existing note |
| `todo` | Create a new todo |
| `sync` | Sync notes with the git remote |

### How it works

The MCP server delegates each tool call to the `memoria` CLI with `--json` output. This means:

- If the TUI is running, tool calls go through the IPC socket and the TUI refreshes automatically
- If no TUI is running, stores are opened directly for each call
- No extra ports, no HTTP — just process-level stdio communication

### Environment variables

| Variable | Description |
|----------|-------------|
| `MEMORIA_BIN` | Override the path to the memoria binary used for tool execution |

## Configuration

Config lives at `~/.memoria/config.yaml`:

```yaml
# Directory where notes are stored (supports ~ expansion)
notes_dir: ~/.memoria/notes

# Git remote URL (set via :remote command)
git_remote: ""

# Editor command (leave empty to use $EDITOR, $VISUAL, or vim)
editor: ""

# Color theme: "dark" (Catppuccin Mocha) or "light" (Catppuccin Latte)
theme: dark

# Start with all folders expanded (default: true)
expand_folders: true

# Show the virtual "Pinned" section at the top of the tree (default: true)
show_pinned_notes: true

# Show modification timestamps next to notes (default: true, toggle with t)
show_timestamps: true

# Default folder for todos created with :todo (default: "TODO")
default_todo_folder: "TODO"

# Enable/disable todo features (default: true)
todos_enabled: true
```

### Editor resolution

Memoria picks your editor in this order:

1. `editor` field in config.yaml
2. `$EDITOR` environment variable
3. `$VISUAL` environment variable
4. `vim` (fallback)

Multi-word commands work too (e.g., `editor: emacs -nw`).

### Theme

Memoria ships with two themes based on the [Catppuccin](https://github.com/catppuccin/catppuccin) palette:

| Value | Palette | Best for |
|-------|---------|----------|
| `dark` (default) | Catppuccin Mocha | Dark terminal backgrounds |
| `light` | Catppuccin Latte | Light terminal backgrounds |

Set it in `~/.memoria/config.yaml`:

```yaml
theme: light
```

Restart memoria after changing the theme.

## How notes are stored

Each note is a Markdown file with YAML frontmatter:

```markdown
---
tags:
  - go
  - project
created: 2026-04-01T10:00:00Z
modified: 2026-04-02T15:30:00Z
---

# My Note

Content goes here...
```

### Todos

Todo notes have extra frontmatter fields:

```markdown
---
tags:
  - work
todo: true
done: false
due: 2026-04-15
created: 2026-04-07T12:00:00Z
modified: 2026-04-07T12:00:00Z
---

Details about the task...
```

Create todos with `:todo fix the auth bug #work @due(2026-04-15)`. Press **x** on a todo to toggle done/undone. Use `:todos` to see all todos sorted by due date. Overdue todos are shown in red, due-today in yellow, done items are dimmed. The status bar shows a count of pending todos and overdue items.

Notes live in `~/.memoria/notes/` and can be nested in folders:

```
~/.memoria/notes/
├── work/
│   ├── meeting-notes.md
│   └── project-ideas.md
├── learning/
│   ├── go/
│   │   └── concurrency.md
│   └── rust/
│       └── ownership.md
└── daily.md
```

## Architecture

Memoria uses a three-layer storage design:

| Layer | Location | Purpose |
|-------|----------|---------|
| **Filesystem** | `~/.memoria/notes/` | Source of truth — plain Markdown files |
| **SQLite** | `~/.memoria/meta.db` | Fast metadata lookups, folder listing, tag queries |
| **Bleve** | `~/.memoria/search.bleve/` | Full-text search index |

All three stay in sync automatically. The Markdown files are what gets committed to git.

## Tech stack

- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss v2](https://github.com/charmbracelet/lipgloss) — Terminal styling
- [Glamour v2](https://github.com/charmbracelet/glamour) — Markdown rendering
- [Bleve v2](https://github.com/blevesearch/bleve) — Full-text search engine
- [go-git v5](https://github.com/go-git/go-git) — Pure Go git implementation
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — Pure Go SQLite
- [Catppuccin](https://github.com/catppuccin/catppuccin) — Mocha (dark) & Latte (light) color schemes

Everything is pure Go — no CGO, no system dependencies.

## Development

```bash
# Run tests (327 tests)
make test

# Run with race detector, short mode
make test-short

# Coverage report
make test-cover
open coverage.html

# Lint
make lint

# Build
make build

# Run directly
make run
```

## Releasing

Releases are automated with [GoReleaser](https://goreleaser.com/) via GitHub Actions:

```bash
git tag v0.2.0
git push --tags
```

This builds binaries for all platforms, creates a GitHub Release, and updates the Homebrew formula.

## License

[MIT](LICENSE)
