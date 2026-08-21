# Board — hexdeck
Updated: 2026-08-21T17:59:57Z · 4 backlog · 0 todo · 10 done

## backlog
- T-11 Default columns: backlog → todo → done
  The default flow: plan lots of work in backlog, bring items into todo when ready to pick up, move to done when finished. 'in-progress' becomes opt-in for work that spans multiple PRs. Update InitBoard defaults, the primer, the demo board, docs, and the BUILD-SPEC decision.
- T-12 Comments live on the ticket, not the board view
  The board shows ticket id and title only. Comments are for the ticket view: hexdeck show <ticket> and the web ticket detail. Remove comment counts and inline comments from board.md; keep them in ticketText.
- T-13 Ticket relationships: blocks, related-to
  Agents need to know what can run in parallel and what must come first. Add the ability to link tickets: A blocks B, A relates to B. Rendered on the ticket view and considered by pick.
- T-14 Labels on tickets
  A small set of labels per ticket (e.g. feature, bug, docs, infra), shown on the board card and filterable, to help agents scan and group work.

## todo

## done
- T-1 Migrate the build tracker into the board · 1 comment
  PROGRESS.md's phase table becomes tickets. The board is the tracker.
  - 2026-08-20T22:20:22Z hermes: Phases 1-3.5 complete. The board replaces the phase table.
- T-2 Run the build worker against the board — claimed by hermes · 3 comments
  The worker creates, moves, and comments on tickets as it works. Code and ops land in the same commit.
  - 2026-08-20T22:20:22Z hermes: This chunk: the worker runs against the board.
  - 2026-08-21T06:53:49Z hermes: The worker runbook (docs/worker.md) is written — the protocol this worker follows. Ops and docs land in the same commit.
  - 2026-08-21T07:15:42Z danmurf-hermes: docs/contributing.md replaced the worker runbook — the project prescribes no workflow; anyone (human or agent) reads the board, does a ticket, updates the board as they go.
- T-3 Dogfood acceptance: board.md answers where the project is up to — claimed by hermes · 3 comments
  A human reads board.md and can answer the question without opening anything else.
  - 2026-08-20T22:20:22Z hermes: The acceptance test for Phase 4.
  - 2026-08-21T06:53:49Z hermes: Back to todo: no PR has delivered the acceptance test yet. Chunk 2 delivers the worker runbook.
  - 2026-08-21T07:52:21Z hermes: board.md now renders descriptions and comments — the board answers the question on its own. The acceptance is pinned in ci_test.go.
- T-4 Cold-start test: a fresh agent uses the board in one attempt — claimed by hermes · 2 comments
  A fresh agent with zero context, given only the repo, creates a ticket, moves it, and comments correctly within one attempt.
  - 2026-08-20T22:20:22Z hermes: The acceptance test for Phase 4.
  - 2026-08-21T08:24:10Z danmurf-hermes: Cold-start test passed in one attempt: a fresh agent with zero context, given only the repo, discovered the board via AGENTS.md, created T-6, moved it, commented, and moved it to review — code and ops in one commit. Report: docs/cold-start.md. Acceptance pinned in ci_test.go.
- T-5 V1.1: web view, MCP, snapshots · 2 comments
  Only if V1 earns it. board.svg CI render + README embed, local web view, MCP server, snapshot checkpointing.
  - 2026-08-20T22:20:22Z hermes: Deferred until V1 earns it.
  - 2026-08-21T15:26:43Z danmurf-hermes: Snapshot checkpointing done — snapshot.json is a disposable local replay cache (digest-validated, gitignored, never committed; renders and CI always fold cold). TDD: snapshot_test.go pinned reuse, invalidation (new op, config change), corrupt-cache fallback, RenderCheck cold-fold honesty, gitignore. Closes the last V1.1 item — the board is now all done.
- T-6 Coverage badge is dishonest — the E2E tests run the CLI as a subprocess, which go tool cover cannot see (measures 0%). Library alone is 85.6%; the badge's 44.7% is a measurement artifact. Fix: build the CLI with -cover, run E2E with GOCOVERDIR, merge via go tool covdata so subprocess coverage counts. Target ≥80% honest. · 1 comment
  Coverage measurement fix
  - 2026-08-21T09:41:13Z danmurf-hermes: Honest measurement wired: HEXDECK_E2E_COVER builds the E2E binary with -cover -coverpkg=./..., and go test merges the subprocess coverage into the profile. Total 80.7% (was 44.7% — the CLI's tests did not count). Pinned in ci_test.go: the badge must stay at 80% or above.
- T-7 Docs overhaul: current-state documentation, not build history. Diataxis framework (the standard): tutorials, how-to guides, reference, explanation. Strip process narrative (chunk logs, phase history, cold-start report) from README and docs; document what the app IS: how it is built, how to use, how it works. README: quick start, real example, key concepts. Mermaid diagrams with correct fences so GitHub renders them. Simplified technical English throughout. T-7 · 1 comment
  Docs reassessment
  - 2026-08-21T10:28:42Z danmurf-hermes: Docs restructured into the Diataxis set: tutorial (a real session), how-to (one task, one guide), reference (commands, op types, config, rules), explanation (architecture). README rewritten: quick start, a real session with real output, key concepts, mermaid flowchart. Process narrative stripped from README and docs; components.md folded into the reference. Pinned in ci_test.go: TestDocsDiataxis, TestDocsDescribeTheApp, TestReadmeRealExample, TestMermaidFences.
- T-8 Phase 5 chunk 1: board.svg CI render + README embed · 1 comment
  CI re-renders the demo board's SVG and fails if the committed image drifted. The README embeds the image at the repo root, so GitHub shows the live board on the homepage.
  - 2026-08-21T10:59:41Z danmurf-hermes: TDD: TestBoardSVGInWorkflow and TestReadmeEmbedsBoardSVG pinned first (red), then the workflow honesty step (render --svg, cmp, git diff --exit-code), the root board.svg, and the README embed. E2E tests gained render --svg and render --check SVG coverage. Docs updated in the same commit.
- T-9 Phase 5 chunk 2: local web view · 1 comment
  hexdeck web serves the board in the browser: drag tickets between columns, click to comment, changes panel (diff, suggested message, commit).
  - 2026-08-21T12:01:45Z danmurf-hermes: TDD: web_test.go pinned first (red) — golden page, state, move, comment, changes, commit, errors, E2E over HTTP. Then web.go: the embedded page, the API, the changes panel. Docs updated in the same commit.
- T-10 Phase 5 chunk 3: MCP server · 1 comment
  hexdeck mcp serves the board as an MCP server over stdio: agents ask the board questions without the CLI. Read-only tools: board_show, board_show_ticket, board_log, board_next.
  - 2026-08-21T12:56:17Z danmurf-hermes: TDD: mcp_test.go pinned first (red) — handshake, ping, tools/list golden, four tools, error paths, E2E over stdio. Then mcp.go: the JSON-RPC session, the four read-only tools, the golden tool list. Docs updated in the same commit. Coverage back above 80% (82.6%) with new CLI error-path tests.
