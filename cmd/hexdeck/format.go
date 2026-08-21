package main

import (
	"fmt"
	"os"
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
// and a "since" duration. Warnings print to stderr — the same channel
// the CLI uses.
func opTimeline(boardDir, ticket, actor, since string) (string, error) {
	ops, warnings, err := hexdeck.ReadOpsDir(filepath.Join(boardDir, "ops"))
	if err != nil {
		return "", err
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	var cutoff time.Time
	if since != "" {
		d, err := time.ParseDuration(since)
		if err != nil {
			return "", fmt.Errorf("since: %q is not a duration like 2d or 3h", since)
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
	return b.String(), nil
}

// nextTodo returns the next todo ticket to pick: the first todo ticket
// by id, skipping fresh claims. A stale claim does not block the
// ticket. The second return value reports whether one exists. Shared by
// `hexdeck pick` and the MCP board_next tool.
func nextTodo(state hexdeck.BoardState) (hexdeck.Ticket, bool) {
	var candidates []hexdeck.Ticket
	for _, ticket := range state.Tickets {
		if ticket.Archived || ticket.Status != state.Columns[0] {
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
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0], true
}
