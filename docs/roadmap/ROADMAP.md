# MS-CLI Public Roadmap

## Vision
Build `ms-cli` into a reliable project-state engine that combines machine-readable roadmap progress with human-readable weekly updates.

## Current Focus (Q2 2026 Target)
- Stabilize roadmap status computation and JSON contract.
- Standardize weekly public updates using Markdown front matter.
- Keep CLI status commands simple and dependable for public reporting.

## Workstreams

### 1) Core Engine
- Owner: `@owner-core` (set maintainer)
- Outcome: roadmap parsing, validation, and progress computation are stable and reusable.
- Tracking:
  - Source: `roadmap.yaml`
  - Code: `internal/project/roadmap.go`

### 2) CLI Surface
- Owner: `@owner-cli` (set maintainer)
- Outcome: `roadmap status` and `weekly status` are predictable and script-friendly.
- Tracking:
  - Command entry: `app/main.go`

### 3) Public Status Workflow
- Owner: `@owner-docs` (set maintainer)
- Outcome: weekly updates are published in consistent format and easy for public consumption.
- Tracking:
  - Template: `docs/updates/WEEKLY_TEMPLATE.md`
  - Updates folder: `docs/updates/`

## Milestone Snapshot
Current machine status is defined in `roadmap.yaml`.

As of this snapshot:
- Phase: `Foundation`
- Milestones:
  - `p1-e2e`: `in_progress`
  - `p1-3skills`: `done`

Use CLI to view computed status:

```bash
go run ./app
# then type:
roadmap status roadmap.yaml
```

## Status Legend
- `todo`: planned but not started
- `in_progress`: actively being worked
- `done`: completed
- `blocked`: waiting on dependency or decision

## Notes
- This roadmap is a living document and can change as priorities shift.
- Weekly updates provide execution narrative; roadmap provides long-horizon direction.

## History
- 2026-Q2: initial public roadmap page created.
