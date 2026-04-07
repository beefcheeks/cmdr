package daemon

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
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

// Cached brew outdated result
var brewCache struct {
	mu   sync.RWMutex
	data []byte // raw JSON from brew outdated --json
}

// refreshBrewOutdated runs brew outdated --json and caches the result.
// Publishes via SSE so the frontend updates without a page refresh.
func refreshBrewOutdated(bus *EventBus) {
	cmd := exec.Command("brew", "outdated", "--json")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		log.Printf("cmdr: brew outdated failed: %v", err)
		return
	}

	brewCache.mu.Lock()
	brewCache.data = out
	brewCache.mu.Unlock()

	// Parse and publish via SSE
	var result brewOutdated
	if err := json.Unmarshal(out, &result); err == nil {
		total := len(result.Formulae) + len(result.Casks)
		log.Printf("cmdr: brew outdated: %d formulae, %d casks", len(result.Formulae), len(result.Casks))
		if total > 0 {
			bus.Publish(Event{Type: "brew:outdated", Data: result})
		}
	}
}

func handleBrewOutdated() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		brewCache.mu.RLock()
		data := brewCache.data
		brewCache.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if data != nil {
			w.Write(data)
		} else {
			// No cache yet — return empty
			w.Write([]byte(`{"formulae":[],"casks":[]}`))
		}
	}
}

func handleBrewUpgrade(bus *EventBus) http.HandlerFunc {
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

		// Refresh cache after upgrade so the card updates
		go refreshBrewOutdated(bus)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"output": string(out),
		})
	}
}
