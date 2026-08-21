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
// the MCP board_show_ticket tool both print it: fields, links, comments,
// and history. One formatter, two consumers.
func ticketText(ticket hexdeck.Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", ticket.ID, ticket.Title)
	fmt.Fprintf(&b, "status: %s\n", ticket.Status)
	if ticket.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", ticket.Description)
	}
	if len(ticket.Blocks) > 0 {
		fmt.Fprintf(&b, "blocks: %s\n", strings.Join(ticket.Blocks, ", "))
	}
	if len(ticket.BlockedBy) > 0 {
		fmt.Fprintf(&b, "blocked by: %s\n", strings.Join(ticket.BlockedBy, ", "))
	}
	if len(ticket.Related) > 0 {
		fmt.Fprintf(&b, "related: %s\n", strings.Join(ticket.Related, ", "))
	}
	if len(ticket.Labels) > 0 {
		fmt.Fprintf(&b, "labels: %s\n", strings.Join(ticket.Labels, ", "))
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
// by id, skipping fresh claims and blocked tickets. A ticket is blocked
// when one of its blockers is not done — the blocker is either not on
// the board (the fold warned and dropped the link) or still open. A
// stale claim does not block the ticket. The second return value
// reports whether one exists. Shared by `hexdeck pick` and the MCP
// board_next tool.
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
		if blocked(state, ticket) {
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

// blocked reports whether a ticket's blockers stand in the way: at
// least one blocker that is on the board is not done. A blocker that is
// not on the board does not block — the fold drops links to missing
// tickets with a warning, so a stale link can never stall the pick.
func blocked(state hexdeck.BoardState, ticket hexdeck.Ticket) bool {
	for _, blocker := range ticket.BlockedBy {
		bt, ok := state.Tickets[blocker]
		if ok && bt.Status != "done" {
			return true
		}
	}
	return false
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
