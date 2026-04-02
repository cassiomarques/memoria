package editor

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
)

// Editor wraps an external text editor command.
type Editor struct {
	command string
}

// New creates an Editor with the given command.
func New(editorCmd string) *Editor {
	return &Editor{command: editorCmd}
}

// Detect auto-detects an editor by checking $EDITOR, $VISUAL, then
// falling back to "vim" and "nano". Returns the first one found on PATH.
func Detect() *Editor {
	candidates := []string{}

	if env := os.Getenv("EDITOR"); env != "" {
		candidates = append(candidates, env)
	}
	if env := os.Getenv("VISUAL"); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates, "vim", "nano")

	for _, cmd := range candidates {
		if _, err := exec.LookPath(cmd); err == nil {
			return &Editor{command: cmd}
		}
	}

	// Last resort: return nano even if not found, so we have something.
	return &Editor{command: "nano"}
}

// Command returns the editor command string.
func (e *Editor) Command() string {
	return e.command
}

// OpenCmd returns an *exec.Cmd configured to open filePath in the editor,
// with Stdin/Stdout/Stderr wired to the current process.
func (e *Editor) OpenCmd(filePath string) *exec.Cmd {
	cmd := exec.Command(e.command, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// EditFile launches the editor on filePath and waits for it to exit.
func (e *Editor) EditFile(filePath string) error {
	return e.OpenCmd(filePath).Run()
}

// ContentHash returns the hex-encoded SHA256 hash of a file's content.
func ContentHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

// HasChanged reports whether the file's current hash differs from hashBefore.
func HasChanged(hashBefore string, filePath string) (bool, error) {
	current, err := ContentHash(filePath)
	if err != nil {
		return false, err
	}
	return current != hashBefore, nil
}
