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
| 3.5 | CI pipeline (GitHub Actions: lint, test, build, render --check, README badges) | ✅ done | `556151a` | All three chunks done: `ci.yml` (lint, test, build), `render --check` in CI, README badges (CI + coverage). Phase 4 (dogfood) is next |
| 4 | Dogfood on a real project | ✅ done | `5677647` | Target: hexdeck itself. The board lives in `.kanban/` — it is the tracker now. Chunk 1 done: board init + migration + CI check. Chunk 2 done: the worker runs against the board, contribution guide in docs/contributing.md. Chunk 3 done: the acceptance read — board.md renders descriptions and comments, the board answers the question on its own. Chunk 4 done: the cold-start test — a fresh agent with zero context, given only the repo, created a ticket, moved it, and commented correctly in one attempt. Report: docs/cold-start.md |
| 5 | V1.1 (web view, MCP, snapshots) | ⏳ in progress | `14706ba` | Chunk 1 done: board.svg CI render + README embed. CI re-renders the demo board's SVG and fails if the committed image drifted; the README embeds the image at the repo root, so GitHub shows the live board on the homepage. Chunk 2 done: the local web view — `hexdeck web` serves the board in the browser (drag to move, click to comment, changes panel with diff → suggested message → commit). Chunk 3 done: the MCP server — `hexdeck mcp` serves the board over stdio; agents ask the board questions without the CLI (board_show, board_show_ticket, board_log, board_next — all read-only). Chunk 4 (snapshots) is next |

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
| 3.5.3 | README badges: CI passing + code coverage. | ✅ done — `362f742` |

## Phase 4 chunks (dogfood on hexdeck itself)

| Chunk | Work | Status |
|---|---|---|
| 4.1 | `hexdeck init` in the hexdeck repo, migrate PROGRESS.md's phase table into tickets, CI checks the dogfood board. | ✅ done — `942757c` |
| 4.2 | The worker runs against the board: creates, moves, and comments on tickets as it works. Code and ops land in the same commit. | ✅ done — `c64f7d5` |
| 4.3 | Dogfood acceptance: a human reads `board.md` and can answer "where is the project up to" without opening anything else. | ✅ done — `86318e0` |
| 4.4 | Cold-start test: a fresh agent with zero context, given only the repo, creates a ticket, moves it, and comments correctly within one attempt. | ✅ done — `5677647` |

## Phase 5 chunks (V1.1 — only if V1 earns it)

| Chunk | Work | Status |
|---|---|---|
| 5.1 | board.svg CI render + README embed: CI re-renders the demo board's SVG and fails if the committed image drifted; the README embeds the image at the repo root. | ✅ done — `14706ba` |
| 5.2 | Local web view: drag tickets between columns, click to comment, changes panel. | ✅ done — `270c643` |
| 5.3 | MCP server: agents ask "what's the status?" without the CLI. | ✅ done — `62db9a2` |
| 5.4 | Snapshot checkpointing: replay from snapshot + delta. | ⏳ pending |

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
- Merge rule (Aug 20): a ticket in `review` is done when its PR is merged. The worker moves it to `done` as an ops-only commit straight to main — no second PR (spec §5, "The merge rule").
- Phase 3.5 chunk 1 done: `ci.yml` — three jobs (lint, test, build) on every push and PR. Lint runs `gofmt -l .` (must print nothing) and `go vet ./...`. Test runs `go test -race ./...`. Build runs `go build ./...`. All use the Go version from `go.mod`. Verified locally: gofmt clean, vet clean, race tests green. Chunks 2 (`render --check` in CI) and 3 (README badges) are next.
- Phase 3.5 chunk 2 done: `render --check` in CI — a fourth job builds the CLI and runs `hexdeck render --check --dir docs/demo`. The demo board is the repo's own dogfood: its committed projections must match its ops, or the job fails. To make that possible: `RenderCheck` now covers `board.svg` too (when it exists), `--dir` accepts a bare board dir (config.json + ops/ directly, no `.kanban/`), the demo board gained committed `board.md`/`board.json`, and its claim timeout is ten years so the projections never change with the wall clock. Verified locally: the exact CI commands pass on the committed board and fail on a hand-edited one. Chunk 3 (README badges) is next.
- Phase 4 chunk 1 done: the dogfood board. `hexdeck init` in the hexdeck repo — the board lives in `.kanban/` and is the build tracker now. The phase table migrated into tickets: T-1 (migrate the tracker), T-2 (worker runs against the board), T-3 (acceptance: board.md answers the question), T-4 (cold-start test), T-5 (V1.1). The claim timeout is ten years, so the committed projections never change with the wall clock. CI gained a second render check: `hexdeck render --check --dir .kanban` — the dogfood board must match its ops, or the build fails. The contract is tested in `ci_test.go`. Chunk 2 (the worker runs against the board) is next.
- Phase 4 chunk 2 done: the worker runs against the board. `docs/contributing.md` is the contribution guide — for humans and agents alike: how the board is used (pick a ticket, comment at milestones, move to review at the end, the same-commit rule), the quality bar, and the rules. It replaced the worker runbook (docs/worker.md) — the project does not prescribe a workflow; anyone reads the board, does a ticket, and updates the board as they go. This run exercised it: the merge rule closed T-1 (PR #21 merged), the worker picked T-2, wrote the guide with the board ops in the same commit, and moved T-2 to `review` — it waits for a human merge now. T-2 and T-3 went back to `todo` first: PR #21 named them, but it did not deliver them — a ticket is done when the PR that delivers it merges. Chunk 3 (the acceptance read) is next.
- Phase 4 chunk 3 done: the acceptance read. `board.md` now renders ticket descriptions (indented under the title) and comments (nested bullets with ts, actor, and text) — the board carries the story on its own, not just the titles. The dogfood board answers "where is the project up to" without opening anything else: the done column shows the phase history, the todo column shows what is next, and the descriptions and comments explain them. The acceptance is pinned in `ci_test.go` (`TestDogfoodBoardAnswersTheQuestion`) — CI fails if the committed board loses the story. Golden files updated; the demo board and the dogfood board re-rendered. Chunk 4 (the cold-start test) is next.
- Phase 4 chunk 4 done: the cold-start test. A fresh agent with zero context — empty memory, no skills, only the model credential — got a clean clone of the repo and one neutral task. It discovered the board itself through the repo's own files (`AGENTS.md` → `.kanban/README.md` → `docs/contributing.md`), built the CLI, created a ticket, moved it, commented, moved it to review, and committed the code and the board ops together — in one attempt. One wrinkle: `pick` claimed the wrong ticket (T-4) as a side effect; the agent noticed, released it, and moved it back — corrections as new ops, per the rules. The report is `docs/cold-start.md`; the acceptance is pinned in `ci_test.go` (`TestColdStartReport`, `TestColdStartDiscoveryChain`). **Phase 4 complete.** Phase 5 (V1.1) is next — only if V1 earns it.
- Merge rule applied (Aug 21): PR #24 merged (08:52) and delivered both T-3 and T-4 (the PR comment named T-4). Both tickets moved to `done` as ops-only commits straight to main. No second PR.
- T-6 done (honest coverage): the coverage badge counted only the library (44.7%) because the CLI's end-to-end tests run the binary as a subprocess, which `go tool cover` cannot see. Fix: `HEXDECK_E2E_COVER` makes the E2E test build the binary with `-cover -coverpkg=./...`, and the go command merges the subprocess coverage into the profile. The badge now shows 80.7%. Pinned: `TestCoverageBadgeHonest` fails below 80%, and `TestCoverageJobInWorkflow` checks the new CI command. E2E tests also gained three new cases (actor fallback, log filters, show with claim) and a path fix (tests run from any working directory).
- T-7 done (docs overhaul, `66493ba`): the docs describe the app, not the build. Diataxis set: `docs/tutorial.md` (a real session, step by step), `docs/how-to.md` (one task, one guide), `docs/reference.md` (commands, op types, config, rules), `docs/architecture.md` (explanation, with a mermaid state diagram). README rewritten: quick start, a real session with real output, key concepts, mermaid flowchart. Process narrative (chunk logs, phase history, cold-start report) stripped from README and docs; `docs/components.md` folded into the reference. Pinned in `ci_test.go`: `TestDocsDiataxis`, `TestDocsDescribeTheApp`, `TestReadmeRealExample`, `TestMermaidFences`. T-7 moved to `review` — waits for a human merge.
- Phase 5 chunk 1 done (board.svg CI render + README embed, `14706ba`): the README embeds `board.svg` at the repo root, so GitHub shows the live board on the homepage. CI keeps it honest — the render-check job re-renders the demo board's SVG, compares it to the root image with `cmp`, and fails if it drifted. Pinned in `ci_test.go`: `TestBoardSVGInWorkflow`, `TestReadmeEmbedsBoardSVG`. E2E tests gained `render --svg` and `render --check` SVG coverage. Docs updated in the same commit. T-8 moved to `review` — waits for a human merge.
- Phase 5 chunk 2 done (local web view): `hexdeck web` serves the board in the browser at 127.0.0.1:8080 — drag tickets between columns, click to comment, changes panel with the staged diff, the suggested commit message, and a Commit button (the GitHub Desktop pattern). The page is a render: one deterministic HTML file embedded in the binary, pinned by a golden test. Every write goes through the same path as the CLI (AppendOp, RenderAll, git staging, pull before append), so the web view and the CLI can never disagree. TDD: `web_test.go` pinned first (red) — golden page, state, move, comment, changes, commit, errors, E2E over HTTP. Docs updated in the same commit; the contract is pinned in `ci_test.go` (`TestWebViewDocumented`). T-9 moved to `review` — waits for a human merge.
- Phase 5 chunk 3 done (MCP server): `hexdeck mcp` serves the board as an MCP server over stdio — an agent harness starts the process and asks the board questions without the CLI. Four read-only tools: `board_show` (the markdown board), `board_show_ticket` (one ticket), `board_log` (the timeline, with the CLI's filters), `board_next` (the next todo ticket, chosen like `pick`). The session is one JSON-RPC message per line (MCP 2025-06-18); the tool list is a render, pinned by a golden test. TDD: `mcp_test.go` pinned first (red) — handshake, ping, tools/list golden, the four tools, error paths, E2E over stdio. Docs updated in the same commit; the contract is pinned in `ci_test.go` (`TestMCPDocumented`). The coverage badge dropped below 80% after the web view (77.8%) — new CLI error-path tests brought the honest total back to 82.6%, and `coverage.json` is updated. T-10 moved to `review` — waits for a human merge.
