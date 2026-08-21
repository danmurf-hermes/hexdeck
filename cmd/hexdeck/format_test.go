package main

import (
	"testing"
	"time"

	"github.com/danmurf/hexdeck"
)

// TestNextTodo is a table-driven check of the pick rules: the sort is
// numeric (same as the render — T-2 before T-10), a fresh claim
// blocks, a stale claim does not, and archived tickets are skipped.
func TestNextTodo(t *testing.T) {
	now := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	claimedAt := now.Add(-time.Hour) // stale under a 2h timeout
	freshAt := now
	stale := hexdeck.Ticket{ID: "T-3", Title: "stale claim", Status: "todo", ClaimedBy: "claude-a", ClaimedAt: &claimedAt, ClaimStale: true, Comments: []hexdeck.Comment{}}
	fresh := hexdeck.Ticket{ID: "T-4", Title: "fresh claim", Status: "todo", ClaimedBy: "claude-a", ClaimedAt: &freshAt, Comments: []hexdeck.Comment{}}
	tests := []struct {
		name   string
		state  hexdeck.BoardState
		wantID string
		wantOK bool
	}{
		{name: "empty board", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo"}, Tickets: map[string]hexdeck.Ticket{}}, wantOK: false},
		{name: "numeric order T-2 before T-10", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo"}, Tickets: map[string]hexdeck.Ticket{
			"T-10": {ID: "T-10", Title: "ten", Status: "todo", Comments: []hexdeck.Comment{}},
			"T-2":  {ID: "T-2", Title: "two", Status: "todo", Comments: []hexdeck.Comment{}},
		}}, wantID: "T-2", wantOK: true},
		{name: "fresh claim blocks", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo"}, Tickets: map[string]hexdeck.Ticket{
			"T-1": {ID: "T-1", Title: "one", Status: "todo", Comments: []hexdeck.Comment{}},
			"T-4": fresh,
		}}, wantID: "T-1", wantOK: true},
		{name: "stale claim does not block", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo"}, Tickets: map[string]hexdeck.Ticket{
			"T-3": stale,
		}}, wantID: "T-3", wantOK: true},
		{name: "archived skipped", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo"}, Tickets: map[string]hexdeck.Ticket{
			"T-1": {ID: "T-1", Title: "gone", Status: "todo", Archived: true, Comments: []hexdeck.Comment{}},
		}}, wantOK: false},
		{name: "only fresh claims", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo"}, Tickets: map[string]hexdeck.Ticket{
			"T-4": fresh,
		}}, wantOK: false},
		{name: "not in todo skipped", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo", "in-progress"}, Tickets: map[string]hexdeck.Ticket{
			"T-1": {ID: "T-1", Title: "moved", Status: "in-progress", Comments: []hexdeck.Comment{}},
		}}, wantOK: false},
		{name: "backlog not pickable on the default flow", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"backlog", "todo", "done"}, Tickets: map[string]hexdeck.Ticket{
			"T-1": {ID: "T-1", Title: "planned", Status: "backlog", Comments: []hexdeck.Comment{}},
		}}, wantOK: false},
		{name: "todo pickable on the default flow", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"backlog", "todo", "done"}, Tickets: map[string]hexdeck.Ticket{
			"T-1": {ID: "T-1", Title: "ready", Status: "todo", Comments: []hexdeck.Comment{}},
		}}, wantID: "T-1", wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nextTodo(tt.state)
			if ok != tt.wantOK {
				t.Fatalf("nextTodo ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got.ID != tt.wantID {
				t.Errorf("nextTodo = %s, want %s", got.ID, tt.wantID)
			}
		})
	}
}

// TestPickColumn checks the pick source: the column named "todo" when
// the board has one, else the first column. The default flow is
// backlog → todo → done — a ticket becomes pickable when it moves to
// todo, not when it is created.
func TestPickColumn(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		want    string
	}{
		{name: "default flow", columns: []string{"backlog", "todo", "done"}, want: "todo"},
		{name: "todo not first", columns: []string{"todo", "in-progress", "done"}, want: "todo"},
		{name: "no todo column", columns: []string{"backlog", "done"}, want: "backlog"},
		{name: "single column", columns: []string{"todo"}, want: "todo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickColumn(hexdeck.BoardState{Columns: tt.columns}); got != tt.want {
				t.Errorf("pickColumn(%v) = %q, want %q", tt.columns, got, tt.want)
			}
		})
	}
}

// TestPickTarget checks the pick destination: in-progress when the
// board has one, else empty — the claim alone marks the pick and the
// ticket stays in todo.
func TestPickTarget(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		want    string
	}{
		{name: "in-progress opt-in", columns: []string{"todo", "in-progress", "done"}, want: "in-progress"},
		{name: "default flow", columns: []string{"backlog", "todo", "done"}, want: ""},
		{name: "single column", columns: []string{"todo"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickTarget(hexdeck.BoardState{Columns: tt.columns}); got != tt.want {
				t.Errorf("pickTarget(%v) = %q, want %q", tt.columns, got, tt.want)
			}
		})
	}
}
