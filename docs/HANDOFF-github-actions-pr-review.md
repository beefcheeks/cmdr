# HANDOFF: GitHub Actions PR Review Prompt Package

## Goal

Recreate Cmdr's current commit review process inside GitHub Actions for PR review.

Cmdr's review flow is not a single prompt. It is a small prompt package:

1. A **reviewer philosophy/system prompt** from `~/.cmdr/agents/review.md`
2. A **review task/user prompt template** from `internal/prompts/review.md`
3. Optional reviewer notes/annotations
4. A separate **implementation/refactoring bracket** used when turning findings into code changes

For GitHub Actions, mirror that structure instead of copying only `~/.cmdr/agents/review.md`.

## Cmdr Source of Truth

Relevant files in this repo:

- `~/.cmdr/agents/review.md`
  - User-level review override.
  - Frontmatter selects the agent/output format.
  - Body is used as the review system prompt.

- `internal/prompts/review.md`
  - Main review prompt template.
  - Injects repository metadata, commit metadata, diff text, reviewer notes, and annotations.
  - Defines the required output format for findings.

- `internal/daemon/handle_review.go`
  - Builds the review prompt from commit metadata and `gitlocal.CommitDiff()`.
  - Resolves the `review` override.
  - Runs the selected agent headlessly in the repository working directory.

- `internal/daemon/handle_tasks.go`
  - Contains the review-finding-to-implementation bracketing prompt.
  - Search for `case parentType == "review":`.

- `internal/prompts/intents/implementation.md`
  - System prompt for implementing an approved plan or review findings.

## Important Cmdr Behavior To Preserve

Cmdr review is currently **commit-centric**, not PR-centric.

For GitHub Actions, adapt it to PRs by replacing commit metadata with PR metadata and using a PR diff instead of a single commit diff.

Preserve these behaviors:

- Run the agent from the checked-out repository root.
- Give it the full PR diff.
- Tell it explicitly to inspect surrounding code, touched files, adjacent modules, and relevant `docs/PATTERNS*.md` files.
- Keep the review focused on codebase health, maintainability, architecture, boundaries, cohesion, and readability — not generic correctness unless it affects long-term health.
- Require structured findings with severity, confidence, snippet, impact, and implementation plan.
- If the change is clean, require a one-sentence clean result and no filler.

## Recommended Prompt Package Layout

For the GitHub Actions integration, prepare three prompt artifacts.

### 1. Review system prompt

Source: body of `~/.cmdr/agents/review.md`.

Do **not** include the frontmatter when passing it as a system prompt:

```md
---
agent: pi
output: markdown
---
```

Only pass the body after the closing `---`.

Purpose:

- Establishes senior-reviewer taste.
- Defines heuristic priority.
- Emphasizes boundaries, local reasoning, legibility, dependency-threading smells, abstraction fit, and DRY discipline.

### 2. PR review user prompt

Source: adapt `internal/prompts/review.md`.

Keep the same finding format, but replace the `## Commit` section with PR metadata.

Suggested PR version:

```md
You are reviewing a pull request for **codebase health**. The system prompt defines the review philosophy and strictness. Your job here is to review the supplied PR using the materials below and return structured findings.

Do not review primarily for functional correctness, security, or feature completeness unless a problem materially affects codebase health, maintainability, or architectural integrity. Treat local precedent as evidence, not automatic absolution: repeated patterns in the codebase may reflect stable convention or existing debt, and your job is to tell the difference.

## Pull Request
- Repository: {{REPO}}
- PR: #{{PR_NUMBER}}
- Title: {{PR_TITLE}}
- Author: {{PR_AUTHOR}}
- Base: {{BASE_REF}} / {{BASE_SHA}}
- Head: {{HEAD_REF}} / {{HEAD_SHA}}

## PR Description
```md
{{PR_BODY}}
```

## Diff
```diff
{{PR_DIFF}}
```

## Review Scope

Review the diff in context. Read the surrounding code in touched files and, when necessary, adjacent modules to understand whether the change belongs in that layer, file, and pattern family.

If `docs/PATTERNS*.md` files exist and are relevant to the touched code, read them and use them as project context.

Focus findings on these areas:

1. **Boundary / Architecture** — whether responsibilities remain in the correct layer and dependency flow stays coherent
2. **Cohesion / API Shape** — whether functions, objects, and module APIs stay understandable and cohesive
3. **Readability / Local Reasoning** — whether the code is easy to understand in place via clear naming, straightforward control flow, visible decisions, low cognitive overhead, and enough documentation/JSDoc for non-trivial contracts and side effects
4. **Organizational Fit** — whether code belongs in the current file/module and whether new files or abstractions are justified
5. **Consistency / Local Pattern Fit** — whether the change aligns with established patterns in nearby code and project docs, while distinguishing real convention from repeated weak precedent
6. **Side Effects / Imperative Shell** — whether side effects stay visible at the edges and pure transformation logic remains separable
7. **DRY / Abstraction Fit** — whether duplication indicates a real missing abstraction rather than harmless repetition

Only report meaningful findings. Do not narrate what the code does or summarize the change. Avoid surfacing unrelated pre-existing issues unless the diff worsens them or should clearly have aligned with nearby code. Include every materially distinct, high-signal issue you can support from the diff and surrounding context, but do not pad the review with weak findings.

## Output Format

For each finding, use:

### [N. Category] Finding Title
`file/path` · lines X–Y

**Issue:** One-sentence description of what's wrong
**Severity:** must-fix | should-fix | optional
**Confidence:** high | medium | low

```language
// the relevant code snippet from the diff or current file
// include enough surrounding context to understand the problem
// for changes, show the new code as it appears after the PR
```

**Why it matters:** How this degrades the codebase over time

**Plan:**
1. [Step]: what to do and where
2. [Step]: ...

### Code snippet guidelines

Every finding **must** include the relevant code snippet inline. The reader should be able to understand the finding without switching to an editor.

- Show the **actual code** that the finding refers to — not a paraphrase or pseudocode
- Include enough **surrounding context** to understand the code's role
- For diff-related findings, show the code **as it appears after the PR**
- If the finding is about a missing pattern or structural issue, show the code that should change
- Use the correct language identifier for syntax highlighting
- Keep snippets focused — 5–20 lines is ideal

Use N from the seven scope areas above. `N` identifies the scope area, not the priority rank. Order findings by severity first (`must-fix`, then `should-fix`, then `optional`), breaking ties by architectural impact and breadth.

The plan should be a sequence of steps that a refactoring agent can follow. Each step should point at a **location and intention**. Do not include exact implementations in the plan. Keep steps scoped to the finding; do not combine multiple findings into one plan.

Skip scope areas with no findings. Do not omit a real finding merely to keep the review short, and do not merge separate issues just to reduce the count. If the change is clean, say so in one sentence — do not pad with praise or generic observations.
```

### 3. Implementation system prompt

Source: `internal/prompts/intents/implementation.md`.

Use this when a second GitHub Actions job, bot command, or local automation attempts to apply approved review findings.

Purpose:

- Treats the review findings as an approved implementation plan.
- Tells the agent to read project conventions.
- Requires incremental scoped work.
- Prevents unrelated refactors.

## Review Finding Implementation Bracket

Cmdr does not pass review findings directly to an implementation agent. It wraps them with extra guidance from `internal/daemon/handle_tasks.go`.

Use this as the implementation user prompt when applying findings:

```md
Address the following code review findings from PR #{{PR_NUMBER}}.

## How to read these findings

- Each finding includes severity, location, issue description, and a suggested plan
- Severity is for triage and urgency, not a mandate for the size or shape of the fix
- Treat the plan as advisory, not binding; re-evaluate the cleanest fix against the repo's rules, patterns, and boundaries
- Do not assume the reviewer's proposed refactor is the only valid solution
- Prefer the smallest clean fix that resolves the issue without introducing a new boundary or cohesion problem
- If a finding contains a `> User response:` blockquote, treat it as explicit guidance — follow it
- If a finding has multiple valid approaches and no user response, pick the cleanest one
- Only ask me if there is genuine ambiguity that requires a judgment call

## Review Findings

{{REVIEW_FINDINGS}}
```

Pair that user prompt with the implementation system prompt from `internal/prompts/intents/implementation.md`.

## GitHub Actions Data To Collect

The review job should collect:

- `REPO`: `owner/repo`
- `PR_NUMBER`
- `PR_TITLE`
- `PR_BODY`
- `PR_AUTHOR`
- `BASE_REF`
- `BASE_SHA`
- `HEAD_REF`
- `HEAD_SHA`
- `PR_DIFF`

Recommended diff source:

```bash
git fetch origin "$BASE_REF" "$HEAD_REF"
git diff --no-ext-diff --unified=80 "origin/$BASE_REF...HEAD"
```

Use enough context (`--unified=80`, or similar) because the reviewer is reasoning about code structure, not just changed lines.

Also make sure checkout has enough history for merge-base diffing:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
```

## GitHub Actions Review Modes

There are two useful modes.

### Mode A: Advisory PR comment

- Run review on every PR update.
- Post the markdown findings as a PR comment.
- Do not fail the build by default.
- Optionally fail only if `must-fix` findings are present.

This most closely matches Cmdr's review artifact flow: review produces an artifact that a human can accept, edit, or discard.

### Mode B: Blocking quality gate

- Run review on every PR update.
- Fail the job if findings contain `**Severity:** must-fix`.
- Maybe warn but do not fail on `should-fix`.

Use this only after the prompt has been tuned enough to avoid noisy false positives.

## Suggested First Workflow Shape

1. Checkout PR head with full history.
2. Generate PR metadata JSON.
3. Generate PR diff.
4. Render the PR review user prompt from the template.
5. Run the agent headlessly with:
   - system prompt = `~/.cmdr/agents/review.md` body
   - user prompt = rendered PR review prompt
   - working directory = repository root
6. Save review output as an artifact.
7. Post or update a sticky PR comment.
8. Optionally parse severity and fail on `must-fix`.

## Notes On Annotations / Human Guidance

Cmdr supports reviewer notes and line annotations before launching a review. Those are injected into `internal/prompts/review.md` as:

- `## Reviewer's Note`
- `## Reviewer's Annotations`
- `## Notes to Address`

For GitHub Actions v1, this can probably be omitted.

For v2, consider supporting:

- A `/review note ...` PR comment command
- Review comments from maintainers as extra annotations
- Labels such as `review:architecture`, `review:readability`, or `review:strict`

Those can be appended to the PR review prompt in the same style as Cmdr's annotation sections.

## Practical Recommendation

Start with only the review stage.

Do not immediately auto-apply findings from CI. Cmdr intentionally keeps review and implementation separate:

- Review produces a structured artifact.
- Human can remove findings or add guidance.
- Implementation receives a bracketed prompt that says plans are advisory and fixes should be minimal/clean.

For GitHub Actions, the safest equivalent is:

1. Bot posts findings.
2. Human comments/approves which findings to address.
3. A separate manual workflow or bot command runs the implementation bracket.

## Minimum Viable Prompt Package

Prepare these files or equivalent secret/config values for the GitHub Actions system:

```text
prompts/
  review-system.md              # body copied from ~/.cmdr/agents/review.md
  pr-review-user-template.md    # adapted from internal/prompts/review.md
  implementation-system.md      # copied from internal/prompts/intents/implementation.md
  implement-review-user.md      # bracket copied/adapted from handle_tasks.go
```

That package is enough to reproduce Cmdr's current review behavior for PRs with high fidelity.
