package tui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/memoria/internal/note"
	"github.com/cassiomarques/memoria/internal/service"
	"github.com/cassiomarques/memoria/internal/storage"
	"github.com/cassiomarques/memoria/internal/tui/components"
	"github.com/cassiomarques/memoria/internal/tui/theme"
)

// focusedPane tracks which pane currently has keyboard focus.
type focusedPane int

const (
	focusList focusedPane = iota
	focusPreview
)

// filterState tracks the current phase of the fuzzy filter workflow.
type filterState int

const (
	filterOff      filterState = iota // no filter active
	filterTyping                      // user is typing a filter query
	filterBrowsing                    // filtered results with normal keybindings
)

// editorFinishedMsg is sent when an external editor process completes.
type editorFinishedMsg struct {
	path string
	err  error
}

// commandResultMsg is sent when a command completes with a status message.
type commandResultMsg struct {
	message string
	isError bool
}

// clearMessageMsg signals that the status bar message should be cleared.
type clearMessageMsg struct{}

// refreshTickMsg triggers a periodic re-render so relative timestamps stay fresh.
type refreshTickMsg struct{}

// ExternalRefreshMsg is sent by the IPC server (via tea.Program.Send) when a
// CLI command mutates data. The TUI handles it by refreshing the note list and
// tags — the same way it refreshes after an internal write operation.
type ExternalRefreshMsg struct{}

// gitSyncMsg is sent when the background git sync worker completes a commit+push.
// A nil Err means the push succeeded; non-nil means it failed (data is safe locally).
type gitSyncMsg struct {
	Err error
}

// App is the root Bubble Tea model that composes all TUI components.
type App struct {
	noteList   components.NoteList
	commandBar components.CommandBar
	statusBar  components.StatusBar
	preview    components.Preview

	focusedPane focusedPane
	// Track which note is currently previewed
	previewedPath string
	// customPreview is true when the preview pane shows non-note content
	// (e.g., tags list, help). Auto-preview should not overwrite it.
	customPreview bool

	width  int
	height int
	styles theme.Styles

	svc           *service.NoteService
	currentFolder string
	allTags       []string
	pendingClear  bool // schedule a message clear after setMessage

	// Delete confirmation state
	pendingDelete         bool
	pendingDeletePath     string // note path or folder path
	pendingDeleteIsFolder bool

	// Create-in-folder state
	pendingCreate       bool
	pendingCreateFolder string // target folder path

	// Fuzzy filter state (/ key)
	filterState filterState
	filterBuf   string // current filter text

	version           string
	defaultTodoFolder string
	headerCache       string // rendered header, updated on resize
}

// NewApp creates a new App with all sub-components initialized (no service).
func NewApp() App {
	return App{
		noteList:    components.NewNoteList(),
		commandBar:  components.NewCommandBar(),
		statusBar:   components.NewStatusBar(),
		preview:     components.NewPreview(),
		focusedPane: focusList,
		styles:      theme.DefaultStyles(),
	}
}

// AppOptions holds optional configuration for the TUI app.
type AppOptions struct {
	ExpandFolders     bool
	ShowPinnedNotes   bool
	ShowTimestamps    bool
	Version           string
	DefaultTodoFolder string
}

// NewAppWithService creates an App wired to the NoteService, loading initial data.
func NewAppWithService(svc *service.NoteService, opts AppOptions) App {
	noteList := components.NewNoteList()
	noteList.SetExpandAll(opts.ExpandFolders)
	noteList.SetShowPinned(opts.ShowPinnedNotes)
	noteList.SetShowModified(opts.ShowTimestamps)
	noteList.SetTodoFolder(opts.DefaultTodoFolder)

	a := App{
		noteList:          noteList,
		commandBar:        components.NewCommandBar(),
		statusBar:         components.NewStatusBar(),
		preview:           components.NewPreview(),
		focusedPane:       focusList,
		styles:            theme.DefaultStyles(),
		svc:               svc,
		version:           opts.Version,
		defaultTodoFolder: opts.DefaultTodoFolder,
	}

	_ = a.refreshNoteList()
	a.refreshTags()
	a.statusBar.SetSynced(true)

	return a
}

func (a App) Init() tea.Cmd {
	cmds := []tea.Cmd{refreshTickCmd()}
	if a.svc != nil {
		cmds = append(cmds, waitForSyncCmd(a.svc.SyncResults()))
	}
	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.resizeComponents()
		return a, nil

	case editorFinishedMsg:
		if msg.err != nil {
			a.setMessage("Editor error: "+msg.err.Error(), true)
		} else if a.svc != nil {
			_, err := a.svc.AfterEdit(msg.path)
			if err != nil {
				a.setMessage("After edit error: "+err.Error(), true)
			} else {
				a.setMessage("Edited: "+msg.path, false)
				_ = a.refreshNoteList()
				// Refresh preview if we edited the previewed note
				if a.preview.Visible() && a.previewedPath == msg.path {
					n, err := a.svc.Get(msg.path)
					if err == nil {
						a.preview.SetContent(n.Title, n.Content)
					}
				}
			}
		}
		return a, clearMessageCmd()

	case commandResultMsg:
		a.setMessage(msg.message, msg.isError)
		return a, clearMessageCmd()

	case clearMessageMsg:
		// Don't auto-clear confirmation prompts that require user input
		if a.pendingDelete {
			return a, nil
		}
		a.statusBar.ClearMessage()
		// Restore persistent filter indicator if still filtering
		if a.filterState != filterOff {
			a.updateFilterStatus()
		}
		return a, nil

	case refreshTickMsg:
		// No state change needed — returning triggers View() which recalculates
		// relative timestamps. Schedule the next tick.
		return a, refreshTickCmd()

	case ExternalRefreshMsg:
		// An external CLI command mutated data. Refresh the note list and tags
		// so the TUI reflects the changes immediately.
		_ = a.refreshNoteList()
		a.refreshTags()
		a.setMessage("Refreshed (external change)", false)
		return a, clearMessageCmd()

	case gitSyncMsg:
		// The background git worker finished a commit+push cycle.
		// Re-subscribe for the next result, and update the sync indicator.
		var nextCmd tea.Cmd
		if a.svc != nil {
			nextCmd = waitForSyncCmd(a.svc.SyncResults())
		}
		if msg.Err != nil {
			a.statusBar.SetSynced(false)
			a.setMessage("Sync failed: "+msg.Err.Error(), true)
			return a, tea.Batch(nextCmd, clearMessageCmd())
		}
		a.statusBar.SetSynced(true)
		return a, nextCmd

	case tea.KeyPressMsg:
		key := msg.String()

		// Handle delete confirmation
		if a.pendingDelete {
			if key == "y" || key == "Y" {
				a.confirmDelete()
			} else {
				a.statusBar.ClearMessage()
			}
			a.pendingDelete = false
			a.pendingDeletePath = ""
			a.pendingDeleteIsFolder = false
			return a, clearMessageCmd()
		}

		// Fuzzy filter typing mode — intercept all keys
		if a.filterState == filterTyping {
			return a.handleFilterKey(key)
		}

		// Fuzzy filter browsing mode — Esc closes preview first, then clears filter
		if a.filterState == filterBrowsing {
			switch key {
			case "esc":
				if a.preview.Visible() {
					a.preview.Toggle()
					a.previewedPath = ""
					a.focusedPane = focusList
					a.resizeComponents()
					a.updateFocusStyles()
					return a, nil
				}
				a.clearFilter()
				return a, nil
			case "/":
				a.filterState = filterTyping
				return a, nil
			}
			// All other keys fall through to normal handling below
		}

		// Global quit keys (only when command bar is not active)
		if !a.commandBar.Active() {
			switch key {
			case "ctrl+c":
				return a, tea.Quit
			case "q":
				// Contextual: close preview/help if visible, otherwise quit
				if a.preview.Visible() {
					a.preview.Toggle()
					a.previewedPath = ""
					a.focusedPane = focusList
					a.resizeComponents()
					a.updateFocusStyles()
					a.statusBar.ClearMessage()
					return a, nil
				}
				return a, tea.Quit
			case "esc":
				// Close preview/help pane if visible
				if a.preview.Visible() {
					a.preview.Toggle()
					a.previewedPath = ""
					a.focusedPane = focusList
					a.resizeComponents()
					a.updateFocusStyles()
					a.statusBar.ClearMessage()
					return a, nil
				}
			case ":":
				cmd := a.commandBar.Focus()
				cmds = append(cmds, cmd)
				return a, tea.Batch(cmds...)
			case "/":
				a.filterState = filterTyping
				a.filterBuf = ""
				a.focusedPane = focusList
				a.noteList.SetFilter("")
				a.setMessage("🔍 Type to filter (Esc to cancel)", false)
				return a, nil
			case "p":
				sel := a.noteList.SelectedItem()
				if sel == nil {
					// On a folder — just toggle visibility
					a.preview.Toggle()
					a.resizeComponents()
					a.updateFocusStyles()
					return a, nil
				}
				if a.preview.Visible() && a.previewedPath == sel.Path {
					// Same note — toggle off
					a.preview.Toggle()
					a.previewedPath = ""
					a.resizeComponents()
					a.updateFocusStyles()
				} else {
					// Load this note into preview
					a.loadPreview(sel)
					if !a.preview.Visible() {
						a.preview.Toggle()
						a.resizeComponents()
					}
					a.updateFocusStyles()
				}
				return a, nil
			case "tab":
				if a.preview.Visible() {
					if a.focusedPane == focusList {
						a.focusedPane = focusPreview
					} else {
						a.focusedPane = focusList
					}
					a.updateFocusStyles()
				}
				return a, nil
			case "e":
				// Edit the previewed note when preview is focused
				if a.focusedPane == focusPreview && a.preview.Visible() && a.previewedPath != "" {
					cmd := a.openInEditor(a.previewedPath, a.preview.EstimateSourceLine())
					return a, cmd
				}
			case "y":
				// Copy previewed note content to clipboard
				if a.focusedPane == focusPreview && a.preview.Visible() {
					a.copyPreviewToClipboard()
					return a, nil
				}
			case "enter":
				// Open selected note in editor
				if a.svc != nil {
					sel := a.noteList.SelectedItem()
					if sel != nil {
						cmd := a.openInEditor(sel.Path, 0)
						return a, cmd
					}
				}
				return a, nil
			case "h", "left":
				// Collapse focused folder, or collapse parent if on a note
				a.noteList.CollapseSelected()
				return a, nil
			case "H":
				a.noteList.CollapseAll()
				return a, nil
			case "l", "right":
				// Expand collapsed folder
				if a.noteList.SelectedIsFolder() && !a.noteList.SelectedIsExpanded() {
					a.noteList.ExpandSelected()
				}
				return a, nil
			case "L":
				a.noteList.ExpandAll()
				return a, nil
			case "?":
				a.cmdHelp()
				return a, nil
			case "d":
				a.initiateDelete()
				return a, nil
			case "n":
				a.initiateCreate()
				return a, nil
			case "b":
				a.togglePin()
				return a, nil
			case "x":
				a.toggleTodoDone()
				return a, clearMessageCmd()
			case "t":
				show := a.noteList.ToggleShowModified()
				if show {
					a.setMessage("🕐 Showing modification times", false)
				} else {
					a.setMessage("🕐 Hiding modification times", false)
				}
				return a, clearMessageCmd()
			}
		} else {
			// Command bar is active

			// When suggestion menu is showing, intercept navigation keys
			if a.commandBar.ShowingMenu() {
				switch key {
				case "esc":
					a.commandBar.DismissMenu()
					return a, nil
				case "tab", "down", "j":
					a.commandBar.NextSuggestion()
					return a, nil
				case "up", "k":
					a.commandBar.PrevSuggestion()
					return a, nil
				case "enter":
					if a.commandBar.AcceptSuggestion() {
						return a, nil
					}
					// No suggestion selected — dismiss and fall through to execute
					a.commandBar.DismissMenu()
				default:
					// Any other key dismisses the menu and falls through to normal input
					a.commandBar.DismissMenu()
				}
			}

			switch key {
			case "ctrl+c":
				return a, tea.Quit
			case "esc":
				a.commandBar.Blur()
				a.commandBar.Reset()
				a.pendingCreate = false
				a.pendingCreateFolder = ""
				a.updateFocusStyles()
				return a, nil
			case "tab":
				a.updateSuggestions()
				a.commandBar.CycleSuggestion()
				return a, nil
			case "enter":
				input := a.commandBar.Value()
				a.commandBar.Reset()
				a.commandBar.Blur()
				a.updateFocusStyles()

				// If we're in create-in-folder mode, treat input as note name
				if a.pendingCreate {
					folder := a.pendingCreateFolder
					a.pendingCreate = false
					a.pendingCreateFolder = ""
					createCmd := a.createNoteInFolder(folder, input)
					return a, createCmd
				}

				cmd, err := ParseCommand(input)
				if err != nil {
					a.setMessage(err.Error(), true)
					return a, clearMessageCmd()
				}
				teaCmd := a.executeCommand(cmd)
				if a.pendingClear {
					a.pendingClear = false
					if teaCmd != nil {
						return a, tea.Batch(teaCmd, clearMessageCmd())
					}
					return a, clearMessageCmd()
				}
				if teaCmd != nil {
					return a, teaCmd
				}
				return a, nil
			}

			// Update suggestions on each keystroke (for non-special keys)
			var inputCmd tea.Cmd
			a.commandBar, inputCmd = a.commandBar.Update(msg)
			cmds = append(cmds, inputCmd)
			a.updateSuggestions()
			return a, tea.Batch(cmds...)
		}
	}

	// Route updates to focused component
	switch {
	case a.commandBar.Active():
		var cmd tea.Cmd
		a.commandBar, cmd = a.commandBar.Update(msg)
		cmds = append(cmds, cmd)
	case a.focusedPane == focusPreview && a.preview.Visible():
		var cmd tea.Cmd
		a.preview, cmd = a.preview.Update(msg)
		cmds = append(cmds, cmd)
	default:
		var cmd tea.Cmd
		a.noteList, cmd = a.noteList.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Auto-update preview when navigating the tree (skip custom content like tags/help)
	if a.preview.Visible() && a.focusedPane == focusList && !a.customPreview {
		if sel := a.noteList.SelectedItem(); sel != nil && sel.Path != a.previewedPath {
			a.loadPreview(sel)
		}
	}

	// Schedule auto-clear for status messages
	if a.pendingClear {
		a.pendingClear = false
		cmds = append(cmds, clearMessageCmd())
	}

	return a, tea.Batch(cmds...)
}

// loadPreview loads the selected note's content into the preview pane.
func (a *App) loadPreview(sel *components.NoteItem) {
	if a.svc != nil {
		n, err := a.svc.Get(sel.Path)
		if err != nil {
			a.preview.SetContent(sel.Title, fmt.Sprintf("*Error loading note: %s*", err))
		} else {
			a.preview.SetContent(sel.Title, n.Content)
		}
	} else {
		a.preview.SetContent(sel.Title, fmt.Sprintf("# %s\n\n*No service configured*", sel.Title))
	}
	a.previewedPath = sel.Path
	a.customPreview = false
}

// initiateDelete starts the delete confirmation flow for the selected item.
func (a *App) initiateDelete() {
	if a.svc == nil {
		return
	}

	if a.noteList.SelectedIsFolder() {
		folder := a.noteList.SelectedFolderPath()
		count := a.noteList.SelectedFolderNoteCount()
		if folder == "" {
			return
		}
		a.pendingDelete = true
		a.pendingDeletePath = folder
		a.pendingDeleteIsFolder = true
		a.setMessage(fmt.Sprintf("Delete folder '%s' and all %d notes? (y/N)", folder, count), true)
	} else {
		sel := a.noteList.SelectedItem()
		if sel == nil {
			return
		}
		a.pendingDelete = true
		a.pendingDeletePath = sel.Path
		a.pendingDeleteIsFolder = false
		a.setMessage(fmt.Sprintf("Delete '%s'? (y/N)", sel.Path), true)
	}
}

// confirmDelete executes the pending deletion.
func (a *App) confirmDelete() {
	if a.svc == nil {
		return
	}

	a.noteList.PrepareCursorForDelete()

	if a.pendingDeleteIsFolder {
		count, err := a.svc.DeleteFolder(a.pendingDeletePath)
		if err != nil {
			a.setMessage("Delete failed: "+err.Error(), true)
		} else {
			a.setMessage(fmt.Sprintf("Deleted folder '%s' (%d notes)", a.pendingDeletePath, count), false)
		}
	} else {
		a.deleteNote(a.pendingDeletePath)
	}
	_ = a.refreshNoteList()
	a.refreshTags()
}

// deleteNote deletes a single note and clears the preview if it was showing
// that note. Caller is responsible for refreshNoteList/refreshTags.
func (a *App) deleteNote(path string) bool {
	err := a.svc.Delete(path)
	if err != nil {
		a.setMessage("Delete failed: "+err.Error(), true)
		return false
	}
	a.setMessage(fmt.Sprintf("Deleted: %s", path), false)
	if a.previewedPath == path {
		a.preview.SetContent("", "")
		a.previewedPath = ""
	}
	return true
}

// togglePin pins or unpins the currently selected note.
func (a *App) togglePin() {
	if a.svc == nil {
		return
	}
	sel := a.noteList.SelectedItem()
	if sel == nil {
		a.setMessage("📌 Select a note to bookmark", false)
		return
	}
	nowPinned, err := a.svc.TogglePin(sel.Path)
	if err != nil {
		a.setMessage("Error: "+err.Error(), true)
		return
	}
	_ = a.refreshNoteList()
	if nowPinned {
		a.setMessage("📌 Bookmarked: "+sel.Title, false)
	} else {
		a.setMessage("Removed bookmark: "+sel.Title, false)
	}
}

// toggleTodoDone flips the done status of the currently selected todo note.
func (a *App) toggleTodoDone() {
	if a.svc == nil {
		return
	}
	sel := a.noteList.SelectedItem()
	if sel == nil {
		a.setMessage("Select a note first", false)
		return
	}
	if !sel.Todo {
		a.setMessage("Not a todo", false)
		return
	}
	nowDone, err := a.svc.ToggleTodoDone(sel.Path)
	if err != nil {
		a.setMessage("Error: "+err.Error(), true)
		return
	}
	_ = a.refreshNoteList()
	if nowDone {
		a.setMessage("✅ Done: "+sel.Title, false)
	} else {
		a.setMessage("⭕ Reopened: "+sel.Title, false)
	}
}

// copyPreviewToClipboard copies the raw markdown of the previewed note to the system clipboard.
func (a *App) copyPreviewToClipboard() {
	content := a.preview.Content()
	if content == "" {
		a.setMessage("Nothing to copy", false)
		return
	}
	if err := clipboard.WriteAll(content); err != nil {
		a.setMessage("Copy failed: "+err.Error(), true)
		return
	}
	a.setMessage("📋 Copied to clipboard", false)
}

// initiateCreate starts the create-note-in-folder flow.
func (a *App) initiateCreate() {
	if a.svc == nil {
		return
	}

	// Determine target folder
	var folder string
	if a.noteList.SelectedIsFolder() {
		folder = a.noteList.SelectedFolderPath()
	} else if sel := a.noteList.SelectedItem(); sel != nil && sel.Folder != "" {
		folder = sel.Folder
	}

	a.pendingCreate = true
	a.pendingCreateFolder = folder

	a.commandBar.SetLabel("NEW")
	if folder != "" {
		a.commandBar.SetPlaceholder(folder + "/...")
	} else {
		a.commandBar.SetPlaceholder("note name...")
	}
	a.commandBar.Focus()
}

// createNoteInFolder creates a note inside the given folder and opens it in the editor.
func (a *App) createNoteInFolder(folder, name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		a.setMessage("Note name cannot be empty", true)
		return clearMessageCmd()
	}

	var path string
	if folder != "" {
		path = folder + "/" + name
	} else {
		path = name
	}

	return a.cmdNew([]string{path})
}

// updateFocusStyles sets the focused state on the preview pane.
func (a *App) updateFocusStyles() {
	a.preview.SetFocused(a.focusedPane == focusPreview && a.preview.Visible())
}

// setMessage shows a message in the status bar and schedules auto-clear.
func (a *App) setMessage(msg string, isError bool) {
	var style lipgloss.Style
	if isError {
		style = a.styles.ErrorMessage
	} else {
		style = a.styles.SuccessMessage
	}
	a.statusBar.SetMessage(msg, style)
	a.pendingClear = true
}

// clearMessageCmd returns a command that clears the message after a delay.
func clearMessageCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return clearMessageMsg{}
	})
}

// refreshTickCmd returns a command that triggers a re-render after one minute.
func refreshTickCmd() tea.Cmd {
	return tea.Tick(time.Minute, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

// waitForSyncCmd returns a tea.Cmd that blocks until the next git sync result
// arrives, then delivers it as a gitSyncMsg. This is how Bubble Tea's Elm
// Architecture handles async I/O: the Cmd runs in a separate goroutine (managed
// by the runtime), keeping Update() non-blocking. When the channel read
// completes, the runtime feeds the resulting Msg back into Update().
func waitForSyncCmd(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		return gitSyncMsg{Err: <-ch}
	}
}

// refreshNoteList reloads notes from the service.
func (a *App) refreshNoteList() error {
	if a.svc == nil {
		return nil
	}

	var items []components.NoteItem

	if a.currentFolder == "" {
		notes, err := a.svc.ListAll()
		if err != nil {
			return err
		}
		for _, n := range notes {
			items = append(items, components.NoteItem{
				Path:     n.Path,
				Title:    n.Title,
				Folder:   n.Folder,
				Tags:     n.Tags,
				Modified: n.Modified,
				Todo:     n.Todo,
				Done:     n.Done,
				Due:      n.Due,
			})
		}
	} else {
		notes, err := a.svc.List(a.currentFolder)
		if err != nil {
			return err
		}
		for _, n := range notes {
			items = append(items, components.NoteItem{
				Path:     n.Path,
				Title:    n.Title,
				Folder:   n.Folder,
				Tags:     n.Tags,
				Modified: n.Modified,
				Todo:     n.Todo,
				Done:     n.Done,
				Due:      n.Due,
			})
		}
	}

	// Mark pinned items
	pinned, err := a.svc.ListPinned()
	if err == nil {
		pinnedSet := make(map[string]bool, len(pinned))
		for _, p := range pinned {
			pinnedSet[p] = true
		}
		for i := range items {
			if pinnedSet[items[i].Path] {
				items[i].Pinned = true
			}
		}
	}

	a.noteList.SetItems(items)
	a.statusBar.SetFolder(a.currentFolder)
	a.statusBar.SetNoteCount(len(items))

	// Reapply filter if browsing filtered results
	if a.filterState == filterBrowsing && a.filterBuf != "" {
		a.applyFilter()
	}

	return nil
}

// refreshTags refreshes the cached tag list for autocomplete.
func (a *App) refreshTags() {
	if a.svc == nil {
		return
	}
	tags, err := a.svc.ListTags()
	if err != nil {
		return
	}
	a.allTags = make([]string, 0, len(tags))
	for _, t := range tags {
		a.allTags = append(a.allTags, t.Tag)
	}
}

// updateSuggestions refreshes autocomplete suggestions based on current input.
func (a *App) updateSuggestions() {
	input := a.commandBar.Value()
	items := a.currentNoteItems()
	suggestions := Completions(input, items, a.allTags)
	a.commandBar.SetSuggestions(suggestions)
}

// currentNoteItems returns the current note list items for completions.
func (a *App) currentNoteItems() []components.NoteItem {
	var items []components.NoteItem
	for i := 0; i < a.noteList.ItemCount(); i++ {
		sel := a.noteList.ItemAt(i)
		if sel != nil {
			items = append(items, *sel)
		}
	}
	return items
}

// executeCommand dispatches a parsed command to the appropriate handler.
func (a *App) executeCommand(cmd *Command) tea.Cmd {
	switch cmd.Name {
	case "new":
		return a.cmdNew(cmd.Args)
	case "open":
		return a.cmdOpen(cmd.Args)
	case "search":
		return a.cmdSearch(cmd.Args)
	case "recent":
		return a.cmdRecent(cmd.Args)
	case "all":
		return a.cmdAll()
	case "tag":
		return a.cmdTag(cmd.Args)
	case "untag":
		return a.cmdUntag(cmd.Args)
	case "ls":
		return a.cmdLs(cmd.Args)
	case "cd":
		return a.cmdCd(cmd.Args)
	case "mv":
		return a.cmdMv(cmd.Args)
	case "rm":
		return a.cmdRm(cmd.Args)
	case "tags":
		return a.cmdTags()
	case "todo":
		return a.cmdTodo(cmd.Args)
	case "todos":
		return a.cmdTodos(strings.Join(cmd.Args, " "))
	case "sync":
		return a.cmdSync()
	case "remote":
		return a.cmdRemote(cmd.Args)
	case "fixfm":
		return a.cmdFixFm()
	case "help":
		a.cmdHelp()
		return nil
	case "quit", "q":
		return tea.Quit
	default:
		a.setMessage("Unknown command: "+cmd.Name, true)
		return nil
	}
}

func (a *App) cmdNew(args []string) tea.Cmd {
	if len(args) == 0 {
		a.setMessage("Usage: new <path> [tag1 tag2...]", true)
		return nil
	}

	path := args[0]
	var tags []string
	if len(args) > 1 {
		tags = args[1:]
	}

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	_, err := a.svc.Create(path, "", tags)
	if err != nil {
		a.setMessage("Create failed: "+err.Error(), true)
		return nil
	}

	_ = a.refreshNoteList()
	a.refreshTags()
	return a.openInEditor(path, 0)
}

// cmdTodo creates a new todo note.
// Syntax: :todo <title words> [#tag1 #tag2] [@due(YYYY-MM-DD)] [--folder <path>]
func (a *App) cmdTodo(args []string) tea.Cmd {
	if len(args) == 0 {
		a.setMessage("Usage: todo <title> [#tag] [@due(YYYY-MM-DD)] [--folder <path>]", true)
		return nil
	}

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	var titleWords []string
	var tags []string
	var dueDate *time.Time
	folder := a.defaultTodoFolder
	if folder == "" {
		folder = "TODO"
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--folder" && i+1 < len(args):
			i++
			folder = args[i]
		case strings.HasPrefix(arg, "#"):
			tag := strings.TrimPrefix(arg, "#")
			if tag != "" {
				tags = append(tags, tag)
			}
		case strings.HasPrefix(arg, "@due(") && strings.HasSuffix(arg, ")"):
			dateStr := arg[5 : len(arg)-1] // strip @due( and )
			t, err := time.Parse(time.DateOnly, dateStr)
			if err != nil {
				a.setMessage("Invalid due date (use YYYY-MM-DD): "+dateStr, true)
				return nil
			}
			dueDate = &t
		default:
			titleWords = append(titleWords, arg)
		}
	}

	title := strings.Join(titleWords, " ")
	if title == "" {
		a.setMessage("Todo title cannot be empty", true)
		return nil
	}

	opts := service.CreateTodoOptions{
		Title:  title,
		Folder: folder,
		Tags:   tags,
		Due:    dueDate,
	}

	n, err := a.svc.CreateTodo(opts)
	if err != nil {
		a.setMessage("Create todo failed: "+err.Error(), true)
		return nil
	}

	_ = a.refreshNoteList()
	a.refreshTags()
	a.noteList.SelectByPath(n.Path)
	if a.preview.Visible() {
		if sel := a.noteList.SelectedItem(); sel != nil {
			a.loadPreview(sel)
		}
	}
	a.setMessage("⭕ Created todo: "+title, false)
	return clearMessageCmd()
}

func (a *App) cmdOpen(args []string) tea.Cmd {
	if len(args) == 0 {
		a.setMessage("Usage: open <path>", true)
		return nil
	}

	path := args[0]
	return a.openInEditor(path, 0)
}

func (a *App) cmdSearch(args []string) tea.Cmd {
	if len(args) == 0 {
		a.setMessage("Usage: search <query>", true)
		return nil
	}

	query := strings.Join(args, " ")

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	results, err := a.svc.SearchFuzzy(query, 50)
	if err != nil {
		a.setMessage("Search failed: "+err.Error(), true)
		return nil
	}

	var items []components.NoteItem
	for _, r := range results {
		n, err := a.svc.Get(r.Path)
		if err != nil {
			continue
		}
		items = append(items, components.NoteItem{
			Path:     n.Path,
			Title:    n.Title,
			Folder:   n.Folder,
			Tags:     n.Tags,
			Modified: n.Modified,
			Todo:     n.Todo,
			Done:     n.Done,
			Due:      n.Due,
		})
	}

	a.noteList.SetItems(items)
	a.statusBar.SetNoteCount(len(items))
	a.setMessage(fmt.Sprintf("Found %d results for %q", len(items), query), false)
	return nil
}

func (a *App) cmdRecent(args []string) tea.Cmd {
	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	limit := 20
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			limit = n
		}
	}

	recent, err := a.svc.ListRecent(limit)
	if err != nil {
		a.setMessage("Recent failed: "+err.Error(), true)
		return nil
	}

	var items []components.NoteItem
	for _, nm := range recent {
		items = append(items, components.NoteItem{
			Path:     nm.Path,
			Title:    nm.Title,
			Folder:   nm.Folder,
			Tags:     nm.Tags,
			Modified: nm.Modified,
			Todo:     nm.Todo,
			Done:     nm.Done,
			Due:      nm.Due,
		})
	}

	a.noteList.SetItems(items)
	a.statusBar.SetNoteCount(len(items))
	a.setMessage(fmt.Sprintf("📋 %d most recent notes", len(items)), false)
	return nil
}

func (a *App) cmdAll() tea.Cmd {
	a.currentFolder = ""
	_ = a.refreshNoteList()
	a.setMessage("Showing all notes", false)
	return nil
}

func (a *App) cmdTag(args []string) tea.Cmd {
	if len(args) < 2 {
		a.setMessage("Usage: tag <path> <tag1> [tag2...]", true)
		return nil
	}

	path := args[0]
	tags := args[1:]

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	_, err := a.svc.AddTags(path, tags)
	if err != nil {
		a.setMessage("Tag failed: "+err.Error(), true)
		return nil
	}

	_ = a.refreshNoteList()
	a.refreshTags()
	a.setMessage(fmt.Sprintf("Added tags %v to %s", tags, path), false)
	return nil
}

func (a *App) cmdUntag(args []string) tea.Cmd {
	if len(args) < 2 {
		a.setMessage("Usage: untag <path> <tag1> [tag2...]", true)
		return nil
	}

	path := args[0]
	tags := args[1:]

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	_, err := a.svc.RemoveTags(path, tags)
	if err != nil {
		a.setMessage("Untag failed: "+err.Error(), true)
		return nil
	}

	_ = a.refreshNoteList()
	a.refreshTags()
	a.setMessage(fmt.Sprintf("Removed tags %v from %s", tags, path), false)
	return nil
}

func (a *App) cmdLs(args []string) tea.Cmd {
	folder := ""
	if len(args) > 0 {
		folder = args[0]
	}

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	var items []components.NoteItem
	if folder == "" {
		notes, err := a.svc.ListAll()
		if err != nil {
			a.setMessage("List failed: "+err.Error(), true)
			return nil
		}
		for _, n := range notes {
			items = append(items, components.NoteItem{
				Path: n.Path, Title: n.Title, Folder: n.Folder,
				Tags: n.Tags, Modified: n.Modified,
				Todo: n.Todo, Done: n.Done, Due: n.Due,
			})
		}
	} else {
		notes, err := a.svc.List(folder)
		if err != nil {
			a.setMessage("List failed: "+err.Error(), true)
			return nil
		}
		for _, n := range notes {
			items = append(items, components.NoteItem{
				Path: n.Path, Title: n.Title, Folder: n.Folder,
				Tags: n.Tags, Modified: n.Modified,
				Todo: n.Todo, Done: n.Done, Due: n.Due,
			})
		}
	}

	a.noteList.SetItems(items)
	a.statusBar.SetNoteCount(len(items))
	if folder == "" {
		a.setMessage(fmt.Sprintf("Listed %d notes", len(items)), false)
	} else {
		a.setMessage(fmt.Sprintf("Listed %d notes in %s", len(items), folder), false)
	}
	return nil
}

func (a *App) cmdCd(args []string) tea.Cmd {
	if len(args) == 0 {
		a.currentFolder = ""
	} else {
		folder := args[0]
		if folder == "/" || folder == "~" || folder == ".." {
			a.currentFolder = ""
		} else {
			a.currentFolder = folder
		}
	}

	_ = a.refreshNoteList()
	a.setMessage(fmt.Sprintf("Changed to: %s", a.folderDisplay()), false)
	return nil
}

func (a *App) cmdMv(args []string) tea.Cmd {
	if len(args) < 2 {
		a.setMessage("Usage: mv <old-path> <new-path>", true)
		return nil
	}

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	err := a.svc.Move(args[0], args[1])
	_ = a.refreshNoteList()
	if err != nil {
		a.setMessage("Move failed: "+err.Error(), true)
		return clearMessageCmd()
	}

	a.setMessage(fmt.Sprintf("Moved %s → %s", args[0], args[1]), false)
	return nil
}

func (a *App) cmdRm(args []string) tea.Cmd {
	if len(args) == 0 {
		a.setMessage("Usage: rm <path>", true)
		return nil
	}

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	a.noteList.PrepareCursorForDelete()
	if a.deleteNote(args[0]) {
		_ = a.refreshNoteList()
		a.refreshTags()
	}
	return nil
}

func (a *App) cmdTags() tea.Cmd {
	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	tags, err := a.svc.ListTags()
	if err != nil {
		a.setMessage("Tags failed: "+err.Error(), true)
		return nil
	}

	if len(tags) == 0 {
		a.setMessage("No tags found", false)
		return nil
	}

	var lines []string
	lines = append(lines, "# Tags\n")
	for _, t := range tags {
		lines = append(lines, fmt.Sprintf("- **#%s** (%d notes)", t.Tag, t.Count))
	}
	a.preview.SetContent("Tags", strings.Join(lines, "\n"))
	a.previewedPath = ""
	a.customPreview = true
	if !a.preview.Visible() {
		a.preview.Toggle()
		a.resizeComponents()
	}

	a.setMessage(fmt.Sprintf("%d tags found", len(tags)), false)
	return nil
}

func (a *App) cmdTodos(args string) tea.Cmd {
	// Parse optional filter: overdue, today, pending, done
	filter := strings.TrimSpace(strings.ToLower(args))
	validFilters := map[string]bool{"": true, "overdue": true, "today": true, "pending": true, "done": true}
	if !validFilters[filter] {
		a.setMessage("Unknown filter: "+filter+" (use overdue, today, pending, done)", true)
		return nil
	}

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	todos, err := a.svc.ListTodos()
	if err != nil {
		a.setMessage("Todos failed: "+err.Error(), true)
		return nil
	}

	now := time.Now()
	y1, m1, d1 := now.Date()
	today := time.Date(y1, m1, d1, 0, 0, 0, 0, time.Local)

	// Helper closures for classification
	isOverdue := func(t *storage.NoteMeta) bool {
		return !t.Done && t.Due != nil && time.Date(t.Due.Year(), t.Due.Month(), t.Due.Day(), 0, 0, 0, 0, time.Local).Before(today)
	}
	isDueToday := func(t *storage.NoteMeta) bool {
		if t.Done || t.Due == nil {
			return false
		}
		y2, m2, d2 := t.Due.Date()
		return y1 == y2 && m1 == m2 && d1 == d2
	}

	// Apply filter
	var filtered []*storage.NoteMeta
	for _, t := range todos {
		switch filter {
		case "overdue":
			if isOverdue(t) {
				filtered = append(filtered, t)
			}
		case "today":
			if isDueToday(t) {
				filtered = append(filtered, t)
			}
		case "pending":
			if !t.Done {
				filtered = append(filtered, t)
			}
		case "done":
			if t.Done {
				filtered = append(filtered, t)
			}
		default:
			filtered = append(filtered, t)
		}
	}

	if len(filtered) == 0 {
		label := "todos"
		if filter != "" {
			label = filter + " todos"
		}
		a.setMessage("No "+label+" found", false)
		return nil
	}

	// Build preview
	title := "Todos"
	if filter != "" {
		title = strings.ToUpper(filter[:1]) + filter[1:] + " Todos"
	}

	var lines []string
	lines = append(lines, "# "+title+"\n")

	var pending, done int
	for _, t := range filtered {
		icon := "⭕"
		if t.Done {
			icon = "✅"
			done++
		} else {
			pending++
		}
		dueStr := ""
		if t.Due != nil {
			dueStr = " — due " + t.Due.Format(time.DateOnly)
			if !t.Done {
				if isOverdue(t) {
					dueStr += " ⚠️ **OVERDUE**"
				} else if isDueToday(t) {
					dueStr += " ⏰ **TODAY**"
				}
			}
		}
		tagStr := ""
		if len(t.Tags) > 0 {
			tagStr = " `" + strings.Join(t.Tags, "` `") + "`"
		}
		noteTitle := strings.TrimSuffix(t.Title, ".md")
		lines = append(lines, fmt.Sprintf("- %s **%s**%s%s", icon, noteTitle, dueStr, tagStr))
	}

	lines = append(lines, fmt.Sprintf("\n*%d pending, %d done*", pending, done))
	a.preview.SetContent(title, strings.Join(lines, "\n"))
	a.previewedPath = ""
	a.customPreview = true
	if !a.preview.Visible() {
		a.preview.Toggle()
		a.resizeComponents()
	}

	a.setMessage(fmt.Sprintf("%d %s (%d pending)", len(filtered), strings.ToLower(title), pending), false)
	return nil
}

func (a *App) cmdSync() tea.Cmd {
	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	err := a.svc.Sync()
	if err != nil {
		a.setMessage("Sync failed: "+err.Error(), true)
		return nil
	}

	_ = a.refreshNoteList()
	a.refreshTags()

	if a.svc.HasRemote() {
		a.statusBar.SetSynced(true)
		a.setMessage("Synced with remote", false)
	} else {
		a.statusBar.SetSynced(false)
		a.setMessage("Notes reloaded (no git remote configured)", false)
	}
	return nil
}

func (a *App) cmdFixFm() tea.Cmd {
	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}
	count, err := a.svc.EnsureFrontmatter()
	if err != nil {
		a.setMessage("Fix frontmatter failed: "+err.Error(), true)
		return nil
	}
	if count == 0 {
		a.setMessage("All notes already have frontmatter", false)
	} else {
		a.setMessage(fmt.Sprintf("Added frontmatter to %d notes", count), false)
	}
	_ = a.refreshNoteList()
	a.refreshTags()
	return nil
}

func (a *App) cmdRemote(args []string) tea.Cmd {
	if len(args) == 0 {
		if a.svc != nil && a.svc.HasRemote() {
			a.setMessage("Remote is already configured", false)
		} else {
			a.setMessage("Usage: remote <git-url>", true)
		}
		return nil
	}

	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	url := args[0]
	err := a.svc.SetRemote(url)
	if err != nil {
		a.setMessage("Remote setup failed: "+err.Error(), true)
		return nil
	}

	_ = a.refreshNoteList()
	a.refreshTags()
	a.statusBar.SetSynced(true)
	a.setMessage("Remote configured and notes pulled", false)
	return nil
}

func (a *App) cmdHelp() {
	helpContent := `# Memoria — Commands

| Command | Description |
|---------|-------------|
| **new** *path* [tags...] | Create a new note and open in editor |
| **open** *path* | Open a note in your editor |
| **search** *query* | Fuzzy search notes |
| **recent** [N] | Show N most recently modified notes (default 20) |
| **all** | Show all notes (reset filtered/recent view) |
| **tag** *path* *tag1* [tag2...] | Add tags to a note |
| **untag** *path* *tag1* [tag2...] | Remove tags from a note |
| **ls** [folder] | List notes (optionally in a folder) |
| **cd** [folder] | Change current folder |
| **mv** *old* *new* | Move/rename a note |
| **rm** *path* | Delete a note |
| **tags** | Show all tags |
| **todo** *title* [#tag] [@due(YYYY-MM-DD)] [--folder *path*] | Create a todo note |
| **todos** | Show all todos sorted by due date |
| **sync** | Sync with git remote |
| **remote** *url* | Set git remote and pull notes |
| **fixfm** | Add frontmatter to notes missing it |
| **help** | Show this help |
| **quit** / **q** | Exit |

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| **:** | Open command bar |
| **/** | Search/filter notes |
| **Tab** | Switch focus / autocomplete |
| **p** | Preview selected note |
| **e** | Edit previewed note (when preview focused) |
| **y** | Copy note content to clipboard (when preview focused) |
| **d** | Delete selected note or folder |
| **n** | Create a new note (in focused folder) |
| **b** | Toggle bookmark on selected note |
| **x** | Toggle todo done/undone |
| **j/k** | Navigate list |
| **h/l, ←/→** | Collapse/expand folder |
| **H/L** | Collapse/expand all folders |
| **Enter** | Open note / toggle folder |
| **?** | Show help |
| **Esc/q** | Close preview/help |
| **q** | Quit (when only tree visible) |

## Search Syntax

| Pattern | Meaning |
|---------|---------|
| **foo** | Fuzzy match word "foo" |
| **foo bar** | Match "foo" AND "bar" (both must match) |
| **"exact phrase"** | Match exact phrase |
| **#tag** | Filter by tag |
| **foo #work** | Match "foo" AND tag "work" |

Type **/** to start searching, **Enter** to lock results and browse, **Esc** to clear.
`
	a.preview.SetContent("Help", helpContent)
	a.previewedPath = ""
	a.customPreview = true
	if !a.preview.Visible() {
		a.preview.Toggle()
		a.resizeComponents()
	}
	a.setMessage("Press Esc to close help", false)
}

// openInEditor launches the configured editor for the given note path.
// If lineNum > 0, passes +N to jump to that line (works with vim, nvim, nano, emacs, etc.).
func (a *App) openInEditor(notePath string, lineNum int) tea.Cmd {
	if a.svc == nil {
		a.setMessage("No service configured", true)
		return nil
	}

	// Ensure .md extension
	if !strings.HasSuffix(notePath, ".md") {
		notePath += ".md"
	}

	absPath := a.svc.AbsPath(notePath)
	editorCmd := a.svc.EditorCommand()
	if editorCmd == "" {
		editorCmd = "vim"
	}

	parts := strings.Fields(editorCmd)
	args := parts[1:]
	if lineNum > 0 {
		args = append(args, fmt.Sprintf("+%d", lineNum))
	}
	args = append(args, absPath)
	c := exec.Command(parts[0], args...)

	path := notePath
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{path: path, err: err}
	})
}

// handleFilterKey processes key events while in fuzzy filter typing mode.
// All printable keys go to the filter input; only arrows navigate results.
func (a App) handleFilterKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		a.clearFilter()
		return a, nil
	case "enter":
		// Transition to browsing mode if there's an active query with results.
		// Empty query or zero results → exit filter.
		if a.filterBuf == "" || a.noteList.FilteredCount() == 0 {
			a.clearFilter()
			return a, nil
		}
		a.filterState = filterBrowsing
		a.updateFilterStatus()
		return a, nil
	case "ctrl+c":
		return a, tea.Quit
	case "backspace":
		if len(a.filterBuf) > 0 {
			a.filterBuf = a.filterBuf[:len(a.filterBuf)-1]
			a.applyFilter()
		}
		return a, nil
	case "down":
		a.noteList.MoveDown()
		return a, nil
	case "up":
		a.noteList.MoveUp()
		return a, nil
	default:
		// Append printable characters to the filter buffer
		switch {
		case key == "space":
			a.filterBuf += " "
		case len(key) == 1 && key[0] >= 32:
			a.filterBuf += key
		default:
			return a, nil
		}
		a.applyFilter()
		return a, nil
	}
}

// applyFilter runs the combined filter: Bleve content search + in-memory
// fuzzy match on title/path/folder/tags, merged and deduped.
// Uses parsed query tokens with AND semantics.
func (a *App) applyFilter() {
	if a.filterBuf == "" {
		a.noteList.ClearFilter()
		_ = a.refreshNoteList()
		a.updateFilterStatus()
		return
	}

	tokens := note.ParseQuery(a.filterBuf)
	tagTokens := note.TagTokens(tokens)

	seen := make(map[string]bool)
	var items []components.NoteItem

	// Phase 1: Bleve full-text search (handles text/phrase tokens)
	if a.svc != nil {
		results, err := a.svc.SearchFuzzy(a.filterBuf, 50)
		if err == nil {
			for _, r := range results {
				if seen[r.Path] {
					continue
				}
				n, err := a.svc.Get(r.Path)
				if err != nil {
					continue
				}
				// Also check tag filters (Bleve doesn't handle #tag syntax)
				if !matchesTags(n.Tags, tagTokens) {
					continue
				}
				seen[r.Path] = true
				items = append(items, components.NoteItem{
					Path:     n.Path,
					Title:    n.Title,
					Folder:   n.Folder,
					Tags:     n.Tags,
					Modified: n.Modified,
					Todo:     n.Todo,
					Done:     n.Done,
					Due:      n.Due,
				})
			}
		}
	}

	// Phase 2: in-memory fuzzy match on title/path/folder/tags (catches
	// structural matches Bleve may miss, e.g. folder name subsequences)
	allItems := a.noteList.AllItems()
	for i := range allItems {
		item := &allItems[i]
		if seen[item.Path] {
			continue
		}
		if ok, _ := components.NoteMatchesFilter(item, a.filterBuf); ok {
			seen[item.Path] = true
			items = append(items, *item)
		}
	}

	a.noteList.SetFilteredItems(items, a.filterBuf)
	a.updateFilterStatus()
}

// matchesTags checks that all tag tokens match at least one of the note's tags.
func matchesTags(noteTags []string, tagTokens []note.QueryToken) bool {
	for _, tok := range tagTokens {
		found := false
		for _, t := range noteTags {
			if strings.EqualFold(t, tok.Text) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// clearFilter resets filter state to off and restores the full note list.
func (a *App) clearFilter() {
	a.filterState = filterOff
	a.filterBuf = ""
	a.noteList.ClearFilter()
	_ = a.refreshNoteList()
	a.statusBar.ClearMessage()
}

func (a *App) updateFilterStatus() {
	if a.filterBuf == "" {
		a.statusBar.ClearMessage()
		return
	}
	filtered := a.noteList.FilteredCount()
	total := len(a.noteList.AllItems())
	msg := fmt.Sprintf("🔍 /%s  (%d/%d notes)", a.filterBuf, filtered, total)
	// Set status bar directly — filter status should not auto-clear.
	a.statusBar.SetMessage(msg, a.styles.SuccessMessage)
}

func (a *App) folderDisplay() string {
	if a.currentFolder == "" {
		return "/"
	}
	return a.currentFolder
}

const (
	statusBarHeight  = 1
	commandBarHeight = 1
)

// memoriaASCII is the ASCII art title.
var memoriaASCII = []string{
	"▗▄ ▄▖                      █",
	"▐█ █▌                      ▀",
	"▐███▌ ▟█▙ ▐█▙█▖ ▟█▙  █▟█▌ ██   ▟██▖",
	"▐▌█▐▌▐▙▄▟▌▐▌█▐▌▐▛ ▜▌ █▘    █   ▘▄▟▌",
	"▐▌▀▐▌▐▛▀▀▘▐▌█▐▌▐▌ ▐▌ █     █  ▗█▀▜▌",
	"▐▌ ▐▌▝█▄▄▌▐▌█▐▌▝█▄█▘ █   ▗▄█▄▖▐▙▄█▌",
	"▝▘ ▝▘ ▝▀▀ ▝▘▀▝▘ ▝▀▘  ▀   ▝▀▀▀▘ ▀▀▝▘",
}

// colorizeASCII applies Catppuccin colors to the ASCII art.
func colorizeASCII(lines []string) string {
	style := lipgloss.NewStyle().Foreground(theme.ColorLavender)
	var result strings.Builder
	for i, line := range lines {
		result.WriteString(style.Render(line))
		if i < len(lines)-1 {
			result.WriteRune('\n')
		}
	}
	return result.String()
}

func (a *App) buildHeader() string {
	art := colorizeASCII(memoriaASCII)

	versionStyle := lipgloss.NewStyle().Foreground(theme.ColorOverlay0).Italic(true)
	versionLabel := ""
	if a.version != "" {
		versionLabel = versionStyle.Render("v" + a.version)
	}

	tipKey := lipgloss.NewStyle().Foreground(theme.ColorMauve).Bold(true)
	tipText := lipgloss.NewStyle().Foreground(theme.ColorOverlay1)
	tip := tipText.Render("  Tip: ") +
		tipKey.Render("?") + tipText.Render(" help · ") +
		tipKey.Render(":") + tipText.Render(" commands · ") +
		tipKey.Render("/") + tipText.Render(" search")

	inner := art
	if versionLabel != "" {
		inner += "  " + versionLabel
	}
	inner += "\n\n" + tip

	return lipgloss.NewStyle().
		Width(a.width).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSurface2).
		BorderBottom(true).
		BorderLeft(false).
		BorderRight(false).
		BorderTop(false).
		Render(inner)
}

func (a *App) resizeComponents() {
	a.statusBar.SetWidth(a.width)
	a.commandBar.SetWidth(a.width)

	a.headerCache = a.buildHeader()
	headerH := lipgloss.Height(a.headerCache)

	contentHeight := a.height - statusBarHeight - commandBarHeight - headerH
	if contentHeight < 1 {
		contentHeight = 1
	}

	if a.preview.Visible() {
		// Split: ~40% list, ~60% preview
		listWidth := a.width * 40 / 100
		previewWidth := a.width - listWidth
		if listWidth < 20 {
			listWidth = 20
			previewWidth = a.width - listWidth
		}
		a.noteList.SetSize(listWidth, contentHeight)
		a.preview.SetSize(previewWidth, contentHeight)
	} else {
		a.noteList.SetSize(a.width, contentHeight)
	}
}

func (a App) View() tea.View {
	if a.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	var mainContent string
	if a.preview.Visible() {
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top,
			a.noteList.View(),
			a.preview.View(),
		)
	} else {
		mainContent = a.noteList.View()
	}

	// Show filter bar or command bar
	var barView string
	if a.filterState == filterTyping {
		barView = a.renderFilterBar()
	} else {
		barView = a.commandBar.View()
	}

	statusView := a.statusBar.View()

	content := lipgloss.JoinVertical(lipgloss.Left,
		a.headerCache,
		mainContent,
		barView,
		statusView,
	)

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a App) renderFilterBar() string {
	prompt := lipgloss.NewStyle().
		Foreground(theme.ColorMauve).
		Bold(true).
		Render("/")
	text := lipgloss.NewStyle().
		Foreground(theme.ColorText).
		Render(a.filterBuf)
	cursor := lipgloss.NewStyle().
		Foreground(theme.ColorMauve).
		Render("▏")

	bar := lipgloss.NewStyle().
		Width(a.width).
		Padding(0, 1).
		Background(theme.ColorSurface0).
		Render(prompt + text + cursor)
	return bar
}
