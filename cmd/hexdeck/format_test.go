package main

import (
	"strings"
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

// TestTicketTextLinks checks the ticket view renders the links: blocks,
// blocked by, and related, one per line.
func TestTicketTextLinks(t *testing.T) {
	ticket := hexdeck.Ticket{
		ID:        "T-1",
		Title:     "one",
		Status:    "todo",
		Created:   time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC),
		Comments:  []hexdeck.Comment{},
		Blocks:    []string{"T-2"},
		BlockedBy: []string{"T-3"},
		Related:   []string{"T-4"},
	}
	text := ticketText(ticket)
	for _, want := range []string{
		"blocks: T-2",
		"blocked by: T-3",
		"related: T-4",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("ticketText missing %q:\n%s", want, text)
		}
	}
}

// TestTicketTextNoLinks checks a ticket without links renders no link
// lines.
func TestTicketTextNoLinks(t *testing.T) {
	ticket := hexdeck.Ticket{
		ID:       "T-1",
		Title:    "one",
		Status:   "todo",
		Created:  time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC),
		Comments: []hexdeck.Comment{},
	}
	text := ticketText(ticket)
	for _, banned := range []string{"blocks:", "blocked by:", "related:"} {
		if strings.Contains(text, banned) {
			t.Errorf("ticketText renders %q for a ticket without links:\n%s", banned, text)
		}
	}
}

// TestTicketTextLabels checks the ticket view renders the labels, one
// line, comma-separated.
func TestTicketTextLabels(t *testing.T) {
	ticket := hexdeck.Ticket{
		ID:       "T-1",
		Title:    "one",
		Status:   "todo",
		Created:  time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC),
		Comments: []hexdeck.Comment{},
		Labels:   []string{"feature", "docs"},
	}
	text := ticketText(ticket)
	if !strings.Contains(text, "labels: feature, docs") {
		t.Errorf("ticketText missing the labels line:\n%s", text)
	}
}

// TestTicketTextNoLabels checks a ticket without labels renders no
// labels line.
func TestTicketTextNoLabels(t *testing.T) {
	ticket := hexdeck.Ticket{
		ID:       "T-1",
		Title:    "one",
		Status:   "todo",
		Created:  time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC),
		Comments: []hexdeck.Comment{},
	}
	text := ticketText(ticket)
	if strings.Contains(text, "labels:") {
		t.Errorf("ticketText renders a labels line for a ticket without labels:\n%s", text)
	}
}

// TestNextTodoBlocked checks the blocking rule: a todo ticket whose
// blocker is not done is not pickable, whatever its id. A ticket whose
// blockers are all done is pickable. A blocks link to a missing ticket
// (the fold warns and drops it) never blocks.
func TestNextTodoBlocked(t *testing.T) {
	blockerDone := hexdeck.Ticket{ID: "T-1", Title: "done blocker", Status: "done", Comments: []hexdeck.Comment{}}
	blockerTodo := hexdeck.Ticket{ID: "T-2", Title: "open blocker", Status: "todo", Comments: []hexdeck.Comment{}}
	blockerClaimed := hexdeck.Ticket{ID: "T-2", Title: "open blocker", Status: "todo", ClaimedBy: "claude-a", Comments: []hexdeck.Comment{}}
	blocked := hexdeck.Ticket{ID: "T-3", Title: "blocked", Status: "todo", BlockedBy: []string{"T-2"}, Comments: []hexdeck.Comment{}}
	unblocked := hexdeck.Ticket{ID: "T-4", Title: "unblocked", Status: "todo", BlockedBy: []string{"T-1"}, Comments: []hexdeck.Comment{}}
	missingBlocker := hexdeck.Ticket{ID: "T-5", Title: "missing blocker", Status: "todo", BlockedBy: []string{"T-99"}, Comments: []hexdeck.Comment{}}
	plain := hexdeck.Ticket{ID: "T-6", Title: "plain", Status: "todo", Comments: []hexdeck.Comment{}}
	tests := []struct {
		name   string
		state  hexdeck.BoardState
		wantID string
		wantOK bool
	}{
		{name: "blocked ticket skipped", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo", "done"}, Tickets: map[string]hexdeck.Ticket{
			"T-2": blockerTodo,
			"T-3": blocked,
		}}, wantID: "T-2", wantOK: true},
		{name: "done blocker does not block", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo", "done"}, Tickets: map[string]hexdeck.Ticket{
			"T-1": blockerDone,
			"T-4": unblocked,
		}}, wantID: "T-4", wantOK: true},
		{name: "all blocked", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo", "done"}, Tickets: map[string]hexdeck.Ticket{
			"T-2": blockerClaimed,
			"T-3": blocked,
		}}, wantOK: false},
		{name: "missing blocker does not block", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo", "done"}, Tickets: map[string]hexdeck.Ticket{
			"T-5": missingBlocker,
		}}, wantID: "T-5", wantOK: true},
		{name: "blocked skipped, plain picked", state: hexdeck.BoardState{Prefix: "T", Columns: []string{"todo", "done"}, Tickets: map[string]hexdeck.Ticket{
			"T-2": blockerClaimed,
			"T-3": blocked,
			"T-6": plain,
		}}, wantID: "T-6", wantOK: true},
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
