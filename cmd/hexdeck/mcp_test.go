package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newMCPTestServer builds an MCP session over a fresh temp repo with a
// board holding two todo tickets, and returns the session and its
// output buffer.
func newMCPTestServer(t *testing.T) (*mcpSession, *bytes.Buffer) {
	t.Helper()
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
	// New tickets start in backlog — move them to todo so the board
	// has pickable work.
	if out, code := runHexdeck(t, dir, "move", "T-1", "todo", "--as", "claude-a"); code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "move", "T-2", "todo", "--as", "claude-a"); code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}
	out := &bytes.Buffer{}
	s := newMCPSession(filepath.Join(dir, ".kanban"), strings.NewReader(""), out)
	return s, out
}

// mcpExchange feeds the session one message per line and returns the
// parsed responses, in order.
func mcpExchange(t *testing.T, s *mcpSession, out *bytes.Buffer, lines ...string) []map[string]any {
	t.Helper()
	s.in = strings.NewReader(strings.Join(lines, "\n") + "\n")
	if err := s.run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	var responses []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("parse response %q: %v", line, err)
		}
		responses = append(responses, msg)
	}
	return responses
}

// mcpInit is the standard initialize message.
const mcpInit = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`

// mcpInitialized is the standard initialized notification.
const mcpInitialized = `{"jsonrpc":"2.0","method":"notifications/initialized"}`

// TestMCPInitialize checks the handshake: the server answers with the
// protocol version, the tools capability, and its name.
func TestMCPInitialize(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInit)
	if len(responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(responses))
	}
	resp := responses[0]
	if resp["id"] != float64(1) {
		t.Errorf("id = %v, want 1", resp["id"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", resp)
	}
	if result["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v, want 2025-06-18", result["protocolVersion"])
	}
	caps, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("no capabilities: %v", result)
	}
	if _, ok := caps["tools"]; !ok {
		t.Error("capabilities do not declare tools")
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("no serverInfo: %v", result)
	}
	if info["name"] != "hexdeck" {
		t.Errorf("serverInfo.name = %v, want hexdeck", info["name"])
	}
}

// TestMCPPing checks ping answers with an empty result.
func TestMCPPing(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized, `{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	if _, ok := responses[1]["result"]; !ok {
		t.Errorf("ping has no result: %v", responses[1])
	}
}

// TestMCPToolsListGolden pins the tools/list result byte for byte. The
// tool list is a render — the MCP view of the board — so it gets the
// same golden treatment as board.md and the web page.
func TestMCPToolsListGolden(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	result, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list has no result: %v", responses[1])
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	data = append(data, '\n')
	golden := filepath.Join("testdata", "golden", "mcp-tools.json")
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
		t.Errorf("tools/list drifted from the golden file — run with -update to refresh")
	}
}

// toolText returns the text content of a tools/call response.
func toolText(t *testing.T, resp map[string]any) string {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content: %v", result)
	}
	item, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content item is not an object: %v", content[0])
	}
	text, _ := item["text"].(string)
	return text
}

// isError reports whether a tools/call response is an error result.
func isError(resp map[string]any) bool {
	result, ok := resp["result"].(map[string]any)
	if !ok {
		return false
	}
	flag, _ := result["isError"].(bool)
	return flag
}

// errorCode returns the JSON-RPC error code of a response.
func errorCode(t *testing.T, resp map[string]any) int {
	t.Helper()
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error in response: %v", resp)
	}
	code, _ := errObj["code"].(float64)
	return int(code)
}

// TestMCPBoardShow checks tools/call board_show returns the markdown
// board.
func TestMCPBoardShow(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_show","arguments":{}}}`)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	text := toolText(t, responses[1])
	for _, want := range []string{"# Board —", "## todo", "T-1 One", "T-2 Two"} {
		if !strings.Contains(text, want) {
			t.Errorf("board_show output missing %q:\n%s", want, text)
		}
	}
	if isError(responses[1]) {
		t.Errorf("board_show isError = true:\n%s", text)
	}
}

// TestMCPBoardShowTicket checks tools/call board_show_ticket returns
// one ticket, and an unknown ticket is an isError result.
func TestMCPBoardShowTicket(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_show_ticket","arguments":{"ticket":"T-1"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"board_show_ticket","arguments":{"ticket":"T-99"}}}`)
	if len(responses) != 3 {
		t.Fatalf("responses = %d, want 3", len(responses))
	}
	text := toolText(t, responses[1])
	for _, want := range []string{"T-1 One", "status: todo"} {
		if !strings.Contains(text, want) {
			t.Errorf("board_show_ticket output missing %q:\n%s", want, text)
		}
	}
	if !isError(responses[2]) {
		t.Errorf("unknown ticket: isError = false, want true:\n%s", toolText(t, responses[2]))
	}
	if !strings.Contains(toolText(t, responses[2]), "does not exist") {
		t.Errorf("unknown ticket error does not explain:\n%s", toolText(t, responses[2]))
	}
}

// TestMCPBoardShowLabelFilter checks tools/call board_show with a
// label argument: only tickets with the label render.
func TestMCPBoardShowLabelFilter(t *testing.T) {
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
	if out, code := runHexdeck(t, dir, "label", "T-1", "bug", "--as", "claude-a"); code != 0 {
		t.Fatalf("label: exit %d\n%s", code, out)
	}
	outBuf := &bytes.Buffer{}
	s := newMCPSession(filepath.Join(dir, ".kanban"), strings.NewReader(""), outBuf)
	responses := mcpExchange(t, s, outBuf, mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_show","arguments":{"label":"bug"}}}`)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	text := toolText(t, responses[1])
	if !strings.Contains(text, "T-1 One") {
		t.Errorf("board_show label filter missing the matching ticket:\n%s", text)
	}
	if strings.Contains(text, "T-2 Two") {
		t.Errorf("board_show label filter carries the non-matching ticket:\n%s", text)
	}
}

// TestMCPBoardLogWarnings checks that board_log surfaces read
// warnings in the result text: an agent must see that ops were
// skipped, or it would trust a timeline with holes in it.
func TestMCPBoardLogWarnings(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	// Drop a broken op file into the ops dir.
	opsDir := filepath.Join(dir, ".kanban", "ops")
	bad := filepath.Join(opsDir, "0000000000000099-99999999-9999-4999-8999-999999999999.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write bad op: %v", err)
	}
	outBuf := &bytes.Buffer{}
	s := newMCPSession(filepath.Join(dir, ".kanban"), strings.NewReader(""), outBuf)
	responses := mcpExchange(t, s, outBuf,
		mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_log","arguments":{}}}`)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	text := toolText(t, responses[1])
	if !strings.Contains(text, "warning:") {
		t.Errorf("board_log result does not surface the read warning:\n%s", text)
	}
	// The good ops must still render alongside the warning.
	if !strings.Contains(text, "ticket.created T-1") {
		t.Errorf("board_log result missing the good op:\n%s", text)
	}
}

// TestMCPBoardLog checks tools/call board_log returns the timeline,
// with the filters applied.
func TestMCPBoardLog(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_log","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"board_log","arguments":{"ticket":"T-1"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"board_log","arguments":{"actor":"nobody"}}}`)
	if len(responses) != 4 {
		t.Fatalf("responses = %d, want 4", len(responses))
	}
	text := toolText(t, responses[1])
	for _, want := range []string{"board.created", "ticket.created T-1", "ticket.created T-2"} {
		if !strings.Contains(text, want) {
			t.Errorf("board_log output missing %q:\n%s", want, text)
		}
	}
	filtered := toolText(t, responses[2])
	if !strings.Contains(filtered, "T-1") || strings.Contains(filtered, "T-2") {
		t.Errorf("board_log ticket filter wrong:\n%s", filtered)
	}
	empty := toolText(t, responses[3])
	if strings.TrimSpace(empty) != "" {
		t.Errorf("board_log actor filter should be empty:\n%s", empty)
	}
}

// TestMCPBoardNext checks tools/call board_next returns the next todo
// ticket.
func TestMCPBoardNext(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_next","arguments":{}}}`)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	text := toolText(t, responses[1])
	if !strings.Contains(text, "T-1 One") {
		t.Errorf("board_next = %q, want T-1 One", text)
	}
}

// TestMCPBoardNextEmpty checks board_next on a board with no todo
// tickets.
func TestMCPBoardNextEmpty(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "move", "T-1", "done", "--as", "claude-a", "--no-pull"); code != 0 {
		t.Fatalf("move: exit %d\n%s", code, out)
	}
	out := &bytes.Buffer{}
	s := newMCPSession(filepath.Join(dir, ".kanban"), strings.NewReader(""), out)
	responses := mcpExchange(t, s, out, mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_next","arguments":{}}}`)
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	if text := toolText(t, responses[1]); text != "no todo tickets to pick" {
		t.Errorf("board_next = %q, want the empty answer", text)
	}
}

// TestMCPErrors checks the error paths: bad JSON, unknown method,
// unknown tool, and a missing required argument.
func TestMCPErrors(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out,
		"{not json",
		`{"jsonrpc":"2.0","id":2,"method":"nope"}`,
		mcpInit, mcpInitialized,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"board_show_ticket","arguments":{}}}`)
	if len(responses) != 5 {
		t.Fatalf("responses = %d, want 5", len(responses))
	}
	if code := errorCode(t, responses[0]); code != -32700 {
		t.Errorf("bad JSON: code = %d, want -32700", code)
	}
	if code := errorCode(t, responses[1]); code != -32601 {
		t.Errorf("unknown method: code = %d, want -32601", code)
	}
	if code := errorCode(t, responses[3]); code != -32602 {
		t.Errorf("unknown tool: code = %d, want -32602", code)
	}
	if code := errorCode(t, responses[4]); code != -32602 {
		t.Errorf("missing argument: code = %d, want -32602", code)
	}
}

// TestMCPNotifications checks notifications get no response.
func TestMCPNotifications(t *testing.T) {
	s, out := newMCPTestServer(t)
	responses := mcpExchange(t, s, out, mcpInitialized)
	if len(responses) != 0 {
		t.Errorf("notification got %d responses, want 0: %v", len(responses), responses)
	}
}

// TestE2EMCP runs the real binary over stdio: the handshake, the tool
// list, and a board_show call. This proves the command wiring — stdin
// to stdout, the board dir resolution, and the tool execution.
func TestE2EMCP(t *testing.T) {
	dir := initRepo(t)
	if out, code := runHexdeck(t, dir, "init", "--as", "claude-a"); code != 0 {
		t.Fatalf("init: exit %d\n%s", code, out)
	}
	if out, code := runHexdeck(t, dir, "create", "One", "--as", "claude-a"); code != 0 {
		t.Fatalf("create: exit %d\n%s", code, out)
	}
	bin := buildHexdeck(t)
	cmd := exec.Command(bin, "mcp", "--dir", dir)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test-agent", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test-agent", "GIT_COMMITTER_EMAIL=test@example.com")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mcp: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	sc := bufio.NewScanner(stdout)
	// initialize
	fmt.Fprintln(stdin, mcpInit)
	if !sc.Scan() {
		t.Fatalf("no response to initialize: %v", sc.Err())
	}
	var initResp map[string]any
	if err := json.Unmarshal([]byte(sc.Text()), &initResp); err != nil {
		t.Fatalf("parse initialize response: %v", err)
	}
	result, _ := initResp["result"].(map[string]any)
	if result == nil || result["protocolVersion"] != "2025-06-18" {
		t.Errorf("initialize response wrong: %v", initResp)
	}
	// initialized notification, then tools/list
	fmt.Fprintln(stdin, mcpInitialized)
	fmt.Fprintln(stdin, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if !sc.Scan() {
		t.Fatalf("no response to tools/list: %v", sc.Err())
	}
	var listResp map[string]any
	if err := json.Unmarshal([]byte(sc.Text()), &listResp); err != nil {
		t.Fatalf("parse tools/list response: %v", err)
	}
	listResult, _ := listResp["result"].(map[string]any)
	tools, _ := listResult["tools"].([]any)
	if len(tools) != 4 {
		t.Errorf("tools = %d, want 4", len(tools))
	}
	// tools/call board_show
	fmt.Fprintln(stdin, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"board_show","arguments":{}}}`)
	if !sc.Scan() {
		t.Fatalf("no response to tools/call: %v", sc.Err())
	}
	var callResp map[string]any
	if err := json.Unmarshal([]byte(sc.Text()), &callResp); err != nil {
		t.Fatalf("parse tools/call response: %v", err)
	}
	callResult, _ := callResp["result"].(map[string]any)
	content, _ := callResult["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	if !strings.Contains(text, "T-1 One") {
		t.Errorf("board_show over stdio missing the ticket:\n%s", text)
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("mcp exited: %v", err)
	}
}
