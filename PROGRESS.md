# hexdeck — Build Progress

> Git-native kanban for AI agents. Full spec: `docs/BUILD-SPEC.md`.
> Stack: to be decided (Phase 0). Conventions: TDD, conventional commits, tests green before commit, simplified technical English in docs.

## Phase status

| # | Phase | Status | Commit | Notes |
|---|---|---|---|---|
| 0 | Decisions (stack, columns, claims, dogfood) | ⏳ pending | — | Open questions tracked as GitHub issues |
| 1 | Core library (ops, projection, renders) | ⏳ pending | — | |
| 2 | CLI (all commands, git staging) | ⏳ pending | — | |
| 3 | Concurrency hardening (merge matrix, claims) | ⏳ pending | — | |
| 4 | Dogfood on a real project | ⏳ pending | — | |
| 5 | V1.1 (web view, MCP, snapshots) | ⏳ pending | — | Only if V1 earns it |

## How to pick up work

1. Read `docs/BUILD-SPEC.md` — the format (§3), the projection (§4), the CLI (§5), the build plan (§8).
2. Read this file — the phase table shows what is done and what is next. Only one phase is IN PROGRESS at a time; pick the first ⏳ pending one.
3. Do the phase. End state for EVERY phase: tests green, lint clean, commit with the phase's commit message, push to the fork, open a PR.
4. Update this table (status → ✅ done, commit hash, notes) and commit it as `chore: update progress` before finishing.

## Current state

- Design complete: build spec v3.0 in `docs/BUILD-SPEC.md` (op-file event log, deterministic projection, three board views, domain glossary, quality bar).
- Nothing is built yet. Phase 0 decisions are tracked as GitHub issues.
