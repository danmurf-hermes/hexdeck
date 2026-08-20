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
//
// Tickets sort by id within a column, numerically (T-2 before T-10).
// Archived tickets are hidden. Tickets in a column that is not in the
// config render in a trailing section named after the column.
func RenderMarkdown(state BoardState) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Board — %s\n", state.Name)
	fmt.Fprintf(&b, "Updated: %s · %s\n", state.Updated.UTC().Format("2006-01-02T15:04:05Z"), columnCounts(state))
	columns := append([]string(nil), state.Columns...)
	for _, ticket := range sortedTickets(state) {
		if ticket.Archived {
			continue
		}
		if !contains(columns, ticket.Status) {
			columns = append(columns, ticket.Status)
		}
	}
	for _, column := range columns {
		fmt.Fprintf(&b, "\n## %s\n", column)
		for _, ticket := range sortedTickets(state) {
			if ticket.Archived || ticket.Status != column {
				continue
			}
			fmt.Fprintf(&b, "- %s %s", ticket.ID, ticket.Title)
			if ticket.ClaimedBy != "" {
				fmt.Fprintf(&b, " — claimed by %s", ticket.ClaimedBy)
			}
			if len(ticket.Comments) > 0 {
				fmt.Fprintf(&b, " · %d comment", len(ticket.Comments))
				if len(ticket.Comments) > 1 {
					b.WriteString("s")
				}
			}
			b.WriteString("\n")
		}
	}
	return b.Bytes()
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

// columnCounts builds the "3 todo · 2 in-progress · ..." part of the
// Updated line. One count per column, in config order. Columns that hold
// tickets but are not in the config come last, in first-seen order.
func columnCounts(state BoardState) string {
	columns := append([]string(nil), state.Columns...)
	for _, ticket := range sortedTickets(state) {
		if ticket.Archived {
			continue
		}
		if !contains(columns, ticket.Status) {
			columns = append(columns, ticket.Status)
		}
	}
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
// before T-10. Ids that do not match T-<number> sort after the numbered
// ones, in plain string order.
func sortedTickets(state BoardState) []Ticket {
	tickets := make([]Ticket, 0, len(state.Tickets))
	for _, ticket := range state.Tickets {
		tickets = append(tickets, ticket)
	}
	sort.Slice(tickets, func(i, j int) bool {
		return ticketIDLess(tickets[i].ID, tickets[j].ID)
	})
	return tickets
}

// ticketIDLess compares two ticket ids. T-2 < T-10 < T-11 < X-1.
func ticketIDLess(a, b string) bool {
	an, aok := ticketNumber(a)
	bn, bok := ticketNumber(b)
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

// ticketNumber returns the number part of a T-<number> id.
func ticketNumber(id string) (int, bool) {
	if !strings.HasPrefix(id, "T-") {
		return 0, false
	}
	n, err := strconv.Atoi(id[2:])
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
