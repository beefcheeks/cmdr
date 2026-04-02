package tasks

import (
	"fmt"
	"os/exec"
)

// Claude shells out to `claude -p` with the given prompt and allowed tools.
// This is the building block for any Claude-powered task.
func Claude(prompt string, allowedTools ...string) (string, error) {
	args := []string{"-p", prompt}
	if len(allowedTools) > 0 {
		for _, tool := range allowedTools {
			args = append(args, "--allowedTools", tool)
		}
	}

	cmd := exec.Command("claude", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("claude: %w\n%s", err, out)
	}
	return string(out), nil
}
