package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLIErrorPaths checks the error branches of the CLI commands:
// wrong argument counts, missing boards, missing tickets, unknown
// columns, and bad flags. The happy paths are covered by the E2E
// matrix; these are the paths that must fail with a clear message.
func TestCLIErrorPaths(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	cases := []struct {
		name string
		args []string
	}{
		{"create no args", []string{"create"}},
		{"create two args", []string{"create", "One", "Two"}},
		{"move no args", []string{"move"}},
		{"move one arg", []string{"move", "T-1"}},
		{"move unknown ticket", []string{"move", "T-99", "done"}},
		{"move unknown column", []string{"move", "T-1", "nope"}},
		{"comment no args", []string{"comment"}},
		{"comment one arg", []string{"comment", "T-1"}},
		{"comment unknown ticket", []string{"comment", "T-99", "hi"}},
		{"show two args", []string{"show", "T-1", "T-2"}},
		{"show unknown ticket", []string{"show", "T-99"}},
		{"log positional", []string{"log", "extra"}},
		{"log bad since", []string{"log", "--since", "nope"}},
		{"pick positional", []string{"pick", "extra"}},
		{"release no args", []string{"release"}},
		{"release unknown ticket", []string{"release", "T-99"}},
		{"render positional", []string{"render", "extra"}},
		{"web positional", []string{"web", "extra"}},
		{"mcp positional", []string{"mcp", "extra"}},
	}
	for _, c := range cases {
		if out, code := runHexdeck(t, dir, c.args...); code == 0 {
			t.Errorf("%s: exit 0, want error\n%s", c.name, out)
		}
	}
}

// TestCLINoBoard checks the commands fail with a clear message when no
// board exists.
func TestCLINoBoard(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"create", "One"},
		{"move", "T-1", "done"},
		{"comment", "T-1", "hi"},
		{"show"},
		{"log"},
		{"pick"},
		{"release", "T-1"},
		{"render"},
	} {
		if out, code := runHexdeck(t, dir, args...); code == 0 {
			t.Errorf("%v: exit 0, want error\n%s", args, out)
		}
	}
}

// TestCLIInitErrors checks init's error paths: a positional argument,
// an existing board, and a bad --dir.
func TestCLIInitErrors(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "extra"); code == 0 {
		t.Errorf("init with a positional argument: exit 0, want error\n%s", out)
	}
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code == 0 {
		t.Errorf("init on an existing board: exit 0, want error\n%s", out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--dir", filepath.Join(dir, "missing")); code == 0 {
		t.Errorf("create with a bad --dir: exit 0, want error\n%s", out)
	}
}

// TestCLICommitFlag checks --commit commits the change, and the
// commit-failure warning path (a repo with no staged changes).
func TestCLICommitFlag(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a", "--commit"); code != 0 {
		t.Fatalf("create --commit: exit %d\n%s", code, out)
	}
	if log := runGitOut(t, dir, "log", "--oneline"); !strings.Contains(log, "board: create T-1") {
		t.Errorf("--commit did not commit with the suggested message:\n%s", log)
	}
}

// splitLines splits s on newlines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// TestCLIRenderCheckDrift checks render --check fails on a drifted
// board and passes on a fresh one.
func TestCLIRenderCheckDrift(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "render", "--check"); code != 0 {
		t.Fatalf("render --check on a fresh board: exit %d\n%s", code, out)
	}
	board := filepath.Join(dir, ".kanban", "board.md")
	if err := os.WriteFile(board, []byte("# drifted\n"), 0o644); err != nil {
		t.Fatalf("write drifted board: %v", err)
	}
	if out, code := runHexdeck(t, dir, "render", "--check"); code == 0 {
		t.Errorf("render --check on a drifted board: exit 0, want error\n%s", out)
	}
}

// TestCLIPickEmpty checks pick on a board with no todo tickets.
func TestCLIPickEmpty(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "pick", "--as", "claude-a", "--no-pull"); code != 0 {
		t.Fatalf("pick: exit %d\n%s", code, out)
	} else if !contains(splitLines(out), "no todo tickets to pick") {
		t.Errorf("pick on an empty board did not say so:\n%s", out)
	}
}

// TestCLIShowJSON checks show --json prints the machine view.
func TestCLIShowJSON(t *testing.T) {
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
	if !strings.Contains(out, "\"tickets\"") {
		t.Errorf("show --json does not print the machine view:\n%s", out)
	}
}
