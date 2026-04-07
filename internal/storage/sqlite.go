package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cassiomarques/memoria/internal/note"
	_ "modernc.org/sqlite"
)

var ErrNoteNotFound = errors.New("note not found")

// NoteMeta holds note metadata as stored in the database.
type NoteMeta struct {
	Path     string
	Title    string
	Folder   string
	Tags     []string
	Created  time.Time
	Modified time.Time
	Todo     bool
	Done     bool
	Due      *time.Time
}

// TagInfo holds a tag name and how many notes use it.
type TagInfo struct {
	Tag   string
	Count int
}

// MetaStore persists note metadata in a SQLite database.
type MetaStore struct {
	db *sql.DB
}

// NewMetaStore opens (or creates) a SQLite database at dbPath,
// enables WAL mode and foreign keys, and runs schema migrations.
func NewMetaStore(dbPath string) (*MetaStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	return initStore(db)
}

// NewMemoryMetaStore creates an in-memory SQLite database for testing.
func NewMemoryMetaStore() (*MetaStore, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	return initStore(db)
}

func initStore(db *sql.DB) (*MetaStore, error) {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &MetaStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notes (
			path     TEXT PRIMARY KEY,
			title    TEXT NOT NULL,
			folder   TEXT NOT NULL DEFAULT '',
			created  DATETIME NOT NULL,
			modified DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			note_path TEXT NOT NULL,
			tag       TEXT NOT NULL,
			PRIMARY KEY (note_path, tag),
			FOREIGN KEY (note_path) REFERENCES notes(path) ON DELETE CASCADE ON UPDATE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_folder ON notes(folder)`,
		`CREATE TABLE IF NOT EXISTS bookmarks (
			note_path   TEXT PRIMARY KEY,
			order_index INTEGER NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (note_path) REFERENCES notes(path) ON DELETE CASCADE ON UPDATE CASCADE
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	// Idempotent column additions for todo support
	todoColumns := []struct {
		name string
		ddl  string
	}{
		{"todo", `ALTER TABLE notes ADD COLUMN todo BOOLEAN NOT NULL DEFAULT 0`},
		{"done", `ALTER TABLE notes ADD COLUMN done BOOLEAN NOT NULL DEFAULT 0`},
		{"due", `ALTER TABLE notes ADD COLUMN due DATETIME`},
	}
	for _, col := range todoColumns {
		if !hasColumn(db, "notes", col.name) {
			if _, err := db.Exec(col.ddl); err != nil {
				return fmt.Errorf("adding column %s: %w", col.name, err)
			}
		}
	}

	return nil
}

// hasColumn checks whether a table has a column with the given name.
func hasColumn(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// Close closes the underlying database connection.
func (m *MetaStore) Close() error {
	return m.db.Close()
}

// UpsertNote inserts or replaces note metadata and its tags inside a transaction.
func (m *MetaStore) UpsertNote(n *note.Note) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO notes (path, title, folder, created, modified, todo, done, due)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title    = excluded.title,
			folder   = excluded.folder,
			created  = excluded.created,
			modified = excluded.modified,
			todo     = excluded.todo,
			done     = excluded.done,
			due      = excluded.due`,
		n.Path, n.Title, n.Folder, n.Created, n.Modified, n.Todo, n.Done, n.Due,
	)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM tags WHERE note_path = ?`, n.Path); err != nil {
		return err
	}

	if len(n.Tags) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO tags (note_path, tag) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, tag := range n.Tags {
			if _, err := stmt.Exec(n.Path, tag); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// DeleteNote removes a note and its tags (via CASCADE).
func (m *MetaStore) DeleteNote(path string) error {
	_, err := m.db.Exec(`DELETE FROM notes WHERE path = ?`, path)
	return err
}

// MoveNote updates the path and folder of an existing note.
func (m *MetaStore) MoveNote(oldPath, newPath string, newFolder string) error {
	res, err := m.db.Exec(
		`UPDATE notes SET path = ?, folder = ? WHERE path = ?`,
		newPath, newFolder, oldPath,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNoteNotFound
	}
	return nil
}

// GetNote retrieves note metadata by path. Returns ErrNoteNotFound when the path does not exist.
func (m *MetaStore) GetNote(path string) (*NoteMeta, error) {
	nm := &NoteMeta{}
	err := m.db.QueryRow(
		`SELECT path, title, folder, created, modified, todo, done, due FROM notes WHERE path = ?`, path,
	).Scan(&nm.Path, &nm.Title, &nm.Folder, &nm.Created, &nm.Modified, &nm.Todo, &nm.Done, &nm.Due)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoteNotFound
	}
	if err != nil {
		return nil, err
	}

	tags, err := m.GetTags(path)
	if err != nil {
		return nil, err
	}
	nm.Tags = tags
	return nm, nil
}

// ListByFolder returns all notes in the given folder, sorted by title.
func (m *MetaStore) ListByFolder(folder string) ([]*NoteMeta, error) {
	rows, err := m.db.Query(
		`SELECT path, title, folder, created, modified, todo, done, due FROM notes WHERE folder = ? ORDER BY title`, folder,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return m.scanNotesWithTags(rows)
}

// ListByTag returns all notes that have the given tag, sorted by title.
func (m *MetaStore) ListByTag(tag string) ([]*NoteMeta, error) {
	rows, err := m.db.Query(
		`SELECT n.path, n.title, n.folder, n.created, n.modified, n.todo, n.done, n.due
		 FROM notes n
		 JOIN tags t ON n.path = t.note_path
		 WHERE t.tag = ?
		 ORDER BY n.title`, tag,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return m.scanNotesWithTags(rows)
}

// ListAllTags returns every tag with the number of notes using it, sorted by tag name.
func (m *MetaStore) ListAllTags() ([]TagInfo, error) {
	rows, err := m.db.Query(
		`SELECT tag, COUNT(*) as cnt FROM tags GROUP BY tag ORDER BY tag`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TagInfo
	for rows.Next() {
		var ti TagInfo
		if err := rows.Scan(&ti.Tag, &ti.Count); err != nil {
			return nil, err
		}
		result = append(result, ti)
	}
	return result, rows.Err()
}

// ListAll returns all notes sorted by path.
func (m *MetaStore) ListAll() ([]*NoteMeta, error) {
	rows, err := m.db.Query(
		`SELECT path, title, folder, created, modified, todo, done, due FROM notes ORDER BY path`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return m.scanNotesWithTags(rows)
}

// GetTags returns the tags for the note at path.
func (m *MetaStore) GetTags(path string) ([]string, error) {
	rows, err := m.db.Query(`SELECT tag FROM tags WHERE note_path = ? ORDER BY tag`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// PinNote adds a note to the bookmarks list. If already pinned, this is a no-op.
func (m *MetaStore) PinNote(path string) error {
	_, err := m.db.Exec(`
		INSERT OR IGNORE INTO bookmarks (note_path, order_index)
		VALUES (?, COALESCE((SELECT MAX(order_index) FROM bookmarks), -1) + 1)`,
		path)
	return err
}

// UnpinNote removes a note from the bookmarks list.
func (m *MetaStore) UnpinNote(path string) error {
	_, err := m.db.Exec(`DELETE FROM bookmarks WHERE note_path = ?`, path)
	return err
}

// IsPinned checks whether a note is bookmarked.
func (m *MetaStore) IsPinned(path string) (bool, error) {
	var count int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE note_path = ?`, path).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListPinned returns bookmarked notes in order.
func (m *MetaStore) ListPinned() ([]string, error) {
	rows, err := m.db.Query(`SELECT note_path FROM bookmarks ORDER BY order_index`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// ListRecent returns notes ordered by most recently modified.
func (m *MetaStore) ListRecent(limit int) ([]*NoteMeta, error) {
	rows, err := m.db.Query(
		`SELECT path, title, folder, created, modified, todo, done, due FROM notes ORDER BY modified DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return m.scanNotesWithTags(rows)
}
func (m *MetaStore) scanNotesWithTags(rows *sql.Rows) ([]*NoteMeta, error) {
	var notes []*NoteMeta
	for rows.Next() {
		nm := &NoteMeta{}
		if err := rows.Scan(&nm.Path, &nm.Title, &nm.Folder, &nm.Created, &nm.Modified, &nm.Todo, &nm.Done, &nm.Due); err != nil {
			return nil, err
		}
		notes = append(notes, nm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, nm := range notes {
		tags, err := m.GetTags(nm.Path)
		if err != nil {
			return nil, err
		}
		nm.Tags = tags
	}
	return notes, nil
}

// ListTodos returns all notes marked as todos, ordered by due date (nulls last), then title.
func (m *MetaStore) ListTodos() ([]*NoteMeta, error) {
	rows, err := m.db.Query(
		`SELECT path, title, folder, created, modified, todo, done, due
		 FROM notes
		 WHERE todo = 1
		 ORDER BY done ASC, due IS NULL, due ASC, title`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return m.scanNotesWithTags(rows)
}

// SetTodoDone sets the done status of a todo note.
func (m *MetaStore) SetTodoDone(path string, done bool) error {
	res, err := m.db.Exec(`UPDATE notes SET done = ? WHERE path = ? AND todo = 1`, done, path)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("note %q is not a todo or does not exist", path)
	}
	return nil
}
