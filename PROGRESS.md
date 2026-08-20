# hexdeck — Build Progress

> Git-native kanban for AI agents. Full spec: `docs/BUILD-SPEC.md`.
> Stack: Go (decided Aug 20 2026). Conventions: TDD, conventional commits, tests green before commit, simplified technical English in docs.
> Worker: background cron job — one chunk per run, PRs into danmurf/hexdeck, feedback via PR comments.

## Phase status

| # | Phase | Status | Commit | Notes |
|---|---|---|---|---|
| 0 | Decisions (stack, board dir, columns, claims) | ✅ done | — | Decided Aug 20 2026: Go, `.kanban/`, 4 columns, claims yes. Dogfood target: hexdeck itself |
| 1 | Core library (ops, projection, renders) | ✅ done | `6113c2d` | All four chunks done |
| 2 | CLI (all commands, git staging) | ✅ done | `86601f6` | All commands + E2E tests in temp repos. Library grew: write path, ticket prefix, claim timestamp |
| 3 | Concurrency hardening (merge matrix, claims) | ✅ done | `9a6a3e2` | 18-scenario merge matrix: zero conflicts, identical projections. Claim race: first claim by (seq, opId) wins, second warns. Claim expiry: stale claims marked, shown in renders, pickable |
| 3.5 | CI pipeline (GitHub Actions: lint, test, build, render --check, README badges) | 🔨 in progress | `556151a` | Chunk 1 done: `ci.yml` — lint (gofmt + vet), test (`-race`), build, on every push and PR. Chunks 2 (`render --check` in CI) and 3 (README badges) pending |
| 4 | Dogfood on a real project | ⏳ pending | — | Target decided: hexdeck itself. Migrate PROGRESS.md into the board once the CLI works |
| 5 | V1.1 (web view, MCP, snapshots) | ⏳ pending | — | Only if V1 earns it |

## Phase 1 chunks (Go, module github.com/danmurf/hexdeck)

| Chunk | Work | Status |
|---|---|---|
| 1.1 | Op schema: types, JSON parse, validation, sort by (seq, opId). Golden tests. | ✅ done — `ba57159` |
| 1.2 | Fold: apply ops → BoardState. Golden tests: every op type, seq collisions, duplicate ticket ids, unparseable ops. | ✅ done — `c63cc87` |
| 1.3 | Render board.md + board.json. Golden tests. | ✅ done — `caf3516` |
| 1.4 | Render board.svg — deterministic, byte-for-byte golden test. | ✅ done — `6113c2d` |

## Phase 3.5 chunks (CI pipeline)

| Chunk | Work | Status |
|---|---|---|
| 3.5.1 | `ci.yml`: gofmt check, `go vet`, `go test -race ./...`, `go build`. Every push and PR. | ✅ done — `556151a` |
| 3.5.2 | Wire `render --check` into the workflow — the CI-honesty job. | ✅ done — `d113e63` |
| 3.5.3 | README badges: CI passing + code coverage. | ⏳ pending |

## How to pick up work

1. Read `docs/BUILD-SPEC.md` — the format (§3), the projection (§4), the CLI (§5), the build plan (§8).
2. Read this file — the phase table shows what is done and what is next. Only one phase is IN PROGRESS at a time; pick the first ⏳ pending one.
3. Do the phase. End state for EVERY phase: tests green, lint clean, commit with the phase's commit message, push to the fork, open a PR.
4. Update this table (status → ✅ done, commit hash, notes) and commit it as `chore: update progress` before finishing.

## Current state

- Design complete: build spec v3.0 in `docs/BUILD-SPEC.md` (op-file event log, deterministic projection, three board views, domain glossary, quality bar).
- Phase 0 decisions made: Go, `.kanban/`, 4 columns, claims in V1.
- Chunk 1.1 done (`ba57159`): op schema, parse, validation, deterministic sort, golden tests.
- Chunk 1.2 done (`c63cc87`): the fold — apply ops in order to build the board state. Golden tests for every op type, seq collisions, duplicate ticket ids, missing tickets, and unparseable ops.
- Chunk 1.3 done (`caf3516`): the renders — `board.md` (human-readable) and `board.json` (machine view) from the board state. Deterministic: the `Updated:` line uses the newest op ts, never the wall clock. Golden tests for both renders over every fixture board.
- Chunk 1.4 done (`6113c2d`): the board image — `board.svg` from the board state. Deterministic by construction: fixed layout and palette, no external fonts, no random ids, XML-escaped text. The canvas grows with the board (width with the column count, height with the longest column). Golden tests over every fixture board, byte for byte, plus determinism, well-formedness, and escaping tests. **Phase 1 complete.** Phase 2 (the CLI) is next.
- Phase 2 done: the CLI — all commands (`init`, `create`, `move`, `comment`, `show`, `log`, `pick`, `release`, `render`), git staging, `--commit`, pull before append. E2E tests in temp git repos cover the full command matrix. The library grew the write path (`write.go`: `InitBoard`, `AppendOp`, `NextTicketID`, `RenderAll`, `RenderCheck`), the configurable ticket prefix (spec change from upstream, Aug 20), and the claim timestamp. Phase 3 (concurrency hardening) is next.
- Phase 3 done (`9a6a3e2`): concurrency hardening. The merge matrix (`merge_test.go`) runs 18 scenarios — two writers in two git clones append ops at the same time, then merge. Every scenario merges with zero conflicts and identical projections on both sides. The claim-race rule: two claims on the same ticket, the first by (seq, opId) wins, the second renders a warning. Claim expiry: a claim older than the claim timeout is marked stale in the projection, shown as "(stale claim)" in board.md and "(stale)" in the SVG badge, and `pick` takes it. The projection grew `projectAt` (explicit clock) so staleness is testable; golden tests use a fixed clock. Phase 3.5 (CI pipeline) is next.
- CI added to the plan (Aug 20): Phase 3.5 — GitHub Actions pipeline (gofmt, vet, tests, build, `render --check`) + README badges for CI passing and coverage. It runs after concurrency hardening, before dogfood.
- Phase 3.5 chunk 1 done: `ci.yml` — three jobs (lint, test, build) on every push and PR. Lint runs `gofmt -l .` (must print nothing) and `go vet ./...`. Test runs `go test -race ./...`. Build runs `go build ./...`. All use the Go version from `go.mod`. Verified locally: gofmt clean, vet clean, race tests green. Chunks 2 (`render --check` in CI) and 3 (README badges) are next.
- Phase 3.5 chunk 2 done: `render --check` in CI — a fourth job builds the CLI and runs `hexdeck render --check --dir docs/demo`. The demo board is the repo's own dogfood: its committed projections must match its ops, or the job fails. To make that possible: `RenderCheck` now covers `board.svg` too (when it exists), `--dir` accepts a bare board dir (config.json + ops/ directly, no `.kanban/`), the demo board gained committed `board.md`/`board.json`, and its claim timeout is ten years so the projections never change with the wall clock. Verified locally: the exact CI commands pass on the committed board and fail on a hand-edited one. Chunk 3 (README badges) is next.
