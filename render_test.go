package hexdeck

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRenderMarkdownGolden renders board.md for every fixture board and
// compares it to a golden file, byte for byte.
func TestRenderMarkdownGolden(t *testing.T) {
	cases := []string{"basic", "seq-collision", "unparseable", "duplicate-ticket", "missing-ticket", "render"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "ops", name)
			state, err := projectAt(dir, fixtureNow)
			if err != nil {
				t.Fatalf("projectAt: %v", err)
			}
			got := RenderMarkdown(state)
			golden := filepath.Join("testdata", "golden", "board-"+name+".md")
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}

// TestRenderJSONGolden renders board.json for every fixture board and
// compares it to a golden file, byte for byte.
func TestRenderJSONGolden(t *testing.T) {
	cases := []string{"basic", "seq-collision", "unparseable", "duplicate-ticket", "missing-ticket", "render"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "ops", name)
			state, err := projectAt(dir, fixtureNow)
			if err != nil {
				t.Fatalf("projectAt: %v", err)
			}
			got, err := RenderJSON(state)
			if err != nil {
				t.Fatalf("RenderJSON: %v", err)
			}
			golden := filepath.Join("testdata", "golden", "board-"+name+".json")
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}

// TestRenderMarkdownDeterministic renders the same state twice and checks
// the bytes are identical. The render must never depend on map order.
func TestRenderMarkdownDeterministic(t *testing.T) {
	state, err := projectAt(filepath.Join("testdata", "ops", "render"), fixtureNow)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	first := RenderMarkdown(state)
	for i := 0; i < 20; i++ {
		if got := RenderMarkdown(state); !bytes.Equal(got, first) {
			t.Fatalf("render %d differs from the first render", i)
		}
	}
}

// TestRenderMarkdownStaleClaim checks the stale-claim display: a stale
// claim renders "(stale claim)" after the claim, a fresh claim does not.
func TestRenderMarkdownStaleClaim(t *testing.T) {
	claimedAt := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	state := BoardState{
		Name:    "stale",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "stale one", Status: "todo", ClaimedBy: "claude-a", ClaimedAt: &claimedAt, ClaimStale: true, Comments: []Comment{}},
			"T-2": {ID: "T-2", Title: "fresh one", Status: "todo", ClaimedBy: "codex-1", ClaimedAt: &claimedAt, Comments: []Comment{}},
		},
	}
	md := string(RenderMarkdown(state))
	if !strings.Contains(md, "T-1 stale one — claimed by claude-a (stale claim)") {
		t.Errorf("board.md does not mark the stale claim:\n%s", md)
	}
	if !strings.Contains(md, "T-2 fresh one — claimed by codex-1\n") {
		t.Errorf("board.md does not render the fresh claim plainly:\n%s", md)
	}
}

// TestRenderJSONDeterministic renders the same state twice and checks the
// bytes are identical. The render must never depend on map order.
func TestRenderJSONDeterministic(t *testing.T) {
	state, err := projectAt(filepath.Join("testdata", "ops", "render"), fixtureNow)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	first, err := RenderJSON(state)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := RenderJSON(state)
		if err != nil {
			t.Fatalf("RenderJSON: %v", err)
		}
		if !bytes.Equal(got, first) {
			t.Fatalf("render %d differs from the first render", i)
		}
	}
}
