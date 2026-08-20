package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danmurf/hexdeck"
)

// runHexdeck runs the built hexdeck binary in dir and returns its
// combined output and exit code.
func runHexdeck(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	bin := buildHexdeck(t)
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test-agent", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test-agent", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	return string(out), code
}

// buildHexdeck builds the binary once per test run.
func buildHexdeck(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "hexdeck")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join("..", "..", "cmd", "hexdeck")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build hexdeck: %v\n%s", err, out)
	}
	return bin
}

// initRepo makes a temp git repo with a commit, so git commands work.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "test-agent")
	runGit(t, dir, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

// runGit runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test-agent", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test-agent", "GIT_COMMITTER_EMAIL=test@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestE2E runs the full command matrix in a temp repo:
// init → create → move → comment → show → log → render.
func TestE2E(t *testing.T) {
	dir := initRepo(t)

	out, code := runHexdeck(t, dir, "init", "--as", "claude-a")
	if code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "suggested commit: board: init") {
		t.Errorf("init output missing suggested commit:\n%s", out)
	}
	boardDir := filepath.Join(dir, ".kanban")
	for _, file := range []string{"README.md", "config.json", "board.md", "board.json"} {
		if _, err := os.Stat(filepath.Join(boardDir, file)); err != nil {
			t.Errorf("%s missing after init: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md missing after init: %v", err)
	}

	out, code = runHexdeck(t, dir, "create", "Fix login bug", "-d", "it is broken", "--as", "claude-a")
	if code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "T-1") {
		t.Errorf("create output missing ticket id:\n%s", out)
	}

	out, code = runHexdeck(t, dir, "create", "Add sound settings", "--as", "claude-a")
	if code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "T-2") {
		t.Errorf("create output missing ticket id:\n%s", out)
	}

	out, code = runHexdeck(t, dir, "move", "T-1", "in-progress", "--as", "claude-a")
	if code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}

	out, code = runHexdeck(t, dir, "comment", "T-1", "on it", "--as", "claude-a")
	if code != 0 {
		t.Fatalf("comment: exit %d\n%s", code, out)
	}

	out, code = runHexdeck(t, dir, "show")
	if code != 0 {
		t.Fatalf("show: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "Fix login bug") || !strings.Contains(out, "Add sound settings") {
		t.Errorf("show output missing tickets:\n%s", out)
	}
	if !strings.Contains(out, "claimed by") && !strings.Contains(out, "in-progress") {
		t.Errorf("show output missing the moved ticket:\n%s", out)
	}

	out, code = runHexdeck(t, dir, "show", "T-1")
	if code != 0 {
		t.Fatalf("show T-1: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "on it") {
		t.Errorf("show T-1 output missing the comment:\n%s", out)
	}

	out, code = runHexdeck(t, dir, "log")
	if code != 0 {
		t.Fatalf("log: exit %d\n%s", code, out)
	}
	for _, want := range []string{"board.created", "ticket.created", "ticket.moved", "comment.added"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %s:\n%s", want, out)
		}
	}

	out, code = runHexdeck(t, dir, "log", "--ticket", "T-1")
	if code != 0 {
		t.Fatalf("log --ticket: exit %d\n%s", code, out)
	}
	if strings.Contains(out, "T-2") {
		t.Errorf("log --ticket T-1 shows T-2 ops:\n%s", out)
	}

	out, code = runHexdeck(t, dir, "render")
	if code != 0 {
		t.Fatalf("render: exit %d\n%s", code, out)
	}

	out, code = runHexdeck(t, dir, "render", "--check")
	if code != 0 {
		t.Fatalf("render --check: exit %d\n%s", code, out)
	}

	// The board must project correctly after the whole matrix.
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(state.Tickets) != 2 {
		t.Errorf("tickets = %d, want 2", len(state.Tickets))
	}
	t1 := state.Tickets["T-1"]
	if t1.Status != "in-progress" {
		t.Errorf("T-1 status = %q, want in-progress", t1.Status)
	}
	if len(t1.Comments) != 1 || t1.Comments[0].Text != "on it" {
		t.Errorf("T-1 comments = %v, want one comment", t1.Comments)
	}
}

// TestE2ECommit checks --commit: the op and the board files land in a
// commit with the suggested message.
func TestE2ECommit(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a", "--commit"); code != 0 {
		t.Fatalf("create --commit: exit %d\n%s", code, out)
	}
	out := runGitOut(t, dir, "log", "--oneline", "-1")
	if !strings.Contains(out, "board: create T-1") {
		t.Errorf("last commit = %q, want the suggested message", out)
	}
	// The working tree must be clean after --commit.
	out = runGitOut(t, dir, "status", "--porcelain")
	if strings.TrimSpace(out) != "" {
		t.Errorf("working tree not clean after --commit:\n%s", out)
	}
}

// TestE2EStaged checks the same-commit rule: after a move, the op and
// the board files are staged, nothing is committed.
func TestE2EStaged(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "move", "T-1", "in-progress", "--as", "claude-a"); code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}
	out := runGitOut(t, dir, "status", "--porcelain")
	for _, want := range []string{".kanban/ops/", ".kanban/board.md", ".kanban/board.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("staged files missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "??") {
		t.Errorf("unstaged files present:\n%s", out)
	}
}

// TestE2ECustomPrefix checks init --prefix: ticket ids use the prefix.
func TestE2ECustomPrefix(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--prefix", "HDX", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a")
	if code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "HDX-1") {
		t.Errorf("create output = %q, want HDX-1", out)
	}
	state, err := hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if _, ok := state.Tickets["HDX-1"]; !ok {
		t.Errorf("HDX-1 missing from state: %v", state.Tickets)
	}
}

// TestE2EPickRelease checks pick: it claims and moves the next todo
// ticket, and release clears the claim.
func TestE2EPickRelease(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "Two", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	out, code := runHexdeck(t, dir, "pick", "--as", "codex-1")
	if code != 0 {
		t.Fatalf("pick: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "T-1") {
		t.Errorf("pick output = %q, want T-1", out)
	}
	state, err := hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	t1 := state.Tickets["T-1"]
	if t1.ClaimedBy != "codex-1" {
		t.Errorf("T-1 claimedBy = %q, want codex-1", t1.ClaimedBy)
	}
	if t1.Status != "in-progress" {
		t.Errorf("T-1 status = %q, want in-progress", t1.Status)
	}
	if t1.ClaimedAt == nil {
		t.Errorf("T-1 claimedAt is nil")
	}

	// The second pick takes T-2, not T-1 again.
	out, code = runHexdeck(t, dir, "pick", "--as", "claude-a")
	if code != 0 {
		t.Fatalf("pick: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "T-2") {
		t.Errorf("second pick output = %q, want T-2", out)
	}

	if out, code := runHexdeck(t, dir, "release", "T-1", "--as", "codex-1"); code != 0 {
		t.Fatalf("release: exit %d\n%s", code, out)
	}
	state, err = hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if state.Tickets["T-1"].ClaimedBy != "" {
		t.Errorf("T-1 still claimed after release: %q", state.Tickets["T-1"].ClaimedBy)
	}
}

// TestE2EErrors checks the error paths: unknown ticket, unknown column,
// unknown command, and init over an existing board.
func TestE2EErrors(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code == 0 {
		t.Errorf("second init succeeded, want error:\n%s", out)
	}
	if out, code := runHexdeck(t, dir, "move", "T-99", "done", "--as", "claude-a"); code == 0 {
		t.Errorf("move of unknown ticket succeeded, want error:\n%s", out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "move", "T-1", "nope", "--as", "claude-a"); code == 0 {
		t.Errorf("move to unknown column succeeded, want error:\n%s", out)
	}
	if out, code := runHexdeck(t, dir, "frobnicate"); code == 0 {
		t.Errorf("unknown command succeeded, want error:\n%s", out)
	}
}

// TestE2ERenderCheckDrift checks that render --check fails after a
// hand-edit of board.md.
func TestE2ERenderCheckDrift(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	mdPath := filepath.Join(dir, ".kanban", "board.md")
	f, err := os.OpenFile(mdPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open board.md: %v", err)
	}
	if _, err := f.WriteString("\n# hand-edited\n"); err != nil {
		t.Fatalf("edit board.md: %v", err)
	}
	f.Close()
	out, code := runHexdeck(t, dir, "render", "--check")
	if code == 0 {
		t.Errorf("render --check after hand-edit succeeded, want failure:\n%s", out)
	}
	if !strings.Contains(out, "board.md") {
		t.Errorf("render --check output = %q, want it to name board.md", out)
	}
}

// TestE2EShowJSON checks show --json prints the machine view.
func TestE2EShowJSON(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	out, code := runHexdeck(t, dir, "show", "--json")
	if code != 0 {
		t.Fatalf("show --json: exit %d\n%s", code, out)
	}
	var state hexdeck.BoardState
	if err := json.Unmarshal([]byte(out), &state); err != nil {
		t.Fatalf("show --json output is not valid JSON: %v\n%s", err, out)
	}
	if len(state.Tickets) != 1 {
		t.Errorf("tickets = %d, want 1", len(state.Tickets))
	}
}

// runGitOut runs a git command and returns its output, failing the test
// on error.
func runGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test-agent", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test-agent", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
