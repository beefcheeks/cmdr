package daemon

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mikehu/cmdr/internal/claude"
	"github.com/mikehu/cmdr/internal/scheduler"
	"github.com/mikehu/cmdr/internal/tmux"
)

// startPoller runs server-side polling and publishes events to the bus.
func startPoller(bus *EventBus, s *scheduler.Scheduler, db *sql.DB) func() {
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Run one tick immediately
		pollTick(bus, s, db)

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				pollTick(bus, s, db)
			}
		}
	}()

	return func() { close(done) }
}

// pollTick runs a single polling cycle: fetch tmux once, publish all events, record analytics.
func pollTick(bus *EventBus, s *scheduler.Scheduler, db *sql.DB) {
	publishStatus(bus, s)

	tmuxSessions, err := tmux.ListSessions()
	if err != nil {
		log.Printf("cmdr: poller: tmux list error: %v", err)
		tmuxSessions = []tmux.Session{}
	}
	bus.Publish(Event{Type: "tmux:sessions", Data: tmuxSessions})

	claudeSessions := enrichAndPublishClaude(bus, tmuxSessions)

	recordActivity(db, tmuxSessions, claudeSessions, time.Now())
}

func publishStatus(bus *EventBus, s *scheduler.Scheduler) {
	bus.Publish(Event{
		Type: "status",
		Data: map[string]any{
			"status":  "running",
			"version": Version,
			"pid":     os.Getpid(),
			"tasks":   len(s.Tasks()),
		},
	})
}

// enrichAndPublishClaude matches Claude sessions to tmux panes and publishes them.
// Returns the enriched sessions for use by analytics.
func enrichAndPublishClaude(bus *EventBus, tmuxSessions []tmux.Session) []claude.Session {
	sessions, err := claude.ListSessions()
	if err != nil {
		log.Printf("cmdr: poller: claude list error: %v", err)
		return nil
	}

	claudePanes := collectClaudePanes(tmuxSessions)
	ppidMap := getParentPIDs()

	shellPIDs := make(map[int]*claudePane)
	for i := range claudePanes {
		shellPIDs[claudePanes[i].shellPID] = &claudePanes[i]
	}

	for i := range sessions {
		if cp := findAncestorPane(sessions[i].PID, ppidMap, shellPIDs); cp != nil {
			sessions[i].TmuxTarget = cp.target
			sessions[i].Status = claude.PaneStatus(cp.target)
		}
	}

	bus.Publish(Event{Type: "claude:sessions", Data: sessions})
	return sessions
}

type claudePane struct {
	target   string // e.g. "cmdr:1.3"
	shellPID int    // PID of the shell process in the pane
}

func collectClaudePanes(sessions []tmux.Session) []claudePane {
	var panes []claudePane
	for _, s := range sessions {
		for _, w := range s.Windows {
			for _, p := range w.Panes {
				if p.Command == "claude" {
					target := fmt.Sprintf("%s:%d.%d", s.Name, w.Index, p.Index)
					panes = append(panes, claudePane{target: target, shellPID: p.PID})
				}
			}
		}
	}
	return panes
}

// findAncestorPane walks up the process tree from pid to find a matching pane shell.
// Handles intermediate processes (e.g., zsh → volta-shim → node).
func findAncestorPane(pid int, ppidMap map[int]int, shellPIDs map[int]*claudePane) *claudePane {
	visited := make(map[int]bool)
	for cur := pid; cur > 1 && !visited[cur]; cur = ppidMap[cur] {
		visited[cur] = true
		if cp, ok := shellPIDs[cur]; ok {
			return cp
		}
	}
	return nil
}

// getParentPIDs returns a map of PID → PPID for all processes.
// Single `ps` call, efficient for matching Claude PIDs to pane shell PIDs.
func getParentPIDs() map[int]int {
	out, err := exec.Command("ps", "-eo", "pid,ppid").Output()
	if err != nil {
		return nil
	}
	m := make(map[int]int)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 == nil && err2 == nil {
			m[pid] = ppid
		}
	}
	return m
}
