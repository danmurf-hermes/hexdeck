# hexdeck

A kanban board stored in git, built for AI agents. Tickets, columns, comments, and a progress timeline — all plain files in the repo. Agents and humans read and write the same files; the board is always a projection of them. No database, no server, no lock-in.

**Status: in build.** Phase 1 (core library) is done: op schema, fold, and all three renders. Phase 2 (the CLI) is next. Nothing is usable yet.

## How it works

- Every change to the board is an **op** — one JSON file per event in `.kanban/ops/`.
- Ops are never edited or deleted. Corrections are new ops.
- The board is rebuilt from the ops every time. Same ops, same board, always.
- Ops sort by `(seq, opId)`, so concurrent writers never conflict.

## What exists so far

- The op schema: types, JSON parse, validation, deterministic sort.
- The fold: apply ops in order to build the board state.
- The renders: `board.md` (human-readable), `board.json` (machine view), and `board.svg` (the board image) from the board state.

## What comes next

- The CLI.

## Docs

- `docs/BUILD-SPEC.md` — the full spec.
- `docs/architecture.md` — how the code is built.
- `PROGRESS.md` — the build tracker.

## Development

```
go test ./...   # run the tests
```

Go 1.26+, stdlib only.
