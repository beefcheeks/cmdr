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
// Returns "working", "waiting", or "idle".
func PaneStatus(tmuxTarget string) string {
	out, err := exec.Command("tmux", "-S", tmuxSocketPath(), "capture-pane", "-t", tmuxTarget, "-p").Output()
	if err != nil {
		return "unknown"
	}

	// Only check the last non-empty line — that's where the hint text lives
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	lastLine := ""
	if len(lines) > 0 {
		lastLine = lines[len(lines)-1]
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if strings.Contains(lastLine, workingSignal) {
		// "esc to interrupt" — actively working
		tracker.lastWorking[tmuxTarget] = time.Now()
		return "working"
	}

	if strings.Contains(lastLine, idleSignal) {
		// "hold Space to speak" — prompt is up
		lastWork, exists := tracker.lastWorking[tmuxTarget]
		if exists && time.Since(lastWork) < idleThreshold {
			return "waiting"
		}
		return "idle"
	}

	// Neither signal found — could be a permission prompt or other interactive state
	// Treat as working since it's not at the idle prompt
	tracker.lastWorking[tmuxTarget] = time.Now()
	return "working"
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
