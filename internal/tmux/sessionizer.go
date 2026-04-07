package tmux

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// SessionName computes the tmux session name for a directory,
// matching the naming logic from tmux-sessionizer.sh.
// If the dir is a git worktree with a .bare parent, names it "parent_worktree".
// Otherwise uses the directory basename. Dots, spaces, and hyphens become underscores.
func SessionName(dir string) string {
	name := filepath.Base(dir)

	// Check if this is a git worktree with a .bare parent
	topLevel, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err == nil {
		parent := filepath.Dir(topLevel)
		bare := filepath.Join(parent, ".bare")
		if isDir(bare) {
			name = filepath.Base(parent) + "_" + filepath.Base(dir)
		}
	}

	// Replace problematic chars (matches: tr '. -' '___')
	r := strings.NewReplacer(".", "_", " ", "_", "-", "_")
	return r.Replace(name)
}

// CreateSession creates a new detached tmux session for the given directory.
// If a session with that name already exists, it returns the existing name.
func CreateSession(dir string) (string, error) {
	name := SessionName(dir)

	// Check if session already exists
	if err := tmuxCmd("has-session", "-t="+name).Run(); err == nil {
		return name, nil
	}

	// Create detached session
	if out, err := tmuxCmd("new-session", "-ds", name, "-c", dir).CombinedOutput(); err != nil {
		return "", fmt.Errorf("tmux new-session: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return name, nil
}

const refactorSession = "claude_refactor"

// CreateRefactorWindow creates (or reuses) the claude_refactor session and
// opens a new window with the given name, working directory, and initial command.
// Returns the target string "session:window" for further use.
func CreateRefactorWindow(windowName, dir, shellCmd string) (string, error) {
	// Wrap in a shell so the command string is interpreted properly.
	// When the command exits, the window closes automatically.
	args := []string{"bash", "-c", shellCmd}

	// Ensure session exists
	if err := tmuxCmd("has-session", "-t="+refactorSession).Run(); err != nil {
		// Create session with first window running the command
		cmdArgs := []string{"new-session", "-ds", refactorSession, "-n", windowName, "-c", dir}
		cmdArgs = append(cmdArgs, args...)
		if out, err := tmuxCmd(cmdArgs...).CombinedOutput(); err != nil {
			return "", fmt.Errorf("tmux new-session: %s: %w", strings.TrimSpace(string(out)), err)
		}
	} else {
		// Session exists — add a new window running the command
		cmdArgs := []string{"new-window", "-t", refactorSession, "-n", windowName, "-c", dir}
		cmdArgs = append(cmdArgs, args...)
		if out, err := tmuxCmd(cmdArgs...).CombinedOutput(); err != nil {
			return "", fmt.Errorf("tmux new-window: %s: %w", strings.TrimSpace(string(out)), err)
		}
	}

	// Make this the attached session so the user sees it
	_ = SwitchClient(refactorSession)

	return refactorSession + ":" + windowName, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func isDir(path string) bool {
	cmd := exec.Command("test", "-d", path)
	return cmd.Run() == nil
}
