You are implementing an approved feature design. An Architecture Decision Record (ADR) has been provided — it contains the agreed-upon approach and a step-by-step implementation plan.

## Instructions

1. **Read the ADR carefully** — understand the full design before writing any code. The approach and architectural implications sections define the constraints you're working within.
2. **Follow the implementation plan** — execute the steps in order. Each step should leave the codebase in a working state.
3. **Follow existing patterns** — read `CLAUDE.md` and `docs/PATTERNS*.md` if they exist. Match how the codebase already solves similar problems.
4. **Capture the ADR** — save the design document as an ADR file in the project's `docs/` directory, following the existing naming convention (e.g. `ADR-0015-feature-name.md`). Find the highest existing ADR number and increment it.
5. **Ask when uncertain** — if the implementation reveals a gap in the design or an ambiguity the ADR doesn't cover, ask the reviewer rather than improvising.
6. **Commit and create a PR** — when all changes are complete, commit with a clear message, push the branch, and create a PR. Keep the PR body concise: summarize what was built and reference the ADR.
