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

	"github.com/mikehu/cmdr/internal/prompts"
	"github.com/mikehu/cmdr/internal/tmux"
)

// handleCreateDirective creates a new claude_task in draft status.
func handleCreateDirective(db *sql.DB, bus *EventBus) http.HandlerFunc {
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

		bus.Publish(Event{Type: "claude:task", Data: map[string]any{
			"id": int(id), "status": "draft", "title": directiveTitle(body.Content),
		}})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": int(id), "status": "draft"})
	}
}

// handleSaveDirective updates the prompt content of a draft task.
func handleSaveDirective(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			ID       int    `json:"id"`
			RepoPath string `json:"repoPath"`
			Content  string `json:"content"`
			Intent   string `json:"intent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == 0 {
			http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
			return
		}

		// Read old values to diff against
		var oldRepo string
		db.QueryRow(`SELECT COALESCE(repo_path, '') FROM claude_tasks WHERE id=?`, body.ID).Scan(&oldRepo)

		db.Exec(`UPDATE claude_tasks SET repo_path=?, prompt=?, intent=? WHERE id=? AND status='draft'`,
			body.RepoPath, body.Content, body.Intent, body.ID)

		if body.RepoPath != oldRepo {
			bus.Publish(Event{Type: "claude:task", Data: map[string]any{
				"id": body.ID, "status": "draft", "repoPath": body.RepoPath,
			}})
		}

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
			ID     int    `json:"id"`
			Intent string `json:"intent"` // optional intent ID (e.g. "bug-fix")
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

		// Build command with optional intent system prompt
		var cmd string
		if body.Intent != "" {
			intentPrompt, err := prompts.GetIntentPrompt(body.Intent)
			if err == nil {
				escapedIntent := strings.ReplaceAll(intentPrompt, "'", "'\\''")
				cmd = fmt.Sprintf("claude --name 'cmdr-task-%d' --append-system-prompt '%s' '%s'", body.ID, escapedIntent, escaped)
			} else {
				cmd = fmt.Sprintf("claude --name 'cmdr-task-%d' '%s'", body.ID, escaped)
			}
		} else {
			cmd = fmt.Sprintf("claude --name 'cmdr-task-%d' '%s'", body.ID, escaped)
		}

		target, err := tmux.CreateDraftWindow(sessionName, windowName, repoPath, cmd)
		if err != nil {
			log.Printf("cmdr: directive/submit: window: %v", err)
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		// Update task status and prefix title with intent
		now := time.Now().Format(time.RFC3339)
		if body.Intent != "" {
			// Find the intent name for the title prefix
			for _, intent := range prompts.ListIntents() {
				if intent.ID == body.Intent {
					var currentTitle string
					db.QueryRow(`SELECT COALESCE(title, '') FROM claude_tasks WHERE id=?`, body.ID).Scan(&currentTitle)
					prefixed := strings.ToLower(intent.Name) + ": " + currentTitle
					db.Exec(`UPDATE claude_tasks SET status='running', started_at=?, title=? WHERE id=?`, now, prefixed, body.ID)
					break
				}
			}
		} else {
			db.Exec(`UPDATE claude_tasks SET status='running', started_at=? WHERE id=?`, now, body.ID)
		}

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

// handleListIntents returns available directive intent presets.
func handleListIntents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prompts.ListIntents())
	}
}

func resolveSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// directiveTitle extracts a title from directive markdown content.
// Takes the first non-empty line, truncated to 80 chars.
// Code refs (@file) are used as-is. Image blocks are skipped.
func directiveTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "![") {
			continue
		}
		// Strip markdown heading prefix
		line = strings.TrimLeft(line, "# ")
		if len(line) > 80 {
			return line[:77] + "..."
		}
		return line
	}
	return "Untitled directive"
}
