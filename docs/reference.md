# Reference

The commands, the op types, the config, and the rules. Facts, no
stories.

## Commands

```
hexdeck init [--prefix T] [--name <board>] [--as <actor>]
hexdeck create "Title" [-d "description"] [--as <actor>] [--commit]
hexdeck move <ticket> <column> [--as <actor>] [--commit]
hexdeck comment <ticket> "text" [--as <actor>] [--commit]
hexdeck show [<ticket>] [--json]
hexdeck log [--since 2d] [--ticket <ticket>] [--actor <actor>]
hexdeck pick --as <actor> [--commit]
hexdeck release <ticket> --as <actor> [--commit]
hexdeck render [--svg] [--check]
hexdeck web [--port 8080] [--no-pull]
hexdeck mcp
```

Common flags:

- `--dir <path>` — the board dir. Default: `.kanban` in the current
  dir or a parent dir.
- `--as <actor>` — your name, stable per writer. Default:
  `git config user.name`.
- `--commit` — commit the change with the suggested message.
- `--no-pull` — skip `git pull --rebase` before appending.

### init

Creates `.kanban/` in the repo: the config, the first op
(`board.created`), the rendered board files, and a README that is the
whole manual. Appends one line to `AGENTS.md` so agents find the board.
Fails if the board already exists.

### create

Creates a ticket in `todo`. Prints the new id. `-d` sets the
description.

### move

Moves a ticket to a column. The columns come from the config; the
defaults are `todo`, `in-progress`, `review`, `done`.

### comment

Adds a comment to a ticket. Comments are part of the ticket's history.

### show

Prints the board as markdown. With a ticket id, prints one ticket:
fields, comments, and history. `--json` prints the machine view
(`board.json`).

### log

Prints the op timeline, newest first. Filters: `--since` (a duration
like `2d` or `3h`), `--ticket`, `--actor`.

### pick

Claims the next `todo` ticket and moves it to `in-progress`. A stale
claim does not block — `pick` takes the ticket anyway.

### release

Clears a claim. The ticket stays in its column.

### render

Rebuilds `board.md` and `board.json` from the ops. `--svg` also
rebuilds `board.svg`. `--check` re-renders and compares to the
committed files — it fails if they drifted. `board.svg` is checked
too, once it exists.

### web

Serves the local web view at `http://127.0.0.1:8080` (change the port
with `--port`). The page shows the board; drag a ticket to move it,
type in the box on a ticket to comment. Every change is an op, staged
in git, and listed in the changes panel with the staged diff and the
suggested commit message. Edit the message and press Commit — the
changes land in one commit. The web view writes through the same path
as the CLI, so the two can never disagree about the board.

### mcp

Serves the board as an MCP server over stdio. An MCP client (an agent
harness) starts `hexdeck mcp` and asks the board questions without the
CLI. The server speaks the MCP protocol (version 2025-06-18) and
exposes four tools:

- `board_show` — the whole board as markdown.
- `board_show_ticket` — one ticket. Argument: `ticket`.
- `board_log` — the op timeline. Optional arguments: `ticket`,
  `actor`, `since` (a duration like `2d`).
- `board_next` — the next todo ticket to pick.

The server is read-only: every tool answers from the projection, and
nothing writes to the board. Stdout carries the protocol; the board
dir is printed to stderr.

## Op types

One op = one JSON file in `.kanban/ops/`. Fields: `schema`, `opId`,
`seq`, `ts`, `actor`, `type`, `ticket`, `payload`.

| Type | Payload | Effect |
|---|---|---|
| `board.created` | `{"name": "..."}` | Sets the board name. |
| `ticket.created` | `{"title": "...", "description": "..."}` | Adds a ticket in the first column. |
| `ticket.moved` | `{"from": "...", "to": "..."}` | Changes the ticket's column. |
| `ticket.updated` | `{"title": "...", "description": "..."}` | Merges title and description changes. |
| `comment.added` | `{"text": "..."}` | Appends a comment. |
| `ticket.claimed` | `{"by": "..."}` | Sets the claim. |
| `ticket.released` | `{"by": "..."}` | Clears the claim. |
| `ticket.archived` | `{}` | Hides the ticket from the default board. |

Ops sort by `(seq, opId)` — never by file order, never by timestamp.
Two writers can produce the same seq; the opId breaks the tie.

## Config

`.kanban/config.json`:

```json
{
  "schema": 1,
  "board": "demo",
  "columns": ["todo", "in-progress", "review", "done"],
  "ticketPrefix": "T",
  "claimTimeout": "4h",
  "autoPush": false
}
```

- `board` — the board name.
- `columns` — the columns, in order. Default: `todo`, `in-progress`,
  `review`, `done`.
- `ticketPrefix` — the ticket id prefix. Default: `T`.
- `claimTimeout` — a claim older than this is stale. Default: `4h`.
- `autoPush` — reserved for V1.1. Always `false` in V1.

A missing config is fine — the defaults apply. A broken config is an
error.

## Board files

- `.kanban/ops/` — the op log. The source of truth. One JSON file per
  event, named `%016d-seq-<opId>.json` so lexicographic order equals
  numeric order.
- `.kanban/board.md` — the human-readable view. Committed, diffable in
  PRs.
- `.kanban/board.json` — the machine view. The full state, for UIs and
  agents.
- `.kanban/board.svg` — the board image. Opt-in via `render --svg`.
- `.kanban/README.md` — the manual, written at `init`.
- `.kanban/config.json` — the config.

The board files are projections. They are disposable — every view can
be regenerated from the ops alone.

## Rules

- One op per file. Never edit or delete an op after it is committed.
  Corrections are new ops.
- Commit ops with your code, in the same commit. The commit is the
  evidence.
- `git pull --rebase` before appending ops.
- A ticket is done only when moved to `done`. No other signal counts.
- A claim older than the claim timeout is stale. The board marks it
  `(stale claim)`, and `pick` takes the ticket anyway.
- Two claims on the same ticket: the first claim by `(seq, opId)`
  wins, the second renders a warning.
- Ops about a ticket that was never created are skipped with a
  warning. The board must always build.
- Unparseable op files are skipped with a warning, never fatal.
