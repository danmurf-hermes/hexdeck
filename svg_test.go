package hexdeck

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRenderSVGGolden renders board.svg for every fixture board and
// compares it to a golden file, byte for byte.
func TestRenderSVGGolden(t *testing.T) {
	cases := []string{"basic", "seq-collision", "unparseable", "duplicate-ticket", "missing-ticket", "render"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "ops", name)
			state, err := projectAt(dir, fixtureNow)
			if err != nil {
				t.Fatalf("projectAt: %v", err)
			}
			got := RenderSVG(state)
			golden := filepath.Join("testdata", "golden", "board-"+name+".svg")
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

// TestRenderSVGDeterministic renders the same state twice and checks the
// bytes are identical. The render must never depend on map order.
func TestRenderSVGDeterministic(t *testing.T) {
	state, err := projectAt(filepath.Join("testdata", "ops", "render"), fixtureNow)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	first := RenderSVG(state)
	for i := 0; i < 20; i++ {
		if got := RenderSVG(state); !bytes.Equal(got, first) {
			t.Fatalf("render %d differs from the first render", i)
		}
	}
}

// TestRenderSVGWellFormed checks the output is well-formed XML with an
// svg root element.
func TestRenderSVGWellFormed(t *testing.T) {
	state, err := projectAt(filepath.Join("testdata", "ops", "render"), fixtureNow)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	var doc struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(RenderSVG(state), &doc); err != nil {
		t.Fatalf("svg is not well-formed XML: %v", err)
	}
	if doc.XMLName.Local != "svg" {
		t.Fatalf("root element is %q, want svg", doc.XMLName.Local)
	}
}

// TestRenderSVGEscapesText checks that titles and names with XML special
// characters are escaped, so the output stays well-formed.
func TestRenderSVGEscapesText(t *testing.T) {
	state := BoardState{
		Name:    `a & b <c>`,
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: `x < y & "z"`, Status: "todo", Comments: []Comment{}},
		},
	}
	svg := string(RenderSVG(state))
	for _, raw := range []string{`<c>`, `x < y & "z"`} {
		if strings.Contains(svg, raw) {
			t.Fatalf("svg contains unescaped text %q:\n%s", raw, svg)
		}
	}
	if !strings.Contains(svg, "a &amp; b &lt;c&gt;") {
		t.Fatalf("svg does not contain the escaped board name:\n%s", svg)
	}
}

// TestRenderSVGTruncatesLongText checks that a long title and a long
// claimer name are truncated with an ellipsis, so no card text can
// overflow the fixed card width into the adjacent column.
func TestRenderSVGTruncatesLongText(t *testing.T) {
	claimedAt := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	longTitle := "this is a very long ticket title that would overflow the card entirely"
	longActor := "a-very-long-claimer-name-that-would-overflow"
	state := BoardState{
		Name:    "truncate",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: longTitle, Status: "todo", ClaimedBy: longActor, ClaimedAt: &claimedAt, Comments: []Comment{}},
		},
	}
	svg := string(RenderSVG(state))
	if strings.Contains(svg, longTitle) {
		t.Errorf("svg contains the full long title, want truncation:\n%s", svg)
	}
	if strings.Contains(svg, longActor) {
		t.Errorf("svg contains the full long claimer:\n%s", svg)
	}
	if !strings.Contains(svg, "…") {
		t.Errorf("svg has no ellipsis — nothing was truncated:\n%s", svg)
	}
}

// TestRenderSVGStaleClaim checks the stale-claim badge: a stale claim
// renders "(stale)" in the badge, a fresh claim does not.
func TestRenderSVGStaleClaim(t *testing.T) {
	claimedAt := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	state := BoardState{
		Name:    "stale",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "stale one", Status: "todo", ClaimedBy: "claude-a", ClaimedAt: &claimedAt, ClaimStale: true, Comments: []Comment{}},
			"T-2": {ID: "T-2", Title: "fresh one", Status: "todo", ClaimedBy: "codex-1", ClaimedAt: &claimedAt, Comments: []Comment{}},
		},
	}
	svg := string(RenderSVG(state))
	if !strings.Contains(svg, "claimed by claude-a (stale)") {
		t.Errorf("svg does not mark the stale claim:\n%s", svg)
	}
	if !strings.Contains(svg, "claimed by codex-1") {
		t.Errorf("svg does not render the fresh claim:\n%s", svg)
	}
}

// TestRenderSVGLabelBadges checks that the card renders a badge per
// label, and that a ticket without labels renders no label badge.
func TestRenderSVGLabelBadges(t *testing.T) {
	state := BoardState{
		Name:    "labels",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "with labels", Status: "todo", Labels: []string{"feature", "docs"}, Comments: []Comment{}},
			"T-2": {ID: "T-2", Title: "plain", Status: "todo", Comments: []Comment{}},
		},
	}
	svg := string(RenderSVG(state))
	for _, want := range []string{">feature<", ">docs<"} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing the %s label badge:\n%s", want, svg)
		}
	}
	// One pill rect per label — the label fill appears exactly twice.
	if n := strings.Count(svg, "#fff8e5"); n != 2 {
		t.Errorf("label pill fill appears %d times, want 2 (one per label):\n%s", n, svg)
	}
}

// TestRenderSVGSaaSDesign locks the T-16 design language on the board
// image: the same Linear-flavoured light theme as the web view — light
// canvas, brand indigo accent, soft-shadowed cards, and rounded
// corners. The prototype palette (dark header bar, github blues, grey
// columns) is gone. One palette, two surfaces.
func TestRenderSVGSaaSDesign(t *testing.T) {
	state, err := projectAt(filepath.Join("testdata", "ops", "render"), fixtureNow)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	svg := string(RenderSVG(state))
	for _, want := range []string{
		"#f7f8f8", // page canvas
		"#5e6ad2", // brand indigo — header and accent
		"#7170ff", // accent hover
		"filter=", // drop shadows on cards
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg lost the SaaS design token %q — the polish did not land", want)
		}
	}
	for _, banned := range []string{
		"#24292f", // dark header bar
		"#eaeef2", // grey column
		"#0969da", // github blue
		"#ddf4ff", // claim tint
		"#d0d7de", // github border
	} {
		if strings.Contains(svg, banned) {
			t.Errorf("svg still carries the prototype token %q — the design language did not change", banned)
		}
	}
}

// TestRenderSVGOmitsCommentBadge checks that cards carry no comment
// badge — comments live on the ticket view, not on the board image.
func TestRenderSVGOmitsCommentBadge(t *testing.T) {
	ts := time.Date(2026, 8, 20, 14, 4, 0, 0, time.UTC)
	state := BoardState{
		Name:    "notes",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"T-1": {ID: "T-1", Title: "with notes", Status: "todo", Comments: []Comment{
				{TS: ts, Actor: "claude-a", Text: "on it"},
			}},
		},
	}
	svg := string(RenderSVG(state))
	for _, banned := range []string{"comment", "claude-a", "on it", "#dafbe1", "#1a7f37"} {
		if strings.Contains(svg, banned) {
			t.Errorf("svg carries %q — comments live on the ticket view, not the board image:\n%s", banned, svg)
		}
	}
}
