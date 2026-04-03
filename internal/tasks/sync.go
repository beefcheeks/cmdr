package tasks

import (
	"database/sql"
	"log"
	"time"

	"github.com/mikehu/cmdr/internal/gitlocal"
)

// SyncCommits returns a task function that fetches new commits for all monitored repos.
func SyncCommits(db *sql.DB) func() error {
	return func() error {
		rows, err := db.Query(`SELECT id, name, path, default_branch, last_synced_at FROM repos`)
		if err != nil {
			return err
		}
		defer rows.Close()

		type repo struct {
			id             int
			name, path     string
			defaultBranch  string
			lastSyncedAt   *string
		}

		var repos []repo
		for rows.Next() {
			var r repo
			if err := rows.Scan(&r.id, &r.name, &r.path, &r.defaultBranch, &r.lastSyncedAt); err != nil {
				continue
			}
			repos = append(repos, r)
		}

		for _, r := range repos {
			SyncOne(db, r.id, r.path, r.defaultBranch, r.lastSyncedAt)
		}
		return nil
	}
}

// SyncOne fetches and stores new commits for a single repo.
func SyncOne(db *sql.DB, repoID int, repoPath, defaultBranch string, lastSynced *string) {
	// Fetch latest from remote
	if err := gitlocal.Fetch(repoPath); err != nil {
		log.Printf("cmdr: sync: %s: fetch failed: %v", repoPath, err)
		return
	}

	var since time.Time
	if lastSynced != nil && *lastSynced != "" {
		since, _ = time.Parse(time.RFC3339, *lastSynced)
		if since.IsZero() {
			since, _ = time.Parse("2006-01-02T15:04:05Z", *lastSynced)
		}
	}

	commits, err := gitlocal.Log(repoPath, defaultBranch, since, 50)
	if err != nil {
		log.Printf("cmdr: sync: %s: log failed: %v", repoPath, err)
		return
	}

	inserted := 0
	for _, c := range commits {
		_, err := db.Exec(`
			INSERT OR IGNORE INTO commits (repo_id, sha, author, message, committed_at, url)
			VALUES (?, ?, ?, ?, ?, ?)
		`, repoID, c.SHA, c.Author, c.Message, c.CommittedAt.Format(time.RFC3339), c.URL)
		if err == nil {
			inserted++
		}
	}

	db.Exec(`UPDATE repos SET last_synced_at = ? WHERE id = ?`, time.Now().Format(time.RFC3339), repoID)

	if inserted > 0 {
		log.Printf("cmdr: sync: %s: %d new commits", repoPath, inserted)
	}
}
