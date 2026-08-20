# hexdeck

A kanban board stored in git, built for AI agents. Tickets, columns, comments, and a progress timeline — all plain files in the repo. Agents and humans read and write the same files; the board is always a projection of them. No database, no server, no lock-in.

**Status: in build.** Phase 1 (core library) and Phase 2 (the CLI) are done. The CLI works end to end: init a board, create and move tickets, comment, show, log, pick, release, render. Phase 3 (concurrency hardening) is next. Not yet dogfooded.

## How it works

- Every change to the board is an **op** — one JSON file per event in `.kanban/ops/`.
- Ops are never edited or deleted. Corrections are new ops.
- The board is rebuilt from the ops every time. Same ops, same board, always.
- Ops sort by `(seq, opId)`, so concurrent writers never conflict.

## Quick start

```
go install github.com/danmurf/hexdeck/cmd/hexdeck@latest
cd your-project
hexdeck init --as your-name          # create the board
hexdeck create "Fix login bug"       # new ticket
hexdeck move T-1 in-progress         # change column
hexdeck comment T-1 "on it"          # add a comment
hexdeck show                         # print the board
hexdeck log --since 2d               # what happened recently
hexdeck pick --as your-name          # claim the next todo ticket
hexdeck render --check               # CI: board files match the ops
```

Every change stages the op and the board files and prints a suggested commit message. `--commit` commits it. The board lives in `.kanban/` — read `.kanban/README.md` for the full manual.

## What exists so far

- The op schema: types, JSON parse, validation, deterministic sort.
- The fold: apply ops in order to build the board state.
- The renders: `board.md` (human-readable), `board.json` (machine view), and `board.svg` (the board image) from the board state.
- The CLI: all commands, git staging, `--commit`, pull before append.

## What comes next

- Concurrency hardening: the merge matrix, claim expiry, stale-claim rendering.
- CI pipeline, then dogfood.

## Docs

- `docs/BUILD-SPEC.md` — the full spec.
- `docs/architecture.md` — how the code is built.
- `docs/components.md` — the parts, one section each.
- `PROGRESS.md` — the build tracker.

## Development

```
go test ./...   # run the tests
```

Go 1.26+, stdlib only.
