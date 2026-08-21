# Components

The parts of hexdeck, one section each. Written in simplified technical English. Updated as the code changes.

## The op log

The source of truth. One JSON file per event in `.kanban/ops/`. Ops are never edited or deleted — corrections are new ops. The filename is `%016d-seq-<opId>.json`, so lexicographic order equals numeric order.

Code: `op.go` (schema, parse, validation, sort).

## The projection

The board is a pure function of the ops. `Project(dir)` reads the config and the ops, sorts them by `(seq, opId)`, and folds them into a `BoardState`. Same ops, same state, always.

The fold resolves the concurrency races deterministically:

- Two claims on the same ticket: the first claim by `(seq, opId)` wins, the second renders a warning.
- A claim older than the claim timeout is marked stale. The claim still stands; the flag changes the display and lets `pick` take the ticket.

Code: `fold.go`.

## The renders

Three views over the same `BoardState`, all deterministic:

- `board.md` — the human-readable view. Committed, diffable in PRs. A stale claim renders `(stale claim)`. Ticket descriptions render under their titles, and comments render as nested bullets with the ts, actor, and text — the board carries the story on its own.
- `board.json` — the machine view. The full state, for UIs and agents.
- `board.svg` — the board image for the README. Fixed layout and palette, no external fonts, no random ids. A stale claim renders `(stale)` in the claim badge.

Code: `render.go`, `svg.go`.

## The write path

The only code that changes a board. `InitBoard` creates one; `AppendOp` writes one op (filling seq, opId, ts, schema, and validating first); `RenderAll` rebuilds the board files; `RenderCheck` catches drift. `NextTicketID` picks the next ticket id.

Code: `write.go`.

## The CLI

The `hexdeck` binary. A thin shell over the library: it parses flags, finds the board dir, resolves the actor name, and calls the library. It owns the git behaviour — staging, suggested commit messages, `--commit`, and pull before append.

Commands: `init`, `create`, `move`, `comment`, `show`, `log`, `pick`, `release`, `render`.

Code: `cmd/hexdeck/main.go`.

## The merge matrix

The concurrency proof. `merge_test.go` runs 18 scenarios: two writers in two git clones append ops at the same time, then merge. Every scenario must merge with zero conflicts, and the projection must be identical on both sides after the merge. The scenarios cover every op type, same-ticket and different-ticket writes, seq collisions, and stale checkouts.

The matrix proves the design: one op per file means concurrent appends never conflict. The committed board files can conflict, but the resolution is mechanical — re-render from the merged ops, never hand-edit.

Code: `merge_test.go`.

## The CI pipeline

The quality gates. `.github/workflows/ci.yml` runs five jobs on every push and every pull request: lint (`gofmt` + `go vet`), test (`go test -race ./...`), build (`go build ./...`), render check (`hexdeck render --check --dir docs/demo` — the committed demo board must match its ops), and coverage badge (pushes to `main` only — measures coverage and writes `coverage.json`). The jobs use the Go version from `go.mod`, so the pipeline and the local toolchain never drift apart.

Code: `.github/workflows/ci.yml`.

## The README badges

Two shields.io badges under the README title. The CI badge shows the state of the `ci.yml` workflow on `main`. The coverage badge is an endpoint badge over `coverage.json` — the coverage job in CI writes that file after every push to `main`, so the badge always shows the real measured number. The contract is tested: `ci_test.go` reads the real workflow, badge file, and README, and fails if any part is missing.

Code: `coverage.json`, `ci_test.go`.

## The demo board

A real board in `docs/demo/` — config, ops, and the three committed projections (`board.md`, `board.json`, `board.svg`). The README embeds its SVG. CI runs `render --check` on it, so the demo is the repo's own dogfood: a hand-edited or stale projection fails the build. Its claim timeout is ten years, so the projections never change with the wall clock — the check fails only on real drift.

## The dogfood board

The repo's own board in `.kanban/`. It is the build tracker: the phase table from PROGRESS.md migrated into tickets, and the build worker creates, moves, and comments on tickets as it works. CI runs `render --check` on it — the same honesty gate as the demo board. Its claim timeout is ten years, for the same reason: the committed projections never change with the wall clock.

The merge rule applies here: a ticket in `review` is done when its PR merges. The worker moves it to `done` as an ops-only commit straight to main — no second PR.

## The contribution guide

`docs/contributing.md` is the contribution guide — for humans and agents alike. It describes how the board is used (`pick` a ticket, `comment` at milestones, `move <ticket> review` at the end, the same-commit rule), the quality bar, and the rules (no force-push, no op edits, ask on the PR when ambiguous).

The guide is part of the dogfood: a fresh agent with zero context reads it once and can run a chunk against the board. The cold-start test (T-4) is the proof.

## The cold-start test

The Phase 4 acceptance, run against a real agent. A fresh agent with zero context — empty memory, no skills, only the model credential — got a clean clone of the repo and one neutral task. It had to discover the board itself, through the repo's own files: `AGENTS.md` → `.kanban/README.md` → `docs/contributing.md`.

The agent created a ticket, moved it, commented, moved it to review, and committed the code and the board ops together — in one attempt. The report is `docs/cold-start.md`; the acceptance is pinned in `ci_test.go` (the report must exist and record the result, and the discovery chain must be intact).
