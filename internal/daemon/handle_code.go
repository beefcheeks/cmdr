package daemon

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Cached file lists per repo (10-second TTL)
var fileCache struct {
	mu    sync.RWMutex
	items map[string]fileCacheEntry
}

type fileCacheEntry struct {
	files []string
	at    time.Time
}

func init() {
	fileCache.items = make(map[string]fileCacheEntry)
}

// handleCodeFiles returns tracked file paths for a repo (via git ls-files).
// Query params: repo (required), q (optional filter, min 3 chars).
func handleCodeFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoPath := r.URL.Query().Get("repo")
		if repoPath == "" {
			http.Error(w, `{"error":"repo is required"}`, http.StatusBadRequest)
			return
		}

		query := strings.ToLower(r.URL.Query().Get("q"))

		files := getCachedFiles(repoPath)
		if files == nil {
			// Fetch fresh
			out, err := exec.Command("git", "-C", repoPath, "ls-files").Output()
			if err != nil {
				http.Error(w, `{"error":"git ls-files failed"}`, http.StatusInternalServerError)
				return
			}
			files = strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(files) == 1 && files[0] == "" {
				files = []string{}
			}

			fileCache.mu.Lock()
			fileCache.items[repoPath] = fileCacheEntry{files: files, at: time.Now()}
			fileCache.mu.Unlock()
		}

		// Filter if query provided
		var results []string
		if query != "" {
			for _, f := range files {
				if fuzzyMatch(strings.ToLower(f), query) {
					results = append(results, f)
					if len(results) >= 20 {
						break
					}
				}
			}
		} else {
			results = files
			if len(results) > 50 {
				results = results[:50]
			}
		}

		if results == nil {
			results = []string{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func getCachedFiles(repoPath string) []string {
	fileCache.mu.RLock()
	defer fileCache.mu.RUnlock()
	entry, ok := fileCache.items[repoPath]
	if !ok || time.Since(entry.at) > 10*time.Second {
		return nil
	}
	return entry.files
}

// fuzzyMatch checks if all characters of query appear in target in order.
func fuzzyMatch(target, query string) bool {
	qi := 0
	for i := 0; i < len(target) && qi < len(query); i++ {
		if target[i] == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}
