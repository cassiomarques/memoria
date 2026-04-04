package editor

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNew(t *testing.T) {
	e := New("nvim")
	if e.Command() != "nvim" {
		t.Errorf("expected command %q, got %q", "nvim", e.Command())
	}
}

func TestDetect(t *testing.T) {
	// Find a command that actually exists on PATH to use as fallback reference.
	// We need "true" or similar guaranteed binary.
	truePath, _ := exec.LookPath("true")
	hasTrueCmd := truePath != ""

	t.Run("uses EDITOR env var first", func(t *testing.T) {
		if !hasTrueCmd {
			t.Skip("no 'true' command on PATH")
		}
		t.Setenv("EDITOR", "true")
		t.Setenv("VISUAL", "some-nonexistent-editor-xyz")
		e := Detect()
		if e.Command() != "true" {
			t.Errorf("expected command %q, got %q", "true", e.Command())
		}
	})

	t.Run("falls back to VISUAL if EDITOR not set", func(t *testing.T) {
		if !hasTrueCmd {
			t.Skip("no 'true' command on PATH")
		}
		t.Setenv("EDITOR", "")
		t.Setenv("VISUAL", "true")
		e := Detect()
		if e.Command() != "true" {
			t.Errorf("expected command %q, got %q", "true", e.Command())
		}
	})

	t.Run("falls back to VISUAL if EDITOR not on PATH", func(t *testing.T) {
		if !hasTrueCmd {
			t.Skip("no 'true' command on PATH")
		}
		t.Setenv("EDITOR", "some-nonexistent-editor-xyz")
		t.Setenv("VISUAL", "true")
		e := Detect()
		if e.Command() != "true" {
			t.Errorf("expected command %q, got %q", "true", e.Command())
		}
	})

	t.Run("falls back to vim or nano when env vars unset", func(t *testing.T) {
		t.Setenv("EDITOR", "")
		t.Setenv("VISUAL", "")
		e := Detect()
		cmd := e.Command()
		if cmd != "vim" && cmd != "nano" {
			t.Errorf("expected 'vim' or 'nano', got %q", cmd)
		}
	})
}

func TestOpenCmd(t *testing.T) {
	e := New("vim")
	filePath := "/some/test/file.md"
	cmd := e.OpenCmd(filePath)

	// Path may be resolved or not depending on LookPath, just check Args
	if len(cmd.Args) < 2 || cmd.Args[len(cmd.Args)-1] != filePath {
		t.Errorf("expected last arg to be %q, got %v", filePath, cmd.Args)
	}
	if cmd.Stdin != os.Stdin {
		t.Error("expected Stdin to be os.Stdin")
	}
	if cmd.Stdout != os.Stdout {
		t.Error("expected Stdout to be os.Stdout")
	}
	if cmd.Stderr != os.Stderr {
		t.Error("expected Stderr to be os.Stderr")
	}
}

func TestContentHash(t *testing.T) {
	dir := t.TempDir()

	t.Run("produces consistent hash", func(t *testing.T) {
		path := filepath.Join(dir, "test1.txt")
		content := []byte("hello world")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		hash1, err := ContentHash(path)
		if err != nil {
			t.Fatal(err)
		}
		hash2, err := ContentHash(path)
		if err != nil {
			t.Fatal(err)
		}
		if hash1 != hash2 {
			t.Errorf("hashes differ: %q vs %q", hash1, hash2)
		}

		// Verify it matches expected SHA256
		expected := fmt.Sprintf("%x", sha256.Sum256(content))
		if hash1 != expected {
			t.Errorf("expected hash %q, got %q", expected, hash1)
		}
	})

	t.Run("different content produces different hash", func(t *testing.T) {
		path1 := filepath.Join(dir, "a.txt")
		path2 := filepath.Join(dir, "b.txt")
		os.WriteFile(path1, []byte("content a"), 0644)
		os.WriteFile(path2, []byte("content b"), 0644)

		h1, _ := ContentHash(path1)
		h2, _ := ContentHash(path2)
		if h1 == h2 {
			t.Error("expected different hashes for different content")
		}
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		_, err := ContentHash(filepath.Join(dir, "nope.txt"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestHasChanged(t *testing.T) {
	dir := t.TempDir()

	t.Run("detects file changes", func(t *testing.T) {
		path := filepath.Join(dir, "changing.txt")
		os.WriteFile(path, []byte("original"), 0644)

		hashBefore, err := ContentHash(path)
		if err != nil {
			t.Fatal(err)
		}

		os.WriteFile(path, []byte("modified"), 0644)

		changed, err := HasChanged(hashBefore, path)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Error("expected HasChanged to return true after modification")
		}
	})

	t.Run("returns false when file unchanged", func(t *testing.T) {
		path := filepath.Join(dir, "stable.txt")
		os.WriteFile(path, []byte("same content"), 0644)

		hashBefore, err := ContentHash(path)
		if err != nil {
			t.Fatal(err)
		}

		changed, err := HasChanged(hashBefore, path)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Error("expected HasChanged to return false when file unchanged")
		}
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		_, err := HasChanged("abc", filepath.Join(dir, "gone.txt"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestEditFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test not supported on Windows")
	}

	dir := t.TempDir()

	t.Run("runs editor successfully", func(t *testing.T) {
		// Create a fake editor script that appends content to the file
		editorScript := filepath.Join(dir, "fake-editor.sh")
		os.WriteFile(editorScript, []byte("#!/bin/sh\necho 'edited' >> \"$1\"\n"), 0755)

		targetFile := filepath.Join(dir, "note.md")
		os.WriteFile(targetFile, []byte("original\n"), 0644)

		e := New(editorScript)
		err := e.EditFile(targetFile)
		if err != nil {
			t.Fatalf("EditFile failed: %v", err)
		}

		content, _ := os.ReadFile(targetFile)
		if string(content) != "original\nedited\n" {
			t.Errorf("expected file to be edited, got %q", string(content))
		}
	})

	t.Run("returns error when editor fails", func(t *testing.T) {
		editorScript := filepath.Join(dir, "bad-editor.sh")
		os.WriteFile(editorScript, []byte("#!/bin/sh\nexit 1\n"), 0755)

		targetFile := filepath.Join(dir, "note2.md")
		os.WriteFile(targetFile, []byte("content"), 0644)

		e := New(editorScript)
		err := e.EditFile(targetFile)
		if err == nil {
			t.Error("expected error when editor exits with non-zero status")
		}
	})
}
