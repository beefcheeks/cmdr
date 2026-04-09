package daemon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/mikehu/cmdr/internal/tmux"
)

// handleCreateDirective creates a new claude_task in draft status.
func handleCreateDirective(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			RepoPath string `json:"repoPath"`
			Content  string `json:"content"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		now := time.Now().Format(time.RFC3339)
		result, err := db.Exec(
			`INSERT INTO claude_tasks (type, status, repo_path, prompt, created_at, started_at)
			 VALUES ('directive', 'draft', ?, ?, ?, ?)`,
			body.RepoPath, body.Content, now, now,
		)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		id, _ := result.LastInsertId()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": int(id), "status": "draft"})
	}
}

// handleSaveDirective updates the prompt content of a draft task.
func handleSaveDirective(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			ID       int    `json:"id"`
			RepoPath string `json:"repoPath"`
			Content  string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == 0 {
			http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
			return
		}

		db.Exec(`UPDATE claude_tasks SET repo_path=?, prompt=? WHERE id=? AND status='draft'`,
			body.RepoPath, body.Content, body.ID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleSubmitDirective launches Claude with the draft's prompt in a tmux window.
func handleSubmitDirective(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == 0 {
			http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
			return
		}

		var repoPath, prompt string
		err := db.QueryRow(`SELECT repo_path, prompt FROM claude_tasks WHERE id=? AND status='draft'`, body.ID).
			Scan(&repoPath, &prompt)
		if err != nil {
			http.Error(w, `{"error":"draft not found"}`, http.StatusNotFound)
			return
		}

		if repoPath == "" || prompt == "" {
			http.Error(w, `{"error":"draft must have a repo and content"}`, http.StatusBadRequest)
			return
		}

		// Find or create the tmux session for this repo
		sessionName, err := findOrCreateSession(repoPath)
		if err != nil {
			log.Printf("cmdr: directive/submit: session: %v", err)
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		// Launch claude in a new window
		escaped := strings.ReplaceAll(prompt, "'", "'\\''")
		windowName := fmt.Sprintf("task-%d", body.ID)
		cmd := fmt.Sprintf("claude --name 'cmdr-task-%d' '%s'", body.ID, escaped)

		target, err := tmux.CreateDraftWindow(sessionName, windowName, repoPath, cmd)
		if err != nil {
			log.Printf("cmdr: directive/submit: window: %v", err)
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		// Update task status
		now := time.Now().Format(time.RFC3339)
		db.Exec(`UPDATE claude_tasks SET status='running', started_at=? WHERE id=?`, now, body.ID)

		log.Printf("cmdr: directive submitted (task %d, session %s, target %s)", body.ID, sessionName, target)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"target":  target,
			"session": sessionName,
		})
	}
}

// findOrCreateSession finds an existing tmux session for the repo or creates one.
func findOrCreateSession(repoPath string) (string, error) {
	sessions, _ := tmux.ListSessions()
	resolved := repoPath
	if r, err := resolveSymlinks(repoPath); err == nil {
		resolved = r
	}

	for _, s := range sessions {
		for _, w := range s.Windows {
			for _, p := range w.Panes {
				if p.CWD == resolved || p.CWD == repoPath {
					return s.Name, nil
				}
			}
		}
	}

	return tmux.CreateSession(repoPath)
}

func resolveSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
