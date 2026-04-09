package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func imagesDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cmdr", "images")
	os.MkdirAll(dir, 0o700)
	return dir
}

// handleImageUpload accepts an image via multipart/form-data and saves it.
func handleImageUpload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		r.ParseMultipartForm(10 << 20) // 10MB max

		file, header, err := r.FormFile("image")
		if err != nil {
			http.Error(w, `{"error":"image field required"}`, http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Determine extension from content type or filename
		ext := ".png"
		if ct := header.Header.Get("Content-Type"); ct != "" {
			switch ct {
			case "image/jpeg":
				ext = ".jpg"
			case "image/gif":
				ext = ".gif"
			case "image/webp":
				ext = ".webp"
			}
		} else if e := filepath.Ext(header.Filename); e != "" {
			ext = e
		}

		filename := uuid.New().String() + ext
		destPath := filepath.Join(imagesDir(), filename)

		dest, err := os.Create(destPath)
		if err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}
		defer dest.Close()

		if _, err := io.Copy(dest, file); err != nil {
			http.Error(w, jsonErr(err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"path": destPath,
			"url":  "/api/images/" + filename,
		})
	}
}

// handleImageServe serves images from ~/.cmdr/images/.
func handleImageServe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract filename from path: /api/images/{filename}
		parts := strings.SplitN(r.URL.Path, "/api/images/", 2)
		if len(parts) < 2 || parts[1] == "" {
			http.Error(w, `{"error":"missing filename"}`, http.StatusBadRequest)
			return
		}

		filename := filepath.Base(parts[1]) // prevent path traversal
		filePath := filepath.Join(imagesDir(), filename)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		// Set content type based on extension
		switch filepath.Ext(filename) {
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".gif":
			w.Header().Set("Content-Type", "image/gif")
		case ".webp":
			w.Header().Set("Content-Type", "image/webp")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}

		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, filePath)
	}
}

// cleanupOrphanImages removes images not referenced by any draft.
// Called periodically or on submit.
func cleanupOrphanImages(referenced map[string]bool) {
	dir := imagesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if !referenced[path] && !referenced[fmt.Sprintf("/api/images/%s", entry.Name())] {
			os.Remove(path)
		}
	}
}
