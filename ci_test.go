package hexdeck

import (
	"encoding/json"
	"os"
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
		"go test -coverprofile=coverage.out ./...",
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
