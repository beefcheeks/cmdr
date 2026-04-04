package daemon

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
)

type brewFormula struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
	Pinned            bool     `json:"pinned"`
}

type brewOutdated struct {
	Formulae []brewFormula `json:"formulae"`
	Casks    []brewFormula `json:"casks"`
}

func handleBrewOutdated() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := exec.Command("brew", "outdated", "--json").Output()
		if err != nil {
			http.Error(w, `{"error":"brew outdated failed"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// Pass through brew's JSON directly
		w.Write(out)
	}
}

func handleBrewUpgrade() http.HandlerFunc {
	type upgradeReq struct {
		Formula string `json:"formula"` // empty = upgrade all
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req upgradeReq
		json.NewDecoder(r.Body).Decode(&req)

		var cmd *exec.Cmd
		if req.Formula != "" {
			cmd = exec.Command("brew", "upgrade", req.Formula)
		} else {
			cmd = exec.Command("brew", "upgrade")
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("cmdr: brew upgrade failed: %s", string(out))
			http.Error(w, `{"error":"brew upgrade failed","output":"`+string(out)+`"}`, http.StatusInternalServerError)
			return
		}

		log.Printf("cmdr: brew upgrade completed: %s", req.Formula)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"output": string(out),
		})
	}
}
