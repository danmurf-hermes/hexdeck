# The build worker

This file is the runbook for the agent that builds hexdeck. It runs as a
background job, one chunk per run. It talks to humans only through
GitHub pull requests. It tracks its own work in the dogfood board at
`.kanban/`.

## The loop (every run)

1. **Sync.** `git fetch upstream && git fetch origin && git checkout
   main && git merge --ff-only upstream/main && git push origin main`.
   If the merge fails, stop and report the conflict in a PR comment.
2. **Close merged tickets.** Check the merged PRs. If a PR names a
   ticket in its title or description and that ticket sits in `review`
   or `in-progress`, move it to `done` with
   `hexdeck move <ticket> done --commit --no-pull`. This is an ops-only
   commit straight to main. A ticket is done when its PR merges. Never
   open a PR whose only purpose is closing a ticket.
3. **Check for feedback.** Look at the comments on the open PRs. Fix
   valid issues, answer questions, reply to vetoes, and merge on
   approval. Say why, politely and briefly, when a comment is not
   valid. Do not change code for a non-issue.
4. **Do one chunk.** Read `PROGRESS.md` and `docs/BUILD-SPEC.md`. Take
   the first pending chunk. Do only that chunk. TDD: failing tests
   first, then the minimal implementation. Go: idiomatic, stdlib-first,
   gofmt clean, no unnecessary dependencies. Golden tests for the
   projection and the renders, byte for byte for `board.svg`. End
   state: `go test ./...` green, `gofmt -l .` empty, `go vet ./...`
   clean. Commit with a conventional message (`feat:`/`test:`/`docs:`/
   `chore:`), push to origin main. Update `PROGRESS.md` and commit it
   as `chore: update progress`.
5. **Documentation — every chunk, same commit.** Update
   `docs/architecture.md` in the same commit as the code it describes.
   Keep `docs/components.md` current. Update `README.md` when the
   status or usage changes. Simplified technical English only. No
   undocumented features, ever.
6. **Open a PR** into `danmurf/hexdeck` (base main, head
   `danmurf-hermes:main`) with a one-screen description in simplified
   technical English: what the chunk does, what to check, test results.
   If a PR is already open and unmerged, push to the same branch
   instead — the existing PR picks up the new commits. Do not open a
   second PR.
7. **Stop.** One chunk per run. Do not start the next chunk. Do not
   merge your own PRs unless a human approved in a comment.

## How the board is used

- **Before a chunk:** `hexdeck pick --as <actor>` claims and moves the
  next todo ticket to `in-progress`. The op lands in the same commit
  as the chunk's first commit — the same-commit rule.
- **During a chunk:** `hexdeck comment <ticket> "what just happened"`
  at the milestones worth remembering. Comments are part of the
  ticket's history; a fresh agent reads them to pick up context.
- **At the end of a chunk:** `hexdeck move <ticket> review` in the
  same commit as the chunk's last commit. The ticket stays in `review`
  until its PR merges — then the merge rule (step 2) moves it to
  `done`. No second PR, ever.
- **Ops and code land together.** Never commit board changes in a
  separate commit from the code they describe — except the two
  ops-only moments: the merge rule (step 2) and the
  `chore: update progress` commit.

## Hard rules

- Never leak private details: no names, no vault references, no other
  projects. The repo is public. Generic language only.
- Never force-push. Never rewrite history.
- Never merge your own PR without an approval comment from a human.
- If anything is ambiguous, ask on the PR — do not guess.
- If a chunk is already done (PR merged), move to the next one.
- Keep PR descriptions to one screen, simplified technical English.
- If the model usage guard blocks a run, stop and report on the PR.

## The board is the truth

The dogfood board is the build tracker. `PROGRESS.md` keeps the
phase-and-chunk tables for humans; the board tracks the live work.
CI checks both committed projections with `hexdeck render --check` —
a drifted board fails the build.

The board's primer (`.kanban/README.md`) is the manual for any agent
that meets the board for the first time. The cold-start test (T-4) is
the proof it works: a fresh agent with zero context, given only the
repo, creates, moves, and comments correctly in one attempt.
