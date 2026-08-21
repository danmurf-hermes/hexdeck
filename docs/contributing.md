# Contributing to hexdeck

Anyone can work on this project — a human or an AI agent. The board in
`.kanban/` is the state of the work. Read it first, do a ticket, and
update the board as you go. There is no prescribed workflow beyond
that.

## Where the project is up to

- `.kanban/board.md` — the live board. Read this first.
- `docs/BUILD-SPEC.md` — the full spec.

## How to work

1. Read `.kanban/README.md` — the board manual: how ops work, the
   commands, the columns.
2. Pick a ticket: `hexdeck pick --as <your-name>` claims the next
   todo ticket. Or move a specific one: `hexdeck move <ticket> todo`.
3. Do the work. Comment at milestones worth remembering:
   `hexdeck comment <ticket> "what just happened"`. Comments are part
   of the ticket's history; the next person reads them to pick up
   context.
4. When the work is done, move the ticket to `done`:
   `hexdeck move <ticket> done` — in the same commit as your last
   change.
5. Open a pull request. A ticket is done when its PR merges — the
   merge rule. Never open a PR whose only purpose is closing a ticket.

## The quality bar

- TDD: write the failing test first, then the implementation.
- Tests green, `gofmt -l .` empty, `go vet ./...` clean.
- Docs in the same commit as the code they describe. Simplified
  technical English. No undocumented features.
- Conventional commit messages (`feat:`, `test:`, `docs:`, `chore:`).
- Keep PR descriptions to one screen.

## The rules

- Never edit or delete ops. Corrections are new ops.
- Commit board changes in the same commit as the code they describe.
  The commit is the evidence.
- Never force-push. Never rewrite history.
- If anything is ambiguous, ask on the PR — do not guess.
- The board is the truth. CI checks the committed projections with
  `hexdeck render --check` — a drifted board fails the build.
