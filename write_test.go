package hexdeck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNextTicketID checks the next-ticket-id rule: the highest numeric
// suffix plus one, with the board's prefix.
func TestNextTicketID(t *testing.T) {
	tests := []struct {
		name  string
		state BoardState
		want  string
	}{
		{
			name:  "empty board",
			state: BoardState{Prefix: "T", Tickets: map[string]Ticket{}},
			want:  "T-1",
		},
		{
			name: "gap in numbers",
			state: BoardState{Prefix: "T", Tickets: map[string]Ticket{
				"T-1": {ID: "T-1"},
				"T-3": {ID: "T-3"},
			}},
			want: "T-4",
		},
		{
			name: "custom prefix",
			state: BoardState{Prefix: "HDX", Tickets: map[string]Ticket{
				"HDX-1": {ID: "HDX-1"},
				"HDX-2": {ID: "HDX-2"},
			}},
			want: "HDX-3",
		},
		{
			name: "non-numeric ids ignored",
			state: BoardState{Prefix: "T", Tickets: map[string]Ticket{
				"T-1":   {ID: "T-1"},
				"weird": {ID: "weird"},
			}},
			want: "T-2",
		},
		{
			name:  "empty prefix defaults to T",
			state: BoardState{Tickets: map[string]Ticket{}},
			want:  "T-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextTicketID(tt.state); got != tt.want {
				t.Errorf("NextTicketID = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestAppendOp checks that AppendOp fills the seq, opId, and ts, writes
// the file with the canonical name, and increments the seq per append.
func TestAppendOp(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := json.RawMessage(`{"title":"one"}`)
	op1, err := AppendOp(opsDir, Op{Type: OpTicketCreated, Ticket: "T-1", Actor: "claude-a", Payload: payload})
	if err != nil {
		t.Fatalf("AppendOp: %v", err)
	}
	if op1.Seq != 1 {
		t.Errorf("seq = %d, want 1", op1.Seq)
	}
	if op1.OpID == "" {
		t.Errorf("opId is empty")
	}
	if op1.TS.IsZero() {
		t.Errorf("ts is zero")
	}
	if op1.Schema != SchemaVersion {
		t.Errorf("schema = %d, want %d", op1.Schema, SchemaVersion)
	}
	if _, err := os.Stat(filepath.Join(opsDir, OpFilename(op1.Seq, op1.OpID))); err != nil {
		t.Errorf("op file missing: %v", err)
	}
	op2, err := AppendOp(opsDir, Op{Type: OpTicketCreated, Ticket: "T-2", Actor: "claude-a", Payload: payload})
	if err != nil {
		t.Fatalf("AppendOp: %v", err)
	}
	if op2.Seq != 2 {
		t.Errorf("second seq = %d, want 2", op2.Seq)
	}
	ops, warnings, err := ReadOpsDir(opsDir)
	if err != nil {
		t.Fatalf("ReadOpsDir: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("ops = %d, want 2", len(ops))
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
}

// TestAppendOpInvalid checks that AppendOp refuses an op that does not
// pass validation — nothing is written.
func TestAppendOpInvalid(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := AppendOp(opsDir, Op{Type: OpTicketMoved, Ticket: "T-1", Actor: "x", Payload: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatalf("AppendOp: expected error, got nil")
	}
	entries, err := os.ReadDir(opsDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ops dir has %d files, want 0", len(entries))
	}
}

// TestInitBoard checks that InitBoard creates the full board structure:
// the primer README, the config, the ops dir with the board.created op,
// the rendered board files, and the AGENTS.md discovery hook.
func TestInitBoard(t *testing.T) {
	dir := t.TempDir()
	if err := InitBoard(dir, "my-project", "T", "claude-a"); err != nil {
		t.Fatalf("InitBoard: %v", err)
	}
	boardDir := filepath.Join(dir, ".kanban")

	primer, err := os.ReadFile(filepath.Join(boardDir, "README.md"))
	if err != nil {
		t.Fatalf("read primer: %v", err)
	}
	if !strings.Contains(string(primer), "hexdeck create") {
		t.Errorf("primer does not teach the create command")
	}

	configData, err := os.ReadFile(filepath.Join(boardDir, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if config.Board != "my-project" {
		t.Errorf("config board = %q, want %q", config.Board, "my-project")
	}
	if config.TicketPrefix != "T" {
		t.Errorf("config ticketPrefix = %q, want %q", config.TicketPrefix, "T")
	}
	if len(config.Columns) != 4 {
		t.Errorf("config columns = %v, want 4 default columns", config.Columns)
	}

	state, err := Project(boardDir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Name != "my-project" {
		t.Errorf("state name = %q, want %q", state.Name, "my-project")
	}
	if state.Prefix != "T" {
		t.Errorf("state prefix = %q, want %q", state.Prefix, "T")
	}
	if len(state.Tickets) != 0 {
		t.Errorf("tickets = %v, want none", state.Tickets)
	}

	for _, file := range []string{"board.md", "board.json"} {
		if _, err := os.Stat(filepath.Join(boardDir, file)); err != nil {
			t.Errorf("%s missing after init: %v", file, err)
		}
	}

	agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agents), ".kanban/README.md") {
		t.Errorf("AGENTS.md does not point at the primer: %q", string(agents))
	}
}

// TestInitBoardCustomPrefix checks that the prefix flag lands in the
// config and the projection.
func TestInitBoardCustomPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := InitBoard(dir, "my-project", "HDX", "claude-a"); err != nil {
		t.Fatalf("InitBoard: %v", err)
	}
	state, err := Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Prefix != "HDX" {
		t.Errorf("prefix = %q, want %q", state.Prefix, "HDX")
	}
}

// TestInitBoardTwice checks that init refuses to run over an existing
// board.
func TestInitBoardTwice(t *testing.T) {
	dir := t.TempDir()
	if err := InitBoard(dir, "my-project", "T", "claude-a"); err != nil {
		t.Fatalf("InitBoard: %v", err)
	}
	if err := InitBoard(dir, "my-project", "T", "claude-a"); err == nil {
		t.Fatalf("second InitBoard: expected error, got nil")
	}
}

// TestProjectPrefix checks the prefix default: config wins, missing
// config gives T.
func TestProjectPrefix(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"p","columns":["todo","in-progress","review","done"],"ticketPrefix":"HDX","claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Prefix != "HDX" {
		t.Errorf("prefix = %q, want %q", state.Prefix, "HDX")
	}

	empty := t.TempDir()
	if err := os.Mkdir(filepath.Join(empty, "ops"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	state, err = Project(empty)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Prefix != "T" {
		t.Errorf("default prefix = %q, want %q", state.Prefix, "T")
	}
}

// TestPrefixSort checks that tickets sort numerically with a custom
// prefix: HDX-2 before HDX-10.
func TestPrefixSort(t *testing.T) {
	state := BoardState{
		Name:    "p",
		Prefix:  "HDX",
		Columns: []string{"todo"},
		Tickets: map[string]Ticket{
			"HDX-10": {ID: "HDX-10", Title: "ten", Status: "todo"},
			"HDX-2":  {ID: "HDX-2", Title: "two", Status: "todo"},
		},
	}
	md := string(RenderMarkdown(state))
	two := strings.Index(md, "HDX-2 two")
	ten := strings.Index(md, "HDX-10 ten")
	if two == -1 || ten == -1 {
		t.Fatalf("tickets missing from render:\n%s", md)
	}
	if two > ten {
		t.Errorf("HDX-2 renders after HDX-10:\n%s", md)
	}
}

// TestRenderCheck checks the drift detector: a fresh render passes, a
// hand-edited board file fails with the file named.
func TestRenderCheck(t *testing.T) {
	dir := t.TempDir()
	if err := InitBoard(dir, "my-project", "T", "claude-a"); err != nil {
		t.Fatalf("InitBoard: %v", err)
	}
	boardDir := filepath.Join(dir, ".kanban")
	if _, err := AppendOp(filepath.Join(boardDir, "ops"), Op{
		Type: OpTicketCreated, Ticket: "T-1", Actor: "claude-a",
		Payload: json.RawMessage(`{"title":"one"}`),
	}); err != nil {
		t.Fatalf("AppendOp: %v", err)
	}
	if err := RenderAll(boardDir, false); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	if err := RenderCheck(boardDir); err != nil {
		t.Fatalf("RenderCheck after fresh render: %v", err)
	}

	mdPath := filepath.Join(boardDir, "board.md")
	f, err := os.OpenFile(mdPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open board.md: %v", err)
	}
	if _, err := f.WriteString("\n# hand-edited\n"); err != nil {
		t.Fatalf("edit board.md: %v", err)
	}
	f.Close()

	err = RenderCheck(boardDir)
	if err == nil {
		t.Fatalf("RenderCheck after hand-edit: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "board.md") {
		t.Errorf("error = %q, want it to name board.md", err)
	}
}

// TestRenderAllSVG checks that RenderAll with the svg flag writes
// board.svg too.
func TestRenderAllSVG(t *testing.T) {
	dir := t.TempDir()
	if err := InitBoard(dir, "my-project", "T", "claude-a"); err != nil {
		t.Fatalf("InitBoard: %v", err)
	}
	boardDir := filepath.Join(dir, ".kanban")
	if err := RenderAll(boardDir, true); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "board.svg")); err != nil {
		t.Errorf("board.svg missing: %v", err)
	}
}
