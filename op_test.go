package hexdeck

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestParseOpValid(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "board.created",
			json: `{"schema":1,"opId":"3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"claude-a","type":"board.created","payload":{"name":"my-project"}}`,
		},
		{
			name: "ticket.created",
			json: `{"schema":1,"opId":"3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d","seq":2,"ts":"2026-08-20T14:03:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-1","payload":{"title":"Fix login bug","description":"it is broken"}}`,
		},
		{
			name: "ticket.moved",
			json: `{"schema":1,"opId":"3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d","seq":3,"ts":"2026-08-20T14:03:00Z","actor":"claude-a","type":"ticket.moved","ticket":"T-1","payload":{"from":"todo","to":"in-progress"}}`,
		},
		{
			name: "ticket.updated",
			json: `{"schema":1,"opId":"3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d","seq":4,"ts":"2026-08-20T14:03:00Z","actor":"claude-a","type":"ticket.updated","ticket":"T-1","payload":{"title":"Fix login race"}}`,
		},
		{
			name: "comment.added",
			json: `{"schema":1,"opId":"3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d","seq":5,"ts":"2026-08-20T14:03:00Z","actor":"claude-a","type":"comment.added","ticket":"T-1","payload":{"text":"on it"}}`,
		},
		{
			name: "ticket.claimed",
			json: `{"schema":1,"opId":"3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d","seq":6,"ts":"2026-08-20T14:03:00Z","actor":"claude-a","type":"ticket.claimed","ticket":"T-1","payload":{"by":"claude-a"}}`,
		},
		{
			name: "ticket.released",
			json: `{"schema":1,"opId":"3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d","seq":7,"ts":"2026-08-20T14:03:00Z","actor":"claude-a","type":"ticket.released","ticket":"T-1","payload":{"by":"claude-a"}}`,
		},
		{
			name: "ticket.archived",
			json: `{"schema":1,"opId":"3f2a9c1b-7d4e-4a11-9b2c-0e5f6a7b8c9d","seq":8,"ts":"2026-08-20T14:03:00Z","actor":"claude-a","type":"ticket.archived","ticket":"T-1","payload":{}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := ParseOp([]byte(tt.json))
			if err != nil {
				t.Fatalf("ParseOp: %v", err)
			}
			if op.Type != OpType(tt.name) {
				t.Errorf("type = %q, want %q", op.Type, tt.name)
			}
		})
	}
}

func TestParseOpInvalid(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{
			name:    "not json",
			json:    `{`,
			wantErr: "invalid op JSON",
		},
		{
			name:    "bad schema",
			json:    `{"schema":2,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.archived","ticket":"T-1","payload":{}}`,
			wantErr: "schema must be 1",
		},
		{
			name:    "missing opId",
			json:    `{"schema":1,"seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.archived","ticket":"T-1","payload":{}}`,
			wantErr: "opId is required",
		},
		{
			name:    "seq zero",
			json:    `{"schema":1,"opId":"a","seq":0,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.archived","ticket":"T-1","payload":{}}`,
			wantErr: "seq must be >= 1",
		},
		{
			name:    "bad ts",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"yesterday","actor":"x","type":"ticket.archived","ticket":"T-1","payload":{}}`,
			wantErr: "ts is not a valid RFC3339",
		},
		{
			name:    "missing actor",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","type":"ticket.archived","ticket":"T-1","payload":{}}`,
			wantErr: "actor is required",
		},
		{
			name:    "unknown type",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.deleted","ticket":"T-1","payload":{}}`,
			wantErr: "unknown op type",
		},
		{
			name:    "missing ticket",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.moved","payload":{"from":"todo","to":"done"}}`,
			wantErr: "ticket is required",
		},
		{
			name:    "ticket on board.created",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"board.created","ticket":"T-1","payload":{"name":"p"}}`,
			wantErr: "must not have a ticket",
		},
		{
			name:    "missing payload",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.moved","ticket":"T-1"}`,
			wantErr: "payload is required",
		},
		{
			name:    "bad payload shape",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.moved","ticket":"T-1","payload":{"from":"todo"}}`,
			wantErr: "to is required",
		},
		{
			name:    "empty title",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.created","ticket":"T-1","payload":{"title":""}}`,
			wantErr: "title is required",
		},
		{
			name:    "empty update",
			json:    `{"schema":1,"opId":"a","seq":1,"ts":"2026-08-20T14:03:00Z","actor":"x","type":"ticket.updated","ticket":"T-1","payload":{}}`,
			wantErr: "at least one of title or description",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseOp([]byte(tt.json))
			if err == nil {
				t.Fatalf("ParseOp: expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestSortOps(t *testing.T) {
	ops := []Op{
		{Seq: 2, OpID: "b"},
		{Seq: 1, OpID: "z"},
		{Seq: 2, OpID: "a"},
		{Seq: 1, OpID: "a"},
	}
	SortOps(ops)
	var got []string
	for _, op := range ops {
		got = append(got, op.OpID)
	}
	want := []string{"a", "z", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ops[%d].OpID = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOpFilename(t *testing.T) {
	got := OpFilename(42, "3f2a9c1b")
	want := "0000000000000042-3f2a9c1b.json"
	if got != want {
		t.Errorf("OpFilename = %q, want %q", got, want)
	}
}

func TestReadOpsDirGolden(t *testing.T) {
	cases := []string{"basic", "seq-collision", "unparseable"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "ops", name)
			ops, warnings, err := ReadOpsDir(dir)
			if err != nil {
				t.Fatalf("ReadOpsDir: %v", err)
			}
			got := struct {
				Ops      []Op     `json:"ops"`
				Warnings []string `json:"warnings"`
			}{ops, warnings}
			data, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			data = append(data, '\n')
			golden := filepath.Join("testdata", "golden", name+".json")
			if *update {
				if err := os.WriteFile(golden, data, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(data, want) {
				t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, data, want)
			}
		})
	}
}
