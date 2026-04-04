package tasks

import (
	"log"
	"os/exec"
)

// BrewUpdate runs `brew update` to refresh the formula index.
func BrewUpdate() error {
	out, err := exec.Command("brew", "update").CombinedOutput()
	if err != nil {
		log.Printf("cmdr: brew update failed: %s", string(out))
		return err
	}
	log.Printf("cmdr: brew update completed")
	return nil
}
