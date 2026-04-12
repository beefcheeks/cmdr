package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	cmdr "github.com/mikehu/cmdr"
	"github.com/mikehu/cmdr/internal/daemon"
	"github.com/mikehu/cmdr/internal/db"
	"github.com/mikehu/cmdr/internal/scheduler"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	// Set version and embedded SPA filesystem for the daemon
	daemon.Version = version
	if webFS, err := fs.Sub(cmdr.WebBuildFS, "web/build"); err == nil {
		daemon.WebFS = webFS
	}

	root := &cobra.Command{
		Use:     "cmdr",
		Short:   "Personal command runner and automation daemon",
		Version: version,
	}

	root.AddCommand(startCmd())
	root.AddCommand(stopCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(runCmd())
	root.AddCommand(listCmd())
	root.AddCommand(contextCmd())
	root.AddCommand(initCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func startCmd() *cobra.Command {
	var foreground bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the cmdr daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if foreground {
				return daemon.Run()
			}
			return daemon.Start()
		},
	}
	cmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "Run in foreground (used by launchd)")
	return cmd
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the cmdr daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return daemon.Stop()
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return daemon.Status()
		},
	}
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [task]",
		Short: "Run a task immediately",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open()
			if err != nil {
				return err
			}
			defer database.Close()
			s := scheduler.New(database, scheduler.Hooks{})
			return s.RunTask(args[0])
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open()
			if err != nil {
				return err
			}
			defer database.Close()
			s := scheduler.New(database, scheduler.Hooks{})
			for _, t := range s.Tasks() {
				fmt.Printf("  %-20s %s\t%s\n", t.Name, t.Schedule, t.Description)
			}
			return nil
		},
	}
}

func contextCmd() *cobra.Command {
	var repoPath string
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Output squad context JSON for Claude Code SessionStart hook",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repoPath == "" {
				var err error
				repoPath, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			// Resolve symlinks for path matching
			if resolved, err := filepath.EvalSymlinks(repoPath); err == nil {
				repoPath = resolved
			}
			repoPath = filepath.Clean(repoPath)

			database, err := db.Open()
			if err != nil {
				return err
			}
			defer database.Close()

			return printSquadContext(database, repoPath)
		},
	}
	cmd.Flags().StringVar(&repoPath, "repo", "", "Repository path (defaults to cwd)")
	return cmd
}

func printSquadContext(database *sql.DB, repoPath string) error {
	var squadName, alias string
	err := database.QueryRow(
		`SELECT squad, squad_alias FROM repos WHERE path = ?`, repoPath,
	).Scan(&squadName, &alias)

	// Try resolving stored paths if exact match fails
	if err != nil {
		rows, _ := database.Query(`SELECT path, squad, squad_alias FROM repos WHERE squad != ''`)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var p, s, a string
				rows.Scan(&p, &s, &a)
				resolved, resolveErr := filepath.EvalSymlinks(p)
				if resolveErr == nil {
					resolved = filepath.Clean(resolved)
				}
				if resolved == repoPath || filepath.Clean(p) == repoPath {
					squadName, alias = s, a
					break
				}
			}
		}
	}

	type hookOutput struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}

	var out hookOutput
	out.HookSpecificOutput.HookEventName = "SessionStart"

	if squadName != "" {
		// Query other squad members
		rows, err := database.Query(
			`SELECT squad_alias, path FROM repos WHERE squad = ? AND path != ? ORDER BY squad_alias`,
			squadName, repoPath,
		)
		if err == nil {
			defer rows.Close()
			var members []string
			for rows.Next() {
				var mAlias, mPath string
				rows.Scan(&mAlias, &mPath)
				members = append(members, fmt.Sprintf("%s (%s)", mAlias, mPath))
			}

			ctx := fmt.Sprintf("You are in squad '%s' as '%s'.", squadName, alias)
			if len(members) > 0 {
				ctx += fmt.Sprintf(" Squad members: %s.", strings.Join(members, ", "))
			}
			ctx += " Use /enlist to request work from squad members."
			out.HookSpecificOutput.AdditionalContext = ctx
		}
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set up Claude Code integration (hooks and commands)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			// Install /enlist command
			cmdDir := filepath.Join(home, ".claude", "commands")
			os.MkdirAll(cmdDir, 0o755)

			enlistPath := filepath.Join(cmdDir, "enlist.md")
			if err := installEnlistCommand(enlistPath); err != nil {
				return fmt.Errorf("installing enlist command: %w", err)
			}
			fmt.Printf("cmdr: installed %s\n", enlistPath)

			// Merge SessionStart hook into settings.local.json
			settingsPath := filepath.Join(home, ".claude", "settings.local.json")
			if err := mergeSessionStartHook(settingsPath); err != nil {
				return fmt.Errorf("merging hook: %w", err)
			}
			fmt.Printf("cmdr: configured SessionStart hook in %s\n", settingsPath)

			return nil
		},
	}
}

func installEnlistCommand(path string) error {
	content := `Enlist a squad member to help with cross-repo work.

You are part of a squad — a group of repos managed by cmdr that can collaborate on cross-repo work.

## When to use

When your current task requires changes in another repository that is part of your squad. For example:
- You need a new API endpoint in a sibling service
- You need a shared type exported from a common library
- You need a config change in an infrastructure repo

## How to enlist

Write a request file so cmdr can dispatch work to the target repo:

1. Create the directory if needed: ` + "`mkdir -p ~/.cmdr/squads/{squad-name}/requests`" + `
2. Write the request as JSON:

` + "```json" + `
{
  "from": "{your-squad-alias}",
  "to": "{target-squad-alias}",
  "summary": "Brief description of what you need",
  "details": "Full specification — be precise about interfaces, types, behavior",
  "intent": "bug-fix|refactor|new-feature"
}
` + "```" + `

Save to: ` + "`~/.cmdr/squads/{squad-name}/requests/{your-alias}-{timestamp}.json`" + `

3. Continue working on parts of your task that don't depend on the enlisted work.
4. Cmdr will pick up the request, launch a Claude session in the target repo, and notify you when complete.

## Checking results

Reports from completed enlistments appear in ` + "`~/.cmdr/squads/{squad-name}/reports/`" + `.
Your SessionStart context will also notify you of completed enlistments.
`
	return os.WriteFile(path, []byte(content), 0o644)
}

func mergeSessionStartHook(path string) error {
	hookCmd := `cmdr context --repo "${CLAUDE_PROJECT_DIR:-$PWD}"`

	// Read existing settings if present
	var settings map[string]any
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	// Ensure hooks.SessionStart array exists and contains our hook
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	sessionStart, _ := hooks["SessionStart"].([]any)

	// Check if our hook already exists
	for _, h := range sessionStart {
		if hMap, ok := h.(map[string]any); ok {
			if cmd, _ := hMap["command"].(string); cmd == hookCmd {
				return nil // already configured
			}
		}
	}

	sessionStart = append(sessionStart, map[string]any{
		"type":    "command",
		"command": hookCmd,
	})
	hooks["SessionStart"] = sessionStart
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}
