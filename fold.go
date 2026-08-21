package hexdeck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultColumns are the board columns when config.json is missing:
// backlog for planned work, todo for work ready to pick up, done for
// finished work. in-progress is opt-in — a board that wants it adds it
// to config.json.
var DefaultColumns = []string{"backlog", "todo", "done"}

// Config is the board config from .kanban/config.json.
type Config struct {
	Schema       int      `json:"schema"`
	Board        string   `json:"board"`
	Columns      []string `json:"columns"`
	TicketPrefix string   `json:"ticketPrefix"`
	ClaimTimeout string   `json:"claimTimeout"`
	AutoPush     bool     `json:"autoPush"`
}

// Comment is one comment on a ticket.
type Comment struct {
	TS    time.Time `json:"ts"`
	Actor string    `json:"actor"`
	Text  string    `json:"text"`
}

// Ticket is one unit of work. Built by folding the ops about it.
type Ticket struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Comments    []Comment  `json:"comments"`
	Created     time.Time  `json:"created"`
	ClaimedBy   string     `json:"claimedBy,omitempty"`
	ClaimedAt   *time.Time `json:"claimedAt,omitempty"`
	// ClaimStale is true when the claim is older than the claim
	// timeout. The claim still stands — the flag only marks it for
	// display and for pick.
	ClaimStale bool `json:"claimStale,omitempty"`
	Archived   bool `json:"archived"`
}

// BoardState is the board after the fold. The projection of the ops.
type BoardState struct {
	Name         string            `json:"name"`
	Columns      []string          `json:"columns"`
	Prefix       string            `json:"prefix"`
	ClaimTimeout string            `json:"claimTimeout"`
	Tickets      map[string]Ticket `json:"tickets"`
	Warnings     []string          `json:"warnings"`
	// Updated is display-only — the fold never reads it.
	Updated time.Time `json:"updated"`
}

// Project reads the board dir and folds the ops into a BoardState.
// The board is a pure function of the ops: same ops, same state, always.
// Project uses the snapshot cache to skip the replay when nothing
// changed. The cache is an accelerator, never a source of truth — the
// renders and the CI honesty gate always fold cold.
func Project(dir string) (BoardState, error) {
	return projectCached(dir, time.Now())
}

// projectCached is Project with an explicit clock. It tries the
// snapshot cache first; on a miss it folds cold and writes a fresh
// snapshot.
func projectCached(dir string, now time.Time) (BoardState, error) {
	digest, err := snapshotDigest(dir)
	if err == nil {
		if snap, ok := readSnapshot(dir); ok && snap.Digest == digest {
			state := snap.State
			if state.Tickets == nil {
				state.Tickets = map[string]Ticket{}
			}
			markStaleClaims(&state, now)
			return state, nil
		}
	}
	state, err := projectAt(dir, now)
	if err != nil {
		return BoardState{}, err
	}
	if digest != "" {
		writeSnapshot(dir, digest, state)
	}
	return state, nil
}

// projectFold is the pure fold: no snapshot cache, no wall clock, no
// stale-claim marking. The board is a pure function of the ops — same
// ops, same bytes, always. The committed renders and the honesty gate
// use it, so board.md and board.json can never change under a clock.
func projectFold(dir string) (BoardState, error) {
	state := BoardState{
		Columns:  append([]string(nil), DefaultColumns...),
		Prefix:   DefaultPrefix,
		Tickets:  map[string]Ticket{},
		Warnings: []string{},
	}
	config, err := readConfig(filepath.Join(dir, "config.json"))
	if err != nil {
		return BoardState{}, err
	}
	if config != nil {
		if config.Board != "" {
			state.Name = config.Board
		}
		if len(config.Columns) > 0 {
			state.Columns = append([]string(nil), config.Columns...)
		}
		if config.TicketPrefix != "" {
			state.Prefix = config.TicketPrefix
		}
		if config.ClaimTimeout != "" {
			state.ClaimTimeout = config.ClaimTimeout
		}
	}
	ops, warnings, err := ReadOpsDir(filepath.Join(dir, "ops"))
	if err != nil {
		return BoardState{}, err
	}
	state.Warnings = append(state.Warnings, warnings...)
	for _, op := range ops {
		if op.TS.After(state.Updated) {
			state.Updated = op.TS
		}
		apply(op, &state)
	}
	return state, nil
}

// projectAt is the fold at an explicit clock: the pure board plus the
// stale-claim marks. Interactive paths (Project, show, web, pick) use
// it so claims age visibly; the committed renders never do.
func projectAt(dir string, now time.Time) (BoardState, error) {
	state, err := projectFold(dir)
	if err != nil {
		return BoardState{}, err
	}
	markStaleClaims(&state, now)
	return state, nil
}

// markStaleClaims flags every claim older than the claim timeout. The
// claim still stands — the flag only marks it for display and for pick.
// A missing or unparseable timeout means no claim is ever stale.
func markStaleClaims(state *BoardState, now time.Time) {
	timeout, err := time.ParseDuration(state.ClaimTimeout)
	if err != nil {
		return
	}
	for id, ticket := range state.Tickets {
		if ticket.ClaimedBy == "" || ticket.ClaimedAt == nil {
			continue
		}
		if now.Sub(*ticket.ClaimedAt) > timeout {
			ticket.ClaimStale = true
			state.Tickets[id] = ticket
		}
	}
}

// readConfig reads config.json. A missing file is fine — the defaults
// apply. A broken file is an error: the board must not silently change.
func readConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &config, nil
}

// apply folds one op into the state. Ops about a missing ticket are
// skipped with a warning — visible, never fatal.
func apply(op Op, state *BoardState) {
	switch op.Type {
	case OpBoardCreated:
		var p BoardCreatedPayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s: bad payload: %v", op.Type, err))
			return
		}
		state.Name = p.Name
	case OpTicketCreated:
		if _, exists := state.Tickets[op.Ticket]; exists {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket already exists, keeping the first", op.Type, op.Ticket))
			return
		}
		var p TicketCreatedPayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: bad payload: %v", op.Type, op.Ticket, err))
			return
		}
		state.Tickets[op.Ticket] = Ticket{
			ID:          op.Ticket,
			Title:       p.Title,
			Description: p.Description,
			Status:      state.Columns[0],
			Comments:    []Comment{},
			Created:     op.TS,
		}
	case OpTicketMoved:
		ticket, ok := state.Tickets[op.Ticket]
		if !ok {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket does not exist, skipping", op.Type, op.Ticket))
			return
		}
		var p TicketMovedPayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: bad payload: %v", op.Type, op.Ticket, err))
			return
		}
		if p.From != "" && p.From != ticket.Status {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: from %q does not match current status %q", op.Type, op.Ticket, p.From, ticket.Status))
		}
		ticket.Status = p.To
		state.Tickets[op.Ticket] = ticket
	case OpTicketUpdated:
		ticket, ok := state.Tickets[op.Ticket]
		if !ok {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket does not exist, skipping", op.Type, op.Ticket))
			return
		}
		var p TicketUpdatedPayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: bad payload: %v", op.Type, op.Ticket, err))
			return
		}
		if p.Title != nil {
			ticket.Title = *p.Title
		}
		if p.Description != nil {
			ticket.Description = *p.Description
		}
		state.Tickets[op.Ticket] = ticket
	case OpCommentAdded:
		ticket, ok := state.Tickets[op.Ticket]
		if !ok {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket does not exist, skipping", op.Type, op.Ticket))
			return
		}
		var p CommentAddedPayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: bad payload: %v", op.Type, op.Ticket, err))
			return
		}
		ticket.Comments = append(ticket.Comments, Comment{TS: op.TS, Actor: op.Actor, Text: p.Text})
		state.Tickets[op.Ticket] = ticket
	case OpTicketClaimed:
		ticket, ok := state.Tickets[op.Ticket]
		if !ok {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket does not exist, skipping", op.Type, op.Ticket))
			return
		}
		var p TicketClaimedPayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: bad payload: %v", op.Type, op.Ticket, err))
			return
		}
		// A claim on an already-claimed ticket is a race: two writers
		// picked the same ticket. The first claim by (seq, opId) wins;
		// the second renders a warning. The claim is cooperative, not a
		// security boundary — the warning is the whole resolution.
		if ticket.ClaimedBy != "" {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: already claimed by %s, keeping the first claim", op.Type, op.Ticket, ticket.ClaimedBy))
			return
		}
		ticket.ClaimedBy = p.By
		claimedAt := op.TS
		ticket.ClaimedAt = &claimedAt
		ticket.ClaimStale = false
		state.Tickets[op.Ticket] = ticket
	case OpTicketReleased:
		ticket, ok := state.Tickets[op.Ticket]
		if !ok {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket does not exist, skipping", op.Type, op.Ticket))
			return
		}
		ticket.ClaimedBy = ""
		ticket.ClaimedAt = nil
		ticket.ClaimStale = false
		state.Tickets[op.Ticket] = ticket
	case OpTicketArchived:
		ticket, ok := state.Tickets[op.Ticket]
		if !ok {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket does not exist, skipping", op.Type, op.Ticket))
			return
		}
		ticket.Archived = true
		state.Tickets[op.Ticket] = ticket
	}
}
