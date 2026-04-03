package tmux

import (
	"fmt"
	"strings"
)

// SwitchClient switches the most recently active tmux client to the given session.
func SwitchClient(sessionName string) error {
	out, err := tmuxCmd("switch-client", "-t", sessionName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux switch-client: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
