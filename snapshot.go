package hexdeck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Snapshot is the local replay cache. It holds a folded BoardState plus
// a digest of everything the fold reads: the config file and the op
// files (names and contents). When the digest matches, Project returns
// the cached state without folding the ops again.
//
// The snapshot is never the source of truth. It is a disposable cache:
// corrupt it, delete it, or let it go stale and the board rebuilds from
// the ops. It is never committed — InitBoard writes a .gitignore that
// hides it, and no command stages it.
//
// Why the digest covers op contents, not just names: an op edited in
// place keeps its name but changes the fold. The digest must change
// with the content, or the cache would serve stale state. RenderCheck
// folds cold (never trusts the cache), so a poisoned cache can never
// make the honesty gate lie.
type Snapshot struct {
	Schema int        `json:"schema"`
	Digest string     `json:"digest"`
	State  BoardState `json:"state"`
}

// snapshotName is the cache file inside the board dir.
const snapshotName = "snapshot.json"

// snapshotGitignore hides the cache from git. Written at init and
// repaired by the cache writer if missing.
const snapshotGitignore = "snapshot.json\n"

// snapshotDigest hashes everything the fold reads: the config file
// bytes and, in sorted order, every op file's name and bytes. Any
// change — a new op, a deleted op, an edited op, a config change —
// changes the digest, so a matching digest means the fold is byte-for-
// byte the same. A presence marker keeps a missing config distinct
// from an empty one: both hash differently.
func snapshotDigest(boardDir string) (string, error) {
	h := sha256.New()
	config, err := os.ReadFile(filepath.Join(boardDir, "config.json"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if config == nil {
		h.Write([]byte{1}) // config missing
	} else {
		h.Write([]byte{0})
		h.Write(config)
	}
	entries, err := os.ReadDir(filepath.Join(boardDir, "ops"))
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) < 5 || entry.Name()[len(entry.Name())-5:] != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(boardDir, "ops", name))
		if err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readSnapshot loads the cache. A missing file, a corrupt file, or a
// schema mismatch all come back as (nil, false) — the cache is
// disposable, never fatal.
func readSnapshot(boardDir string) (*Snapshot, bool) {
	data, err := os.ReadFile(filepath.Join(boardDir, snapshotName))
	if err != nil {
		return nil, false
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, false
	}
	if snap.Schema != SchemaVersion || snap.Digest == "" {
		return nil, false
	}
	return &snap, true
}

// writeSnapshot atomically writes the cache: temp file, then rename,
// so a crash never leaves a half-written snapshot. It also makes sure
// the .gitignore hides the cache — the snapshot line is appended to
// whatever is already there, never replacing the file, so user entries
// are preserved. Failures are ignored — the cache is disposable; a
// failed write just costs a re-fold.
func writeSnapshot(boardDir, digest string, state BoardState) {
	ignorePath := filepath.Join(boardDir, ".gitignore")
	if data, err := os.ReadFile(ignorePath); err != nil || !containsLine(string(data), snapshotName) {
		// Append the line, keeping the file's existing content.
		line := snapshotName + "\n"
		if len(data) > 0 && data[len(data)-1] != '\n' {
			line = "\n" + line
		}
		_ = os.WriteFile(ignorePath, append(data, []byte(line)...), 0o644)
	}
	snap := Snapshot{Schema: SchemaVersion, Digest: digest, State: state}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return
	}
	tmp := filepath.Join(boardDir, snapshotName+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(boardDir, snapshotName))
}

// containsLine reports whether s contains line as a whole line.
func containsLine(s, line string) bool {
	for _, l := range splitLines(s) {
		if l == line {
			return true
		}
	}
	return false
}

// splitLines splits s on newlines, dropping empty and blank lines.
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			line := s[start:i]
			if line != "" {
				out = append(out, line)
			}
			start = i + 1
		}
	}
	return out
}
