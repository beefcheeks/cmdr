package tasks

import (
	"fmt"
	"os/exec"
)

// Claude shells out to `claude -p` with the given prompt and allowed tools.
// If dir is non-empty, the command runs in that directory.
func Claude(prompt string, dir string, allowedTools ...string) (string, error) {
	args := []string{"-p", prompt}
	for _, tool := range allowedTools {
		args = append(args, "--allowedTools", tool)
	}

	cmd := exec.Command("claude", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("claude: %w\n%s", err, out)
	}
	return string(out), nil
}
