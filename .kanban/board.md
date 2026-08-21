# Board — hexdeck
Updated: 2026-08-21T08:55:47Z · 2 todo · 0 in-progress · 2 review · 2 done

## todo
- T-5 V1.1: web view, MCP, snapshots · 1 comment
  Only if V1 earns it. board.svg CI render + README embed, local web view, MCP server, snapshot checkpointing.
  - 2026-08-20T22:20:22Z hermes: Deferred until V1 earns it.
- T-6 Coverage badge is dishonest — the E2E tests run the CLI as a subprocess, which go tool cover cannot see (measures 0%). Library alone is 85.6%; the badge's 44.7% is a measurement artifact. Fix: build the CLI with -cover, run E2E with GOCOVERDIR, merge via go tool covdata so subprocess coverage counts. Target ≥80% honest.
  Coverage measurement fix

## in-progress

## review
- T-3 Dogfood acceptance: board.md answers where the project is up to — claimed by hermes · 3 comments
  A human reads board.md and can answer the question without opening anything else.
  - 2026-08-20T22:20:22Z hermes: The acceptance test for Phase 4.
  - 2026-08-21T06:53:49Z hermes: Back to todo: no PR has delivered the acceptance test yet. Chunk 2 delivers the worker runbook.
  - 2026-08-21T07:52:21Z hermes: board.md now renders descriptions and comments — the board answers the question on its own. The acceptance is pinned in ci_test.go.
- T-4 Cold-start test: a fresh agent uses the board in one attempt — claimed by hermes · 2 comments
  A fresh agent with zero context, given only the repo, creates a ticket, moves it, and comments correctly within one attempt.
  - 2026-08-20T22:20:22Z hermes: The acceptance test for Phase 4.
  - 2026-08-21T08:24:10Z danmurf-hermes: Cold-start test passed in one attempt: a fresh agent with zero context, given only the repo, discovered the board via AGENTS.md, created T-6, moved it, commented, and moved it to review — code and ops in one commit. Report: docs/cold-start.md. Acceptance pinned in ci_test.go.

## done
- T-1 Migrate the build tracker into the board · 1 comment
  PROGRESS.md's phase table becomes tickets. The board is the tracker.
  - 2026-08-20T22:20:22Z hermes: Phases 1-3.5 complete. The board replaces the phase table.
- T-2 Run the build worker against the board — claimed by hermes · 3 comments
  The worker creates, moves, and comments on tickets as it works. Code and ops land in the same commit.
  - 2026-08-20T22:20:22Z hermes: This chunk: the worker runs against the board.
  - 2026-08-21T06:53:49Z hermes: The worker runbook (docs/worker.md) is written — the protocol this worker follows. Ops and docs land in the same commit.
  - 2026-08-21T07:15:42Z danmurf-hermes: docs/contributing.md replaced the worker runbook — the project prescribes no workflow; anyone (human or agent) reads the board, does a ticket, updates the board as they go.
