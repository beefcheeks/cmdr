package tasks

import (
	"fmt"
	"time"
)

// Hello is an example task that prints a greeting.
func Hello() error {
	fmt.Printf("cmdr: hello from task runner at %s\n", time.Now().Format(time.RFC3339))
	return nil
}
