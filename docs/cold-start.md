# Cold-start test

The Phase 4 acceptance: a fresh agent with zero context, given only the
repo, must create a ticket, move it, and comment correctly within one
attempt.

**Result: passed — one attempt.**

## How the test ran

- A fresh agent with zero context: an empty memory, no skills, no
  project knowledge. Only the model credential.
- A clean clone of the repo. The agent got one neutral task: add a
  Contributing link to the README's Docs section, and track the work
  the way the repo says work is tracked.
- The agent had to discover the board itself. No hints beyond the repo.

## What the agent did

1. Read `AGENTS.md` — the one line that points at the board manual.
2. Read `.kanban/README.md` — the manual. Learned the commands and the
   rules from it.
3. Read `docs/contributing.md` — the contribution guide. Learned the
   workflow: pick a ticket, comment at milestones, move to review,
   board changes in the same commit as the code.
4. Built the CLI from the repo (`go build ./cmd/hexdeck`).
5. Created ticket T-6, moved it to `in-progress`, commented at the
   milestone, moved it to `review`.
6. Made the README change, ran `hexdeck render --check --dir .kanban`
   (passed), and committed the code and the board ops together.

The op log shows the full story: `ticket.created`, `ticket.moved`,
`comment.added`, `ticket.moved` — all in one commit with the code.

## One wrinkle, handled correctly

`hexdeck pick` claims the next `todo` ticket by design. The agent used
it and claimed T-4 — the wrong ticket for this task. It noticed,
released T-4, and moved it back to `todo`. Corrections as new ops, per
the rules. The board ended accurate.

## What this proves

The discovery chain works: `AGENTS.md` → `.kanban/README.md` →
`docs/contributing.md`. A fresh agent with zero context found the
board, learned it, and used it correctly in one attempt. The board
survives a context reset — that is the point of the tool.

The acceptance is pinned in `ci_test.go`: the report must exist and
record the result, and the discovery chain must be intact.
