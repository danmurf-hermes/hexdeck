// Command hexdeck is the CLI for hexdeck, a git-native kanban board for
// AI agents. Every change to the board is an op — one JSON file per
// event in .kanban/ops/. The board is always rebuilt from the ops.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/danmurf/hexdeck"
)

// usage is the top-level help. One screen, plain words.
const usage = `hexdeck — a kanban board stored in git

Usage:
  hexdeck init [--prefix T] [--name <board>] [--as <actor>]
  hexdeck create "Title" [-d "description"] [--as <actor>] [--commit]
  hexdeck move <ticket> <column> [--as <actor>] [--commit]
  hexdeck comment <ticket> "text" [--as <actor>] [--commit]
  hexdeck show [<ticket>] [--json]
  hexdeck log [--since 2d] [--ticket <ticket>] [--actor <actor>]
  hexdeck pick --as <actor> [--commit]
  hexdeck release <ticket> --as <actor> [--commit]
  hexdeck render [--svg] [--check]

Every change is an op file in .kanban/ops/. Ops are never edited or
deleted. The board is always rebuilt from the ops.

Flags:
  --dir <path>   board dir (default: .kanban in the current dir or a
                 parent dir)
  --as <actor>   your name, stable per writer (default: git user.name)
  --commit       commit the change with the suggested message
  --no-pull      skip git pull --rebase before appending
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	command, args := os.Args[1], reorderArgs(os.Args[2:])
	var err error
	switch command {
	case "init":
		err = runInit(args)
	case "create":
		err = runCreate(args)
	case "move":
		err = runMove(args)
	case "comment":
		err = runComment(args)
	case "show":
		err = runShow(args)
	case "log":
		err = runLog(args)
	case "pick":
		err = runPick(args)
	case "release":
		err = runRelease(args)
	case "render":
		err = runRender(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "hexdeck: unknown command %q\n\n%s", command, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hexdeck: %v\n", err)
		os.Exit(1)
	}
}

// commonFlags are the flags every command shares.
type commonFlags struct {
	dir    string
	actor  string
	commit bool
	noPull bool
}

// addCommon registers the shared flags on a FlagSet.
func addCommon(fs *flag.FlagSet, cf *commonFlags) {
	fs.StringVar(&cf.dir, "dir", "", "board dir (default: .kanban in the current dir or a parent dir)")
	fs.StringVar(&cf.actor, "as", "", "your name, stable per writer (default: git user.name)")
	fs.BoolVar(&cf.commit, "commit", false, "commit the change with the suggested message")
	fs.BoolVar(&cf.noPull, "no-pull", false, "skip git pull --rebase before appending")
}

// findBoardDir walks up from the working directory looking for .kanban.
func findBoardDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		boardDir := filepath.Join(dir, ".kanban")
		if info, err := os.Stat(boardDir); err == nil && info.IsDir() {
			return boardDir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .kanban board found here or in any parent dir — run `hexdeck init` first")
		}
		dir = parent
	}
}

// resolveBoardDir returns the board dir from --dir or the search.
func resolveBoardDir(cf commonFlags) (string, error) {
	if cf.dir != "" {
		boardDir := filepath.Join(cf.dir, ".kanban")
		if info, err := os.Stat(boardDir); err != nil || !info.IsDir() {
			return "", fmt.Errorf("no .kanban board in %s", cf.dir)
		}
		return boardDir, nil
	}
	return findBoardDir()
}

// resolveActor returns the actor name from --as or git user.name.
func resolveActor(cf commonFlags) (string, error) {
	if cf.actor != "" {
		return cf.actor, nil
	}
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return "", fmt.Errorf("no --as flag and no git user.name — set one: git config user.name <your-name>")
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return "", fmt.Errorf("no --as flag and git user.name is empty — set one: git config user.name <your-name>")
	}
	return name, nil
}

// runInit creates a new board.
func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	prefix := fs.String("prefix", "T", "ticket id prefix (default T)")
	name := fs.String("name", "", "board name (default: the repo dir name)")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("init takes no positional arguments")
	}
	dir := cf.dir
	if dir == "" {
		dir = "."
	}
	if *name == "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		*name = filepath.Base(abs)
	}
	actor, err := resolveActor(cf)
	if err != nil {
		return err
	}
	if err := hexdeck.InitBoard(dir, *name, *prefix, actor); err != nil {
		return err
	}
	boardDir := filepath.Join(dir, ".kanban")
	stageGit(dir, boardDir, "README.md", "config.json", "board.md", "board.json")
	stageGit(dir, dir, "AGENTS.md")
	fmt.Printf("board %q created in %s\n", *name, boardDir)
	fmt.Printf("suggested commit: board: init %s\n", *name)
	return nil
}

// runCreate appends a ticket.created op.
func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	desc := fs.String("d", "", "description")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("create takes one argument: the title")
	}
	title := fs.Arg(0)
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	actor, err := resolveActor(cf)
	if err != nil {
		return err
	}
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		return err
	}
	id := hexdeck.NextTicketID(state)
	payload, err := json.Marshal(struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}{title, *desc})
	if err != nil {
		return err
	}
	op, err := appendOp(boardDir, cf, hexdeck.Op{
		Type:    hexdeck.OpTicketCreated,
		Ticket:  id,
		Actor:   actor,
		Payload: payload,
	})
	if err != nil {
		return err
	}
	_ = op
	fmt.Println(id)
	suggestCommit(cf, fmt.Sprintf("board: create %s", id))
	return nil
}

// runMove appends a ticket.moved op.
func runMove(args []string) error {
	fs := flag.NewFlagSet("move", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("move takes two arguments: the ticket and the column")
	}
	ticket, to := fs.Arg(0), fs.Arg(1)
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	actor, err := resolveActor(cf)
	if err != nil {
		return err
	}
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		return err
	}
	t, ok := state.Tickets[ticket]
	if !ok {
		return fmt.Errorf("ticket %s does not exist", ticket)
	}
	if !contains(state.Columns, to) {
		return fmt.Errorf("column %q does not exist — columns: %s", to, strings.Join(state.Columns, ", "))
	}
	payload, err := json.Marshal(struct {
		From string `json:"from"`
		To   string `json:"to"`
	}{t.Status, to})
	if err != nil {
		return err
	}
	if _, err := appendOp(boardDir, cf, hexdeck.Op{
		Type:    hexdeck.OpTicketMoved,
		Ticket:  ticket,
		Actor:   actor,
		Payload: payload,
	}); err != nil {
		return err
	}
	suggestCommit(cf, fmt.Sprintf("board: move %s → %s", ticket, to))
	return nil
}

// runComment appends a comment.added op.
func runComment(args []string) error {
	fs := flag.NewFlagSet("comment", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("comment takes two arguments: the ticket and the text")
	}
	ticket, text := fs.Arg(0), fs.Arg(1)
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	actor, err := resolveActor(cf)
	if err != nil {
		return err
	}
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		return err
	}
	if _, ok := state.Tickets[ticket]; !ok {
		return fmt.Errorf("ticket %s does not exist", ticket)
	}
	payload, err := json.Marshal(struct {
		Text string `json:"text"`
	}{text})
	if err != nil {
		return err
	}
	if _, err := appendOp(boardDir, cf, hexdeck.Op{
		Type:    hexdeck.OpCommentAdded,
		Ticket:  ticket,
		Actor:   actor,
		Payload: payload,
	}); err != nil {
		return err
	}
	suggestCommit(cf, fmt.Sprintf("board: comment on %s", ticket))
	return nil
}

// runShow prints the board or one ticket.
func runShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	asJSON := fs.Bool("json", false, "print the machine view (board.json)")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("show takes at most one argument: the ticket")
	}
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		return err
	}
	if *asJSON {
		data, err := hexdeck.RenderJSON(state)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	}
	if fs.NArg() == 1 {
		return showTicket(state, fs.Arg(0))
	}
	fmt.Print(string(hexdeck.RenderMarkdown(state)))
	return nil
}

// showTicket prints one ticket: fields, comments, and history.
func showTicket(state hexdeck.BoardState, id string) error {
	ticket, ok := state.Tickets[id]
	if !ok {
		return fmt.Errorf("ticket %s does not exist", id)
	}
	fmt.Printf("%s %s\n", ticket.ID, ticket.Title)
	fmt.Printf("status: %s\n", ticket.Status)
	if ticket.Description != "" {
		fmt.Printf("description: %s\n", ticket.Description)
	}
	if ticket.ClaimedBy != "" {
		fmt.Printf("claimed by: %s\n", ticket.ClaimedBy)
	}
	fmt.Printf("created: %s\n", ticket.Created.UTC().Format("2006-01-02T15:04:05Z"))
	if len(ticket.Comments) > 0 {
		fmt.Println("comments:")
		for _, comment := range ticket.Comments {
			fmt.Printf("  %s %s: %s\n", comment.TS.UTC().Format("2006-01-02T15:04:05Z"), comment.Actor, comment.Text)
		}
	}
	return nil
}

// runLog prints the event timeline.
func runLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	since := fs.String("since", "", "only ops newer than this (e.g. 2d, 3h)")
	ticket := fs.String("ticket", "", "only ops about this ticket")
	actor := fs.String("actor", "", "only ops by this actor")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("log takes no positional arguments")
	}
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	ops, warnings, err := hexdeck.ReadOpsDir(filepath.Join(boardDir, "ops"))
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	var cutoff time.Time
	if *since != "" {
		d, err := time.ParseDuration(*since)
		if err != nil {
			return fmt.Errorf("--since: %q is not a duration like 2d or 3h", *since)
		}
		cutoff = time.Now().UTC().Add(-d)
	}
	for _, op := range ops {
		if *ticket != "" && op.Ticket != *ticket {
			continue
		}
		if *actor != "" && op.Actor != *actor {
			continue
		}
		if !cutoff.IsZero() && op.TS.Before(cutoff) {
			continue
		}
		fmt.Printf("%s %s %s %s\n", op.TS.UTC().Format("2006-01-02T15:04:05Z"), op.Actor, op.Type, op.Ticket)
	}
	return nil
}

// runPick claims and moves the next todo ticket.
func runPick(args []string) error {
	fs := flag.NewFlagSet("pick", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("pick takes no positional arguments")
	}
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	actor, err := resolveActor(cf)
	if err != nil {
		return err
	}
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		return err
	}
	timeout, err := time.ParseDuration(state.ClaimTimeout)
	if err != nil {
		timeout = 4 * time.Hour
	}
	now := time.Now().UTC()
	var candidates []hexdeck.Ticket
	for _, ticket := range state.Tickets {
		if ticket.Archived || ticket.Status != state.Columns[0] {
			continue
		}
		if ticket.ClaimedBy != "" {
			if ticket.ClaimedAt == nil || now.Sub(*ticket.ClaimedAt) < timeout {
				continue
			}
		}
		candidates = append(candidates, ticket)
	}
	if len(candidates) == 0 {
		fmt.Println("no todo tickets to pick")
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})
	ticket := candidates[0]
	claimPayload, err := json.Marshal(struct {
		By string `json:"by"`
	}{actor})
	if err != nil {
		return err
	}
	if _, err := appendOp(boardDir, cf, hexdeck.Op{
		Type:    hexdeck.OpTicketClaimed,
		Ticket:  ticket.ID,
		Actor:   actor,
		Payload: claimPayload,
	}); err != nil {
		return err
	}
	movePayload, err := json.Marshal(struct {
		From string `json:"from"`
		To   string `json:"to"`
	}{ticket.Status, state.Columns[1]})
	if err != nil {
		return err
	}
	if _, err := appendOp(boardDir, cf, hexdeck.Op{
		Type:    hexdeck.OpTicketMoved,
		Ticket:  ticket.ID,
		Actor:   actor,
		Payload: movePayload,
	}); err != nil {
		return err
	}
	fmt.Printf("picked %s %s\n", ticket.ID, ticket.Title)
	suggestCommit(cf, fmt.Sprintf("board: pick %s", ticket.ID))
	return nil
}

// runRelease appends a ticket.released op.
func runRelease(args []string) error {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("release takes one argument: the ticket")
	}
	ticket := fs.Arg(0)
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	actor, err := resolveActor(cf)
	if err != nil {
		return err
	}
	state, err := hexdeck.Project(boardDir)
	if err != nil {
		return err
	}
	if _, ok := state.Tickets[ticket]; !ok {
		return fmt.Errorf("ticket %s does not exist", ticket)
	}
	payload, err := json.Marshal(struct {
		By string `json:"by"`
	}{actor})
	if err != nil {
		return err
	}
	if _, err := appendOp(boardDir, cf, hexdeck.Op{
		Type:    hexdeck.OpTicketReleased,
		Ticket:  ticket,
		Actor:   actor,
		Payload: payload,
	}); err != nil {
		return err
	}
	suggestCommit(cf, fmt.Sprintf("board: release %s", ticket))
	return nil
}

// runRender rebuilds the board files, or checks them for drift.
func runRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	var cf commonFlags
	addCommon(fs, &cf)
	svg := fs.Bool("svg", false, "also rebuild board.svg")
	check := fs.Bool("check", false, "exit 1 if the committed board files drifted from the ops")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("render takes no positional arguments")
	}
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return err
	}
	if *check {
		if err := hexdeck.RenderCheck(boardDir); err != nil {
			return err
		}
		fmt.Println("board files match the ops")
		return nil
	}
	if err := hexdeck.RenderAll(boardDir, *svg); err != nil {
		return err
	}
	fmt.Println("board rendered")
	return nil
}

// appendOp writes the op, re-renders the board, and stages the change.
// It runs git pull --rebase first unless --no-pull is set.
func appendOp(boardDir string, cf commonFlags, op hexdeck.Op) (hexdeck.Op, error) {
	repoDir := filepath.Dir(boardDir)
	if !cf.noPull {
		if err := gitPullRebase(repoDir); err != nil {
			return hexdeck.Op{}, err
		}
	}
	written, err := hexdeck.AppendOp(filepath.Join(boardDir, "ops"), op)
	if err != nil {
		return hexdeck.Op{}, err
	}
	if err := hexdeck.RenderAll(boardDir, false); err != nil {
		return hexdeck.Op{}, err
	}
	stageGit(repoDir, boardDir, "ops", "board.md", "board.json")
	return written, nil
}

// gitPullRebase runs git pull --rebase in dir. A repo with no upstream
// is skipped — there is nothing to pull. A failed pull is an error —
// the op must not be written on a stale checkout.
func gitPullRebase(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return nil
	}
	upstream := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	upstream.Dir = dir
	if err := upstream.Run(); err != nil {
		return nil // no upstream — nothing to pull
	}
	cmd := exec.Command("git", "pull", "--rebase")
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull --rebase failed: %w (use --no-pull to skip)", err)
	}
	return nil
}

// stageGit stages the given paths under boardDir, relative to repoDir.
// Missing files are skipped. A missing repo is not an error — the board
// works outside git too.
func stageGit(repoDir, boardDir string, paths ...string) {
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		return
	}
	rel, err := filepath.Rel(repoDir, boardDir)
	if err != nil {
		return
	}
	var args []string
	for _, path := range paths {
		full := filepath.Join(boardDir, path)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		args = append(args, filepath.Join(rel, path))
	}
	if len(args) == 0 {
		return
	}
	cmd := exec.Command("git", append([]string{"add", "--"}, args...)...)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: git add failed: %v\n", err)
	}
}

// suggestCommit prints the suggested commit message, or commits when
// --commit is set.
func suggestCommit(cf commonFlags, message string) {
	if cf.commit {
		repoDir := filepath.Dir(resolveBoardDirOrCwd(cf))
		cmd := exec.Command("git", "commit", "-m", message)
		cmd.Dir = repoDir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: git commit failed: %v\n", err)
			return
		}
		fmt.Printf("committed: %s\n", message)
		return
	}
	fmt.Printf("suggested commit: %s\n", message)
}

// resolveBoardDirOrCwd returns the board dir for --commit, or .kanban
// in the current dir when the search would fail.
func resolveBoardDirOrCwd(cf commonFlags) string {
	boardDir, err := resolveBoardDir(cf)
	if err != nil {
		return filepath.Join(".", ".kanban")
	}
	return boardDir
}

// boolFlags are the flags that take no value. Every other flag takes
// the next token as its value.
var boolFlags = map[string]bool{
	"commit": true, "no-pull": true, "json": true, "svg": true,
	"check": true, "h": true, "help": true,
}

// reorderArgs moves flags (and their values) before positional
// arguments. The flag package stops at the first positional argument,
// but the spec's usage puts flags after positionals
// (`hexdeck create "Title" -d "desc"`), so the CLI reorders first.
func reorderArgs(args []string) []string {
	var flags, positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags = append(flags, arg)
			name := strings.TrimLeft(arg, "-")
			if eq := strings.Index(name, "="); eq >= 0 {
				name = name[:eq]
			}
			if !boolFlags[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positionals = append(positionals, arg)
		}
	}
	return append(flags, positionals...)
}

// contains reports whether s is in list.
func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
