package daemon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mikehu/cmdr/internal/gitlocal"
	"github.com/mikehu/cmdr/internal/prompts"
	"github.com/mikehu/cmdr/internal/tasks"
	"github.com/mikehu/cmdr/internal/tmux"
)

// --- Review Comments ---

type reviewComment struct {
	ID        int    `json:"id"`
	RepoPath  string `json:"repoPath"`
	SHA       string `json:"sha"`
	LineStart int    `json:"lineStart"`
	LineEnd   int    `json:"lineEnd"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"createdAt"`
}

func handleListReviewComments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo := r.URL.Query().Get("repo")
		sha := r.URL.Query().Get("sha")
		if repo == "" || sha == "" {
			http.Error(w, `{"error":"missing repo or sha"}`, http.StatusBadRequest)
			return
		}

		rows, err := db.Query(`
			SELECT id, repo_path, sha, line_start, line_end, comment, created_at
			FROM review_comments WHERE repo_path = ? AND sha = ?
			ORDER BY line_start
		`, repo, sha)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var comments []reviewComment
		for rows.Next() {
			var c reviewComment
			if err := rows.Scan(&c.ID, &c.RepoPath, &c.SHA, &c.LineStart, &c.LineEnd, &c.Comment, &c.CreatedAt); err != nil {
				continue
			}
			comments = append(comments, c)
		}
		if comments == nil {
			comments = []reviewComment{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comments)
	}
}

func handleSaveReviewComment(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			RepoPath  string `json:"repoPath"`
			SHA       string `json:"sha"`
			LineStart int    `json:"lineStart"`
			LineEnd   int    `json:"lineEnd"`
			Comment   string `json:"comment"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, jsonErr(err), http.StatusBadRequest)
			return
		}

		res, err := db.Exec(`
			INSERT INTO review_comments (repo_path, sha, line_start, line_end, comment)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(repo_path, sha, line_start, line_end) DO UPDATE SET comment = excluded.comment
		`, body.RepoPath, body.SHA, body.LineStart, body.LineEnd, body.Comment)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		id, _ := res.LastInsertId()
		bus.Publish(Event{Type: "commits:sync", Data: true})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int64{"id": id})
	}
}

func handleDeleteReviewComment(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, jsonErr(err), http.StatusBadRequest)
			return
		}

		db.Exec(`DELETE FROM review_comments WHERE id = ?`, body.ID)
		bus.Publish(Event{Type: "commits:sync", Data: true})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// --- Review Submission ---

func handleSubmitReview(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			RepoPath string `json:"repoPath"`
			SHA      string `json:"sha"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, jsonErr(err), http.StatusBadRequest)
			return
		}

		// Load review comments
		rows, err := db.Query(`
			SELECT line_start, line_end, comment
			FROM review_comments WHERE repo_path = ? AND sha = ?
			ORDER BY line_start
		`, body.RepoPath, body.SHA)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var annotations []reviewAnnotation
		var commitNote string
		for rows.Next() {
			var a reviewAnnotation
			if err := rows.Scan(&a.lineStart, &a.lineEnd, &a.comment); err != nil {
				continue
			}
			if a.lineStart == 0 && a.lineEnd == 0 {
				commitNote = a.comment
				continue
			}
			annotations = append(annotations, a)
		}

		// Load commit metadata
		var author, message, committedAt, repoName string
		db.QueryRow(`
			SELECT c.author, c.message, c.committed_at, r.name
			FROM commits c JOIN repos r ON r.id = c.repo_id
			WHERE r.path = ? AND c.sha = ?
		`, body.RepoPath, body.SHA).Scan(&author, &message, &committedAt, &repoName)

		// Load diff (plain text for prompt)
		diffResult, err := gitlocal.CommitDiff(body.RepoPath, body.SHA)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		diffText := diffResult.Diff
		if diffResult.Format == "delta" {
			diffText = stripHTML(diffText)
		}

		// Build prompt from template
		diffLines := strings.Split(diffText, "\n")
		var promptAnnotations []prompts.ReviewAnnotation
		for _, a := range annotations {
			var ctx strings.Builder
			for i := a.lineStart - 1; i < a.lineEnd && i < len(diffLines); i++ {
				if i >= 0 {
					ctx.WriteString(diffLines[i])
					ctx.WriteByte('\n')
				}
			}
			promptAnnotations = append(promptAnnotations, prompts.ReviewAnnotation{
				LineStart: a.lineStart,
				LineEnd:   a.lineEnd,
				Context:   strings.TrimRight(ctx.String(), "\n"),
				Comment:   a.comment,
			})
		}

		prompt, err := prompts.Review(prompts.ReviewData{
			RepoName:    repoName,
			SHA:         body.SHA,
			Author:      author,
			Date:        committedAt,
			Message:     message,
			Diff:        diffText,
			Annotations: promptAnnotations,
			CommitNote:  commitNote,
		})
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		// Create task
		res, err := db.Exec(`
			INSERT INTO claude_tasks (type, status, repo_path, commit_sha, prompt, created_at)
			VALUES ('review', 'pending', ?, ?, ?, ?)
		`, body.RepoPath, body.SHA, prompt, time.Now().Format(time.RFC3339))
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		taskID, _ := res.LastInsertId()

		// Launch async
		go runClaudeReview(db, bus, int(taskID), body.RepoPath, body.SHA, prompt)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": taskID, "status": "pending"})
	}
}

func runClaudeReview(db *sql.DB, bus *EventBus, taskID int, repoPath, sha, prompt string) {
	db.Exec(`UPDATE claude_tasks SET status='running', started_at=? WHERE id=?`,
		time.Now().Format(time.RFC3339), taskID)
	bus.Publish(Event{Type: "claude:task", Data: map[string]any{
		"id": taskID, "status": "running", "repoPath": repoPath, "commitSha": sha,
	}})

	log.Printf("cmdr: claude review started (task %d, %s %s)", taskID, repoPath, sha[:7])

	result, err := tasks.Claude(prompt, repoPath)

	now := time.Now().Format(time.RFC3339)
	if err != nil {
		db.Exec(`UPDATE claude_tasks SET status='failed', error_msg=?, completed_at=? WHERE id=?`,
			err.Error(), now, taskID)
		bus.Publish(Event{Type: "claude:task", Data: map[string]any{
			"id": taskID, "status": "failed",
		}})
		log.Printf("cmdr: claude review failed (task %d): %v", taskID, err)
		return
	}

	title := extractTitle(result)
	db.Exec(`UPDATE claude_tasks SET status='completed', result=?, title=?, completed_at=? WHERE id=?`,
		result, title, now, taskID)
	bus.Publish(Event{Type: "claude:task", Data: map[string]any{
		"id": taskID, "status": "completed", "title": title,
	}})

	// Clean up review comments — they've been consumed
	db.Exec(`DELETE FROM review_comments WHERE repo_path=? AND sha=?`, repoPath, sha)

	// Notify frontend so commit reviewCount refreshes
	bus.Publish(Event{Type: "commits:sync", Data: true})

	log.Printf("cmdr: claude review completed (task %d)", taskID)
}

type reviewAnnotation struct {
	lineStart, lineEnd int
	comment            string
}

// extractTitle pulls a display title from the review result.
// Looks for the first markdown heading, falls back to first non-empty line.
var headingRe = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)

func extractTitle(result string) string {
	var raw string
	if m := headingRe.FindStringSubmatch(result); len(m) > 1 {
		raw = m[1]
	} else {
		// Fall back to first non-empty line
		for _, line := range strings.SplitN(result, "\n", 10) {
			line = strings.TrimSpace(line)
			if line != "" {
				raw = line
				break
			}
		}
	}
	// Strip markdown inline formatting (backticks, bold, italic)
	raw = strings.ReplaceAll(raw, "`", "")
	raw = strings.ReplaceAll(raw, "**", "")
	raw = strings.ReplaceAll(raw, "*", "")
	raw = strings.TrimSpace(raw)
	// Truncate if too long
	if len(raw) > 120 {
		raw = raw[:117] + "..."
	}
	return raw
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	return htmlTagRe.ReplaceAllString(s, "")
}

// --- Claude Tasks ---

func handleListClaudeTasks(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `SELECT id, type, status, repo_path, commit_sha, COALESCE(title, ''), COALESCE(pr_url, ''), error_msg, created_at, started_at, completed_at, COALESCE(refactored, 0)
			FROM claude_tasks ORDER BY created_at DESC LIMIT 50`
		rows, err := db.Query(query)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type task struct {
			ID          int     `json:"id"`
			Type        string  `json:"type"`
			Status      string  `json:"status"`
			RepoPath    string  `json:"repoPath"`
			CommitSHA   string  `json:"commitSha"`
			Title       string  `json:"title,omitempty"`
			PRUrl       string  `json:"prUrl,omitempty"`
			ErrorMsg    string  `json:"errorMsg,omitempty"`
			CreatedAt   string  `json:"createdAt"`
			StartedAt   *string `json:"startedAt"`
			CompletedAt *string `json:"completedAt"`
			Refactored  bool    `json:"refactored"`
		}

		var taskList []task
		for rows.Next() {
			var t task
			if err := rows.Scan(&t.ID, &t.Type, &t.Status, &t.RepoPath, &t.CommitSHA, &t.Title, &t.PRUrl,
				&t.ErrorMsg, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.Refactored); err != nil {
				continue
			}
			taskList = append(taskList, t)
		}
		if taskList == nil {
			taskList = []task{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(taskList)
	}
}

func handleUpdateClaudeTaskResult(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			ID     int    `json:"id"`
			Result string `json:"result"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == 0 {
			http.Error(w, `{"error":"missing id or result"}`, http.StatusBadRequest)
			return
		}

		title := extractTitle(body.Result)
		db.Exec(`UPDATE claude_tasks SET result=?, title=? WHERE id=?`, body.Result, title, body.ID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleGetClaudeTaskResult(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
			return
		}

		var result, prompt, status, errMsg string
		err := db.QueryRow(`SELECT result, prompt, status, error_msg FROM claude_tasks WHERE id = ?`, id).
			Scan(&result, &prompt, &status, &errMsg)
		if err != nil {
			http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
			return
		}

		// For draft tasks, return the prompt as the result
		content := result
		if status == "draft" {
			content = prompt
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"result":   content,
			"status":   status,
			"errorMsg": errMsg,
		})
	}
}

func handleDismissClaudeTask(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			ID  int    `json:"id"`
			All string `json:"all"` // "completed" to clear all completed
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, jsonErr(err), http.StatusBadRequest)
			return
		}

		// Clean up worktrees for refactoring tasks being dismissed
		if body.ID > 0 {
			cleanupRefactorWorktree(db, body.ID)
		} else if body.All == "completed" {
			cleanupAllRefactorWorktrees(db)
		}

		var res sql.Result
		var err error
		if body.All == "completed" {
			res, err = db.Exec(`DELETE FROM claude_tasks WHERE status IN ('completed', 'failed', 'resolved', 'refactoring')`)
		} else if body.ID > 0 {
			res, err = db.Exec(`DELETE FROM claude_tasks WHERE id = ?`, body.ID)
		} else {
			http.Error(w, `{"error":"missing id or all"}`, http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		n, _ := res.RowsAffected()

		if body.ID > 0 && n > 0 {
			bus.Publish(Event{Type: "claude:task", Data: map[string]any{
				"id": body.ID, "status": "dismissed",
			}})
		} else if body.All == "completed" && n > 0 {
			bus.Publish(Event{Type: "claude:task", Data: map[string]any{
				"id": 0, "status": "dismissed",
			}})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int64{"dismissed": n})
	}
}

func handleResolveTask(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			ID    int    `json:"id"`
			PRUrl string `json:"prUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == 0 {
			http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
			return
		}

		now := time.Now().Format(time.RFC3339)
		db.Exec(`UPDATE claude_tasks SET status='resolved', pr_url=?, completed_at=? WHERE id=?`,
			body.PRUrl, now, body.ID)

		bus.Publish(Event{Type: "claude:task", Data: map[string]any{
			"id": body.ID, "status": "resolved", "prUrl": body.PRUrl,
		}})

		log.Printf("cmdr: task %d resolved (PR: %s)", body.ID, body.PRUrl)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "resolved", "prUrl": body.PRUrl})
	}
}

func handleStartRefactor(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			TaskID int `json:"taskId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TaskID == 0 {
			http.Error(w, `{"error":"missing taskId"}`, http.StatusBadRequest)
			return
		}

		// Load the task
		var result, repoPath, commitSha string
		err := db.QueryRow(`SELECT result, repo_path, commit_sha FROM claude_tasks WHERE id = ?`, body.TaskID).
			Scan(&result, &repoPath, &commitSha)
		if err != nil {
			http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
			return
		}

		windowName := fmt.Sprintf("review-%d", body.TaskID)
		worktreeName := fmt.Sprintf("refactor-review-%d", body.TaskID)

		shortSha := commitSha
		if len(shortSha) > 7 {
			shortSha = shortSha[:7]
		}

		// Build autonomous refactor prompt
		prompt := fmt.Sprintf(
			"You are addressing code review findings from commit %s in this repository.\n\n"+
				"## Review Findings\n\n%s\n\n"+
				"## Instructions\n\n"+
				"1. Read the relevant source files to understand the current code\n"+
				"2. Address each finding — make the changes directly\n"+
				"3. If a finding contains a `> User response:` blockquote, treat it as explicit guidance from the reviewer — follow it instead of choosing on your own\n"+
				"4. If a finding has multiple valid approaches and no user response, pick the cleanest one\n"+
				"5. If a finding was removed from the review, it means the reviewer decided it's not applicable — skip it\n"+
				"6. Only ask me if there is genuine ambiguity that requires a judgment call\n"+
				"7. When all changes are complete, commit with a message referencing the findings\n"+
				"8. Push the branch and create a PR — keep the body short: a brief summary of what changed and why, no test plan or checklists\n",
			shortSha, result,
		)

		// Write task ID marker file keyed by branch name (for hook to find)
		refactorDir := filepath.Join(os.Getenv("HOME"), ".cmdr", "refactors")
		os.MkdirAll(refactorDir, 0o700)
		os.WriteFile(filepath.Join(refactorDir, worktreeName), []byte(strconv.Itoa(body.TaskID)), 0o644)

		// Escape single quotes for shell, launch claude with worktree
		escaped := strings.ReplaceAll(prompt, "'", "'\\''")
		cmd := fmt.Sprintf("claude -w %s '%s'", worktreeName, escaped)

		target, err := tmux.CreateRefactorWindow(windowName, repoPath, cmd)
		if err != nil {
			log.Printf("cmdr: refactor window failed: %v", err)
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		// Update task status
		db.Exec(`UPDATE claude_tasks SET status='refactoring', refactored=1 WHERE id=?`, body.TaskID)
		bus.Publish(Event{Type: "claude:task", Data: map[string]any{
			"id": body.TaskID, "status": "refactoring",
		}})

		log.Printf("cmdr: refactor started (task %d, worktree %s, target %s)", body.TaskID, worktreeName, target)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"target": target, "session": "claude_refactor", "window": windowName})
	}
}

// cleanupRefactorWorktree removes the worktree and marker file for a single task.
func cleanupRefactorWorktree(db *sql.DB, taskID int) {
	var repoPath, status string
	err := db.QueryRow(`SELECT repo_path, status FROM claude_tasks WHERE id = ?`, taskID).Scan(&repoPath, &status)
	if err != nil || status != "refactoring" {
		return
	}
	pruneWorktree(repoPath, taskID)
}

// cleanupAllRefactorWorktrees removes worktrees for all refactoring tasks.
func cleanupAllRefactorWorktrees(db *sql.DB) {
	rows, err := db.Query(`SELECT id, repo_path FROM claude_tasks WHERE status = 'refactoring'`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var repoPath string
		if err := rows.Scan(&id, &repoPath); err != nil {
			continue
		}
		pruneWorktree(repoPath, id)
	}
}

func pruneWorktree(repoPath string, taskID int) {
	worktreeName := fmt.Sprintf("refactor-review-%d", taskID)
	worktreePath := filepath.Join(repoPath, ".claude", "worktrees", worktreeName)

	// Remove the worktree via git (handles index cleanup)
	if _, err := os.Stat(worktreePath); err == nil {
		cmd := exec.Command("git", "-C", repoPath, "worktree", "remove", worktreePath, "--force")
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("cmdr: worktree remove failed (task %d): %s: %v", taskID, strings.TrimSpace(string(out)), err)
		} else {
			log.Printf("cmdr: pruned worktree %s (task %d)", worktreeName, taskID)
		}
	}

	// Clean up marker file
	markerPath := filepath.Join(os.Getenv("HOME"), ".cmdr", "refactors", worktreeName)
	os.Remove(markerPath)
}
