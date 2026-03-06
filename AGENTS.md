# AGENTS Collaboration Constraints

This file defines hard engineering collaboration constraints for this repository. Any task executed in this repo (including code, documentation, tests, and refactoring) must follow the rules below.

## Mandatory Rules

1. Before starting work, you must read `docs/ARCHITECTURE.md`.
2. The first work note for each task must explicitly confirm "已对照当前 architecture" (aligned with the current architecture).
3. If a requirement conflicts with the architecture, you must first point out the conflict and impacted modules, then proceed with implementation.
4. If implementation causes architecture changes, you must update `docs/ARCHITECTURE.md` in the same commit.
5. If the change is only a local fix and does not affect architecture, explicitly mark "无架构变更" (no architecture change).
6. Do not directly modify core workflow files under `agent/`, `app/`, or `tools/` before reading the architecture.

## Startup Checklist

1. [ ] Read `docs/ARCHITECTURE.md`
2. [ ] Located the target module in the architecture and identified upstream/downstream dependencies
3. [ ] Determined whether this change requires updating `docs/ARCHITECTURE.md`

## Pre-finish Checklist

1. [ ] Ensure there is no redundant code
2. [ ] Summarize change details under `docs/updates/`
3. [ ] Update `docs/ARCHITECTURE.md` if necessary
