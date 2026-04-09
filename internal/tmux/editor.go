package tmux

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// EditorTarget identifies a tmux pane running nvim.
type EditorTarget struct {
	Session string // e.g. "workers"
	Target  string // e.g. "workers:1.0"
	Fresh   bool   // true if nvim was just launched (file already opened)
}

// FindOrCreateEditor locates an nvim pane in a tmux session whose working
// directory matches repoPath. If no matching session exists, one is created
// via the sessionizer. If the session exists but has no nvim pane, a new
// window is created with nvim opened to the given file+line.
func FindOrCreateEditor(repoPath, file string, line int) (*EditorTarget, error) {
	sessions, err := ListSessions()
	if err != nil {
		return nil, err
	}

	// Resolve symlinks so paths match tmux's real paths
	if resolved, err := filepath.EvalSymlinks(repoPath); err == nil {
		repoPath = resolved
	}
	repoPath = filepath.Clean(repoPath)

	// First pass: find an existing nvim pane in a session matching the repo
	for _, s := range sessions {
		if !sessionMatchesRepo(s, repoPath) {
			continue
		}
		// Found a matching session — look for an nvim pane
		for _, w := range s.Windows {
			for _, p := range w.Panes {
				if p.Command == "nvim" || p.Command == "vim" {
					target := fmt.Sprintf("%s:%d.%d", s.Name, w.Index, p.Index)
					return &EditorTarget{Session: s.Name, Target: target, Fresh: false}, nil
				}
			}
		}
		// Session exists but no nvim pane — create one with the file
		target, err := createNvimWindow(s.Name, repoPath, file, line)
		if err != nil {
			return nil, err
		}
		return &EditorTarget{Session: s.Name, Target: target, Fresh: true}, nil
	}

	// No matching session — create one, then open nvim with the file
	sessName, err := CreateSession(repoPath)
	if err != nil {
		return nil, fmt.Errorf("creating session for %s: %w", repoPath, err)
	}

	target, err := createNvimWindow(sessName, repoPath, file, line)
	if err != nil {
		return nil, fmt.Errorf("creating nvim window: %w", err)
	}

	return &EditorTarget{Session: sessName, Target: target, Fresh: true}, nil
}

// OpenFileInEditor sends a command to an existing nvim pane to open a file at a line.
func OpenFileInEditor(target, file string, line int) error {
	// Esc ensures we're in normal mode
	if err := sendRawKeys(target, "Escape"); err != nil {
		return err
	}
	// Small delay to ensure normal mode is entered
	time.Sleep(50 * time.Millisecond)
	// Send the :e command as literal text
	cmd := fmt.Sprintf(":e +%d %s", line, file)
	return SendKeys(target, cmd, true)
}

// SendKeys sends keystrokes to a tmux pane.
// If literal is true, keys are sent as literal text (Enter appended).
// If false, keys are sent as-is (space-separated key names).
func SendKeys(target, keys string, literal bool) error {
	args := []string{"send-keys", "-t", target}
	if literal {
		args = append(args, keys, "Enter")
	} else {
		args = append(args, strings.Fields(keys)...)
	}
	out, err := tmuxCmd(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("send-keys: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// sendRawKeys sends tmux key names (space-separated) without literal mode.
func sendRawKeys(target string, keys ...string) error {
	args := append([]string{"send-keys", "-t", target}, keys...)
	out, err := tmuxCmd(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("send-keys: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// FocusSession switches the tmux client to the given session.
func FocusSession(sessionName string) error {
	return SwitchClient(sessionName)
}

// sessionMatchesRepo checks if any pane in the session has a CWD matching the repo path.
func sessionMatchesRepo(s Session, repoPath string) bool {
	for _, w := range s.Windows {
		for _, p := range w.Panes {
			if filepath.Clean(p.CWD) == repoPath {
				return true
			}
		}
	}
	return false
}

// createNvimWindow adds a new window to an existing session with nvim opened to a file.
func createNvimWindow(sessionName, dir, file string, line int) (string, error) {
	nvimCmd := fmt.Sprintf("nvim +%d %s", line, file)
	out, err := tmuxCmd("new-window", "-t", sessionName, "-c", dir, "bash", "-c", nvimCmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("new-window nvim: %s: %w", strings.TrimSpace(string(out)), err)
	}
	// The new window is now the active one — find its target
	idxOut, err := tmuxCmd("display-message", "-t", sessionName, "-p", "#{window_index}.#{pane_index}").Output()
	if err != nil {
		return sessionName + ":", nil
	}
	return sessionName + ":" + strings.TrimSpace(string(idxOut)), nil
}
