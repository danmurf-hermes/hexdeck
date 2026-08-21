# Board — hexdeck
Updated: 2026-08-21T08:55:47Z · 2 todo · 0 in-progress · 2 review · 2 done

## todo
- T-5 V1.1: web view, MCP, snapshots · 1 comment
- T-6 Coverage badge is dishonest — the E2E tests run the CLI as a subprocess, which go tool cover cannot see (measures 0%). Library alone is 85.6%; the badge's 44.7% is a measurement artifact. Fix: build the CLI with -cover, run E2E with GOCOVERDIR, merge via go tool covdata so subprocess coverage counts. Target ≥80% honest.

## in-progress

## review
- T-3 Dogfood acceptance: board.md answers where the project is up to — claimed by hermes · 3 comments
- T-4 Cold-start test: a fresh agent uses the board in one attempt — claimed by hermes · 2 comments

## done
- T-1 Migrate the build tracker into the board · 1 comment
- T-2 Run the build worker against the board — claimed by hermes · 3 comments
