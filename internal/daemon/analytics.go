package daemon

import (
	"database/sql"
	"log"
	"time"

	"github.com/mikehu/cmdr/internal/claude"
	"github.com/mikehu/cmdr/internal/tmux"
)

// lastClearedDay tracks which day-of-year we've verified the slot for.
// On startup (-1), we check if existing data is from today before clearing.
var lastClearedDay = -1

// recordActivity persists one 5-second activity snapshot into the fixed-bucket table.
func recordActivity(db *sql.DB, tmuxSessions []tmux.Session, claudeSessions []claude.Session, now time.Time) {
	slot, bucket := currentBucket(now)
	today := now.YearDay()

	// Only clear the slot if the day changed (not just on restart).
	// Check if existing data in this slot is from today — if so, keep it.
	if today != lastClearedDay {
		if !slotHasDataForToday(db, slot, now) {
			clearSlot(db, slot)
		}
		lastClearedDay = today
	}

	activeTool := determineActiveTool(tmuxSessions)
	total, working, waiting, idle, unknown := countClaudeStates(claudeSessions)

	_, err := db.Exec(`INSERT OR REPLACE INTO activity_buckets
		(slot, bucket, active_tool, claude_total, claude_working, claude_waiting, claude_idle, claude_unknown, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		slot, bucket, activeTool, total, working, waiting, idle, unknown, now.Format(time.RFC3339),
	)
	if err != nil {
		log.Printf("cmdr: analytics: record error: %v", err)
	}
}

// currentBucket returns the slot (0 or 1) and bucket index (0..17279) for a given time.
func currentBucket(now time.Time) (slot int, bucket int) {
	slot = now.YearDay() % 2
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	bucket = int(now.Sub(midnight).Seconds()) / 5
	return
}

// slotHasDataForToday checks if the slot already contains data from today.
// Prevents wiping valid data on daemon restart.
func slotHasDataForToday(db *sql.DB, slot int, now time.Time) bool {
	todayPrefix := now.Format("2006-01-02") // matches start of RFC3339
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM activity_buckets
		WHERE slot = ? AND recorded_at IS NOT NULL AND recorded_at LIKE ?
	`, slot, todayPrefix+"%").Scan(&count)
	return err == nil && count > 0
}

// clearSlot wipes all data for a slot so it can be reused for the new day.
func clearSlot(db *sql.DB, slot int) {
	_, err := db.Exec(`DELETE FROM activity_buckets WHERE slot = ?`, slot)
	if err != nil {
		log.Printf("cmdr: analytics: clear slot %d error: %v", slot, err)
	}
}

// determineActiveTool returns what tool is focused in the attached tmux session.
func determineActiveTool(sessions []tmux.Session) string {
	for _, s := range sessions {
		if !s.Attached {
			continue
		}
		for _, w := range s.Windows {
			for _, p := range w.Panes {
				if !p.Active {
					continue
				}
				switch p.Command {
				case "nvim", "vim":
					return "nvim"
				case "claude":
					return "claude"
				default:
					return "other"
				}
			}
		}
		break // only first attached session
	}
	return "inactive"
}

// countClaudeStates tallies Claude session statuses.
func countClaudeStates(sessions []claude.Session) (total, working, waiting, idle, unknown int) {
	for _, s := range sessions {
		total++
		switch s.Status {
		case "working":
			working++
		case "waiting":
			waiting++
		case "idle":
			idle++
		default:
			unknown++
		}
	}
	return
}
