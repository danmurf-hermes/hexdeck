# Components

The parts of hexdeck, one section each. Written in simplified technical English. Updated as the code changes.

## The op log

The source of truth. One JSON file per event in `.kanban/ops/`. Ops are never edited or deleted — corrections are new ops. The filename is `%016d-seq-<opId>.json`, so lexicographic order equals numeric order.

Code: `op.go` (schema, parse, validation, sort).

## The projection

The board is a pure function of the ops. `Project(dir)` reads the config and the ops, sorts them by `(seq, opId)`, and folds them into a `BoardState`. Same ops, same state, always.

Code: `fold.go`.

## The renders

Three views over the same `BoardState`, all deterministic:

- `board.md` — the human-readable view. Committed, diffable in PRs.
- `board.json` — the machine view. The full state, for UIs and agents.
- `board.svg` — the board image for the README. Fixed layout and palette, no external fonts, no random ids.

Code: `render.go`, `svg.go`.

## The write path

The only code that changes a board. `InitBoard` creates one; `AppendOp` writes one op (filling seq, opId, ts, schema, and validating first); `RenderAll` rebuilds the board files; `RenderCheck` catches drift. `NextTicketID` picks the next ticket id.

Code: `write.go`.

## The CLI

The `hexdeck` binary. A thin shell over the library: it parses flags, finds the board dir, resolves the actor name, and calls the library. It owns the git behaviour — staging, suggested commit messages, `--commit`, and pull before append.

Commands: `init`, `create`, `move`, `comment`, `show`, `log`, `pick`, `release`, `render`.

Code: `cmd/hexdeck/main.go`.
