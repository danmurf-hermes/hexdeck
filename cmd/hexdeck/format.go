package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/danmurf/hexdeck"
)

// ticketText renders one ticket the way `hexdeck show <ticket>` and
// the MCP board_show_ticket tool both print it: fields, comments, and
// history. One formatter, two consumers.
func ticketText(ticket hexdeck.Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", ticket.ID, ticket.Title)
	fmt.Fprintf(&b, "status: %s\n", ticket.Status)
	if ticket.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", ticket.Description)
	}
	if ticket.ClaimedBy != "" {
		fmt.Fprintf(&b, "claimed by: %s", ticket.ClaimedBy)
		if ticket.ClaimStale {
			fmt.Fprint(&b, " (stale claim)")
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "created: %s\n", ticket.Created.UTC().Format("2006-01-02T15:04:05Z"))
	if len(ticket.Comments) > 0 {
		fmt.Fprintln(&b, "comments:")
		for _, comment := range ticket.Comments {
			fmt.Fprintf(&b, "  %s %s: %s\n", comment.TS.UTC().Format("2006-01-02T15:04:05Z"), comment.Actor, comment.Text)
		}
	}
	return b.String()
}

// opTimeline renders the op timeline the way `hexdeck log` and the MCP
// board_log tool print it, with the same filters: a ticket, an actor,
// and a "since" duration. Warnings from reading the ops come back
// separately so each surface can show them: the CLI prints them to
// stderr, the MCP tool puts them in the result text — an agent calling
// board_log must see that ops were skipped, or it would trust a
// timeline with holes in it.
func opTimeline(boardDir, ticket, actor, since string) (string, []string, error) {
	ops, warnings, err := hexdeck.ReadOpsDir(filepath.Join(boardDir, "ops"))
	if err != nil {
		return "", nil, err
	}
	var cutoff time.Time
	if since != "" {
		d, err := time.ParseDuration(since)
		if err != nil {
			return "", nil, fmt.Errorf("since: %q is not a duration like 2d or 3h", since)
		}
		cutoff = time.Now().UTC().Add(-d)
	}
	var b strings.Builder
	for _, op := range ops {
		if ticket != "" && op.Ticket != ticket {
			continue
		}
		if actor != "" && op.Actor != actor {
			continue
		}
		if !cutoff.IsZero() && op.TS.Before(cutoff) {
			continue
		}
		fmt.Fprintf(&b, "%s %s %s %s\n", op.TS.UTC().Format("2006-01-02T15:04:05Z"), op.Actor, op.Type, op.Ticket)
	}
	return b.String(), warnings, nil
}

// nextTodo returns the next todo ticket to pick: the first todo ticket
// by id, skipping fresh claims. A stale claim does not block the
// ticket. The second return value reports whether one exists. Shared by
// `hexdeck pick` and the MCP board_next tool.
func nextTodo(state hexdeck.BoardState) (hexdeck.Ticket, bool) {
	column := pickColumn(state)
	var candidates []hexdeck.Ticket
	for _, ticket := range state.Tickets {
		if ticket.Archived || ticket.Status != column {
			continue
		}
		if ticket.ClaimedBy != "" && !ticket.ClaimStale {
			continue
		}
		candidates = append(candidates, ticket)
	}
	if len(candidates) == 0 {
		return hexdeck.Ticket{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return hexdeck.TicketIDLess(state.Prefix, candidates[i].ID, candidates[j].ID)
	})
	return candidates[0], true
}

// pickColumn returns the column pick takes from: the column named
// "todo" when the board has one, else the first column. The default
// flow is backlog → todo → done — a ticket becomes pickable when it
// moves to todo, not when it is created.
func pickColumn(state hexdeck.BoardState) string {
	for _, column := range state.Columns {
		if column == "todo" {
			return column
		}
	}
	return state.Columns[0]
}

// pickTarget returns the column pick moves a ticket to: in-progress
// when the board has one, else empty — the claim alone marks the pick
// and the ticket stays in todo. A move from todo to todo would be
// noise in the log.
func pickTarget(state hexdeck.BoardState) string {
	for _, column := range state.Columns {
		if column == "in-progress" {
			return column
		}
	}
	return ""
}
