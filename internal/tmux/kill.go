package tmux

import (
	"fmt"
	"os/exec"
	"strings"
)

// KillSession kills a tmux session by name.
func KillSession(sessionName string) error {
	out, err := exec.Command("tmux", "kill-session", "-t", sessionName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux kill-session: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
