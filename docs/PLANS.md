# Plans Tracker

## Active Plans

Plans live in `docs/exec-plans/active/`. Each plan file follows this format:

```markdown
# Plan: [Title]

## Goal
[What we're building]

## Steps
1. [Step 1]
2. [Step 2]
...

## Acceptance Criteria
- [ ] [Criteria 1]
- [ ] [Criteria 2]

## Status
[In Progress / Blocked / Complete]
```

When a plan is complete, move it to `docs/exec-plans/completed/`.

## Tech Debt

Track tech debt in `docs/exec-plans/tech-debt-tracker.md`.

## Current Priority

See `openspec/changes/dfcleaner-ai-disk-cleaner/tasks.md` for the master task list covering:

1. Project scaffolding (Wails + Go structure)
2. File system scanner (internal/scanner)
3. LLM provider adapter (internal/llm)
4. AI Agent analyzer (internal/analyzer)
5. Cleanup engine (internal/cleaner)
6. Frontend app shell (routes, layout, i18n)
7. Dashboard view
8. Scanner main view (Treemap + file list + chat sidebar)
9. Integration testing + packaging
