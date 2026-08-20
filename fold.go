package hexdeck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultColumns are the board columns when config.json is missing.
var DefaultColumns = []string{"todo", "in-progress", "review", "done"}

// Config is the board config from .kanban/config.json.
type Config struct {
	Schema       int      `json:"schema"`
	Board        string   `json:"board"`
	Columns      []string `json:"columns"`
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
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Comments    []Comment `json:"comments"`
	Created     time.Time `json:"created"`
	ClaimedBy   string    `json:"claimedBy,omitempty"`
	Archived    bool      `json:"archived"`
}

// BoardState is the board after the fold. The projection of the ops.
type BoardState struct {
	Name     string            `json:"name"`
	Columns  []string          `json:"columns"`
	Tickets  map[string]Ticket `json:"tickets"`
	Warnings []string          `json:"warnings"`
	// Updated is the newest op ts. It is display-only: the fold never
	// uses it, and the renders use it for the "Updated:" line.
	Updated time.Time `json:"updated"`
}

// Project reads the board dir and folds the ops into a BoardState.
// The board is a pure function of the ops: same ops, same state, always.
func Project(dir string) (BoardState, error) {
	state := BoardState{
		Columns:  append([]string(nil), DefaultColumns...),
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
		var p struct {
			Name string `json:"name"`
		}
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
		var p struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		}
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
		var p struct {
			To string `json:"to"`
		}
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: bad payload: %v", op.Type, op.Ticket, err))
			return
		}
		ticket.Status = p.To
		state.Tickets[op.Ticket] = ticket
	case OpTicketUpdated:
		ticket, ok := state.Tickets[op.Ticket]
		if !ok {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket does not exist, skipping", op.Type, op.Ticket))
			return
		}
		var p struct {
			Title       *string `json:"title"`
			Description *string `json:"description"`
		}
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
		var p struct {
			Text string `json:"text"`
		}
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
		var p struct {
			By string `json:"by"`
		}
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: bad payload: %v", op.Type, op.Ticket, err))
			return
		}
		ticket.ClaimedBy = p.By
		state.Tickets[op.Ticket] = ticket
	case OpTicketReleased:
		ticket, ok := state.Tickets[op.Ticket]
		if !ok {
			state.Warnings = append(state.Warnings, fmt.Sprintf("%s for %s: ticket does not exist, skipping", op.Type, op.Ticket))
			return
		}
		ticket.ClaimedBy = ""
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
