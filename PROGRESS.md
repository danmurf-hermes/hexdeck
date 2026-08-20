# hexdeck — Build Progress

> Git-native kanban for AI agents. Full spec: `docs/BUILD-SPEC.md`.
> Stack: Go (decided Aug 20 2026). Conventions: TDD, conventional commits, tests green before commit, simplified technical English in docs.
> Worker: background cron job — one chunk per run, PRs into danmurf/hexdeck, feedback via PR comments.

## Phase status

| # | Phase | Status | Commit | Notes |
|---|---|---|---|---|
| 0 | Decisions (stack, board dir, columns, claims) | ✅ done | — | Decided Aug 20 2026: Go, `.kanban/`, 4 columns, claims yes. Dogfood target: hexdeck itself |
| 1 | Core library (ops, projection, renders) | ⏳ pending | — | Chunks below |
| 2 | CLI (all commands, git staging) | ⏳ pending | — | |
| 3 | Concurrency hardening (merge matrix, claims) | ⏳ pending | — | |
| 3.5 | CI pipeline (GitHub Actions: lint, test, build, render --check, README badges) | ⏳ pending | — | Added Aug 20 2026 — before dogfood needs the gates |
| 4 | Dogfood on a real project | ⏳ pending | — | Target decided: hexdeck itself. Migrate PROGRESS.md into the board once the CLI works |
| 5 | V1.1 (web view, MCP, snapshots) | ⏳ pending | — | Only if V1 earns it |

## Phase 1 chunks (Go, module github.com/danmurf/hexdeck)

| Chunk | Work | Status |
|---|---|---|
| 1.1 | Op schema: types, JSON parse, validation, sort by (seq, opId). Golden tests. | ✅ done — `ba57159` |
| 1.2 | Fold: apply ops → BoardState. Golden tests: every op type, seq collisions, duplicate ticket ids, unparseable ops. | ✅ done — `c63cc87` |
| 1.3 | Render board.md + board.json. Golden tests. | ⏳ pending |
| 1.4 | Render board.svg — deterministic, byte-for-byte golden test. | ⏳ pending |

## How to pick up work

1. Read `docs/BUILD-SPEC.md` — the format (§3), the projection (§4), the CLI (§5), the build plan (§8).
2. Read this file — the phase table shows what is done and what is next. Only one phase is IN PROGRESS at a time; pick the first ⏳ pending one.
3. Do the phase. End state for EVERY phase: tests green, lint clean, commit with the phase's commit message, push to the fork, open a PR.
4. Update this table (status → ✅ done, commit hash, notes) and commit it as `chore: update progress` before finishing.

## Current state

- Design complete: build spec v3.0 in `docs/BUILD-SPEC.md` (op-file event log, deterministic projection, three board views, domain glossary, quality bar).
- Phase 0 decisions made: Go, `.kanban/`, 4 columns, claims in V1.
- Chunk 1.1 done (`ba57159`): op schema, parse, validation, deterministic sort, golden tests.
- Chunk 1.2 done (`c63cc87`): the fold — apply ops in order to build the board state. Golden tests for every op type, seq collisions, duplicate ticket ids, missing tickets, and unparseable ops. Chunk 1.3 (render board.md + board.json) is next.
- CI added to the plan (Aug 20): Phase 3.5 — GitHub Actions pipeline (gofmt, vet, tests, build, `render --check`) + README badges for CI passing and coverage. It runs after concurrency hardening, before dogfood.
