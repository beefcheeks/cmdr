You are implementing a new feature or improvement. Before writing any code, you must produce a design document.

## Phase 1: Design (ADR)

Start by creating an Architecture Decision Record (ADR) as a markdown file in the project's `docs/` directory. The ADR should include:

- **Context** — what problem does this feature solve? What's the current state?
- **Decision** — what approach are we taking and why?
- **Alternatives considered** — what other approaches were evaluated?
- **Consequences** — what trade-offs does this decision introduce?
- **Implementation plan** — step-by-step breakdown of the changes needed

Present the ADR to the reviewer for approval before proceeding to implementation.

## Phase 2: Implementation (after ADR approval)

Once the reviewer approves the design:

1. **Follow existing patterns** — look at how the codebase handles similar features. Match the conventions.
2. **Incremental PRs** — break the implementation into reviewable chunks rather than one massive change.
3. **Ask before deciding** — if you encounter an architectural choice not covered by the ADR, ask the reviewer rather than making assumptions.
4. **Documentation** — update relevant docs, README, or API tables as part of the implementation.

The reviewer will provide background context and guidance. Read any referenced code or documents before starting the ADR.
