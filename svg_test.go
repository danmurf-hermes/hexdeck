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
