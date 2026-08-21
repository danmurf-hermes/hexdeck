package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/danmurf/hexdeck"
)

// mcpProtocolVersion is the MCP protocol version this server speaks.
const mcpProtocolVersion = "2025-06-18"

// mcpServerName is the server name in the initialize handshake.
const mcpServerName = "hexdeck"

// mcpTool is one tool the MCP server exposes. The name, the
// description, and the input schema are the whole contract — the
// client reads them from tools/list and calls the tool by name.
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// mcpTools is the tool list, in a fixed order. The list is a render —
// the MCP view of the board — so it is deterministic and pinned by a
// golden test.
var mcpTools = []mcpTool{
	{
		Name:        "board_show",
		Description: "Show the whole board as markdown: every column with its tickets and claims.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "board_show_ticket",
		Description: "Show one ticket: its title, description, status, links, claim, and comments.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ticket": map[string]any{
					"type":        "string",
					"description": "The ticket id, e.g. T-1.",
				},
			},
			"required": []string{"ticket"},
		},
	},
	{
		Name:        "board_log",
		Description: "Show the op timeline, oldest first. Filter by ticket, actor, or a duration like 2d or 3h.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ticket": map[string]any{
					"type":        "string",
					"description": "Only ops about this ticket.",
				},
				"actor": map[string]any{
					"type":        "string",
					"description": "Only ops by this actor.",
				},
				"since": map[string]any{
					"type":        "string",
					"description": "Only ops newer than this duration, e.g. 2d or 3h.",
				},
			},
		},
	},
	{
		Name:        "board_next",
		Description: "Show the next todo ticket to pick, or say there is none.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}

// mcpSession is one MCP connection over stdio. It reads one JSON-RPC
// message per line from in and writes one response per line to out.
// The session is read-only: every tool answers from the projection,
// and nothing writes to the board.
type mcpSession struct {
	boardDir string
	in       io.Reader
	out      io.Writer
}

// newMCPSession builds an MCP session over a board dir.
func newMCPSession(boardDir string, in io.Reader, out io.Writer) *mcpSession {
	return &mcpSession{boardDir: boardDir, in: in, out: out}
}

// run serves the session until the input ends. It reads one message
// per line, answers it, and writes the response as one line. A
// malformed line gets a parse error response — the session survives.
func (s *mcpSession) run() error {
	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		response := s.handle(line)
		if response == nil {
			continue
		}
		if err := s.write(response); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// handle answers one message. Notifications get no response.
func (s *mcpSession) handle(line string) map[string]any {
	var msg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return errorResponse(nil, -32700, fmt.Sprintf("parse error: %v", err))
	}
	if msg.Method == "" {
		return errorResponse(msg.ID, -32600, "invalid request: no method")
	}
	if msg.ID == nil {
		return nil // a notification — no response
	}
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg.ID, msg.Params)
	case "ping":
		return resultResponse(msg.ID, map[string]any{})
	case "tools/list":
		return s.handleToolsList(msg.ID)
	case "tools/call":
		return s.handleToolsCall(msg.ID, msg.Params)
	default:
		return errorResponse(msg.ID, -32601, fmt.Sprintf("method not found: %s", msg.Method))
	}
}

// handleInitialize answers the handshake: the protocol version, the
// tools capability, and the server name.
func (s *mcpSession) handleInitialize(id json.RawMessage, params json.RawMessage) map[string]any {
	return resultResponse(id, map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    mcpServerName,
			"version": "1.0.0",
		},
	})
}

// handleToolsList answers with the tool list.
func (s *mcpSession) handleToolsList(id json.RawMessage) map[string]any {
	return resultResponse(id, map[string]any{
		"tools": mcpTools,
	})
}

// handleToolsCall runs one tool. The arguments are validated against
// the tool's input schema; a bad call is an invalid-params error.
func (s *mcpSession) handleToolsCall(id json.RawMessage, params json.RawMessage) map[string]any {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return errorResponse(id, -32602, fmt.Sprintf("invalid params: %v", err))
	}
	tool := findMCPTool(call.Name)
	if tool == nil {
		return errorResponse(id, -32602, fmt.Sprintf("unknown tool: %s", call.Name))
	}
	args, err := parseMCPArgs(tool, call.Arguments)
	if err != nil {
		return errorResponse(id, -32602, err.Error())
	}
	text, err := s.runTool(tool.Name, args)
	if err != nil {
		return resultResponse(id, map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		})
	}
	return resultResponse(id, map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": false,
	})
}

// runTool executes one tool and returns its text answer.
func (s *mcpSession) runTool(name string, args map[string]string) (string, error) {
	state, err := hexdeck.Project(s.boardDir)
	if err != nil {
		return "", err
	}
	switch name {
	case "board_show":
		return string(hexdeck.RenderMarkdown(state)), nil
	case "board_show_ticket":
		ticket, ok := state.Tickets[args["ticket"]]
		if !ok {
			return "", fmt.Errorf("ticket %s does not exist", args["ticket"])
		}
		return ticketText(ticket), nil
	case "board_log":
		text, warnings, err := opTimeline(s.boardDir, args["ticket"], args["actor"], args["since"])
		if err != nil {
			return "", err
		}
		// Warnings must reach the agent — a timeline with skipped ops
		// that looks clean would be trusted and wrong.
		for _, warning := range warnings {
			text += "warning: " + warning + "\n"
		}
		return text, nil
	case "board_next":
		ticket, ok := nextTodo(state)
		if !ok {
			return "no todo tickets to pick", nil
		}
		return fmt.Sprintf("%s %s", ticket.ID, ticket.Title), nil
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

// findMCPTool returns the tool with the given name, or nil.
func findMCPTool(name string) *mcpTool {
	for i := range mcpTools {
		if mcpTools[i].Name == name {
			return &mcpTools[i]
		}
	}
	return nil
}

// parseMCPArgs decodes the arguments and checks the required fields.
func parseMCPArgs(tool *mcpTool, raw json.RawMessage) (map[string]string, error) {
	args := map[string]string{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %v", err)
		}
	}
	required, _ := tool.InputSchema["required"].([]string)
	for _, field := range required {
		if args[field] == "" {
			return nil, fmt.Errorf("missing required argument: %s", field)
		}
	}
	return args, nil
}

// resultResponse builds a JSON-RPC success response.
func resultResponse(id json.RawMessage, result any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
}

// errorResponse builds a JSON-RPC error response.
func errorResponse(id json.RawMessage, code int, message string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
}

// write writes one response as one line of JSON.
func (s *mcpSession) write(response map[string]any) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(s.out, string(data)); err != nil {
		return err
	}
	return nil
}

// runMCP serves the MCP server over stdio. It prints the board dir to
// stderr (stdout is the protocol) and blocks until the input ends.
func runMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("mcp takes no positional arguments")
	}
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "hexdeck mcp: serving %s over stdio\n", boardDir)
	session := newMCPSession(boardDir, os.Stdin, os.Stdout)
	return session.run()
}
