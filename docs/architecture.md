# Architecture

How hexdeck is built. Written in simplified technical English. Updated as the code changes.

## The idea in one paragraph

hexdeck is a kanban board stored in git. Every change to the board is an **op** — a small JSON file in `.kanban/ops/`. The board is always rebuilt from the ops. The ops are the truth; the board is a projection.

## Packages

### Root package (`github.com/danmurf/hexdeck`)

The core library. Currently one file:

- `op.go` — the op schema. Defines the op types, parses op files from a directory, validates them, and sorts them in a deterministic order.

## The op

One op = one JSON file. Fields: `schema`, `opId`, `seq`, `ts`, `actor`, `type`, `ticket`, `payload`.

Op types: `board.created`, `ticket.created`, `ticket.moved`, `ticket.updated`, `comment.added`, `ticket.claimed`, `ticket.released`, `ticket.archived`.

## Deterministic ordering

Ops are sorted by `(seq, opId)`. Never by file order, never by timestamp. Two writers can produce the same seq; the opId breaks the tie. Same ops always sort the same way, so the board is always the same.

## What is built so far

- Chunk 1.1: op schema — types, parse, validation, deterministic sort. Golden tests for basic ops, seq collisions, and unparseable ops.

## What comes next

- Chunk 1.2: the fold — apply ops in order to build the board state.
- Chunk 1.3: render `board.md` and `board.json`.
- Chunk 1.4: render `board.svg` (deterministic, byte-for-byte).

Full plan: `docs/BUILD-SPEC.md`. Build tracker: `PROGRESS.md`.
