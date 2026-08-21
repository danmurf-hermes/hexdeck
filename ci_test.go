package hexdeck

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The CI contract. The workflow, the coverage badge file, and the README
// badges are part of the deliverable. These tests read the real files in
// the repo and check the contract holds: the coverage job exists in the
// workflow, the badge file matches the shields endpoint schema, and the
// README links both badges.

func TestCoverageJobInWorkflow(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("read ci.yml: %v", err)
	}
	workflow := string(data)
	for _, want := range []string{
		"coverage:",
		"HEXDECK_E2E_COVER",
		"go test -coverpkg=./... -count=1 -coverprofile=coverage.out ./...",
		"go tool cover -func=coverage.out",
		"coverage.json",
		"schemaVersion",
		"git push",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("ci.yml is missing %q", want)
		}
	}
}

func TestCoverageBadgeFile(t *testing.T) {
	data, err := os.ReadFile("coverage.json")
	if err != nil {
		t.Fatalf("read coverage.json: %v", err)
	}
	var badge struct {
		SchemaVersion int    `json:"schemaVersion"`
		Label         string `json:"label"`
		Message       string `json:"message"`
		Color         string `json:"color"`
	}
	if err := json.Unmarshal(data, &badge); err != nil {
		t.Fatalf("parse coverage.json: %v", err)
	}
	if badge.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", badge.SchemaVersion)
	}
	if badge.Label == "" {
		t.Error("label is empty")
	}
	if !strings.HasSuffix(badge.Message, "%") {
		t.Errorf("message %q does not end with %%", badge.Message)
	}
	if badge.Color == "" {
		t.Error("color is empty")
	}
}

// TestCoverageBadgeHonest checks the T-6 acceptance: the badge number
// counts the CLI's end-to-end tests too. The E2E tests run the CLI as a
// subprocess, which go tool cover cannot see — the honest measurement
// builds the test binaries with -cover, writes the subprocess coverage
// through GOCOVERDIR, and merges it with go tool covdata. The number is
// pinned at 80 or above: a future change that makes the badge dishonest
// again fails this test.
func TestCoverageBadgeHonest(t *testing.T) {
	data, err := os.ReadFile("coverage.json")
	if err != nil {
		t.Fatalf("read coverage.json: %v", err)
	}
	var badge struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &badge); err != nil {
		t.Fatalf("parse coverage.json: %v", err)
	}
	total := strings.TrimSuffix(badge.Message, "%")
	num, err := strconv.ParseFloat(total, 64)
	if err != nil {
		t.Fatalf("coverage.json message %q is not a number", badge.Message)
	}
	if num < 80 {
		t.Errorf("coverage.json shows %.1f%%, want 80 or above — the badge does not count the CLI's subprocess tests", num)
	}
}

// TestDogfoodBoardInWorkflow checks that the CI workflow also checks
// the repo's own dogfood board in .kanban/ — the board the worker uses
// as the build tracker. Same contract as the demo board: if it drifts,
// CI must fail.
func TestDogfoodBoardInWorkflow(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("read ci.yml: %v", err)
	}
	if !strings.Contains(string(data), "render --check --dir .kanban") {
		t.Error("ci.yml does not check the dogfood board in .kanban/")
	}
}

// TestDogfoodBoardAnswersTheQuestion checks the Phase 4 chunk 3
// acceptance: a human reads .kanban/board.md and can answer "where is
// the project up to" without opening anything else. The committed board
// must carry the story on its own: the done column with the phase
// history, the todo column with what is next, and the ticket
// descriptions and comments that explain them.
func TestDogfoodBoardAnswersTheQuestion(t *testing.T) {
	data, err := os.ReadFile(".kanban/board.md")
	if err != nil {
		t.Fatalf("read .kanban/board.md: %v", err)
	}
	board := string(data)
	for _, want := range []string{
		"## done",
		"## todo",
		"Phases 1-3.5 complete",  // the phase history, in T-1's comment
		"T-3 Dogfood acceptance", // what is next
		"T-4 Cold-start test",
		"T-5 V1.1: web view, MCP, snapshots",
		"A human reads board.md and can answer the question", // T-3's description, rendered
	} {
		if !strings.Contains(board, want) {
			t.Errorf(".kanban/board.md is missing %q — the board does not answer \"where is the project up to\"", want)
		}
	}
}

func TestReadmeBadges(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(data)
	for _, want := range []string{
		"img.shields.io/github/actions/workflow/status/danmurf/hexdeck/ci.yml",
		"img.shields.io/endpoint",
		"coverage.json",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md is missing %q", want)
		}
	}
}

// TestColdStartReport checks the Phase 4 chunk 4 acceptance: the
// cold-start test ran against a real agent, and the report records the
// result. A fresh agent with zero context, given only the repo, must
// create a ticket, move it, and comment correctly within one attempt.
// The report is the evidence.
func TestColdStartReport(t *testing.T) {
	data, err := os.ReadFile("docs/cold-start.md")
	if err != nil {
		t.Fatalf("read docs/cold-start.md: %v", err)
	}
	report := string(data)
	for _, want := range []string{
		"passed",
		"one attempt",
		"ticket.created",
		"ticket.moved",
		"comment.added",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("docs/cold-start.md is missing %q — the cold-start acceptance is not recorded", want)
		}
	}
}

// TestColdStartDiscoveryChain checks the discovery chain the cold-start
// test relies on: AGENTS.md points at the board manual, the manual
// teaches the commands, and the README has the quick start. A fresh
// agent with zero context must find the board through these files
// alone.
func TestColdStartDiscoveryChain(t *testing.T) {
	agents, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agents), ".kanban/README.md") {
		t.Error("AGENTS.md does not point at the board manual")
	}
	manual, err := os.ReadFile(".kanban/README.md")
	if err != nil {
		t.Fatalf("read .kanban/README.md: %v", err)
	}
	for _, want := range []string{"hexdeck create", "hexdeck move", "hexdeck comment"} {
		if !strings.Contains(string(manual), want) {
			t.Errorf(".kanban/README.md is missing %q — the manual does not teach the commands", want)
		}
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readme), "hexdeck init") {
		t.Error("README.md is missing the quick start")
	}
}

// TestBoardSVGInWorkflow checks the Phase 5 chunk 1 contract: CI
// renders the demo board's SVG and fails if the committed image
// drifted. The README embeds that image, so the board on the repo
// homepage is always the projection of the ops.
func TestBoardSVGInWorkflow(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("read ci.yml: %v", err)
	}
	workflow := string(data)
	for _, want := range []string{
		"render --svg --dir docs/demo",
		"git diff --exit-code -- docs/demo/board.svg",
		"cmp docs/demo/board.svg board.svg",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("ci.yml is missing %q — the board image is not checked in CI", want)
		}
	}
}

// TestReadmeEmbedsBoardSVG checks the README embeds the board image
// from the repo root, so GitHub shows the live board on the homepage.
func TestReadmeEmbedsBoardSVG(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(data)
	for _, want := range []string{
		"![Board](board.svg)",
		"board.svg",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md is missing %q — the board image is not embedded", want)
		}
	}
}

// TestWebViewDocumented checks the Phase 5 chunk 2 contract: the web
// view ships with its docs. The reference documents the command, the
// how-to shows the workflow, and the README quick start names it. A
// feature ships with its doc line in the same commit — no
// undocumented features.
func TestWebViewDocumented(t *testing.T) {
	checks := []struct {
		path string
		want []string
	}{
		{"docs/reference.md", []string{"hexdeck web", "changes panel", "suggested commit message"}},
		{"docs/how-to.md", []string{"hexdeck web", "Drag a ticket", "changes panel"}},
		{"docs/architecture.md", []string{"hexdeck web", "/api/move", "/api/comment", "/api/commit"}},
		{"README.md", []string{"hexdeck web"}},
	}
	for _, c := range checks {
		data, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("read %s: %v", c.path, err)
		}
		text := string(data)
		for _, want := range c.want {
			if !strings.Contains(text, want) {
				t.Errorf("%s is missing %q — the web view is not documented", c.path, want)
			}
		}
	}
}

// TestMCPDocumented checks the Phase 5 chunk 3 contract: the MCP
// server ships with its docs. The reference documents the command and
// the tools, the how-to shows the workflow, the architecture explains
// the protocol, and the README quick start names it. A feature ships
// with its doc line in the same commit — no undocumented features.
func TestMCPDocumented(t *testing.T) {
	checks := []struct {
		path string
		want []string
	}{
		{"docs/reference.md", []string{"hexdeck mcp", "board_show", "board_show_ticket", "board_log", "board_next", "read-only"}},
		{"docs/how-to.md", []string{"hexdeck mcp", "board_show", "board_next"}},
		{"docs/architecture.md", []string{"hexdeck mcp", "2025-06-18", "tools/list", "tools/call", "TestMCPToolsListGolden"}},
		{"README.md", []string{"hexdeck mcp"}},
	}
	for _, c := range checks {
		data, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("read %s: %v", c.path, err)
		}
		text := string(data)
		for _, want := range c.want {
			if !strings.Contains(text, want) {
				t.Errorf("%s is missing %q — the MCP server is not documented", c.path, want)
			}
		}
	}
}

// TestDocsDiataxis checks the docs follow the Diataxis structure:
// tutorials, how-to guides, reference, explanation. The README links
// all four, and the old components doc is folded into the reference.
func TestDocsDiataxis(t *testing.T) {
	for _, path := range []string{
		"docs/tutorial.md",
		"docs/how-to.md",
		"docs/reference.md",
		"docs/architecture.md",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s is missing — the docs do not follow the Diataxis structure", path)
		}
	}
	if _, err := os.Stat("docs/components.md"); err == nil {
		t.Error("docs/components.md still exists — its content belongs in docs/reference.md")
	}
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(data)
	for _, want := range []string{
		"docs/tutorial.md",
		"docs/how-to.md",
		"docs/reference.md",
		"docs/architecture.md",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md does not link %s", want)
		}
	}
}

// TestDocsDescribeTheApp checks the docs describe the app, not the
// build process. Chunk logs, phase history, and the cold-start report
// are process narrative — they do not belong in the README or the
// docs.
func TestDocsDescribeTheApp(t *testing.T) {
	for _, path := range []string{
		"README.md",
		"docs/architecture.md",
		"docs/tutorial.md",
		"docs/how-to.md",
		"docs/reference.md",
		"docs/contributing.md",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, banned := range []string{
			"chunk",
			"cold-start",
			"Phase ",
			"What is built so far",
			"PROGRESS.md",
		} {
			if strings.Contains(text, banned) {
				t.Errorf("%s contains %q — process narrative does not belong in the docs", path, banned)
			}
		}
	}
}

// TestReadmeRealExample checks the README shows a real session — real
// commands and real output, not a sketch.
func TestReadmeRealExample(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(data)
	for _, want := range []string{
		"hexdeck init --name demo",
		"board \"demo\" created in .kanban",
		"suggested commit: board: init demo",
		"hexdeck create \"Fix login bug\"",
		"hexdeck move T-1 todo",
		"hexdeck comment T-1 \"reproduced it\"",
		"## todo",
		"- T-1 Fix login bug",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md is missing %q — the real example is not real", want)
		}
	}
}

// TestMermaidFences checks the mermaid diagrams use correct fences so
// GitHub renders them: the opening fence is exactly ```mermaid, the
// block has content, and it closes with ```.
func TestMermaidFences(t *testing.T) {
	checks := []struct {
		path string
		want string // a line inside the diagram
	}{
		{"README.md", "flowchart"},
		{"docs/architecture.md", "stateDiagram"},
	}
	for _, c := range checks {
		data, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("read %s: %v", c.path, err)
		}
		text := string(data)
		open := "```mermaid\n"
		idx := strings.Index(text, open)
		if idx == -1 {
			t.Errorf("%s has no mermaid diagram with a correct fence", c.path)
			continue
		}
		rest := text[idx+len(open):]
		if !strings.Contains(rest, "```") {
			t.Errorf("%s: the mermaid block is not closed", c.path)
		}
		if !strings.Contains(rest, c.want) {
			t.Errorf("%s: the mermaid diagram does not contain %q", c.path, c.want)
		}
	}
}
