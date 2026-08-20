# Board — how to use it

This repo tracks work in `.kanban/`. Everything is plain files in git.

## The one rule
Every change to the board is an **op** — a small JSON file appended to
`.kanban/ops/`. Never edit or delete existing ops. The board is always
rebuilt from the ops, so the ops are the truth.

## Where the project is up to
Read board.md — the committed board view. No CLI needed.

## Commands (preferred)
hexdeck create "Title" [-d "description"]   # new ticket
hexdeck move T-12 in-progress               # change column
hexdeck comment T-12 "text"                 # add a comment
hexdeck show                                # print the board (compact)
hexdeck show T-12                           # print one ticket
hexdeck log --since 2d                      # what happened recently
hexdeck pick --as <your-name>               # claim the next todo ticket

## Writing ops by hand (if the CLI is unavailable)
Create `.kanban/ops/<seq>-<uuid>.json`:
{ "schema": 1, "opId": "<uuid>", "seq": <next number>,
  "ts": "<ISO time>", "actor": "<your name>",
  "type": "ticket.moved", "ticket": "T-12",
  "payload": { "from": "todo", "to": "in-progress" } }

Op types: ticket.created, ticket.moved, ticket.updated,
comment.added, ticket.claimed, ticket.released, ticket.archived.

Ticket ids are <prefix>-<number>, prefix from config.json
(default T, e.g. T-12).

## Columns
todo → in-progress → review → done   (see config.json)

## Rules
- One op per file. Never modify an op after it's committed.
- Commit ops with your code (same commit) or use `hexdeck ... --commit`.
- `git pull --rebase` before appending ops.
- A ticket is done only when moved to done. No other signal counts.
- A claim older than the claim timeout is stale — `hexdeck pick`
  takes the ticket anyway. The board marks it "(stale claim)".
