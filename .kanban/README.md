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
hexdeck move T-12 todo                      # change column
hexdeck comment T-12 "text"                 # add a comment
hexdeck link T-12 blocks T-3                # link tickets: blocks, related
hexdeck show                                # print the board (compact)
hexdeck show T-12                           # print one ticket
hexdeck log --since 2d                      # what happened recently
hexdeck pick --as <your-name>               # claim the next todo ticket

## Writing ops by hand (if the CLI is unavailable)
Create `.kanban/ops/<seq>-<uuid>.json`:
{ "schema": 1, "opId": "<uuid>", "seq": <next number>,
  "ts": "<ISO time>", "actor": "<your name>",
  "type": "ticket.moved", "ticket": "T-12",
  "payload": { "from": "backlog", "to": "todo" } }

Op types: ticket.created, ticket.moved, ticket.updated,
comment.added, ticket.claimed, ticket.released, ticket.archived,
ticket.link.added, ticket.link.removed.

Ticket ids are <prefix>-<number>, prefix from config.json
(default T, e.g. T-12).

## Columns
backlog → todo → done   (see config.json)

New tickets start in backlog. Move a ticket to todo when it is ready
to pick up, and to done when it is finished. Add more columns in
config.json when the work needs them — in-progress is opt-in for work
that spans multiple PRs.

## Rules
- One op per file. Never modify an op after it's committed.
- Commit ops with your code (same commit) or use `hexdeck ... --commit`.
- `git pull --rebase` before appending ops.
- A ticket is done only when moved to done. No other signal counts.
- A claim older than the claim timeout is stale — `hexdeck pick`
  takes the ticket anyway. The board marks it "(stale claim)".
- snapshot.json is a local cache. Never commit it — it is gitignored.
