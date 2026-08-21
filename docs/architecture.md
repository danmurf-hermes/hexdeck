# Architecture

How hexdeck is built. Written in simplified technical English. Updated as the code changes.

## The idea in one paragraph

hexdeck is a kanban board stored in git. Every change to the board is an **op** — a small JSON file in `.kanban/ops/`. The board is always rebuilt from the ops. The ops are the truth; the board is a projection.

## Packages

### Root package (`github.com/danmurf/hexdeck`)

The core library. Seven files:

- `op.go` — the op schema. Defines the op types, parses op files from a directory, validates them, and sorts them in a deterministic order.
- `fold.go` — the fold. Applies ops in order to build the board state. Also reads the board config.
- `render.go` — the renders. Turns a `BoardState` into `board.md` (markdown) and `board.json` (JSON).
- `svg.go` — the board image. Turns a `BoardState` into `board.svg`.
- `write.go` — the write path. Creates a board (`InitBoard`), appends ops (`AppendOp`), picks the next ticket id (`NextTicketID`), rebuilds the board files (`RenderAll`), and checks them for drift (`RenderCheck`).
- `op_test.go`, `fold_test.go`, `render_test.go`, `svg_test.go`, `write_test.go`, `merge_test.go` — table-driven tests plus golden files for the projection and the renders, and the merge matrix: two writers in two git clones, merged with zero conflicts.

### CLI package (`github.com/danmurf/hexdeck/cmd/hexdeck`)

The `hexdeck` binary. One file, `main.go`, plus `main_test.go` with end-to-end tests in temp git repos. The CLI is a thin shell over the library: it parses flags, resolves the board dir and the actor name, and calls the library. It never touches the board files directly.

The commands: `init`, `create`, `move`, `comment`, `show`, `log`, `pick`, `release`, `render`.

The CLI owns the git behaviour: it stages the op and the board files after every change, prints the suggested commit message, and commits when `--commit` is set. It runs `git pull --rebase` before appending (skipped with `--no-pull`, or when the repo has no upstream).

## The op

One op = one JSON file. Fields: `schema`, `opId`, `seq`, `ts`, `actor`, `type`, `ticket`, `payload`.

Op types: `board.created`, `ticket.created`, `ticket.moved`, `ticket.updated`, `comment.added`, `ticket.claimed`, `ticket.released`, `ticket.archived`.

## Deterministic ordering

Ops are sorted by `(seq, opId)`. Never by file order, never by timestamp. Two writers can produce the same seq; the opId breaks the tie. Same ops always sort the same way, so the board is always the same.

## The projection

`Project(dir)` reads a board dir and returns a `BoardState`. It is a pure function: same ops, same state, always.

Steps:

1. Read `config.json`. A missing file is fine — the default columns apply. A broken file is an error.
2. Read every op in `ops/`. Unparseable files are skipped with a warning, never fatal.
3. Sort the ops by `(seq, opId)`.
4. Fold: apply each op to the state in order.
5. Mark stale claims: a claim older than the claim timeout gets the `ClaimStale` flag.

The fold rules:

- `board.created` sets the board name.
- `ticket.created` adds a ticket in the first column. If the id already exists, the first one wins and the second renders a warning.
- `ticket.moved` changes the ticket's column.
- `ticket.updated` merges title and description changes.
- `comment.added` appends a comment.
- `ticket.claimed` sets the claim. A claim on an already-claimed ticket is a race: the first claim by `(seq, opId)` wins, the second renders a warning.
- `ticket.released` clears the claim.
- `ticket.archived` marks the ticket archived.

Ops about a ticket that was never created are skipped with a warning — visible, never fatal. The board must always build.

`BoardState` holds the board name, the columns, the ticket id prefix, the claim timeout, the tickets (a map by id), the warnings, and the newest op ts (`Updated`). `Ticket` holds the id, title, description, status, comments, created time, claim (who and when), the stale flag, and the archived flag.

## Claims

A claim is a cooperative lock, not a security boundary. Two rules make it safe under concurrency:

- **The race rule.** Two writers can claim the same ticket. The first claim by `(seq, opId)` wins; the second renders a warning. The projection resolves the race deterministically.
- **The expiry rule.** A claim older than the claim timeout is stale. The projection marks it with the `ClaimStale` flag. The claim still stands — the flag only changes the display (`(stale claim)` in `board.md`, `(stale)` in the SVG badge) and lets `pick` take the ticket.

Staleness is computed at projection time from the wall clock. The fold itself never reads the clock — only the staleness pass does, so the fold stays deterministic. Tests use `projectAt`, the same projection with an explicit clock.

## The renders

Three functions turn a `BoardState` into the committed board files. All are deterministic: same state, same bytes, always.

- `RenderMarkdown(state)` — `board.md`, the human-readable view. A header with the board name, an `Updated:` line with the newest op ts and the ticket count per column, then one section per column. Tickets sort by id within a column, numerically — T-2 comes before T-10. Archived tickets are hidden. A ticket in a column that is not in the config renders in a trailing section named after the column. A stale claim renders `(stale claim)` after the claim. The ticket's description renders under its title, indented, and its comments render as nested bullets with the ts, actor, and text — so the board carries the story on its own, not just the titles.
- `RenderJSON(state)` — `board.json`, the machine view. The full `BoardState`, indented, with a trailing newline.
- `RenderSVG(state)` — `board.svg`, the board image for the README. A header with the board name and the `Updated:` line, then one column per configured column, side by side. Each ticket is a card: the id, the title, and small badges for the claim and the comment count. Archived tickets are hidden. A ticket in a column that is not in the config renders in a trailing column named after the column. A stale claim renders `(stale)` in the claim badge.

The `Updated:` line uses the newest op ts, so rendering is deterministic — it never depends on the wall clock.

The SVG is deterministic by construction: a fixed layout and palette, no external fonts, no random ids, and text is XML-escaped. The canvas grows with the board — the width with the column count, the height with the longest column. Both are pure functions of the state.

## Ticket ids

Ticket ids are `<prefix>-<number>`. The prefix comes from `config.json` (`ticketPrefix`), set at `hexdeck init --prefix` (default `T`). `NextTicketID` returns the highest numeric suffix plus one. Tickets sort numerically within a column, whatever the prefix — `HDX-2` comes before `HDX-10`.

## The write path

`write.go` is the only code that changes a board:

- `InitBoard(dir, name, prefix, actor)` creates `.kanban/` — the primer README, the config, the `board.created` op, and the rendered board files — plus the AGENTS.md discovery hook. It fails if the board already exists.
- `AppendOp(opsDir, op)` writes one op file. It fills the seq (highest seen plus one), the opId (a random UUID), the ts (now, UTC), and the schema. The op is validated first — nothing is written for an invalid op.
- `RenderAll(boardDir, svg)` rebuilds `board.md` and `board.json` from the ops, plus `board.svg` when asked.
- `RenderCheck(boardDir)` compares the committed board files to a fresh render. It returns an error naming the first file that drifted. CI runs it. `board.svg` is checked only when it exists — it is opt-in via `render --svg`.

## The CI pipeline

`.github/workflows/ci.yml` runs five jobs on every push and every pull request:

- **Lint** — `gofmt -l .` must print nothing, then `go vet ./...` must pass.
- **Test** — `go test -race ./...`.
- **Build** — `go build ./...`.
- **Render check** — builds the CLI and runs `hexdeck render --check --dir docs/demo`, then `hexdeck render --check --dir .kanban`. Both committed boards must match their ops. A hand-edited or stale projection fails the job.
- **Coverage badge** — runs only on pushes to `main`. It runs `go test -coverprofile=coverage.out ./...`, reads the total from `go tool cover -func`, and writes `coverage.json` — a shields.io endpoint badge file. If the number changed, it commits and pushes the file. The README badge reads it through `img.shields.io/endpoint`.

The jobs use the Go version from `go.mod` (`go-version-file`), so the pipeline and the local toolchain never drift apart.

The demo board in `docs/demo/` is a real board (config, ops, and the three committed projections) that CI checks on every run. Its claim timeout is ten years, so the committed projections never change with the wall clock — the check fails only on real drift.

## The dogfood board

The repo's own board lives in `.kanban/`. It is the build tracker: the phase table from PROGRESS.md migrated into tickets, and the build worker creates, moves, and comments on tickets as it works. CI checks it with `hexdeck render --check --dir .kanban` — the same honesty gate as the demo board. Its claim timeout is ten years, for the same reason: the committed projections never change with the wall clock.

The merge rule applies to the dogfood board: a ticket in `review` is done when its PR merges. The worker moves it to `done` as an ops-only commit straight to main — no second PR.

`docs/contributing.md` is the contribution guide — for humans and agents alike. It describes how the board is used (`pick` a ticket, `comment` at milestones, `move <ticket> review` at the end, the same-commit rule), the quality bar, and the rules. A fresh agent with zero context reads it once and can run a chunk.

## The README badges

Two badges sit under the README title:

- **CI** — a shields.io GitHub Actions badge for the `ci.yml` workflow on `main`. It shows the state of the last run. No account needed.
- **Coverage** — a shields.io endpoint badge. It reads `coverage.json` from the repo. The coverage job in CI writes that file after every push to `main`, so the badge always shows the real number.

The coverage number is the honest total across both packages. The CLI's end-to-end tests run the binary as a subprocess, so Go's coverage tool cannot see them — the number is lower than the real test coverage. The badge shows the measured value, not a prettier one.

The contract is tested: `ci_test.go` reads the real workflow, badge file, and README, and fails if the coverage job, the badge schema, or the badge links are missing.

## What is built so far

- Chunk 1.1: op schema — types, parse, validation, deterministic sort. Golden tests for basic ops, seq collisions, and unparseable ops.
- Chunk 1.2: the fold — apply ops in order to build the board state. Golden tests for every op type, seq collisions, duplicate ticket ids, missing tickets, and unparseable ops.
- Chunk 1.3: the renders — `board.md` and `board.json` from a `BoardState`. Golden tests for both, over every fixture board.
- Chunk 1.4: the board image — `board.svg` from a `BoardState`. Golden tests over every fixture board, byte for byte, plus determinism, well-formedness, and escaping tests.
- Phase 2: the CLI — all commands, git staging, `--commit`, pull before append. End-to-end tests in temp git repos cover the full command matrix. The library grew the write path (`write.go`), the ticket id prefix, and the claim timestamp.
- Phase 3: concurrency hardening — the merge matrix (18 scenarios, two writers in two clones, zero conflicts, identical projections after merge), the claim-race rule (first claim by `(seq, opId)` wins, second renders a warning), and claim expiry (stale claims marked in the projection, shown in the renders, pickable by `pick`).
- Phase 3.5 chunk 1: the CI pipeline — `ci.yml` with three jobs (lint, test, build) on every push and PR.
- Phase 3.5 chunk 2: `render --check` in CI — a fourth job checks the committed demo board against its ops. `RenderCheck` now covers `board.svg` too, and `--dir` accepts a bare board dir.
- Phase 3.5 chunk 3: README badges — a CI badge (shields.io GitHub Actions) and a coverage badge (shields.io endpoint over `coverage.json`, written by a fifth CI job on pushes to `main`). The contract is tested in `ci_test.go`.
- Phase 4 chunk 1: the dogfood board — `hexdeck init` in the hexdeck repo, the phase table migrated into tickets, and a second render check in CI for `.kanban/`. The board is the build tracker now.
- Phase 4 chunk 2: the build worker runs against the board — the contribution guide (`docs/contributing.md`) replaced the worker runbook, and the worker exercised it: pick, comment, move to review, ops and docs in the same commit.
- Phase 4 chunk 3: the acceptance read — `board.md` now renders ticket descriptions and comments, so the board carries the story on its own. The dogfood board answers "where is the project up to" without opening anything else, and `ci_test.go` pins that contract.

## What comes next

- Phase 4 chunk 4: the cold-start test — a fresh agent with zero context, given only the repo, creates a ticket, moves it, and comments correctly within one attempt.

Full plan: `docs/BUILD-SPEC.md`. Build tracker: `PROGRESS.md`.
