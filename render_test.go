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

// TestRenderMarkdownDescription checks that a ticket's description
// renders under its title, indented, and that a ticket without a
// description renders no extra line.
func TestRenderMarkdownDescription(t *testing.T) {
	state := BoardState{
		Name:    "desc",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "with description", Description: "what it is about", Status: "todo", Comments: []Comment{}},
			"T-2": {ID: "T-2", Title: "without description", Status: "todo", Comments: []Comment{}},
		},
	}
	md := string(RenderMarkdown(state))
	if !strings.Contains(md, "- T-1 with description\n  what it is about\n") {
		t.Errorf("board.md does not render the description under the title:\n%s", md)
	}
	if strings.Contains(md, "without description\n  ") {
		t.Errorf("board.md renders a description line for a ticket without one:\n%s", md)
	}
}

// TestRenderMarkdownComments checks that comments render under the
// ticket as nested bullets with the ts, actor, and text, oldest first.
func TestRenderMarkdownComments(t *testing.T) {
	ts1 := time.Date(2026, 8, 20, 14, 4, 0, 0, time.UTC)
	ts2 := time.Date(2026, 8, 20, 14, 5, 0, 0, time.UTC)
	state := BoardState{
		Name:    "comments",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "with comments", Status: "todo", Comments: []Comment{
				{TS: ts1, Actor: "claude-a", Text: "on it"},
				{TS: ts2, Actor: "codex-1", Text: "fixed"},
			}},
			"T-2": {ID: "T-2", Title: "no comments", Status: "todo", Comments: []Comment{}},
		},
	}
	md := string(RenderMarkdown(state))
	for _, want := range []string{
		"- T-1 with comments · 2 comments\n",
		"  - 2026-08-20T14:04:00Z claude-a: on it\n",
		"  - 2026-08-20T14:05:00Z codex-1: fixed\n",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("board.md missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "no comments\n  - ") {
		t.Errorf("board.md renders comments for a ticket without any:\n%s", md)
	}
}

// TestRenderMarkdownMultiline checks that multi-line descriptions and
// comments render with every line indented.
func TestRenderMarkdownMultiline(t *testing.T) {
	ts := time.Date(2026, 8, 20, 14, 4, 0, 0, time.UTC)
	state := BoardState{
		Name:    "multi",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "multi", Description: "line one\nline two", Status: "todo", Comments: []Comment{
				{TS: ts, Actor: "claude-a", Text: "first line\nsecond line"},
			}},
		},
	}
	md := string(RenderMarkdown(state))
	for _, want := range []string{
		"  line one\n  line two\n",
		"  - 2026-08-20T14:04:00Z claude-a: first line\n    second line\n",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("board.md missing %q:\n%s", want, md)
		}
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
