package scheduler

import (
	"fmt"
	"log"

	"github.com/mikehu/cmdr/internal/tasks"
	"github.com/robfig/cron/v3"
)

// Task represents a scheduled task.
type Task struct {
	Name        string
	Description string
	Schedule    string // cron expression
	Fn          func() error
}

// Scheduler manages cron-scheduled tasks.
type Scheduler struct {
	cron  *cron.Cron
	tasks []Task
}

// New creates a scheduler with all registered tasks.
func New() *Scheduler {
	s := &Scheduler{
		cron: cron.New(cron.WithSeconds()),
	}
	s.register()
	return s
}

// register adds all defined tasks.
func (s *Scheduler) register() {
	s.tasks = []Task{
		{
			Name:        "hello",
			Description: "Example task — prints a message",
			Schedule:    "0 0 * * * *", // every hour
			Fn:          tasks.Hello,
		},
		// Add more tasks here as you build them, e.g.:
		// {
		// 	Name:        "daily-summary",
		// 	Description: "Ask Claude to summarize Slack and generate todos",
		// 	Schedule:    "0 0 9 * * 1-5", // 9am weekdays
		// 	Fn:          tasks.DailySummary,
		// },
	}
}

// Start begins running all scheduled tasks.
func (s *Scheduler) Start() {
	for _, t := range s.tasks {
		task := t // capture
		if _, err := s.cron.AddFunc(task.Schedule, func() {
			log.Printf("cmdr: running task %q", task.Name)
			if err := task.Fn(); err != nil {
				log.Printf("cmdr: task %q failed: %v", task.Name, err)
			}
		}); err != nil {
			log.Printf("cmdr: failed to schedule %q: %v", task.Name, err)
		}
	}
	s.cron.Start()
	log.Printf("cmdr: scheduler started with %d tasks", len(s.tasks))
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// Tasks returns all registered tasks.
func (s *Scheduler) Tasks() []Task {
	return s.tasks
}

// RunTask runs a task by name immediately.
func (s *Scheduler) RunTask(name string) error {
	for _, t := range s.tasks {
		if t.Name == name {
			return t.Fn()
		}
	}
	return fmt.Errorf("unknown task: %s", name)
}
