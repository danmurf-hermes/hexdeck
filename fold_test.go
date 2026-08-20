package hexdeck

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestFoldGolden runs the fold over fixture board dirs and compares the
// resulting BoardState to a golden file, byte for byte.
func TestFoldGolden(t *testing.T) {
	cases := []string{"basic", "seq-collision", "unparseable", "duplicate-ticket", "missing-ticket"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "ops", name)
			state, err := Project(dir)
			if err != nil {
				t.Fatalf("Project: %v", err)
			}
			data, err := json.MarshalIndent(state, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			data = append(data, '\n')
			golden := filepath.Join("testdata", "golden", "fold-"+name+".json")
			if *update {
				if err := os.WriteFile(golden, data, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(data, want) {
				t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, data, want)
			}
		})
	}
}

// TestFoldDuplicateTicket checks the duplicate-ticket rule: the first
// ticket.created wins, the second renders a warning.
func TestFoldDuplicateTicket(t *testing.T) {
	state, err := Project(filepath.Join("testdata", "ops", "duplicate-ticket"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	ticket, ok := state.Tickets["T-1"]
	if !ok {
		t.Fatalf("T-1 missing from state")
	}
	if ticket.Title != "First T-1 wins" {
		t.Errorf("title = %q, want %q", ticket.Title, "First T-1 wins")
	}
	if ticket.Status != "in-progress" {
		t.Errorf("status = %q, want %q", ticket.Status, "in-progress")
	}
	if len(state.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", state.Warnings)
	}
	if state.Warnings[0] != "ticket.created for T-1: ticket already exists, keeping the first" {
		t.Errorf("warning = %q", state.Warnings[0])
	}
}

// TestFoldMissingTicket checks the missing-ticket rule: ops for a ticket
// that was never created are skipped with a warning, never fatal.
func TestFoldMissingTicket(t *testing.T) {
	state, err := Project(filepath.Join("testdata", "ops", "missing-ticket"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if _, ok := state.Tickets["T-99"]; ok {
		t.Errorf("T-99 must not exist in the state")
	}
	if len(state.Warnings) != 2 {
		t.Fatalf("warnings = %v, want exactly 2", state.Warnings)
	}
	want := []string{
		"ticket.moved for T-99: ticket does not exist, skipping",
		"comment.added for T-99: ticket does not exist, skipping",
	}
	for i := range want {
		if state.Warnings[i] != want[i] {
			t.Errorf("warnings[%d] = %q, want %q", i, state.Warnings[i], want[i])
		}
	}
}

// TestFoldEmptyBoard checks the fold over a board with no ops: the state
// is empty, not an error.
func TestFoldEmptyBoard(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ops"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"empty","columns":["todo","in-progress","review","done"],"claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Name != "empty" {
		t.Errorf("name = %q, want %q", state.Name, "empty")
	}
	if len(state.Tickets) != 0 {
		t.Errorf("tickets = %v, want none", state.Tickets)
	}
	if len(state.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", state.Warnings)
	}
}

// TestFoldMissingConfig checks the fold over a board dir with no
// config.json: the default columns are used, not an error.
func TestFoldMissingConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ops"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	want := []string{"todo", "in-progress", "review", "done"}
	if len(state.Columns) != len(want) {
		t.Fatalf("columns = %v, want %v", state.Columns, want)
	}
	for i := range want {
		if state.Columns[i] != want[i] {
			t.Errorf("columns[%d] = %q, want %q", i, state.Columns[i], want[i])
		}
	}
}
