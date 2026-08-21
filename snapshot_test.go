package hexdeck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// snapshotTestNow is the fixed clock for snapshot tests, after every op
// in the fixtures and within every claim timeout.
var snapshotTestNow = time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)

// copyFixture copies a testdata/ops case into a temp dir so tests can
// mutate it freely. The golden tests use projectAt on the read-only
// fixtures; snapshot tests need a writable copy.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", "ops", name)
	dst := t.TempDir()
	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		t.Fatalf("copy fixture %s: %v", name, err)
	}
	return dst
}

// appendTestOp writes one op file directly into a board's ops dir,
// bypassing AppendOp so the test controls the seq and opId.
func appendTestOp(t *testing.T, opsDir string, op Op) {
	t.Helper()
	data, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("marshal op: %v", err)
	}
	if err := os.WriteFile(filepath.Join(opsDir, OpFilename(op.Seq, op.OpID)), data, 0o644); err != nil {
		t.Fatalf("write op: %v", err)
	}
}

// mustPayload marshals v to a JSON payload, failing the test on error.
func mustPayload(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

// TestSnapshotCreatedByProject checks that Project writes snapshot.json
// into the board dir, holding a valid schema and the folded state.
func TestSnapshotCreatedByProject(t *testing.T) {
	dir := copyFixture(t, "basic")
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		t.Fatalf("snapshot.json not written: %v", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("snapshot is not valid JSON: %v", err)
	}
	if snap.Schema != SchemaVersion {
		t.Errorf("snapshot schema = %d, want %d", snap.Schema, SchemaVersion)
	}
	if snap.Digest == "" {
		t.Errorf("snapshot digest is empty")
	}
	if len(snap.State.Tickets) != len(state.Tickets) {
		t.Errorf("snapshot holds %d tickets, Project returned %d", len(snap.State.Tickets), len(state.Tickets))
	}
}

// TestSnapshotReuse checks that a valid snapshot is reused: mutate the
// snapshot's stored state and a subsequent Project must return the
// mutated state — proving the cache produced the board, not a re-fold.
func TestSnapshotReuse(t *testing.T) {
	dir := copyFixture(t, "basic")
	if _, err := Project(dir); err != nil {
		t.Fatalf("project: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	snap.State.Name = "cached-name"
	rewritten, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), rewritten, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if state.Name != "cached-name" {
		t.Errorf("board name = %q, want the cached name — the snapshot was not reused", state.Name)
	}
}

// TestSnapshotInvalidatedByNewOp checks that an op added after the
// snapshot was taken invalidates it: the board must include the new op.
func TestSnapshotInvalidatedByNewOp(t *testing.T) {
	dir := copyFixture(t, "basic")
	if _, err := Project(dir); err != nil {
		t.Fatalf("project: %v", err)
	}
	// Append a new op (seq 100 — after everything in the fixture).
	appendTestOp(t, filepath.Join(dir, "ops"), Op{
		Schema:  SchemaVersion,
		OpID:    "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		Seq:     100,
		TS:      snapshotTestNow.Add(time.Hour),
		Actor:   "alice",
		Type:    OpTicketMoved,
		Ticket:  "T-1",
		Payload: mustPayload(t, map[string]string{"from": "todo", "to": "done"}),
	})
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if got := state.Tickets["T-1"].Status; got != "done" {
		t.Errorf("T-1 status = %q, want done — the snapshot swallowed the new op", got)
	}
}

// TestSnapshotCorruptFallsBack checks that a corrupt snapshot file does
// not break the board: the fold falls back to a full replay.
func TestSnapshotCorruptFallsBack(t *testing.T) {
	dir := copyFixture(t, "basic")
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write corrupt snapshot: %v", err)
	}
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("project with corrupt snapshot: %v", err)
	}
	if state.Name == "" {
		t.Errorf("board has no name — the corrupt snapshot aborted the fold")
	}
}

// TestSnapshotConfigChangeInvalidates checks that a changed config.json
// invalidates the snapshot: the board reflects the new config. The
// columns field is the probe because the fold reads it from config
// (the board.created op does not override columns).
func TestSnapshotConfigChangeInvalidates(t *testing.T) {
	dir := copyFixture(t, "basic")
	state0, err := Project(dir)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if len(state0.Columns) != 4 {
		t.Fatalf("columns = %v, want the fixture's 4", state0.Columns)
	}
	config := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	replaced := strings.Replace(string(data), `"todo", "in-progress", "review", "done"`, `"todo", "in-progress"`, 1)
	if replaced == string(data) {
		t.Fatalf("config replace did not match:\n%s", data)
	}
	if err := os.WriteFile(config, []byte(replaced), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if len(state.Columns) != 2 || state.Columns[1] != "in-progress" {
		t.Errorf("columns = %v, want [todo in-progress] — the snapshot was not invalidated by the config change", state.Columns)
	}
}

// TestRenderCheckDoesNotTrustSnapshot checks the honesty gate: even
// when the snapshot cache is corrupt, RenderCheck compares the
// committed files to a cold fold of the ops — the cache can never make
// the gate lie.
func TestRenderCheckDoesNotTrustSnapshot(t *testing.T) {
	dir := copyFixture(t, "basic")
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := RenderAll(dir, false); err != nil {
		t.Fatalf("render: %v", err)
	}
	// Corrupt the cache on purpose: change the stored board name.
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	snap.State.Name = "different"
	rewritten, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), rewritten, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	// RenderCheck compares to a cold fold — the corrupted cache must
	// not change the verdict.
	if err := RenderCheck(dir); err != nil {
		t.Errorf("RenderCheck failed with a corrupt snapshot: %v", err)
	}
	// And the same rendered files must match a clean re-render.
	if err := RenderCheck(dir); err != nil {
		t.Errorf("RenderCheck failed: %v", err)
	}
	_ = state
}

// TestSnapshotGitignore checks that a snapshot write ensures a
// .kanban/.gitignore that hides snapshot.json, so the cache can never
// be committed.
func TestSnapshotGitignore(t *testing.T) {
	dir := copyFixture(t, "basic")
	if _, err := Project(dir); err != nil {
		t.Fatalf("project: %v", err)
	}
	ignore := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(ignore)
	if err != nil {
		t.Fatalf(".gitignore not written: %v", err)
	}
	if !strings.Contains(string(data), "snapshot.json") {
		t.Errorf(".gitignore does not mention snapshot.json:\n%s", data)
	}
}

// TestSnapshotGitignorePreservesUserContent checks that repairing the
// .gitignore appends the snapshot line instead of replacing the file —
// user entries must survive a routine cache write.
func TestSnapshotGitignorePreservesUserContent(t *testing.T) {
	dir := copyFixture(t, "basic")
	ignore := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(ignore, []byte("local-config.json\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	if _, err := Project(dir); err != nil {
		t.Fatalf("project: %v", err)
	}
	data, err := os.ReadFile(ignore)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), "local-config.json") {
		t.Errorf("user entry lost — .gitignore now:\n%s", data)
	}
	if !strings.Contains(string(data), "snapshot.json") {
		t.Errorf(".gitignore does not mention snapshot.json:\n%s", data)
	}
}

// TestRenderClockIndependence checks the honesty guarantee: the
// committed files are a pure function of the ops. A live claim old
// enough to be stale by the real wall clock must not put a stale
// marker in board.md or board.json — that marker is display-only and
// belongs to the interactive paths. Without this guarantee, CI's
// render --check would fail on a board whose ops did not move.
func TestRenderClockIndependence(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ops"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"clock","columns":["todo","in-progress","review","done"],"claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// A claim 10 hours old: stale by any real clock with a 4h timeout.
	old := time.Now().UTC().Add(-10 * time.Hour).Truncate(time.Second)
	ops := []Op{
		{Schema: SchemaVersion, OpID: "11111111-1111-4111-8111-111111111111", Seq: 1, TS: old.Add(-time.Hour), Actor: "claude-a", Type: OpBoardCreated, Payload: mustPayload(t, BoardCreatedPayload{Name: "clock"})},
		{Schema: SchemaVersion, OpID: "22222222-2222-4222-8222-222222222222", Seq: 2, TS: old, Actor: "claude-a", Type: OpTicketCreated, Ticket: "T-1", Payload: mustPayload(t, TicketCreatedPayload{Title: "stale claim ticket"})},
		{Schema: SchemaVersion, OpID: "33333333-3333-4333-8333-333333333333", Seq: 3, TS: old, Actor: "claude-a", Type: OpTicketClaimed, Ticket: "T-1", Payload: mustPayload(t, TicketClaimedPayload{By: "claude-a"})},
	}
	for _, op := range ops {
		data, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("marshal op: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ops", OpFilename(op.Seq, op.OpID)), data, 0o644); err != nil {
			t.Fatalf("write op: %v", err)
		}
	}
	if err := RenderAll(dir, false); err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := RenderCheck(dir); err != nil {
		t.Fatalf("render check: %v", err)
	}
	md, err := os.ReadFile(filepath.Join(dir, "board.md"))
	if err != nil {
		t.Fatalf("read board.md: %v", err)
	}
	if strings.Contains(string(md), "(stale claim)") {
		t.Errorf("board.md contains a stale-claim marker — committed renders must be a pure function of the ops:\n%s", md)
	}
	bj, err := os.ReadFile(filepath.Join(dir, "board.json"))
	if err != nil {
		t.Fatalf("read board.json: %v", err)
	}
	if strings.Contains(string(bj), "claimStale") {
		t.Errorf("board.json contains a stale-claim marker:\n%s", bj)
	}
	// The interactive path still marks it stale: Project must report
	// the claim as stale at the real clock.
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if !state.Tickets["T-1"].ClaimStale {
		t.Errorf("Project did not mark the aged claim stale — the display path lost the marker")
	}
}
