# Tutorial — a real session

This tutorial runs hexdeck for real. You need Go 1.26+ and git. Follow
the steps in order. At the end you have a board, a ticket, a comment,
and a claim — and you understand the loop.

## 1. Install the CLI

```
go install github.com/danmurf/hexdeck/cmd/hexdeck@latest
```

## 2. Make a repo

```
mkdir demo && cd demo
git init
git config user.name you
git config user.email you@example.com
```

The actor name comes from `git config user.name`. Set it before you
use the board.

## 3. Create the board

```
$ hexdeck init --name demo
board "demo" created in .kanban
suggested commit: board: init demo
```

`init` creates `.kanban/` with the config, the first op, the rendered
board files, and a README that is the whole manual. It also appends one
line to `AGENTS.md` so agents find the board.

## 4. Create a ticket

```
$ hexdeck create "Fix login bug" -d "The login form rejects valid passwords."
T-1
suggested commit: board: create T-1
```

The ticket starts in `backlog`. The description is optional.

## 5. Move it to todo and comment

```
$ hexdeck move T-1 todo
suggested commit: board: move T-1 → todo

$ hexdeck comment T-1 "reproduced it — the bug is in the password check"
suggested commit: board: comment on T-1
```

A ticket becomes pickable when it moves to `todo`. Every command prints
a suggested commit message. Commit the op and the board files together
— the commit is the evidence.

## 6. Look at the board

```
$ hexdeck show
# Board — demo
Updated: 2026-08-21T10:23:50Z · 0 backlog · 1 todo · 0 done

## backlog

## todo
- T-1 Fix login bug
  The login form rejects valid passwords.

## done
```

The board is a projection of the ops. Each ticket shows its id, title,
and description. Comments live on the ticket view, not the board:
`hexdeck show T-1` prints them.

## 7. Look at one ticket

```
$ hexdeck show T-1
T-1 Fix login bug
status: todo
description: The login form rejects valid passwords.
created: 2026-08-21T10:23:50Z
comments:
  2026-08-21T10:23:50Z you: reproduced it — the bug is in the password check
```

## 8. Claim a ticket

```
$ hexdeck create "Add dark mode"
T-2
suggested commit: board: create T-2

$ hexdeck move T-2 todo
suggested commit: board: move T-2 → todo

$ hexdeck pick --as you
picked T-2 Add dark mode
suggested commit: board: pick T-2
```

`pick` claims the next `todo` ticket. The default flow has no
`in-progress` column — the claim alone marks the pick, and the ticket
stays in `todo`. The board shows the claim:

```
$ hexdeck show
# Board — demo
Updated: 2026-08-21T10:23:51Z · 0 backlog · 2 todo · 0 done

## backlog

## todo
- T-1 Fix login bug
  The login form rejects valid passwords.
- T-2 Add dark mode — claimed by you

## done
```

## 9. Link tickets

```
$ hexdeck create "Ship the redesign"
T-3
suggested commit: board: create T-3

$ hexdeck move T-3 todo
suggested commit: board: move T-3 → todo

$ hexdeck link T-2 blocks T-3
suggested commit: board: link T-2 blocks T-3
```

`blocks` says T-3 must wait: it is not pickable until T-2 is in
`done`. The ticket view shows the link on both sides:

```
$ hexdeck show T-3
T-3 Ship the redesign
status: todo
blocked by: T-2
created: 2026-08-21T10:23:52Z
```

`pick` skips a blocked ticket, so it takes T-1, not T-3. Remove the
link with `hexdeck link T-2 blocks T-3 --remove`.

## 9.5 Label a ticket

```sh
$ hexdeck label T-1 bug
suggested commit: board: label T-1 bug
```

A label is one word — a small set per ticket (e.g. `feature`, `bug`,
`docs`, `infra`). The board card shows the labels after the claim:

```sh
$ hexdeck show
# Board — demo
Updated: 2026-08-21T10:23:52Z · 0 backlog · 2 todo · 0 done

## backlog

## todo
- T-1 Fix login bug [bug]
- T-2 Add dark mode — claimed by you
```

Filter the board by label with `hexdeck show --label bug`. Remove a
label with `hexdeck label T-1 bug --remove`.

## 10. Release the claim

```
$ hexdeck release T-2 --as you
suggested commit: board: release T-2
```

A claim is a cooperative lock, not a security boundary. Release it when
you stop working on the ticket.

## 11. Read the log

```
$ hexdeck log
2026-08-21T10:23:50Z you board.created
2026-08-21T10:23:50Z you ticket.created T-1
2026-08-21T10:23:50Z you ticket.moved T-1
2026-08-21T10:23:50Z you comment.added T-1
2026-08-21T10:23:51Z you ticket.created T-2
2026-08-21T10:23:51Z you ticket.moved T-2
2026-08-21T10:23:51Z you ticket.claimed T-2
2026-08-21T10:23:52Z you ticket.created T-3
2026-08-21T10:23:52Z you ticket.moved T-3
2026-08-21T10:23:52Z you ticket.link.added T-3
2026-08-21T10:23:52Z you ticket.released T-2
```

The log is the whole history, newest first. Filter it with `--since`,
`--ticket`, and `--actor`.

## 12. Check the board is honest

```
$ hexdeck render --check
board files match the ops
```

`render --check` re-renders the board from the ops and compares it to
the committed files. It fails if they drifted. CI runs it on every
push.

## Where next

- [How-to guides](how-to.md) — one task, one guide.
- [Reference](reference.md) — commands, op types, config, rules.
- [Explanation](architecture.md) — how the app works.
