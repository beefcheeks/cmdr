package daemon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cmdr-tool/cmdr/internal/tasks"
	"github.com/google/uuid"
)

type squadMember struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Path  string `json:"path"`
	Alias string `json:"alias"`
}

type squad struct {
	Name      string        `json:"name"`
	CreatedAt string        `json:"createdAt"`
	Repos     []squadMember `json:"repos"`
}

func handleListSquads(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sRows, err := db.Query(`SELECT name, created_at FROM squads ORDER BY name`)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		defer sRows.Close()

		var squads []squad
		for sRows.Next() {
			var s squad
			if err := sRows.Scan(&s.Name, &s.CreatedAt); err != nil {
				continue
			}
			s.Repos = []squadMember{}
			squads = append(squads, s)
		}
		if squads == nil {
			squads = []squad{}
		}

		// Load members for each squad
		for i, s := range squads {
			mRows, err := db.Query(
				`SELECT id, name, path, squad_alias FROM repos WHERE squad = ? ORDER BY squad_alias, name`,
				s.Name,
			)
			if err != nil {
				continue
			}
			for mRows.Next() {
				var m squadMember
				if err := mRows.Scan(&m.ID, &m.Name, &m.Path, &m.Alias); err != nil {
					continue
				}
				squads[i].Repos = append(squads[i].Repos, m)
			}
			mRows.Close()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(squads)
	}
}

func handleCreateSquad(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}

		_, err := db.Exec(`INSERT INTO squads (name) VALUES (?)`, body.Name)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "PRIMARY KEY") {
				http.Error(w, `{"error":"squad already exists"}`, http.StatusConflict)
				return
			}
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": body.Name})
	}
}

func handleDeleteSquad(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}

		// Clear squad assignment on member repos
		db.Exec(`UPDATE repos SET squad='', squad_alias='' WHERE squad = ?`, body.Name)
		db.Exec(`DELETE FROM squads WHERE name = ?`, body.Name)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"deleted": body.Name})
	}
}

func handleAssignRepoSquad(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			RepoID int    `json:"repoId"`
			Squad  string `json:"squad"`
			Alias  string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RepoID == 0 {
			http.Error(w, `{"error":"repoId is required"}`, http.StatusBadRequest)
			return
		}

		if body.Squad == "" {
			// Clear assignment
			db.Exec(`UPDATE repos SET squad='', squad_alias='' WHERE id = ?`, body.RepoID)
		} else {
			// Auto-derive alias from repo name if empty
			alias := body.Alias
			if alias == "" {
				var name string
				db.QueryRow(`SELECT name FROM repos WHERE id = ?`, body.RepoID).Scan(&name)
				parts := strings.Split(name, "/")
				alias = parts[len(parts)-1]
			}
			db.Exec(`UPDATE repos SET squad=?, squad_alias=? WHERE id = ?`, body.Squad, alias, body.RepoID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleUpdateRepoMonitor(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			RepoID  int  `json:"repoId"`
			Monitor bool `json:"monitor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RepoID == 0 {
			http.Error(w, `{"error":"repoId is required"}`, http.StatusBadRequest)
			return
		}

		monVal := 0
		if body.Monitor {
			monVal = 1
		}
		db.Exec(`UPDATE repos SET monitor=? WHERE id = ?`, monVal, body.RepoID)

		// Sync commits when monitoring is turned on
		if body.Monitor {
			var path, branch string
			if err := db.QueryRow(`SELECT path, default_branch FROM repos WHERE id = ?`, body.RepoID).Scan(&path, &branch); err == nil {
				log.Printf("cmdr: monitor enabled for repo %d, syncing", body.RepoID)
				go func() {
					if n := tasks.SyncOne(db, body.RepoID, path, branch); n > 0 {
						bus.Publish(Event{Type: "commits:sync", Data: true})
					}
				}()
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleListDelegations(db *sql.DB) http.HandlerFunc {
	type delegation struct {
		ID             int    `json:"id"`
		Status         string `json:"status"`
		Squad          string `json:"squad"`
		DelegationFrom string `json:"delegationFrom"`
		DelegationTo   string `json:"delegationTo"`
		Title          string `json:"title"`
		Summary        string `json:"summary"`
		Branch         string `json:"branch"`
		RepoPath       string `json:"repoPath"`
		Result         string `json:"result,omitempty"`
		CreatedAt      string `json:"createdAt"`
		CompletedAt    string `json:"completedAt,omitempty"`
		Effort         string `json:"effort,omitempty"`
		LeaderCwd      string `json:"leaderCwd,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		squadFilter := r.URL.Query().Get("squad")

		query := `SELECT ct.id, ct.status, d.squad, d.from_alias, d.to_alias,
				COALESCE(ct.title, ''), d.summary, d.branch, ct.repo_path,
				COALESCE(ct.result, ''), ct.created_at, COALESCE(ct.completed_at, ''),
				COALESCE(d.effort, ''), COALESCE(d.leader_cwd, '')
			FROM agent_tasks ct
			JOIN delegations d ON d.task_id = ct.id
			WHERE ct.type = 'delegation'`
		var args []any
		if squadFilter != "" {
			query += ` AND d.squad = ?`
			args = append(args, squadFilter)
		}
		query += ` ORDER BY ct.created_at DESC`

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var delegations []delegation
		for rows.Next() {
			var d delegation
			if err := rows.Scan(&d.ID, &d.Status, &d.Squad, &d.DelegationFrom, &d.DelegationTo, &d.Title, &d.Summary, &d.Branch, &d.RepoPath, &d.Result, &d.CreatedAt, &d.CompletedAt, &d.Effort, &d.LeaderCwd); err != nil {
				continue
			}
			delegations = append(delegations, d)
		}
		if delegations == nil {
			delegations = []delegation{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(delegations)
	}
}

func handleDelegationSummary(db *sql.DB) http.HandlerFunc {
	type summary struct {
		Squad       string   `json:"squad"`
		ActiveCount int      `json:"activeCount"`
		TotalCount  int      `json:"totalCount"`
		Members     []string `json:"members"`
		LatestAt    string   `json:"latestAt"`
		LatestTitle string   `json:"latestTitle"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT d.squad,
				SUM(CASE WHEN ct.status IN ('running','pending') THEN 1 ELSE 0 END),
				COUNT(*),
				MAX(ct.created_at)
			FROM agent_tasks ct
			JOIN delegations d ON d.task_id = ct.id
			WHERE ct.type = 'delegation'
			GROUP BY d.squad
			HAVING COUNT(*) > 0
		`)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var summaries []summary
		for rows.Next() {
			var s summary
			if err := rows.Scan(&s.Squad, &s.ActiveCount, &s.TotalCount, &s.LatestAt); err != nil {
				continue
			}

			// Get unique member aliases
			mRows, _ := db.Query(
				`SELECT DISTINCT from_alias FROM delegations WHERE squad = ?
				 UNION SELECT DISTINCT to_alias FROM delegations WHERE squad = ?`,
				s.Squad, s.Squad,
			)
			if mRows != nil {
				for mRows.Next() {
					var alias string
					mRows.Scan(&alias)
					s.Members = append(s.Members, alias)
				}
				mRows.Close()
			}
			if s.Members == nil {
				s.Members = []string{}
			}

			// Get latest title
			db.QueryRow(
				`SELECT COALESCE(ct.title, d.summary) FROM agent_tasks ct
				 JOIN delegations d ON d.task_id = ct.id
				 WHERE d.squad = ? ORDER BY ct.created_at DESC LIMIT 1`, s.Squad,
			).Scan(&s.LatestTitle)

			summaries = append(summaries, s)
		}
		if summaries == nil {
			summaries = []summary{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

// slugifySummary kebab-cases s and truncates to maxLen, cutting at a word
// boundary where possible. Used as fallback when no explicit --slug is given.
func slugifySummary(s string, maxLen int) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxLen {
		slug = slug[:maxLen]
		if i := strings.LastIndex(slug, "-"); i > maxLen/2 {
			slug = slug[:i]
		}
	}
	return slug
}

// enlistmentPaths returns the effort dir, artifact path, and debrief path for
// an enlistment rooted in the squad leader's working tree. Falls back to
// /tmp/cmdr when no leader cwd was provided (legacy dispatch).
func enlistmentPaths(leaderCwd, effort string, taskID int, slug string) (effortDir, artifactPath, debriefPath string) {
	if leaderCwd == "" {
		dir := filepath.Join(os.TempDir(), "cmdr")
		return dir, "", filepath.Join(dir, fmt.Sprintf("debrief-%d.md", taskID))
	}
	effortDir = filepath.Join(leaderCwd, ".agents", "enlistments", effort)
	base := fmt.Sprintf("enlist-%d_%s", taskID, slug)
	return effortDir, filepath.Join(effortDir, base+".md"), filepath.Join(effortDir, fmt.Sprintf("debrief-%d_%s.md", taskID, slug))
}

// writeEnlistmentArtifact writes (or overwrites) the enlistment record in the
// leader's .agents/enlistments folder: frontmatter metadata + the orders body.
func writeEnlistmentArtifact(path string, meta map[string]string, body string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	keys := []string{"task_id", "effort", "slug", "squad", "from", "to", "branch", "session_id", "repo", "worktree_path", "terminal_target", "debrief", "created_at"}
	var b strings.Builder
	b.WriteString("---\n")
	for _, k := range keys {
		if v, ok := meta[k]; ok && v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	b.WriteString("---\n\n")
	b.WriteString(body)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func handleEnlist(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Squad     string `json:"squad"`
			From      string `json:"from"`
			To        string `json:"to"`
			Summary   string `json:"summary"`
			Details   string `json:"details"`
			PR        bool   `json:"pr"`
			Effort    string `json:"effort"`
			Slug      string `json:"slug"`
			LeaderCwd string `json:"leaderCwd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if body.Squad == "" || body.From == "" || body.To == "" || body.Summary == "" {
			http.Error(w, `{"error":"squad, from, to, and summary are required"}`, http.StatusBadRequest)
			return
		}

		// Validate squad exists
		var squadExists bool
		db.QueryRow(`SELECT COUNT(*) FROM squads WHERE name = ?`, body.Squad).Scan(&squadExists)
		if !squadExists {
			http.Error(w, fmt.Sprintf(`{"error":"squad %q does not exist"}`, body.Squad), http.StatusNotFound)
			return
		}

		// Resolve target alias to repo path
		var targetPath string
		if err := db.QueryRow(`SELECT path FROM repos WHERE squad = ? AND squad_alias = ?`, body.Squad, body.To).Scan(&targetPath); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"squad member %q not found in squad %q"}`, body.To, body.Squad), http.StatusNotFound)
			return
		}

		// Idempotency: an open enlistment already targeting this repo receives
		// the new orders as an amendment instead of spawning a second session.
		var openTaskID int
		db.QueryRow(
			`SELECT ct.id FROM agent_tasks ct JOIN delegations d ON d.task_id = ct.id
			 WHERE ct.type = 'delegation' AND ct.repo_path = ? AND d.squad = ?
			   AND ct.status IN ('pending', 'running')
			 ORDER BY ct.id DESC LIMIT 1`, targetPath, body.Squad,
		).Scan(&openTaskID)
		if openTaskID > 0 {
			amendment := fmt.Sprintf("**%s**\n\n%s", body.Summary, body.Details)
			method, err := amendDelegation(db, bus, openTaskID, amendment)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"existing enlistment (task %d) could not accept amendment: %s"}`, openTaskID, err), http.StatusConflict)
				return
			}
			log.Printf("cmdr: enlist forwarded as amendment to task %d (%s)", openTaskID, method)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"taskId":    openTaskID,
				"forwarded": true,
				"method":    method,
				"note":      fmt.Sprintf("existing enlistment found for %s — orders forwarded as amendment to task %d", body.To, openTaskID),
			})
			return
		}

		// Build delivery instructions based on --pr flag
		var delivery string
		if body.PR {
			delivery = "## Delivery\n\nWhen complete, commit your changes to the branch and push. Then open a pull request with a clear title and description summarizing what you changed and why."
		} else {
			delivery = "## Delivery\n\nWhen complete, commit your changes with a clear message, merge your branch into main, and push."
		}

		// Resolve slug + effort for the leader-side enlistment folder
		slug := body.Slug
		if slug == "" {
			slug = slugifySummary(body.Summary, 40)
		}
		effort := body.Effort
		if effort == "" {
			effort = slug
		}

		// Create task row
		orders := fmt.Sprintf("## Enlistment from %s\n\n**Summary:** %s\n\n%s\n\n%s", body.From, body.Summary, body.Details, delivery)
		now := time.Now().Format(time.RFC3339)
		taskResult, err := db.Exec(
			`INSERT INTO agent_tasks (type, status, repo_path, prompt, intent, created_at, started_at)
			 VALUES ('delegation', 'pending', ?, ?, 'delegation', ?, ?)`,
			targetPath, orders, now, now,
		)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		taskID64, _ := taskResult.LastInsertId()
		taskID := int(taskID64)

		effortDir, artifactPath, debriefPath := enlistmentPaths(body.LeaderCwd, effort, taskID, slug)
		os.MkdirAll(effortDir, 0o755)

		// Insert delegation details
		branchName := fmt.Sprintf("squad/%s/%d", body.Squad, taskID)
		if _, err := db.Exec(
			`INSERT INTO delegations (task_id, squad, from_alias, to_alias, branch, summary, details, effort, slug, leader_cwd, artifact_path, debrief_path)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			taskID, body.Squad, body.From, body.To, branchName, body.Summary, body.Details,
			effort, slug, body.LeaderCwd, artifactPath, debriefPath,
		); err != nil {
			db.Exec(`DELETE FROM agent_tasks WHERE id = ?`, taskID)
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		prompt := orders + fmt.Sprintf("\n\n---\n\nDEBRIEF_PATH: %s", debriefPath)

		// Pre-assign the agent session ID so amendments can --resume later
		sessionID := uuid.NewString()

		// Launch via unified launchTask — gets worktree, tmux session, system prompt
		res, err := launchTask(db, bus, TaskLaunchConfig{
			TaskID:         taskID,
			Intent:         "delegation",
			UserPrompt:     prompt,
			RepoPath:       targetPath,
			WindowPrefix:   "enlist",
			WorktreePrefix: fmt.Sprintf("enlist-%s", body.Squad),
			SessionID:      sessionID,
		})
		if err != nil {
			db.Exec(`DELETE FROM delegations WHERE task_id = ?`, taskID)
			db.Exec(`DELETE FROM agent_tasks WHERE id = ?`, taskID)
			log.Printf("cmdr: enlist launch failed: %v", err)
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		// Record the enlistment in the leader's .agents/enlistments folder
		var worktreeName string
		db.QueryRow(`SELECT worktree FROM agent_tasks WHERE id = ?`, taskID).Scan(&worktreeName)
		var worktreePath string
		if worktreeName != "" {
			worktreePath = worktreeDir(targetPath, worktreeName)
		}
		if err := writeEnlistmentArtifact(artifactPath, map[string]string{
			"task_id":         fmt.Sprintf("%d", taskID),
			"effort":          effort,
			"slug":            slug,
			"squad":           body.Squad,
			"from":            body.From,
			"to":              body.To,
			"branch":          branchName,
			"session_id":      sessionID,
			"repo":            targetPath,
			"worktree_path":   worktreePath,
			"terminal_target": res.Target,
			"debrief":         filepath.Base(debriefPath),
			"created_at":      now,
		}, orders); err != nil {
			log.Printf("cmdr: enlistment artifact write failed (task %d): %v", taskID, err)
		}

		// Notify frontend
		bus.Publish(Event{Type: "delegation:update", Data: map[string]any{
			"squad": body.Squad, "taskId": taskID, "status": "running",
		}})

		log.Printf("cmdr: enlistment dispatched (task %d, squad %s, %s → %s, effort %s)", taskID, body.Squad, body.From, body.To, effort)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"taskId":  taskID,
			"branch":  branchName,
			"session": res.Session,
			"window":  res.Window,
			"effort":  effort,
			"slug":    slug,
		})
	}
}

// amendDelegation routes amended orders to an existing enlistment. Delivery
// preference: live terminal window (send-keys pointer to the amendment file),
// else a fresh window resuming the recorded agent session in the worktree.
// Returns the delivery method used ("send-keys" or "resume").
func amendDelegation(db *sql.DB, bus *EventBus, taskID int, details string) (string, error) {
	var status, repoPath, worktreeName, terminalTarget, sessionID string
	err := db.QueryRow(
		`SELECT status, repo_path, COALESCE(worktree, ''), COALESCE(terminal_target, ''), COALESCE(agent_session_id, '')
		 FROM agent_tasks WHERE id = ? AND type = 'delegation'`, taskID,
	).Scan(&status, &repoPath, &worktreeName, &terminalTarget, &sessionID)
	if err != nil {
		return "", fmt.Errorf("enlistment task %d not found", taskID)
	}
	if status == "failed" {
		return "", fmt.Errorf("task %d failed — re-enlist instead of amending", taskID)
	}

	var squadName, fromAlias, artifactPath, debriefPath string
	db.QueryRow(
		`SELECT squad, from_alias, COALESCE(artifact_path, ''), COALESCE(debrief_path, '')
		 FROM delegations WHERE task_id = ?`, taskID,
	).Scan(&squadName, &fromAlias, &artifactPath, &debriefPath)
	if debriefPath == "" {
		debriefPath = filepath.Join(os.TempDir(), "cmdr", fmt.Sprintf("debrief-%d.md", taskID))
	}

	now := time.Now().Format(time.RFC3339)

	// Archive any prior debrief so its presence doesn't re-trigger the poller's
	// completion signal for the amended round. Keeps the record in the folder.
	if _, err := os.Stat(debriefPath); err == nil {
		base := strings.TrimSuffix(debriefPath, ".md")
		for round := 1; ; round++ {
			archived := fmt.Sprintf("%s.%d.md", base, round)
			if _, err := os.Stat(archived); os.IsNotExist(err) {
				os.Rename(debriefPath, archived)
				break
			}
		}
	}

	// Append the amendment to the leader-side artifact
	if artifactPath != "" {
		if f, err := os.OpenFile(artifactPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "\n\n## Amendment (%s)\n\n%s\n", now, details)
			f.Close()
		}
	}

	// Write the amendment prompt file the enlisted session will read
	promptDir := filepath.Join(os.TempDir(), "cmdr")
	os.MkdirAll(promptDir, 0o700)
	amendFile := filepath.Join(promptDir, fmt.Sprintf("task-%d-amend.md", taskID))
	amendPrompt := fmt.Sprintf(
		"## Amended Orders from %s\n\nYour enlistment (task %d) has updated instructions:\n\n%s\n\n"+
			"Apply these on top of your existing work. When complete, rewrite your debrief at:\n\nDEBRIEF_PATH: %s",
		fromAlias, taskID, details, debriefPath,
	)
	os.WriteFile(amendFile, []byte(amendPrompt), 0o644)

	var method string
	if terminalTarget != "" && term.WindowExists(terminalTarget) {
		// Live session — inject a pointer to the amendment file
		msg := fmt.Sprintf("Amended orders received: read %s and execute. Rewrite your debrief at %s when done.", amendFile, debriefPath)
		if err := term.SendKeys(terminalTarget, msg, true); err != nil {
			return "", fmt.Errorf("send-keys to %s: %w", terminalTarget, err)
		}
		method = "send-keys"
	} else {
		// Session gone — resume in a fresh window, working from the worktree
		if sessionID == "" {
			return "", fmt.Errorf("task %d has no recorded agent session — re-enlist to start fresh", taskID)
		}
		workDir := repoPath
		if worktreeName != "" {
			wtPath := worktreeDir(repoPath, worktreeName)
			if _, err := os.Stat(wtPath); err != nil {
				return "", fmt.Errorf("worktree for task %d is gone (%s) — re-enlist to start fresh", taskID, wtPath)
			}
			workDir = wtPath
		}
		resumeCmd, err := agt.ResumeCommand(sessionID)
		if err != nil {
			return "", fmt.Errorf("resume command: %w", err)
		}
		cmd := fmt.Sprintf("%s < '%s'", resumeCmd, amendFile)
		sessionName, err := findOrCreateSession(repoPath)
		if err != nil {
			return "", fmt.Errorf("session: %w", err)
		}
		target, err := term.CreateWindow(sessionName, fmt.Sprintf("enlist-%d", taskID), workDir, cmd)
		if err != nil {
			return "", fmt.Errorf("window: %w", err)
		}
		db.Exec(`UPDATE agent_tasks SET terminal_target=? WHERE id=?`, target, taskID)
		method = "resume"
	}

	// Re-arm the lifecycle: back to running, clear the previous result so the
	// poller waits for a fresh debrief.
	db.Exec(`UPDATE agent_tasks SET status='running', result='', started_at=?, completed_at=NULL WHERE id=?`, now, taskID)
	bus.Publish(Event{Type: "agent:task", Data: map[string]any{"id": taskID, "status": "running"}})
	if squadName != "" {
		bus.Publish(Event{Type: "delegation:update", Data: map[string]any{
			"squad": squadName, "taskId": taskID, "status": "running",
		}})
	}
	log.Printf("cmdr: task %d amended (delivery: %s)", taskID, method)
	return method, nil
}

func handleAmend(db *sql.DB, bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			TaskID  int    `json:"taskId"`
			Details string `json:"details"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TaskID == 0 || body.Details == "" {
			http.Error(w, `{"error":"taskId and details are required"}`, http.StatusBadRequest)
			return
		}

		method, err := amendDelegation(db, bus, body.TaskID, body.Details)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusConflict)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"taskId": body.TaskID, "method": method})
	}
}
