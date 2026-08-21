package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// buildHexdeck builds the binary once per invocation. When the
// HEXDECK_E2E_COVER env var is set, the build is instrumented with
// -cover for every package in the module, so the subprocess coverage
// counts in the honest measurement: the test runner sets GOCOVERDIR,
// the CLI inherits it, and writes its coverage there on exit.
func buildHexdeck(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "hexdeck")
	args := []string{"build", "-o", bin}
	if os.Getenv("HEXDECK_E2E_COVER") != "" {
		args = append(args, "-cover", "-coverpkg=./...")
	}
	args = append(args, "./cmd/hexdeck")
	cmd := exec.Command("go", args...)
	cmd.Dir = packageDir() // the repo root, so -coverpkg covers the module
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build hexdeck: %v\n%s", err, out)
	}
	return bin
}

// packageDir returns the repo root, from the location of this file
// (cmd/hexdeck/main_test.go), so the build works no matter where the
// test binary runs from.
func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	pkg := filepath.Dir(file)              // .../cmd/hexdeck
	return filepath.Dir(filepath.Dir(pkg)) // .../ (the repo root)
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

	out, code = runHexdeck(t, dir, "move", "T-1", "todo", "--as", "claude-a")
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
	if !strings.Contains(out, "claimed by") && !strings.Contains(out, "todo") {
		t.Errorf("show output missing the moved ticket:\n%s", out)
	}
	// T-12: comments live on the ticket view, not the board view.
	if strings.Contains(out, "on it") || strings.Contains(out, "· 1 comment") {
		t.Errorf("show output carries the comment — comments live on the ticket view:\n%s", out)
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
	if t1.Status != "todo" {
		t.Errorf("T-1 status = %q, want todo", t1.Status)
	}
	if len(t1.Comments) != 1 || t1.Comments[0].Text != "on it" {
		t.Errorf("T-1 comments = %v, want one comment", t1.Comments)
	}
}

// TestE2EPickSingleColumn checks pick on a board with one column: the
// claim lands and the ticket stays put — no in-progress column, no
// move.
func TestE2EPickSingleColumn(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	// Shrink the board to one column.
	config := filepath.Join(dir, ".kanban", "config.json")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	one := strings.Replace(string(data), "\"backlog\",\n    \"todo\",\n    \"done\"", "\"todo\"", 1)
	if one == string(data) {
		t.Fatalf("config replace did not match:\n%s", data)
	}
	if err := os.WriteFile(config, []byte(one), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	out, code := runHexdeck(t, dir, "pick", "--as", "claude-a")
	if code != 0 {
		t.Fatalf("pick on a one-column board: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "T-1") {
		t.Errorf("pick output = %q, want T-1", out)
	}
	state, err := hexdeck.Project(filepath.Join(dir, ".kanban"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	t1 := state.Tickets["T-1"]
	if t1.ClaimedBy != "claude-a" {
		t.Errorf("T-1 claimedBy = %q, want claude-a", t1.ClaimedBy)
	}
	if t1.Status != "todo" {
		t.Errorf("T-1 status = %q, want todo — no in-progress column, no move", t1.Status)
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
	if out, code := runHexdeck(t, dir, "move", "T-1", "todo", "--as", "claude-a"); code != 0 {
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

// TestE2EPickRelease checks pick on the default flow: a ticket starts
// in backlog, becomes pickable when moved to todo, and pick claims it
// without moving it — the claim alone marks the pick. Release clears
// the claim.
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
	// A ticket in backlog is not pickable — it must move to todo first.
	out, code := runHexdeck(t, dir, "pick", "--as", "codex-1")
	if code != 0 {
		t.Fatalf("pick: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "no todo tickets to pick") {
		t.Errorf("pick on a backlog-only board = %q, want the empty answer", out)
	}
	if out, code := runHexdeck(t, dir, "move", "T-1", "todo", "--as", "claude-a"); code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "move", "T-2", "todo", "--as", "claude-a"); code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}
	out, code = runHexdeck(t, dir, "pick", "--as", "codex-1")
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
	if t1.Status != "todo" {
		t.Errorf("T-1 status = %q, want todo — the default flow has no in-progress column", t1.Status)
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

// TestE2EActorFromGitConfig checks the actor fallback: with no --as
// flag, the actor comes from git user.name.
func TestE2EActorFromGitConfig(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	ops, _, err := hexdeck.ReadOpsDir(filepath.Join(dir, ".kanban", "ops"))
	if err != nil {
		t.Fatalf("ReadOpsDir: %v", err)
	}
	for _, op := range ops {
		if op.Type == hexdeck.OpTicketCreated && op.Actor != "test-agent" {
			t.Errorf("ticket.created actor = %q, want test-agent (from git user.name)", op.Actor)
		}
	}
}

// TestE2ELogFilters checks the log filters: --since, --actor, and the
// invalid-duration error path.
func TestE2ELogFilters(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	out, code := runHexdeck(t, dir, "log", "--since", "1h")
	if code != 0 {
		t.Fatalf("log --since: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "ticket.created") {
		t.Errorf("log --since 1h missing the recent op:\n%s", out)
	}
	out, code = runHexdeck(t, dir, "log", "--actor", "nobody")
	if code != 0 {
		t.Fatalf("log --actor: exit %d\n%s", code, out)
	}
	if strings.Contains(out, "ticket.created") {
		t.Errorf("log --actor nobody shows ops:\n%s", out)
	}
	if out, code := runHexdeck(t, dir, "log", "--since", "nope"); code == 0 {
		t.Errorf("log --since nope succeeded, want error:\n%s", out)
	}
}

// TestE2EShowClaimed checks show on a claimed ticket prints the claim
// lines.
func TestE2EShowClaimed(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "-d", "the details", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "move", "T-1", "todo", "--as", "claude-a"); code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "pick", "--as", "codex-1"); code != 0 {
		t.Fatalf("pick: exit %d\n%s", code, out)
	}
	out, code := runHexdeck(t, dir, "show", "T-1")
	if code != 0 {
		t.Fatalf("show T-1: exit %d\n%s", code, out)
	}
	for _, want := range []string{"claimed by: codex-1", "description: the details", "status: todo"} {
		if !strings.Contains(out, want) {
			t.Errorf("show T-1 missing %q:\n%s", want, out)
		}
	}
}

// TestE2ERenderCheckDemo checks that render --check passes on the
// committed demo board in docs/demo — the same board CI checks. The
// demo board is the repo's own dogfood: if it drifts, CI must fail.
func TestE2ERenderCheckDemo(t *testing.T) {
	repoRoot := packageDir()
	out, code := runHexdeck(t, repoRoot, "render", "--check", "--dir", "docs/demo")
	if code != 0 {
		t.Fatalf("render --check on docs/demo: exit %d\n%s", code, out)
	}
}

// TestE2ERenderSVG checks render --svg: it writes board.svg, and the
// file is byte-for-byte the library render of the same state.
func TestE2ERenderSVG(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "render", "--svg", "--as", "claude-a"); code != 0 {
		t.Fatalf("render --svg: exit %d\n%s", code, out)
	}
	boardDir := filepath.Join(dir, ".kanban")
	got, err := os.ReadFile(filepath.Join(boardDir, "board.svg"))
	if err != nil {
		t.Fatalf("read board.svg: %v", err)
	}
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	want := hexdeck.RenderSVG(state)
	if !bytes.Equal(got, want) {
		t.Errorf("board.svg does not match the library render:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestE2ESVGFreshAfterWrite checks that a board which opted into
// board.svg never goes stale under the write path: after a move, the
// committed SVG matches a fresh render and render --check passes.
func TestE2ESVGFreshAfterWrite(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "render", "--svg", "--as", "claude-a"); code != 0 {
		t.Fatalf("render --svg: exit %d\n%s", code, out)
	}
	boardDir := filepath.Join(dir, ".kanban")
	before, err := os.ReadFile(filepath.Join(boardDir, "board.svg"))
	if err != nil {
		t.Fatalf("read board.svg: %v", err)
	}
	// A write that changes the board must refresh the SVG.
	if out, code := runHexdeck(t, dir, "move", "T-1", "todo", "--as", "claude-a"); code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}
	after, err := os.ReadFile(filepath.Join(boardDir, "board.svg"))
	if err != nil {
		t.Fatalf("read board.svg after move: %v", err)
	}
	if bytes.Equal(before, after) {
		t.Errorf("board.svg unchanged after a move — the write path did not refresh it")
	}
	// The committed SVG must match the library render and pass the check.
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	want := hexdeck.RenderSVG(state)
	if !bytes.Equal(after, want) {
		t.Errorf("board.svg does not match a fresh render:\n--- got ---\n%s\n--- want ---\n%s", after, want)
	}
	if out, code := runHexdeck(t, dir, "render", "--check", "--as", "claude-a"); code != 0 {
		t.Errorf("render --check after write: exit %d\n%s", code, out)
	}
}

// TestE2ERenderCheckSVG checks that render --check covers board.svg
// once it exists: a hand-edited SVG fails the check with the file
// named.
func TestE2ERenderCheckSVG(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "render", "--svg", "--as", "claude-a"); code != 0 {
		t.Fatalf("render --svg: exit %d\n%s", code, out)
	}
	svgPath := filepath.Join(dir, ".kanban", "board.svg")
	f, err := os.OpenFile(svgPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open board.svg: %v", err)
	}
	if _, err := f.WriteString("\n<!-- hand-edited -->\n"); err != nil {
		t.Fatalf("edit board.svg: %v", err)
	}
	f.Close()
	out, code := runHexdeck(t, dir, "render", "--check", "--as", "claude-a")
	if code == 0 {
		t.Errorf("render --check after SVG hand-edit succeeded, want failure:\n%s", out)
	}
	if !strings.Contains(out, "board.svg") {
		t.Errorf("render --check output = %q, want it to name board.svg", out)
	}
}

// TestReorderArgs checks the argument reordering: flags (and their
// values) move before positionals, --flag=value stays one token, bool
// flags take no value, and `--` ends flag scanning so a positional
// value that starts with `-` can be passed.
func TestReorderArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "flags after positionals", args: []string{"Title", "-d", "desc", "--as", "claude-a"}, want: []string{"-d", "desc", "--as", "claude-a", "Title"}},
		{name: "flags before positionals", args: []string{"--as", "claude-a", "Title"}, want: []string{"--as", "claude-a", "Title"}},
		{name: "bool flag", args: []string{"Title", "--commit"}, want: []string{"--commit", "Title"}},
		{name: "equals form", args: []string{"Title", "-d=desc"}, want: []string{"-d=desc", "Title"}},
		{name: "double dash drops marker", args: []string{"T-1", "--", "-1"}, want: []string{"T-1", "-1"}},
		{name: "double dash then flags are positional", args: []string{"T-1", "--", "--commit"}, want: []string{"T-1", "--commit"}},
		{name: "dash alone is positional", args: []string{"-"}, want: []string{"-"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderArgs(tt.args)
			if len(got) != len(tt.want) {
				t.Fatalf("reorderArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("reorderArgs(%v) = %v, want %v", tt.args, got, tt.want)
					break
				}
			}
		})
	}
}

// TestE2EDashPositional checks the `--` escape hatch end to end: a
// comment whose text starts with a dash is accepted as a positional,
// not mis-parsed as a flag.
func TestE2EDashPositional(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "comment", "T-1", "--as", "claude-a", "--", "-1 is a typo"); code != 0 {
		t.Fatalf("comment with dash text: exit %d\n%s", code, out)
	}
	out, code := runHexdeck(t, dir, "show", "T-1")
	if code != 0 {
		t.Fatalf("show: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "-1 is a typo") {
		t.Errorf("show T-1 missing the dash-leading comment:\n%s", out)
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
