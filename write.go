package hexdeck

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultPrefix is the ticket id prefix when config.json does not set
// one.
const DefaultPrefix = "T"

// primer is the README.md written into the board dir at init. It is the
// whole manual: an agent that has never seen the system reads it once
// and can do everything.
const primer = `# Board — how to use it

This repo tracks work in ` + "`.kanban/`" + `. Everything is plain files in git.

## The one rule
Every change to the board is an **op** — a small JSON file appended to
` + "`.kanban/ops/`" + `. Never edit or delete existing ops. The board is always
rebuilt from the ops, so the ops are the truth.

## Where the project is up to
Read board.md — the committed board view. No CLI needed.

## Commands (preferred)
hexdeck create "Title" [-d "description"]   # new ticket
hexdeck move T-12 todo                      # change column
hexdeck comment T-12 "text"                 # add a comment
hexdeck show                                # print the board (compact)
hexdeck show T-12                           # print one ticket
hexdeck log --since 2d                      # what happened recently
hexdeck pick --as <your-name>               # claim the next todo ticket

## Writing ops by hand (if the CLI is unavailable)
Create ` + "`.kanban/ops/<seq>-<uuid>.json`" + `:
{ "schema": 1, "opId": "<uuid>", "seq": <next number>,
  "ts": "<ISO time>", "actor": "<your name>",
  "type": "ticket.moved", "ticket": "T-12",
  "payload": { "from": "backlog", "to": "todo" } }

Op types: ticket.created, ticket.moved, ticket.updated,
comment.added, ticket.claimed, ticket.released, ticket.archived.

Ticket ids are <prefix>-<number>, prefix from config.json
(default T, e.g. T-12).

## Columns
backlog → todo → done   (see config.json)

New tickets start in backlog. Move a ticket to todo when it is ready
to pick up, and to done when it is finished. Add more columns in
config.json when the work needs them — in-progress is opt-in for work
that spans multiple PRs.

## Rules
- One op per file. Never modify an op after it's committed.
- Commit ops with your code (same commit) or use ` + "`hexdeck ... --commit`" + `.
- ` + "`git pull --rebase`" + ` before appending ops.
- A ticket is done only when moved to done. No other signal counts.
- A claim older than the claim timeout is stale — ` + "`hexdeck pick`" + `
  takes the ticket anyway. The board marks it "(stale claim)".
- snapshot.json is a local cache. Never commit it — it is gitignored.
`

// agentsHook is the one line appended to AGENTS.md at init. Every major
// agent harness reads AGENTS.md at session start, so the agent is
// pointed at the primer before it does anything.
const agentsHook = "\nWork is tracked in `.kanban/` — read `.kanban/README.md` before touching the board.\n"

// InitBoard creates a new board in dir: the .kanban/ folder with the
// primer, the config, the board.created op, and the rendered board
// files, plus the AGENTS.md discovery hook. It fails if the board
// already exists.
func InitBoard(dir, name, prefix, actor string) error {
	boardDir := filepath.Join(dir, ".kanban")
	if _, err := os.Stat(boardDir); err == nil {
		return fmt.Errorf("board already exists at %s", boardDir)
	}
	if prefix == "" {
		prefix = DefaultPrefix
	}
	if err := os.MkdirAll(filepath.Join(boardDir, "ops"), 0o755); err != nil {
		return fmt.Errorf("create board dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "README.md"), []byte(primer), 0o644); err != nil {
		return fmt.Errorf("write primer: %w", err)
	}
	// The snapshot cache must never be committed. The .gitignore ships
	// with the board from day one.
	if err := os.WriteFile(filepath.Join(boardDir, ".gitignore"), []byte(snapshotGitignore), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}
	config := Config{
		Schema:       SchemaVersion,
		Board:        name,
		Columns:      append([]string(nil), DefaultColumns...),
		TicketPrefix: prefix,
		ClaimTimeout: "4h",
		AutoPush:     false,
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(boardDir, "config.json"), data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
	}{name})
	if err != nil {
		return fmt.Errorf("marshal board.created payload: %w", err)
	}
	if _, err := AppendOp(filepath.Join(boardDir, "ops"), Op{
		Type:    OpBoardCreated,
		Actor:   actor,
		Payload: payload,
	}); err != nil {
		return fmt.Errorf("write board.created op: %w", err)
	}
	if err := RenderAll(boardDir, false); err != nil {
		return fmt.Errorf("render board: %w", err)
	}
	if err := appendAgentsHook(dir); err != nil {
		return fmt.Errorf("write AGENTS.md hook: %w", err)
	}
	return nil
}

// appendAgentsHook appends the discovery line to AGENTS.md. A missing
// file is created; an existing hook is not duplicated.
func appendAgentsHook(dir string) error {
	path := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(data), ".kanban/README.md") {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(agentsHook); err != nil {
		return err
	}
	return nil
}

// AppendOp writes one op to the ops dir. It fills the seq (highest seen
// plus one), the opId (a random UUID), the ts (now, UTC), and the
// schema, then writes the file with the canonical name. The op is
// validated first — nothing is written for an invalid op.
//
// The write is O_EXCL as a belt-and-braces guard: the filename embeds
// a fresh random opId, so a concurrent writer that computed the same
// seq lands a different filename — both ops exist with the same seq,
// which the fold tolerates deterministically ((seq, opId) order). The
// exclusive-create only ever trips on a UUID collision, in which case
// the loop re-rolls. Seq uniqueness across writers is not promised;
// replay order is.
func AppendOp(opsDir string, op Op) (Op, error) {
	op.Schema = SchemaVersion
	op.OpID = newUUID()
	op.TS = time.Now().UTC().Truncate(time.Second)
	data, err := json.Marshal(op)
	if err != nil {
		return Op{}, fmt.Errorf("marshal op: %w", err)
	}
	for {
		seq, err := nextSeq(opsDir)
		if err != nil {
			return Op{}, err
		}
		op.Seq = seq
		data, err = json.Marshal(op)
		if err != nil {
			return Op{}, fmt.Errorf("marshal op: %w", err)
		}
		if _, err := ParseOp(data); err != nil {
			return Op{}, fmt.Errorf("refusing to write invalid op: %w", err)
		}
		path := filepath.Join(opsDir, OpFilename(seq, op.OpID))
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue // a concurrent writer took this seq — re-roll
		}
		if err != nil {
			return Op{}, fmt.Errorf("write op: %w", err)
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			os.Remove(path)
			return Op{}, fmt.Errorf("write op: %w", err)
		}
		if err := f.Close(); err != nil {
			os.Remove(path)
			return Op{}, fmt.Errorf("write op: %w", err)
		}
		return op, nil
	}
}

// nextSeq returns the highest seq in the ops dir plus one. An empty or
// missing dir gives 1.
func nextSeq(opsDir string) (int, error) {
	entries, err := os.ReadDir(opsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("read ops dir: %w", err)
	}
	max := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		seq, err := strconv.Atoi(strings.SplitN(entry.Name(), "-", 2)[0])
		if err != nil {
			continue
		}
		if seq > max {
			max = seq
		}
	}
	return max + 1, nil
}

// writeFileAtomic writes data to path atomically: temp file in the
// same dir, then rename. A crash or a short write can never leave a
// half-written file at path.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// newUUID returns a random RFC 4122 version 4 UUID string.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("hexdeck: cannot read random bytes: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]))
}

// NextTicketID returns the next ticket id for the board: the prefix
// from the state, then the highest numeric suffix plus one. An empty
// board gives <prefix>-1. Ids that do not match <prefix>-<number> are
// ignored.
func NextTicketID(state BoardState) string {
	prefix := state.Prefix
	if prefix == "" {
		prefix = DefaultPrefix
	}
	max := 0
	for id := range state.Tickets {
		if !strings.HasPrefix(id, prefix+"-") {
			continue
		}
		n, err := strconv.Atoi(id[len(prefix)+1:])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s-%d", prefix, max+1)
}

// RenderAll rebuilds every committed board file from the ops: board.md
// and board.json, plus board.svg when svg is true. The files are
// disposable — the ops are the truth. The fold here is the pure one:
// no snapshot cache and no wall clock, so the committed files are a
// pure function of the ops and can never change under a clock. Each
// file is written atomically (temp file, then rename), so a crash can
// never leave a half-written board.
func RenderAll(boardDir string, svg bool) error {
	state, err := projectFold(boardDir)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(boardDir, "board.md"), RenderMarkdown(state), 0o644); err != nil {
		return fmt.Errorf("write board.md: %w", err)
	}
	data, err := RenderJSON(state)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(boardDir, "board.json"), data, 0o644); err != nil {
		return fmt.Errorf("write board.json: %w", err)
	}
	if svg {
		if err := writeFileAtomic(filepath.Join(boardDir, "board.svg"), RenderSVG(state), 0o644); err != nil {
			return fmt.Errorf("write board.svg: %w", err)
		}
	}
	return nil
}

// RenderCheck compares the committed board files to a fresh render of
// the ops. It returns an error naming the first file that drifted. CI
// runs it to catch hand-edited projections. board.svg is checked only
// when it exists — it is opt-in via `render --svg`. The fold is always
// cold and clock-free — the honesty gate never trusts the snapshot
// cache and never fails on a board whose ops did not move.
func RenderCheck(boardDir string) error {
	state, err := projectFold(boardDir)
	if err != nil {
		return err
	}
	checks := []struct {
		file string
		got  []byte
	}{
		{"board.md", RenderMarkdown(state)},
	}
	data, err := RenderJSON(state)
	if err != nil {
		return err
	}
	checks = append(checks, struct {
		file string
		got  []byte
	}{"board.json", data})
	if _, err := os.Stat(filepath.Join(boardDir, "board.svg")); err == nil {
		checks = append(checks, struct {
			file string
			got  []byte
		}{"board.svg", RenderSVG(state)})
	}
	for _, check := range checks {
		want, err := os.ReadFile(filepath.Join(boardDir, check.file))
		if err != nil {
			return fmt.Errorf("read %s: %w", check.file, err)
		}
		if string(want) != string(check.got) {
			return fmt.Errorf("%s drifted from the ops — run `hexdeck render`", check.file)
		}
	}
	return nil
}
