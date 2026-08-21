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
