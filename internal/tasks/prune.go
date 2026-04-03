package tasks

import (
	"database/sql"
	"log"
)

// PruneCommits returns a task function that deletes commits older than 2 weeks.
func PruneCommits(db *sql.DB) func() error {
	return func() error {
		result, err := db.Exec(`DELETE FROM commits WHERE committed_at < datetime('now', '-14 days')`)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n > 0 {
			log.Printf("cmdr: prune-commits: deleted %d old commits", n)
		}
		return nil
	}
}
