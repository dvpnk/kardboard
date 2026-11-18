// Package registry knows which boards exist on this machine: the ones the
// user added, the ones found in the directories they asked to scan, and the
// one they are standing in.
package registry

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dvpnk/kardboard/internal/board"
	"github.com/dvpnk/kardboard/internal/userconfig"
)

// Source says how a board came to be known.
type Source string

const (
	SourceAdded Source = "added" // listed under `boards`
	SourceScan  Source = "scan"  // found below a directory under `scan`
	SourceHere  Source = "here"  // the board of the current directory
)

// maxScanDepth limits how deep a scan looks below its directory.
const maxScanDepth = 6

// Entry is one known board directory.
type Entry struct {
	Root   string
	Source Source
}

// Entries lists the known board directories without reading the boards.
// here, if not empty, is the board the command was started in; it is listed
// even if the user never added it. A board known in several ways is listed
// once, with the first source in the order added, here, scan.
func Entries(cfg *userconfig.Config, here string) []Entry {
	var out []Entry
	seen := map[string]bool{}
	add := func(root string, src Source) {
		key := root
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			key = resolved
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, Entry{Root: root, Source: src})
		}
	}
	for _, p := range cfg.Boards {
		add(userconfig.Expand(p), SourceAdded)
	}
	if here != "" {
		add(here, SourceHere)
	}
	for _, dir := range cfg.Scan {
		for _, root := range Discover(userconfig.Expand(dir)) {
			add(root, SourceScan)
		}
	}
	return out
}

// Discover finds the board directories below dir. It does not look inside
// hidden directories, dependency directories or boards, and stops at
// maxScanDepth.
func Discover(dir string) []string {
	var found []string
	base := strings.Count(filepath.Clean(dir), string(filepath.Separator))
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir // unreadable: nothing of ours in there
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != dir && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor") {
			return filepath.SkipDir
		}
		if info, err := os.Stat(filepath.Join(path, board.ConfigFile)); err == nil && info.Mode().IsRegular() {
			found = append(found, path)
			return filepath.SkipDir
		}
		if strings.Count(path, string(filepath.Separator))-base >= maxScanDepth {
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

// Loaded is a known board together with what reading it gave.
type Loaded struct {
	Entry
	// Board is nil only if the directory could not be read at all.
	Board *board.Board
	// Err says why the board is unusable: the directory is unreadable, or
	// another board uses the same id.
	Err string
}

// ID returns the board's id, or "" if board.yaml is missing or invalid.
func (l *Loaded) ID() string {
	if l.Board == nil || l.Board.Config == nil {
		return ""
	}
	return l.Board.Config.ID
}

// Load reads all given boards. Two boards with the same id are an error:
// the first keeps the id, the others are marked.
func Load(entries []Entry) []*Loaded {
	out := make([]*Loaded, 0, len(entries))
	byID := map[string]string{}
	for _, e := range entries {
		l := &Loaded{Entry: e}
		b, err := board.Load(e.Root)
		if err != nil {
			l.Err = err.Error()
		} else {
			l.Board = b
			if id := l.ID(); id != "" {
				if first, taken := byID[id]; taken {
					l.Err = fmt.Sprintf("id %q is already used by the board in %s", id, first)
				} else {
					byID[id] = e.Root
				}
			}
		}
		out = append(out, l)
	}
	return out
}

// Find returns the usable board with the given id.
func Find(loaded []*Loaded, id string) (*Loaded, bool) {
	if id == "" {
		return nil, false // boards with a broken board.yaml have no id
	}
	for _, l := range loaded {
		if l.Err == "" && l.ID() == id {
			return l, true
		}
	}
	return nil, false
}
