package hexdeck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RenderMarkdown renders the board as board.md — the human-readable view.
// The output is deterministic: same state, same bytes, always.
//
// Layout:
//
//	# Board — <name>
//	Updated: <max op ts> · <counts per column>
//
//	## <column>
//	- T-1 Title — claimed by claude-a · 2 comments
//	  the description, indented
//	  - 2026-08-20T14:04:00Z claude-a: on it
//
// Tickets sort by id within a column, numerically (T-2 before T-10).
// Archived tickets are hidden. Tickets in a column that is not in the
// config render in a trailing section named after the column.
func RenderMarkdown(state BoardState) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Board — %s\n", state.Name)
	fmt.Fprintf(&b, "Updated: %s · %s\n", state.Updated.UTC().Format("2006-01-02T15:04:05Z"), columnCounts(state))
	columns := boardColumns(state)
	for _, column := range columns {
		fmt.Fprintf(&b, "\n## %s\n", column)
		for _, ticket := range sortedTickets(state) {
			if ticket.Archived || ticket.Status != column {
				continue
			}
			fmt.Fprintf(&b, "- %s %s", ticket.ID, ticket.Title)
			if ticket.ClaimedBy != "" {
				fmt.Fprintf(&b, " — claimed by %s", ticket.ClaimedBy)
				if ticket.ClaimStale {
					b.WriteString(" (stale claim)")
				}
			}
			if len(ticket.Comments) > 0 {
				fmt.Fprintf(&b, " · %d comment", len(ticket.Comments))
				if len(ticket.Comments) > 1 {
					b.WriteString("s")
				}
			}
			b.WriteString("\n")
			if ticket.Description != "" {
				writeIndented(&b, ticket.Description, "  ")
			}
			for _, comment := range ticket.Comments {
				lines := strings.Split(comment.Text, "\n")
				fmt.Fprintf(&b, "  - %s %s: %s\n", comment.TS.UTC().Format("2006-01-02T15:04:05Z"), comment.Actor, lines[0])
				for _, line := range lines[1:] {
					b.WriteString("    ")
					b.WriteString(line)
					b.WriteString("\n")
				}
			}
		}
	}
	return b.Bytes()
}

// writeIndented writes text with every line prefixed by indent. The
// text keeps its line breaks; the last line always ends with a newline.
func writeIndented(b *bytes.Buffer, text, indent string) {
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
	}
}

// RenderJSON renders the board as board.json — the machine view. It is
// the full BoardState, indented, with a trailing newline. The output is
// deterministic: same state, same bytes, always.
func RenderJSON(state BoardState) ([]byte, error) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal board state: %w", err)
	}
	return append(data, '\n'), nil
}

// boardColumns returns the columns to render: the config columns, plus
// any column that holds tickets but is not in the config, in first-
// seen order. The renders all share this one definition so a future
// change to column handling needs one edit, not four.
func boardColumns(state BoardState) []string {
	columns := append([]string(nil), state.Columns...)
	for _, ticket := range sortedTickets(state) {
		if ticket.Archived {
			continue
		}
		if !contains(columns, ticket.Status) {
			columns = append(columns, ticket.Status)
		}
	}
	return columns
}

// columnCounts builds the "3 todo · 2 in-progress · ..." part of the
// Updated line. One count per column, in config order. Columns that hold
// tickets but are not in the config come last, in first-seen order.
func columnCounts(state BoardState) string {
	columns := boardColumns(state)
	counts := make([]string, 0, len(columns))
	for _, column := range columns {
		n := 0
		for _, ticket := range state.Tickets {
			if !ticket.Archived && ticket.Status == column {
				n++
			}
		}
		counts = append(counts, fmt.Sprintf("%d %s", n, column))
	}
	return strings.Join(counts, " · ")
}

// sortedTickets returns the tickets sorted by id, numerically: T-2 comes
// before T-10. The number part is the suffix after the board's prefix.
// Ids that do not match <prefix>-<number> sort after the numbered ones,
// in plain string order.
func sortedTickets(state BoardState) []Ticket {
	tickets := make([]Ticket, 0, len(state.Tickets))
	for _, ticket := range state.Tickets {
		tickets = append(tickets, ticket)
	}
	sort.Slice(tickets, func(i, j int) bool {
		return TicketIDLess(state.Prefix, tickets[i].ID, tickets[j].ID)
	})
	return tickets
}

// TicketIDLess compares two ticket ids numerically: T-2 < T-10 <
// T-11 < X-1. It is the same ordering the renders use, so every
// surface (board view, pick, MCP next) agrees on ticket order.
func TicketIDLess(prefix string, a, b string) bool {
	an, aok := ticketNumber(prefix, a)
	bn, bok := ticketNumber(prefix, b)
	switch {
	case aok && bok:
		if an != bn {
			return an < bn
		}
		return a < b
	case aok:
		return true
	case bok:
		return false
	default:
		return a < b
	}
}

// ticketNumber returns the number part of a <prefix>-<number> id. An
// empty prefix means the default T.
func ticketNumber(prefix, id string) (int, bool) {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	if !strings.HasPrefix(id, prefix+"-") {
		return 0, false
	}
	n, err := strconv.Atoi(id[len(prefix)+1:])
	if err != nil {
		return 0, false
	}
	return n, true
}

// contains reports whether s is in list.
func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
