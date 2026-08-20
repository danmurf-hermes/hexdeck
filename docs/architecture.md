# Architecture

How hexdeck is built. Written in simplified technical English. Updated as the code changes.

## The idea in one paragraph

hexdeck is a kanban board stored in git. Every change to the board is an **op** — a small JSON file in `.kanban/ops/`. The board is always rebuilt from the ops. The ops are the truth; the board is a projection.

## Packages

### Root package (`github.com/danmurf/hexdeck`)

The core library. Three files:

- `op.go` — the op schema. Defines the op types, parses op files from a directory, validates them, and sorts them in a deterministic order.
- `fold.go` — the fold. Applies ops in order to build the board state. Also reads the board config.
- `op_test.go`, `fold_test.go` — table-driven tests plus golden files for the projection.

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

The fold rules:

- `board.created` sets the board name.
- `ticket.created` adds a ticket in the first column. If the id already exists, the first one wins and the second renders a warning.
- `ticket.moved` changes the ticket's column.
- `ticket.updated` merges title and description changes.
- `comment.added` appends a comment.
- `ticket.claimed` / `ticket.released` set and clear the claim.
- `ticket.archived` marks the ticket archived.

Ops about a ticket that was never created are skipped with a warning — visible, never fatal. The board must always build.

`BoardState` holds the board name, the columns, the tickets (a map by id), and the warnings. `Ticket` holds the id, title, description, status, comments, created time, claim, and archived flag.

## What is built so far

- Chunk 1.1: op schema — types, parse, validation, deterministic sort. Golden tests for basic ops, seq collisions, and unparseable ops.
- Chunk 1.2: the fold — apply ops in order to build the board state. Golden tests for every op type, seq collisions, duplicate ticket ids, missing tickets, and unparseable ops.

## What comes next

- Chunk 1.3: render `board.md` and `board.json`.
- Chunk 1.4: render `board.svg` (deterministic, byte-for-byte).

Full plan: `docs/BUILD-SPEC.md`. Build tracker: `PROGRESS.md`.
