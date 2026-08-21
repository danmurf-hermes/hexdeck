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

// TestRenderMarkdownOmitsComments checks that the board view carries
// only the ticket line: no comment counts, no inline comments, no
// comment actors. Comments belong to the ticket view (hexdeck show
// T-1, the web ticket detail), not to the board.
func TestRenderMarkdownOmitsComments(t *testing.T) {
	ts1 := time.Date(2026, 8, 20, 14, 4, 0, 0, time.UTC)
	ts2 := time.Date(2026, 8, 20, 14, 5, 0, 0, time.UTC)
	state := BoardState{
		Name:    "comments",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "with notes", Status: "todo", Comments: []Comment{
				{TS: ts1, Actor: "claude-a", Text: "on it"},
				{TS: ts2, Actor: "codex-1", Text: "fixed"},
			}},
			"T-2": {ID: "T-2", Title: "plain", Status: "todo", Comments: []Comment{}},
		},
	}
	md := string(RenderMarkdown(state))
	if !strings.Contains(md, "- T-1 with notes\n") || !strings.Contains(md, "- T-2 plain\n") {
		t.Errorf("board.md does not render the ticket lines:\n%s", md)
	}
	for _, banned := range []string{
		"· 2 comments",
		"· 1 comment",
		"claude-a",
		"codex-1",
		"on it",
		"fixed",
	} {
		if strings.Contains(md, banned) {
			t.Errorf("board.md carries %q — comments live on the ticket view, not the board:\n%s", banned, md)
		}
	}
}

// TestRenderMarkdownLabels checks that labels render on the ticket
// line, after the claim, in the order they were added. A ticket
// without labels renders no label text.
func TestRenderMarkdownLabels(t *testing.T) {
	state := BoardState{
		Name:    "labels",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "with labels", Status: "todo", Labels: []string{"bug", "docs"}, Comments: []Comment{}},
			"T-2": {ID: "T-2", Title: "plain", Status: "todo", Comments: []Comment{}},
		},
	}
	md := string(RenderMarkdown(state))
	if !strings.Contains(md, "- T-1 with labels [bug, docs]\n") {
		t.Errorf("board.md does not render the labels on the ticket line:\n%s", md)
	}
	if !strings.Contains(md, "- T-2 plain\n") {
		t.Errorf("board.md does not render the plain ticket:\n%s", md)
	}
}

// TestRenderMarkdownLabelsWithClaim checks the label position: labels
// come after the claim on the ticket line.
func TestRenderMarkdownLabelsWithClaim(t *testing.T) {
	claimedAt := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	state := BoardState{
		Name:    "labels",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "claimed and labelled", Status: "todo", ClaimedBy: "claude-a", ClaimedAt: &claimedAt, Labels: []string{"bug"}, Comments: []Comment{}},
		},
	}
	md := string(RenderMarkdown(state))
	if !strings.Contains(md, "- T-1 claimed and labelled — claimed by claude-a [bug]\n") {
		t.Errorf("board.md does not render the claim and the labels in order:\n%s", md)
	}
}

// TestRenderSVGLabels checks that labels render as badges on the card,
// after the claim badge.
func TestRenderSVGLabels(t *testing.T) {
	state := BoardState{
		Name:    "labels",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "labelled", Status: "todo", Labels: []string{"bug", "docs"}, Comments: []Comment{}},
		},
	}
	svg := string(RenderSVG(state))
	for _, want := range []string{">bug<", ">docs<"} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg does not render the label badge %q:\n%s", want, svg)
		}
	}
}

// TestRenderSVGOmitsLabelBadge checks that a ticket without labels
// renders no label badge.
func TestRenderSVGOmitsLabelBadge(t *testing.T) {
	state := BoardState{
		Name:    "plain-board",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "plain", Status: "todo", Comments: []Comment{}},
		},
	}
	svg := string(RenderSVG(state))
	if strings.Contains(svg, "badge label") || strings.Contains(svg, ">label<") {
		t.Errorf("svg renders a label badge for a ticket without labels:\n%s", svg)
	}
}

// TestRenderMarkdownLabelFilter checks the label filter: only tickets
// with the label render, the header and column sections stay.
func TestRenderMarkdownLabelFilter(t *testing.T) {
	state := BoardState{
		Name:    "labels",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "buggy", Status: "todo", Labels: []string{"bug"}, Comments: []Comment{}},
			"T-2": {ID: "T-2", Title: "docsy", Status: "todo", Labels: []string{"docs"}, Comments: []Comment{}},
			"T-3": {ID: "T-3", Title: "plain", Status: "todo", Comments: []Comment{}},
		},
	}
	md := string(RenderMarkdownFiltered(state, "bug"))
	if !strings.Contains(md, "- T-1 buggy [bug]\n") {
		t.Errorf("filtered board.md missing the matching ticket:\n%s", md)
	}
	for _, banned := range []string{"T-2 docsy", "T-3 plain"} {
		if strings.Contains(md, banned) {
			t.Errorf("filtered board.md carries %q — the filter did not apply:\n%s", banned, md)
		}
	}
}

// TestRenderMarkdownLabelFilterNoMatch checks the label filter with no
// matching tickets: the board renders with empty columns.
func TestRenderMarkdownLabelFilterNoMatch(t *testing.T) {
	state := BoardState{
		Name:    "labels",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "buggy", Status: "todo", Labels: []string{"bug"}, Comments: []Comment{}},
		},
	}
	md := string(RenderMarkdownFiltered(state, "docs"))
	if strings.Contains(md, "T-1 buggy") {
		t.Errorf("filtered board.md carries the non-matching ticket:\n%s", md)
	}
	if !strings.Contains(md, "## todo") {
		t.Errorf("filtered board.md lost the column section:\n%s", md)
	}
}

// TestRenderMarkdownMultiline checks that multi-line descriptions
// render with every line indented.
func TestRenderMarkdownMultiline(t *testing.T) {
	state := BoardState{
		Name:    "multi",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "multi", Description: "line one\nline two", Status: "todo", Comments: []Comment{
				{TS: time.Date(2026, 8, 20, 14, 4, 0, 0, time.UTC), Actor: "claude-a", Text: "first line\nsecond line"},
			}},
		},
	}
	md := string(RenderMarkdown(state))
	for _, want := range []string{
		"  line one\n  line two\n",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("board.md missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "first line") || strings.Contains(md, "second line") {
		t.Errorf("board.md renders comment text — comments live on the ticket view:\n%s", md)
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
