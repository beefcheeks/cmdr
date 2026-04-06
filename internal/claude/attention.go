package claude

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

func tmuxSocketPath() string {
	return fmt.Sprintf("/private/tmp/tmux-%d/default", os.Getuid())
}

// Signals in the Claude Code pane hint text
const (
	workingSignal = "esc to interrupt"
	idleSignal    = "hold Space to speak"
)

// Waiting signals — Claude needs user input (permission prompts, etc.)
var waitingSignals = []string{
	"accept edits",
	"to accept",
	"to reject",
	"shift+tab to cycle",
}

// How long after the last "working" state before we consider it "idle"
const idleThreshold = 5 * time.Minute

// tracker holds the last-known working timestamp per tmux target
var tracker = struct {
	mu          sync.Mutex
	lastWorking map[string]time.Time
}{
	lastWorking: make(map[string]time.Time),
}

// PaneStatus checks the state of a Claude instance in a tmux pane.
// Returns "working", "waiting", "idle", or "unknown".
func PaneStatus(tmuxTarget string) string {
	out, err := exec.Command("tmux", "-S", tmuxSocketPath(), "capture-pane", "-t", tmuxTarget, "-p").Output()
	if err != nil {
		return "unknown"
	}

	// Scan the last few lines for signals — hint text is usually at the bottom
	// but can shift up slightly due to wrapping or multi-line prompts.
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	tail := lines
	if len(tail) > 5 {
		tail = tail[len(tail)-5:]
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	for _, line := range tail {
		if strings.Contains(line, workingSignal) {
			tracker.lastWorking[tmuxTarget] = time.Now()
			return "working"
		}
	}

	for _, line := range tail {
		if strings.Contains(line, idleSignal) {
			lastWork, exists := tracker.lastWorking[tmuxTarget]
			if exists && time.Since(lastWork) < idleThreshold {
				return "waiting"
			}
			return "idle"
		}
	}

	// Check for permission/action prompts — Claude is waiting for user input
	for _, line := range tail {
		for _, sig := range waitingSignals {
			if strings.Contains(line, sig) {
				return "waiting"
			}
		}
	}

	// Neither signal found — don't assume working.
	// Could be compact output or crashed session.
	return "unknown"
}

// CleanupTracker removes tracking for targets that no longer exist.
func CleanupTracker(activeTargets []string) {
	active := make(map[string]bool, len(activeTargets))
	for _, t := range activeTargets {
		active[t] = true
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for k := range tracker.lastWorking {
		if !active[k] {
			delete(tracker.lastWorking, k)
		}
	}
}
