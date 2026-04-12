package daemon

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/mikehu/cmdr/internal/tasks"
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
