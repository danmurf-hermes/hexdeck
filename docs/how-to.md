# How-to guides

One task, one guide. Each guide shows the commands and the result.

## Start a board in a repo

```
cd your-project
hexdeck init --as your-name
```

`init` creates `.kanban/` with the config, the first op, the rendered
board files, and a README that is the whole manual. It also appends one
line to `AGENTS.md` so agents find the board.

Options:

- `--name <board>` — the board name. Default: the repo dir name.
- `--prefix <prefix>` — the ticket id prefix. Default: `T`. Use `--prefix HDX` and tickets are `HDX-1`, `HDX-2`, and so on.

## Track work with tickets

```
hexdeck create "Fix login bug" -d "The login form rejects valid passwords."
hexdeck move T-1 in-progress
hexdeck comment T-1 "reproduced it"
hexdeck move T-1 review
```

A ticket starts in `todo`. Move it through the columns as the work
moves. Comment at milestones worth remembering — comments are part of
the ticket's history.

Every command stages the op and the board files and prints a suggested
commit message. Commit them together with your code — the commit is the
evidence. Or pass `--commit` and the CLI commits for you.

## Claim and release tickets

```
hexdeck pick --as your-name
```

`pick` claims the next `todo` ticket and moves it to `in-progress`. The
board shows the claim. A claim is a cooperative lock, not a security
boundary — it tells others who is working on the ticket.

```
hexdeck release T-2 --as your-name
```

`release` clears the claim. The ticket stays in its column.

A claim older than the claim timeout is stale. The board marks it
`(stale claim)`, and `pick` takes the ticket anyway.

## Render the board image

```
hexdeck render --svg
```

`render` rebuilds `board.md` and `board.json` from the ops. `--svg`
also rebuilds `board.svg` — the board image for the README. The image
is deterministic: same ops, same bytes, always.

## Show the board on GitHub

GitHub does not run JavaScript in READMEs, so the board image is the
way to show the board on the repo homepage.

```
hexdeck render --svg
cp .kanban/board.svg board.svg
```

Commit both files. Add the image to the README:

```markdown
![Board](board.svg)
```

CI keeps it honest:

```yaml
- name: board.svg honesty
  run: |
    go build -o hexdeck ./cmd/hexdeck
    ./hexdeck render --svg
    cmp .kanban/board.svg board.svg
    git diff --exit-code -- .kanban/board.svg
```

The image is a projection. CI re-renders it and fails if it drifted —
the board on the homepage is always the projection of the ops.

## Check the board in CI

```
hexdeck render --check
```

`render --check` re-renders the board from the ops and compares it to
the committed files. It fails if they drifted. Add it to your CI
workflow:

```yaml
- name: hexdeck render --check
  run: |
    go build -o hexdeck ./cmd/hexdeck
    ./hexdeck render --check
```

A hand-edited or stale projection fails the build. The board is always
honest.

## Write ops by hand

The CLI is the preferred way. If it is unavailable, write the op file
yourself. Create `.kanban/ops/<seq>-<uuid>.json`:

```json
{ "schema": 1, "opId": "<uuid>", "seq": <next number>,
  "ts": "<ISO time>", "actor": "<your name>",
  "type": "ticket.moved", "ticket": "T-12",
  "payload": { "from": "todo", "to": "in-progress" } }
```

One op per file. Never modify an op after it is committed. Then run
`hexdeck render` to rebuild the board files.

## Use a custom ticket prefix

```
hexdeck init --prefix HDX
```

Tickets are `HDX-1`, `HDX-2`, and so on. The prefix lives in
`.kanban/config.json` (`ticketPrefix`). Tickets sort numerically within
a column, whatever the prefix — `HDX-2` comes before `HDX-10`.
