// Package hexdeck is the core library for hexdeck, a git-native kanban
// board for AI agents. The board is a projection of an append-only log of
// ops: one JSON file per event in .kanban/ops/.
package hexdeck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SchemaVersion is the op schema version this library reads and writes.
const SchemaVersion = 1

// OpType is one of the eight op types in the V1 spec.
type OpType string

// The complete list of V1 op types.
const (
	OpBoardCreated   OpType = "board.created"
	OpTicketCreated  OpType = "ticket.created"
	OpTicketMoved    OpType = "ticket.moved"
	OpTicketUpdated  OpType = "ticket.updated"
	OpCommentAdded   OpType = "comment.added"
	OpTicketClaimed  OpType = "ticket.claimed"
	OpTicketReleased OpType = "ticket.released"
	OpTicketArchived OpType = "ticket.archived"
)

// validOpTypes is the set of op types the parser accepts.
var validOpTypes = map[OpType]bool{
	OpBoardCreated:   true,
	OpTicketCreated:  true,
	OpTicketMoved:    true,
	OpTicketUpdated:  true,
	OpCommentAdded:   true,
	OpTicketClaimed:  true,
	OpTicketReleased: true,
	OpTicketArchived: true,
}

// Op is one event from the board log. One op = one JSON file in ops/.
type Op struct {
	Schema  int             `json:"schema"`
	OpID    string          `json:"opId"`
	Seq     int             `json:"seq"`
	TS      time.Time       `json:"ts"`
	Actor   string          `json:"actor"`
	Type    OpType          `json:"type"`
	Ticket  string          `json:"ticket,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// ParseOp parses and validates one op from its JSON bytes.
func ParseOp(data []byte) (Op, error) {
	// ts is parsed as a string first so a bad timestamp gets a clear
	// validation error instead of a JSON unmarshal error.
	var raw struct {
		Schema  int             `json:"schema"`
		OpID    string          `json:"opId"`
		Seq     int             `json:"seq"`
		TS      string          `json:"ts"`
		Actor   string          `json:"actor"`
		Type    OpType          `json:"type"`
		Ticket  string          `json:"ticket,omitempty"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Op{}, fmt.Errorf("invalid op JSON: %w", err)
	}
	op := Op{
		Schema:  raw.Schema,
		OpID:    raw.OpID,
		Seq:     raw.Seq,
		Actor:   raw.Actor,
		Type:    raw.Type,
		Ticket:  raw.Ticket,
		Payload: raw.Payload,
	}
	if op.Schema != SchemaVersion {
		return Op{}, fmt.Errorf("schema must be %d, got %d", SchemaVersion, op.Schema)
	}
	if op.OpID == "" {
		return Op{}, fmt.Errorf("opId is required")
	}
	if op.Seq < 1 {
		return Op{}, fmt.Errorf("seq must be >= 1, got %d", op.Seq)
	}
	ts, err := time.Parse(time.RFC3339, raw.TS)
	if err != nil {
		return Op{}, fmt.Errorf("ts is not a valid RFC3339 timestamp: %q", raw.TS)
	}
	op.TS = ts
	if op.Actor == "" {
		return Op{}, fmt.Errorf("actor is required")
	}
	if !validOpTypes[op.Type] {
		return Op{}, fmt.Errorf("unknown op type %q", op.Type)
	}
	if op.Type == OpBoardCreated {
		if op.Ticket != "" {
			return Op{}, fmt.Errorf("board.created must not have a ticket")
		}
	} else if op.Ticket == "" {
		return Op{}, fmt.Errorf("ticket is required for %s", op.Type)
	}
	if len(op.Payload) == 0 {
		return Op{}, fmt.Errorf("payload is required")
	}
	if err := validatePayload(op); err != nil {
		return Op{}, err
	}
	return op, nil
}

// validatePayload checks the payload shape for the op type.
func validatePayload(op Op) error {
	var err error
	switch op.Type {
	case OpBoardCreated:
		var p struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(op.Payload, &p)
		if err == nil && p.Name == "" {
			err = fmt.Errorf("name is required")
		}
	case OpTicketCreated:
		var p struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		err = json.Unmarshal(op.Payload, &p)
		if err == nil && p.Title == "" {
			err = fmt.Errorf("title is required")
		}
	case OpTicketMoved:
		var p struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		err = json.Unmarshal(op.Payload, &p)
		if err == nil {
			if p.From == "" {
				err = fmt.Errorf("from is required")
			} else if p.To == "" {
				err = fmt.Errorf("to is required")
			}
		}
	case OpTicketUpdated:
		var p struct {
			Title       *string `json:"title"`
			Description *string `json:"description"`
		}
		err = json.Unmarshal(op.Payload, &p)
		if err == nil && p.Title == nil && p.Description == nil {
			err = fmt.Errorf("at least one of title or description is required")
		}
	case OpCommentAdded:
		var p struct {
			Text string `json:"text"`
		}
		err = json.Unmarshal(op.Payload, &p)
		if err == nil && p.Text == "" {
			err = fmt.Errorf("text is required")
		}
	case OpTicketClaimed, OpTicketReleased:
		var p struct {
			By string `json:"by"`
		}
		err = json.Unmarshal(op.Payload, &p)
		if err == nil && p.By == "" {
			err = fmt.Errorf("by is required")
		}
	case OpTicketArchived:
		// Empty payload is valid.
	}
	if err != nil {
		return fmt.Errorf("invalid payload for %s: %w", op.Type, err)
	}
	return nil
}

// SortOps sorts ops in place by (seq asc, opId asc). This is the
// deterministic replay order — never file order.
func SortOps(ops []Op) {
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Seq != ops[j].Seq {
			return ops[i].Seq < ops[j].Seq
		}
		return ops[i].OpID < ops[j].OpID
	})
}

// OpFilename returns the canonical filename for an op: zero-padded seq,
// then the opId, then .json. Zero-padding makes lexicographic order equal
// numeric order.
func OpFilename(seq int, opID string) string {
	return fmt.Sprintf("%016d-%s.json", seq, opID)
}

// ReadOpsDir reads every op file in dir, sorts them by (seq, opId), and
// returns the ops plus warnings for files that could not be parsed.
// Unparseable files are skipped, never fatal — the board must still build.
func ReadOpsDir(dir string) ([]Op, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read ops dir: %w", err)
	}
	var ops []Op
	warnings := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: cannot read: %v", entry.Name(), err))
			continue
		}
		op, err := ParseOp(data)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		ops = append(ops, op)
	}
	SortOps(ops)
	return ops, warnings, nil
}
