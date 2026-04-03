# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Cmdr is a personal "commander portal" — a Go backend daemon paired with a SvelteKit frontend SPA. The Go daemon handles task scheduling and execution; the SvelteKit app provides a web UI with router-based navigation. Use `bun` (not npm) for all frontend operations.

## Build & Run Commands

### Backend (Go)

```bash
# Build and install the daemon binary + restart launchd service
./scripts/install.sh

# Build only (no install)
go build -o cmdr ./cmd/cmdr

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/scheduler/

# CLI commands
go run ./cmd/cmdr list        # list registered tasks
go run ./cmd/cmdr run <task>  # execute a task immediately
go run ./cmd/cmdr status      # check daemon status
```

### Frontend (SvelteKit)

```bash
bun install --cwd web   # install frontend deps
bun run dev              # starts Go daemon (:7370) + Vite dev server, Ctrl+C stops both
bun run build            # production build → web/build/
bun run check            # svelte-check type validation
```

The dev daemon runs with `CMDR_ENV=dev`, which isolates it from the production launchd instance (separate socket, PID file, and port). Both can run simultaneously.

### Reinstalling After Changes

```bash
./scripts/install.sh   # builds binary, stops old launchd agent, installs new one
```

### macOS Service (launchd)

The daemon runs as a launchd user agent (`com.mikehu.cmdr`). `scripts/install.sh` handles building, plist installation, and service bootstrapping. Logs go to `/tmp/cmdr.out.log` and `/tmp/cmdr.err.log`.

## Environment Modes

The daemon uses `CMDR_ENV` to isolate prod vs dev:

| | Production (launchd) | Development (`CMDR_ENV=dev`) |
|---|---|---|
| TCP port | `:7369` | `:7370` |
| Socket | `/tmp/cmdr/cmdr.sock` | `/tmp/cmdr-dev/cmdr.sock` |
| PID file | `/tmp/cmdr/cmdr.pid` | `/tmp/cmdr-dev/cmdr.pid` |

## Architecture

### Backend

- **`cmd/cmdr/`** — CLI entry point using Cobra. Subcommands: `start`, `stop`, `status`, `run`, `list`.
- **`internal/daemon/`** — Daemon lifecycle with dual listeners: Unix socket for CLI IPC and TCP for the web UI. Environment-aware paths/ports via `CMDR_ENV`. API routes are registered with and without `/api` prefix.
- **`internal/scheduler/`** — Wraps `robfig/cron/v3` with seconds precision. Tasks are registered in `New()` with cron expressions.
- **`internal/tasks/`** — Individual task implementations. `Claude()` helper shells out to `claude -p` CLI. Tasks that need dependencies (e.g. `*sql.DB`) return closures. Add new tasks here and register them in the scheduler.
- **`internal/tmux/`** — Tmux integration: session listing (`list-panes -a`), session creation with worktree-aware naming (ported from `tmux-sessionizer.sh`).
- **`internal/db/`** — SQLite database (`~/.cmdr/cmdr.db`) using `modernc.org/sqlite` (pure Go). Schema migrations run on startup. Tables: `repos` (local git repos by path), `commits` (tracked commits with seen state), `task_config` (schedule/enabled overrides).
- **`internal/gitlocal/`** — Local git repo integration. Discovers repos under `CMDR_CODE_DIR` (default `~/Code`), fetches via `git fetch`, reads commits via `git log`, diffs via `difft` (falls back to `git show`). All operations use local filesystem, no GitHub API.

### Frontend

- **SvelteKit SPA** (`web/`) using `adapter-static` with `fallback: 'index.html'` for client-side routing. SSR is disabled (`ssr = false` in root layout).
- **Tailwind CSS v4** for styling — use utility classes only, no custom CSS classes.
- **`web/src/lib/api.ts`** — Typed API client for daemon communication (`/api/status`, `/api/tasks`, `/api/run`).
- **`web/src/routes/`** — File-based routing. Dashboard (`/`) and Settings (`/settings`).

### Design System

"Dark Bourbon" theme — warm, cozy dark UI. Full reference with palette, typography, component snippets, and layout patterns in [`docs/DESIGN.md`](docs/DESIGN.md). Color tokens defined in `web/src/app.css` via Tailwind v4 `@theme`.

Key rules:
- **Orbitron** (`font-display`) for headings/labels/buttons, **Space Grotesk** (`font-sans`) for body text
- Tailwind utility classes only — no custom CSS classes
- `bourbon-*` for surfaces/text, `cmd-*` (purple) for interactive elements, `run-*` (amber) for status/labels

### Adding a New Task

1. Create a function in `internal/tasks/` that returns `error`
2. Register it in `internal/scheduler/New()` with a name, description, cron schedule, and the function

### API Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/api/status` | GET | Daemon status (pid, task count) |
| `/api/tasks` | GET | List all registered tasks |
| `/api/run?task=` | GET/POST | Execute a task by name |
| `/api/tmux/sessions` | GET | List all tmux sessions with windows/panes |
| `/api/tmux/sessions/create` | POST | Create a new tmux session `{"dir": "/path"}` |
| `/api/repos` | GET | List monitored local repos |
| `/api/repos/discover` | GET | Scan `CMDR_CODE_DIR` for git repos not yet monitored |
| `/api/repos/add` | POST | Add a local repo to monitor `{"path": "/path/to/repo", ...}` |
| `/api/repos/remove` | POST | Remove a monitored repo `{"id": 1}` |
| `/api/commits` | GET | List commits (query: `repo`, `unseen`, `limit`) |
| `/api/commits/files` | GET | List files changed in a commit (query: `repo` path, `sha`) |
| `/api/commits/diff` | GET | Get diff for a commit via difft/git (query: `repo` path, `sha`) |
| `/api/commits/seen` | POST | Mark commits as seen `{"ids": [1,2,3]}` |
| `/api/sync` | POST | Trigger `git fetch` + commit sync for all monitored repos |
