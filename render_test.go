package hexdeck

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestRenderMarkdownGolden renders board.md for every fixture board and
// compares it to a golden file, byte for byte.
func TestRenderMarkdownGolden(t *testing.T) {
	cases := []string{"basic", "seq-collision", "unparseable", "duplicate-ticket", "missing-ticket", "render"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "ops", name)
			state, err := Project(dir)
			if err != nil {
				t.Fatalf("Project: %v", err)
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
			state, err := Project(dir)
			if err != nil {
				t.Fatalf("Project: %v", err)
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
	state, err := Project(filepath.Join("testdata", "ops", "render"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	first := RenderMarkdown(state)
	for i := 0; i < 20; i++ {
		if got := RenderMarkdown(state); !bytes.Equal(got, first) {
			t.Fatalf("render %d differs from the first render", i)
		}
	}
}

// TestRenderJSONDeterministic renders the same state twice and checks the
// bytes are identical. The render must never depend on map order.
func TestRenderJSONDeterministic(t *testing.T) {
	state, err := Project(filepath.Join("testdata", "ops", "render"))
	if err != nil {
		t.Fatalf("Project: %v", err)
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
