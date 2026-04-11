package tasks

import (
	"log"
	"os"
)

// Distill runs the /distill command to process unprocessed raw notes in
// ThoughtQuarry into refined concept pages. Runs headlessly from $HOME.
func Distill() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	out, err := Claude("/distill", home)
	if err != nil {
		return err
	}

	// Only log if something was actually processed
	if len(out) > 0 {
		log.Printf("cmdr: distill completed: %s", truncate(out, 200))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
