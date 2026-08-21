package hexdeck

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureNow is the fixed clock for the golden tests. It is after the
// last op ts in every fixture and within the claim timeout of every
// fixture claim, so the goldens never depend on the wall clock.
var fixtureNow = time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)

// TestFoldGolden runs the fold over fixture board dirs and compares the
// resulting BoardState to a golden file, byte for byte.
func TestFoldGolden(t *testing.T) {
	cases := []string{"basic", "seq-collision", "unparseable", "duplicate-ticket", "missing-ticket"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "ops", name)
			state, err := projectAt(dir, fixtureNow)
			if err != nil {
				t.Fatalf("projectAt: %v", err)
			}
			data, err := json.MarshalIndent(state, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			data = append(data, '\n')
			golden := filepath.Join("testdata", "golden", "fold-"+name+".json")
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

// TestFoldDuplicateTicket checks the duplicate-ticket rule: the first
// ticket.created wins, the second renders a warning.
func TestFoldDuplicateTicket(t *testing.T) {
	state, err := projectAt(filepath.Join("testdata", "ops", "duplicate-ticket"), fixtureNow)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	ticket, ok := state.Tickets["T-1"]
	if !ok {
		t.Fatalf("T-1 missing from state")
	}
	if ticket.Title != "First T-1 wins" {
		t.Errorf("title = %q, want %q", ticket.Title, "First T-1 wins")
	}
	if ticket.Status != "in-progress" {
		t.Errorf("status = %q, want %q", ticket.Status, "in-progress")
	}
	if len(state.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", state.Warnings)
	}
	if state.Warnings[0] != "ticket.created for T-1: ticket already exists, keeping the first" {
		t.Errorf("warning = %q", state.Warnings[0])
	}
}

// TestFoldMissingTicket checks the missing-ticket rule: ops for a ticket
// that was never created are skipped with a warning, never fatal.
func TestFoldMissingTicket(t *testing.T) {
	state, err := projectAt(filepath.Join("testdata", "ops", "missing-ticket"), fixtureNow)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if _, ok := state.Tickets["T-99"]; ok {
		t.Errorf("T-99 must not exist in the state")
	}
	if len(state.Warnings) != 2 {
		t.Fatalf("warnings = %v, want exactly 2", state.Warnings)
	}
	want := []string{
		"ticket.moved for T-99: ticket does not exist, skipping",
		"comment.added for T-99: ticket does not exist, skipping",
	}
	for i := range want {
		if state.Warnings[i] != want[i] {
			t.Errorf("warnings[%d] = %q, want %q", i, state.Warnings[i], want[i])
		}
	}
}

// TestFoldEmptyBoard checks the fold over a board with no ops: the state
// is empty, not an error.
func TestFoldEmptyBoard(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ops"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"empty","columns":["todo","in-progress","review","done"],"claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Name != "empty" {
		t.Errorf("name = %q, want %q", state.Name, "empty")
	}
	if len(state.Tickets) != 0 {
		t.Errorf("tickets = %v, want none", state.Tickets)
	}
	if len(state.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", state.Warnings)
	}
}

// TestFoldMissingConfig checks the fold over a board dir with no
// config.json: the default columns are used, not an error.
func TestFoldMissingConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ops"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	state, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	want := []string{"todo", "in-progress", "review", "done"}
	if len(state.Columns) != len(want) {
		t.Fatalf("columns = %v, want %v", state.Columns, want)
	}
	for i := range want {
		if state.Columns[i] != want[i] {
			t.Errorf("columns[%d] = %q, want %q", i, state.Columns[i], want[i])
		}
	}
}

// TestFoldClaimRace checks the claim-race rule: two writers claim the
// same ticket, the first claim by (seq, opId) wins, the second renders
// a warning.
func TestFoldClaimRace(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"race","columns":["todo","in-progress","review","done"],"claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeOp(t, opsDir, 1, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `{"schema":1,"opId":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","seq":1,"ts":"2026-08-20T14:00:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-1","payload":{"title":"one"}}`)
	writeOp(t, opsDir, 2, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", `{"schema":1,"opId":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","seq":2,"ts":"2026-08-20T14:01:00Z","actor":"claude-a","type":"ticket.claimed","ticket":"T-1","payload":{"by":"claude-a"}}`)
	writeOp(t, opsDir, 2, "cccccccc-cccc-4ccc-8ccc-cccccccccccc", `{"schema":1,"opId":"cccccccc-cccc-4ccc-8ccc-cccccccccccc","seq":2,"ts":"2026-08-20T14:01:00Z","actor":"codex-1","type":"ticket.claimed","ticket":"T-1","payload":{"by":"codex-1"}}`)

	state, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	ticket := state.Tickets["T-1"]
	if ticket.ClaimedBy != "claude-a" {
		t.Errorf("claimedBy = %q, want claude-a (first claim by (seq, opId) wins)", ticket.ClaimedBy)
	}
	if len(state.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", state.Warnings)
	}
	if state.Warnings[0] != "ticket.claimed for T-1: already claimed by claude-a, keeping the first claim" {
		t.Errorf("warning = %q", state.Warnings[0])
	}
}

// TestFoldClaimRaceRelease checks the claim-race rule with a release in
// between: the second claim after a release is a normal claim, not a
// race.
func TestFoldClaimRaceRelease(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"race","columns":["todo","in-progress","review","done"],"claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeOp(t, opsDir, 1, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `{"schema":1,"opId":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","seq":1,"ts":"2026-08-20T14:00:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-1","payload":{"title":"one"}}`)
	writeOp(t, opsDir, 2, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", `{"schema":1,"opId":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","seq":2,"ts":"2026-08-20T14:01:00Z","actor":"claude-a","type":"ticket.claimed","ticket":"T-1","payload":{"by":"claude-a"}}`)
	writeOp(t, opsDir, 3, "cccccccc-cccc-4ccc-8ccc-cccccccccccc", `{"schema":1,"opId":"cccccccc-cccc-4ccc-8ccc-cccccccccccc","seq":3,"ts":"2026-08-20T14:02:00Z","actor":"claude-a","type":"ticket.released","ticket":"T-1","payload":{"by":"claude-a"}}`)
	writeOp(t, opsDir, 4, "dddddddd-dddd-4ddd-8ddd-dddddddddddd", `{"schema":1,"opId":"dddddddd-dddd-4ddd-8ddd-dddddddddddd","seq":4,"ts":"2026-08-20T14:03:00Z","actor":"codex-1","type":"ticket.claimed","ticket":"T-1","payload":{"by":"codex-1"}}`)

	state, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	ticket := state.Tickets["T-1"]
	if ticket.ClaimedBy != "codex-1" {
		t.Errorf("claimedBy = %q, want codex-1 (claim after release is normal)", ticket.ClaimedBy)
	}
	if len(state.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", state.Warnings)
	}
}

// TestFoldStaleClaim checks the stale-claim rule: a claim older than the
// claim timeout is marked stale, a fresh claim is not. The claim still
// stands either way.
func TestFoldStaleClaim(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"stale","columns":["todo","in-progress","review","done"],"claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeOp(t, opsDir, 1, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `{"schema":1,"opId":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","seq":1,"ts":"2026-08-20T14:00:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-1","payload":{"title":"one"}}`)
	writeOp(t, opsDir, 2, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", `{"schema":1,"opId":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","seq":2,"ts":"2026-08-20T14:01:00Z","actor":"claude-a","type":"ticket.claimed","ticket":"T-1","payload":{"by":"claude-a"}}`)
	writeOp(t, opsDir, 3, "cccccccc-cccc-4ccc-8ccc-cccccccccccc", `{"schema":1,"opId":"cccccccc-cccc-4ccc-8ccc-cccccccccccc","seq":3,"ts":"2026-08-20T14:02:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-2","payload":{"title":"two"}}`)
	writeOp(t, opsDir, 4, "dddddddd-dddd-4ddd-8ddd-dddddddddddd", `{"schema":1,"opId":"dddddddd-dddd-4ddd-8ddd-dddddddddddd","seq":4,"ts":"2026-08-20T14:03:00Z","actor":"codex-1","type":"ticket.claimed","ticket":"T-2","payload":{"by":"codex-1"}}`)

	// now is 5 hours after both claims: both are stale.
	now := time.Date(2026, 8, 20, 19, 3, 0, 0, time.UTC)
	state, err := projectAt(dir, now)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	if !state.Tickets["T-1"].ClaimStale {
		t.Errorf("T-1 claim is not stale at %s", now)
	}
	if !state.Tickets["T-2"].ClaimStale {
		t.Errorf("T-2 claim is not stale at %s", now)
	}
	if state.Tickets["T-1"].ClaimedBy != "claude-a" {
		t.Errorf("T-1 claimedBy = %q, want claude-a (the claim still stands)", state.Tickets["T-1"].ClaimedBy)
	}

	// now is 1 hour after the claims: neither is stale.
	now = time.Date(2026, 8, 20, 15, 3, 0, 0, time.UTC)
	state, err = projectAt(dir, now)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	if state.Tickets["T-1"].ClaimStale || state.Tickets["T-2"].ClaimStale {
		t.Errorf("claims marked stale at %s, want fresh", now)
	}
}

// TestFoldStaleClaimRelease checks that a release clears the stale flag.
func TestFoldStaleClaimRelease(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"stale","columns":["todo","in-progress","review","done"],"claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeOp(t, opsDir, 1, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `{"schema":1,"opId":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","seq":1,"ts":"2026-08-20T14:00:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-1","payload":{"title":"one"}}`)
	writeOp(t, opsDir, 2, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", `{"schema":1,"opId":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","seq":2,"ts":"2026-08-20T14:01:00Z","actor":"claude-a","type":"ticket.claimed","ticket":"T-1","payload":{"by":"claude-a"}}`)
	writeOp(t, opsDir, 3, "cccccccc-cccc-4ccc-8ccc-cccccccccccc", `{"schema":1,"opId":"cccccccc-cccc-4ccc-8ccc-cccccccccccc","seq":3,"ts":"2026-08-20T14:02:00Z","actor":"claude-a","type":"ticket.released","ticket":"T-1","payload":{"by":"claude-a"}}`)

	now := time.Date(2026, 8, 20, 19, 3, 0, 0, time.UTC)
	state, err := projectAt(dir, now)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	ticket := state.Tickets["T-1"]
	if ticket.ClaimedBy != "" {
		t.Errorf("claimedBy = %q, want empty after release", ticket.ClaimedBy)
	}
	if ticket.ClaimStale {
		t.Errorf("claim marked stale after release")
	}
}

// TestFoldStaleClaimBadTimeout checks that a missing or unparseable
// claim timeout means no claim is ever stale.
func TestFoldStaleClaimBadTimeout(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"stale","columns":["todo","in-progress","review","done"],"claimTimeout":"not-a-duration","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeOp(t, opsDir, 1, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `{"schema":1,"opId":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","seq":1,"ts":"2026-08-20T14:00:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-1","payload":{"title":"one"}}`)
	writeOp(t, opsDir, 2, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", `{"schema":1,"opId":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","seq":2,"ts":"2026-08-20T14:01:00Z","actor":"claude-a","type":"ticket.claimed","ticket":"T-1","payload":{"by":"claude-a"}}`)

	now := time.Date(2026, 8, 20, 19, 3, 0, 0, time.UTC)
	state, err := projectAt(dir, now)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	if state.Tickets["T-1"].ClaimStale {
		t.Errorf("claim marked stale with a bad timeout, want never stale")
	}
}

// TestFoldMoveFromMismatch checks the from-mismatch warning: a move
// whose payload says from "done" on a ticket actually in "todo" folds
// (the move still applies) but warns — the field carries information
// the fold can check for free.
func TestFoldMoveFromMismatch(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := `{"schema":1,"board":"mismatch","columns":["todo","in-progress","review","done"],"claimTimeout":"4h","autoPush":false}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeOp(t, opsDir, 1, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `{"schema":1,"opId":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","seq":1,"ts":"2026-08-20T14:00:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-1","payload":{"title":"one"}}`)
	writeOp(t, opsDir, 2, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", `{"schema":1,"opId":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","seq":2,"ts":"2026-08-20T14:01:00Z","actor":"claude-a","type":"ticket.moved","ticket":"T-1","payload":{"from":"done","to":"in-progress"}}`)

	now := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	state, err := projectAt(dir, now)
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	if state.Tickets["T-1"].Status != "in-progress" {
		t.Errorf("status = %q, want in-progress — the move must still apply", state.Tickets["T-1"].Status)
	}
	if len(state.Warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", state.Warnings)
	}
	if !strings.Contains(state.Warnings[0], `from "done" does not match current status "todo"`) {
		t.Errorf("warning = %q", state.Warnings[0])
	}
}

// TestFoldMissingConfigNoSnapshotCache checks the config-less board
// path against the snapshot cache: a board without config.json folds,
// writes a snapshot, and the cache is reused on the next read — the
// digest's config-presence marker must make a missing config a
// distinct state, not collide with an empty one.
func TestFoldMissingConfigNoSnapshotCache(t *testing.T) {
	dir := t.TempDir()
	opsDir := filepath.Join(dir, "ops")
	if err := os.Mkdir(opsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No config.json — the fold must use defaults.
	writeOp(t, opsDir, 1, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", `{"schema":1,"opId":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","seq":1,"ts":"2026-08-20T14:00:00Z","actor":"claude-a","type":"ticket.created","ticket":"T-1","payload":{"title":"one"}}`)

	state1, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state1.Name != "" || len(state1.Columns) != 4 {
		t.Fatalf("state = %+v, want defaults for a config-less board", state1)
	}
	// The snapshot must exist (written by the first Project).
	snapPath := filepath.Join(dir, "snapshot.json")
	if _, err := os.Stat(snapPath); err != nil {
		t.Fatalf("snapshot.json not written for a config-less board: %v", err)
	}
	// Prove the cache is reused, not re-folded: corrupt the cached
	// state and the second read must return the corrupted state.
	data, err := os.ReadFile(snapPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	snap.State.Name = "cached-configless"
	rewritten, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(snapPath, rewritten, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	state2, err := Project(dir)
	if err != nil {
		t.Fatalf("Project (second): %v", err)
	}
	if state2.Name != "cached-configless" {
		t.Errorf("name = %q, want the cached name — the snapshot was not reused for a config-less board", state2.Name)
	}
	// A brand-new empty config.json must invalidate the cache (the
	// presence marker), not silently serve the cached state.
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write empty config: %v", err)
	}
	state3, err := Project(dir)
	if err != nil {
		t.Fatalf("Project (with empty config): %v", err)
	}
	if state3.Name == "cached-configless" {
		t.Errorf("empty config did not invalidate the config-less snapshot — the presence marker is missing from the digest")
	}
}

// writeOp writes one op file with the canonical name.
func writeOp(t *testing.T, opsDir string, seq int, opID, data string) {
	t.Helper()
	path := filepath.Join(opsDir, OpFilename(seq, opID))
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write op: %v", err)
	}
}
