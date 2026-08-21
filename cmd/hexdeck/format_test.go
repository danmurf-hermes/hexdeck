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
		{name: "not in first column skipped", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo", "in-progress"}, Tickets: map[string]hexdeck.Ticket{
			"T-1": {ID: "T-1", Title: "moved", Status: "in-progress", Comments: []hexdeck.Comment{}},
		}}, wantOK: false},
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
