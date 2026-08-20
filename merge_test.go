package hexdeck

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runGit runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGitCmd(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

// runGitAllowFail runs a git command in dir and returns the output and
// the exit code. Used for commands that may stop on a conflict.
func runGitAllowFail(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	out, err := runGitCmd(dir, args...)
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	return out, code
}

// runGitCmd runs a git command in dir with a fixed identity and a
// non-interactive editor.
func runGitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test-agent", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test-agent", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_EDITOR=true")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// matrixRepo is a bare origin repo plus two clones — one per writer.
type matrixRepo struct {
	origin string
	cloneA string
	cloneB string
}

// setupMatrix builds a bare origin repo with a board holding two
// tickets, and two clones of it.
func setupMatrix(t *testing.T) matrixRepo {
	t.Helper()
	work := t.TempDir()
	runGit(t, work, "init", "-q", "-b", "main")
	if err := InitBoard(work, "matrix", "T", "claude-a"); err != nil {
		t.Fatalf("InitBoard: %v", err)
	}
	boardDir := filepath.Join(work, ".kanban")
	for _, op := range []Op{
		{Type: OpTicketCreated, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"title":"one"}`)},
		{Type: OpTicketCreated, Ticket: "T-2", Actor: "claude-a", Payload: json.RawMessage(`{"title":"two"}`)},
	} {
		if _, err := AppendOp(filepath.Join(boardDir, "ops"), op); err != nil {
			t.Fatalf("AppendOp: %v", err)
		}
	}
	if err := RenderAll(boardDir, false); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	runGit(t, work, "add", "-A")
	runGit(t, work, "commit", "-q", "-m", "board")

	origin := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, filepath.Dir(origin), "clone", "-q", "--bare", work, origin)
	cloneA := filepath.Join(t.TempDir(), "a")
	cloneB := filepath.Join(t.TempDir(), "b")
	runGit(t, filepath.Dir(cloneA), "clone", "-q", origin, cloneA)
	runGit(t, filepath.Dir(cloneB), "clone", "-q", origin, cloneB)
	return matrixRepo{origin: origin, cloneA: cloneA, cloneB: cloneB}
}

// appendAndCommit appends the ops in a clone, re-renders, and commits —
// one writer's side of a concurrent change.
func appendAndCommit(t *testing.T, clone string, ops []Op) {
	t.Helper()
	boardDir := filepath.Join(clone, ".kanban")
	for _, op := range ops {
		if _, err := AppendOp(filepath.Join(boardDir, "ops"), op); err != nil {
			t.Fatalf("AppendOp: %v", err)
		}
	}
	if err := RenderAll(boardDir, false); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	runGit(t, clone, "add", "-A")
	runGit(t, clone, "commit", "-q", "-m", "writer")
}

// mergeAndVerify merges the two writers' work and checks the result:
// zero conflicts, identical projections on both sides, and committed
// board files that match the ops.
func mergeAndVerify(t *testing.T, m matrixRepo) {
	t.Helper()
	// A pushes first.
	runGit(t, m.cloneA, "push", "-q", "origin", "main")
	// B pulls with rebase. The ops merge cleanly — unique files never
	// conflict. The board files may conflict; the resolution is
	// mechanical: re-render, never hand-edit.
	out, code := runGitAllowFail(t, m.cloneB, "pull", "-q", "--rebase", "origin", "main")
	if code != 0 {
		if !strings.Contains(out, "CONFLICT") {
			t.Fatalf("pull --rebase failed: %v\n%s", code, out)
		}
		if err := RenderAll(filepath.Join(m.cloneB, ".kanban"), false); err != nil {
			t.Fatalf("RenderAll after merge: %v", err)
		}
		runGit(t, m.cloneB, "add", "-A")
		runGit(t, m.cloneB, "rebase", "--continue")
	}
	// No conflict markers anywhere.
	if conflicts := findConflictMarkers(t, m.cloneB); len(conflicts) > 0 {
		t.Fatalf("merge left conflict markers: %v", conflicts)
	}
	// B pushes the merged result; A pulls it. Both sides now hold the
	// same ops.
	runGit(t, m.cloneB, "push", "-q", "origin", "main")
	runGit(t, m.cloneA, "pull", "-q", "--rebase", "origin", "main")
	// Re-render both sides — the committed board files must match the
	// merged ops.
	if err := RenderAll(filepath.Join(m.cloneA, ".kanban"), false); err != nil {
		t.Fatalf("RenderAll A: %v", err)
	}
	if err := RenderAll(filepath.Join(m.cloneB, ".kanban"), false); err != nil {
		t.Fatalf("RenderAll B: %v", err)
	}
	// The projections must be identical on both sides. One clock for
	// both, so claim staleness cannot differ.
	now := time.Now()
	stateA, err := projectAt(filepath.Join(m.cloneA, ".kanban"), now)
	if err != nil {
		t.Fatalf("projectAt A: %v", err)
	}
	stateB, err := projectAt(filepath.Join(m.cloneB, ".kanban"), now)
	if err != nil {
		t.Fatalf("projectAt B: %v", err)
	}
	dataA, err := RenderJSON(stateA)
	if err != nil {
		t.Fatalf("RenderJSON A: %v", err)
	}
	dataB, err := RenderJSON(stateB)
	if err != nil {
		t.Fatalf("RenderJSON B: %v", err)
	}
	if !bytes.Equal(dataA, dataB) {
		t.Errorf("projections differ after merge\n--- A ---\n%s\n--- B ---\n%s", dataA, dataB)
	}
	// The committed board files must match the ops on both sides.
	if err := RenderCheck(filepath.Join(m.cloneA, ".kanban")); err != nil {
		t.Errorf("RenderCheck A after merge: %v", err)
	}
	if err := RenderCheck(filepath.Join(m.cloneB, ".kanban")); err != nil {
		t.Errorf("RenderCheck B after merge: %v", err)
	}
}

// findConflictMarkers walks dir and returns the paths of files that
// contain git conflict markers.
func findConflictMarkers(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte("<<<<<<<")) || bytes.Contains(data, []byte(">>>>>>>")) {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return found
}

// TestMergeMatrix runs the concurrency matrix: two writers in two
// clones append ops at the same time, then merge. Every scenario must
// merge with zero conflicts, and the projection must be identical on
// both sides after the merge.
func TestMergeMatrix(t *testing.T) {
	scenarios := []struct {
		name string
		a, b []Op
	}{
		{
			name: "both create different tickets",
			a:    []Op{{Type: OpTicketCreated, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"title":"three"}`)}},
			b:    []Op{{Type: OpTicketCreated, Ticket: "T-4", Actor: "codex-1", Payload: json.RawMessage(`{"title":"four"}`)}},
		},
		{
			name: "both create the same ticket id",
			a:    []Op{{Type: OpTicketCreated, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"title":"three"}`)}},
			b:    []Op{{Type: OpTicketCreated, Ticket: "T-3", Actor: "codex-1", Payload: json.RawMessage(`{"title":"three again"}`)}},
		},
		{
			name: "both move the same ticket",
			a:    []Op{{Type: OpTicketMoved, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"from":"todo","to":"in-progress"}`)}},
			b:    []Op{{Type: OpTicketMoved, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"from":"todo","to":"review"}`)}},
		},
		{
			name: "both move different tickets",
			a:    []Op{{Type: OpTicketMoved, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"from":"todo","to":"in-progress"}`)}},
			b:    []Op{{Type: OpTicketMoved, Ticket: "T-2", Actor: "codex-1", Payload: json.RawMessage(`{"from":"todo","to":"review"}`)}},
		},
		{
			name: "both comment on the same ticket",
			a:    []Op{{Type: OpCommentAdded, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"text":"a"}`)}},
			b:    []Op{{Type: OpCommentAdded, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"text":"b"}`)}},
		},
		{
			name: "both comment on different tickets",
			a:    []Op{{Type: OpCommentAdded, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"text":"a"}`)}},
			b:    []Op{{Type: OpCommentAdded, Ticket: "T-2", Actor: "codex-1", Payload: json.RawMessage(`{"text":"b"}`)}},
		},
		{
			name: "both claim the same ticket",
			a:    []Op{{Type: OpTicketClaimed, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"by":"claude-a"}`)}},
			b:    []Op{{Type: OpTicketClaimed, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"by":"codex-1"}`)}},
		},
		{
			name: "both claim different tickets",
			a:    []Op{{Type: OpTicketClaimed, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"by":"claude-a"}`)}},
			b:    []Op{{Type: OpTicketClaimed, Ticket: "T-2", Actor: "codex-1", Payload: json.RawMessage(`{"by":"codex-1"}`)}},
		},
		{
			name: "one moves, one comments",
			a:    []Op{{Type: OpTicketMoved, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"from":"todo","to":"in-progress"}`)}},
			b:    []Op{{Type: OpCommentAdded, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"text":"b"}`)}},
		},
		{
			name: "one claims, one releases",
			a:    []Op{{Type: OpTicketClaimed, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"by":"claude-a"}`)}},
			b:    []Op{{Type: OpTicketReleased, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"by":"codex-1"}`)}},
		},
		{
			name: "one claims, one moves",
			a:    []Op{{Type: OpTicketClaimed, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"by":"claude-a"}`)}},
			b:    []Op{{Type: OpTicketMoved, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"from":"todo","to":"in-progress"}`)}},
		},
		{
			name: "one creates, one moves the same new ticket",
			a:    []Op{{Type: OpTicketCreated, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"title":"three"}`)}},
			b:    []Op{{Type: OpTicketMoved, Ticket: "T-3", Actor: "codex-1", Payload: json.RawMessage(`{"from":"todo","to":"in-progress"}`)}},
		},
		{
			name: "one creates, one claims the same new ticket",
			a:    []Op{{Type: OpTicketCreated, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"title":"three"}`)}},
			b:    []Op{{Type: OpTicketClaimed, Ticket: "T-3", Actor: "codex-1", Payload: json.RawMessage(`{"by":"codex-1"}`)}},
		},
		{
			name: "one creates, one comments on an existing ticket",
			a:    []Op{{Type: OpTicketCreated, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"title":"three"}`)}},
			b:    []Op{{Type: OpCommentAdded, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"text":"b"}`)}},
		},
		{
			name: "both release the same ticket",
			a:    []Op{{Type: OpTicketReleased, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"by":"claude-a"}`)}},
			b:    []Op{{Type: OpTicketReleased, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"by":"codex-1"}`)}},
		},
		{
			name: "one archives, one comments",
			a:    []Op{{Type: OpTicketArchived, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{}`)}},
			b:    []Op{{Type: OpCommentAdded, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"text":"b"}`)}},
		},
		{
			name: "both append two ops",
			a: []Op{
				{Type: OpTicketCreated, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"title":"three"}`)},
				{Type: OpCommentAdded, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"text":"a"}`)},
			},
			b: []Op{
				{Type: OpTicketCreated, Ticket: "T-4", Actor: "codex-1", Payload: json.RawMessage(`{"title":"four"}`)},
				{Type: OpCommentAdded, Ticket: "T-2", Actor: "codex-1", Payload: json.RawMessage(`{"text":"b"}`)},
			},
		},
		{
			name: "both create and move their own new ticket",
			a: []Op{
				{Type: OpTicketCreated, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"title":"three"}`)},
				{Type: OpTicketMoved, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"from":"todo","to":"in-progress"}`)},
			},
			b: []Op{
				{Type: OpTicketCreated, Ticket: "T-4", Actor: "codex-1", Payload: json.RawMessage(`{"title":"four"}`)},
				{Type: OpTicketMoved, Ticket: "T-4", Actor: "codex-1", Payload: json.RawMessage(`{"from":"todo","to":"review"}`)},
			},
		},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			m := setupMatrix(t)
			appendAndCommit(t, m.cloneA, sc.a)
			appendAndCommit(t, m.cloneB, sc.b)
			mergeAndVerify(t, m)
		})
	}
}

// TestMergeMatrixStaleCheckout checks the stale-checkout scenario:
// writer B appends after writer A pushed, without pulling first. The
// merge is still clean and the projections identical.
func TestMergeMatrixStaleCheckout(t *testing.T) {
	m := setupMatrix(t)
	appendAndCommit(t, m.cloneA, []Op{
		{Type: OpTicketCreated, Ticket: "T-3", Actor: "claude-a", Payload: json.RawMessage(`{"title":"three"}`)},
	})
	runGit(t, m.cloneA, "push", "-q", "origin", "main")
	// B never pulled — a stale checkout. Its ops get seqs that A
	// already used. That is expected and harmless.
	appendAndCommit(t, m.cloneB, []Op{
		{Type: OpTicketCreated, Ticket: "T-4", Actor: "codex-1", Payload: json.RawMessage(`{"title":"four"}`)},
	})
	mergeAndVerify(t, m)
}

// TestMergeMatrixClaimRaceResolution checks the claim-race scenario in
// detail: after the merge, the first claim by (seq, opId) wins and the
// second renders a warning.
func TestMergeMatrixClaimRaceResolution(t *testing.T) {
	m := setupMatrix(t)
	appendAndCommit(t, m.cloneA, []Op{
		{Type: OpTicketClaimed, Ticket: "T-1", Actor: "claude-a", Payload: json.RawMessage(`{"by":"claude-a"}`)},
	})
	appendAndCommit(t, m.cloneB, []Op{
		{Type: OpTicketClaimed, Ticket: "T-1", Actor: "codex-1", Payload: json.RawMessage(`{"by":"codex-1"}`)},
	})
	mergeAndVerify(t, m)

	state, err := projectAt(filepath.Join(m.cloneA, ".kanban"), time.Now())
	if err != nil {
		t.Fatalf("projectAt: %v", err)
	}
	ticket := state.Tickets["T-1"]
	// The opIds are random, so the winner is whichever claim sorts
	// first by (seq, opId). The invariant: exactly one claim won, and
	// the warning names the winner.
	if ticket.ClaimedBy != "claude-a" && ticket.ClaimedBy != "codex-1" {
		t.Errorf("claimedBy = %q, want one of the two claimants", ticket.ClaimedBy)
	}
	found := false
	for _, warning := range state.Warnings {
		if strings.Contains(warning, "already claimed by "+ticket.ClaimedBy) {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings = %v, want the claim-race warning naming %s", state.Warnings, ticket.ClaimedBy)
	}
}
