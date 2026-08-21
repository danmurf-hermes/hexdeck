package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// TestWebPageHandler checks the page handler directly: GET / serves
// the embedded page, and any other path is a 404.
func TestWebPageHandler(t *testing.T) {
	s, _ := newWebTestServer(t)
	rec := doJSON(t, s, "GET", "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hexdeck") {
		t.Error("GET / does not serve the web page")
	}
	req := httptest.NewRequest("GET", "/nope", nil)
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope: status %d, want 404", rec.Code)
	}
}

// TestMCPRunErrors checks the runMCP error paths: positional
// arguments and a missing board dir.
func TestMCPRunErrors(t *testing.T) {
	if err := runMCP([]string{"extra"}); err == nil {
		t.Error("mcp with a positional argument succeeded, want error")
	}
	dir := t.TempDir()
	if err := runMCP([]string{"--dir", dir}); err == nil {
		t.Error("mcp with no board dir succeeded, want error")
	}
}

// TestMCPShowTicketRich checks board_show_ticket on a ticket with a
// description, a claim, and comments — every field renders.
func TestMCPShowTicketRich(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "-d", "the description", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "comment", "T-1", "on it", "--as", "claude-a", "--no-pull"); code != 0 {
		t.Fatalf("comment: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "pick", "--as", "claude-a", "--no-pull"); code != 0 {
		t.Fatalf("pick: exit %d\n%s", code, out)
	}
	out := &bytes.Buffer{}
	s := newMCPSession(filepath.Join(dir, ".kanban"), strings.NewReader(""), out, io.Discard)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_show_ticket","arguments":{"ticket":"T-1"}}}`)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	text := toolText(t, responses[1])
	for _, want := range []string{"T-1 One", "description: the description", "claimed by: claude-a", "comments:", "on it"} {
		if !strings.Contains(text, want) {
			t.Errorf("board_show_ticket output missing %q:\n%s", want, text)
		}
	}
}

// TestMCPLogSince checks the since filter and the bad-duration error.
func TestMCPLogSince(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_log","arguments":{"since":"1h"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"board_log","arguments":{"since":"nope"}}}`)
	if len(responses) != 3 {
		t.Fatalf("responses = %d, want 3", len(responses))
	}
	text := toolText(t, responses[1])
	if !strings.Contains(text, "ticket.created") {
		t.Errorf("board_log --since 1h missing the recent op:\n%s", text)
	}
	if !isError(responses[2]) {
		t.Errorf("board_log --since nope: isError = false, want true:\n%s", toolText(t, responses[2]))
	}
	if !strings.Contains(toolText(t, responses[2]), "duration") {
		t.Errorf("board_log --since nope error does not explain:\n%s", toolText(t, responses[2]))
	}
}
