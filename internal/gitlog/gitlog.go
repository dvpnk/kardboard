// Package gitlog reads the Git history of a board and says what each commit
// did to the cards: added one, moved one to another column, changed one.
//
// It reads what the commits contain, not what their messages say. A card
// that Git records as renamed from backlog/ to done/ was moved, whoever
// committed it and however they worded it.
package gitlog

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Kinds of change to a card.
const (
	KindAdded   = "added"   // the card appeared
	KindMoved   = "moved"   // the card went to another column
	KindEdited  = "edited"  // the card's content changed
	KindRenamed = "renamed" // the card's file name changed
	KindRemoved = "removed" // the card went away
	KindOther   = "other"   // a file that is not a card, e.g. board.yaml
)

// Change is what one commit did to one file.
type Change struct {
	Kind string `json:"kind"`
	// Card is the card's id after the change. Empty for KindOther.
	Card string `json:"card,omitempty"`
	// Path is the file's path after the change, relative to the board.
	Path string `json:"path"`
	// From and To are column ids. A moved card has both, an added card only
	// To, a removed card only From; an edited or renamed card has To.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Was is the previous id of a card whose file name changed.
	Was string `json:"was,omitempty"`
	// Edited is true if a moved or renamed card also changed its content.
	Edited bool `json:"edited,omitempty"`
}

// Entry is one commit that touched the board.
type Entry struct {
	Hash   string `json:"hash"`
	Short  string `json:"short"`
	Author string `json:"author"`
	// Date is when the commit entered the history (the committer date), in
	// RFC 3339. That is the order Git lists commits in; the author date of a
	// rebased commit can be older than that of the commit below it.
	Date    string   `json:"date"`
	Subject string   `json:"subject"`
	Changes []Change `json:"changes"`
}

// Log is the history of a board, or the reason why there is none.
type Log struct {
	// Available is false if the board is not in a Git repository or Git is
	// missing. Reason then says which.
	Available bool    `json:"available"`
	Reason    string  `json:"reason,omitempty"`
	Branch    string  `json:"branch,omitempty"`
	Entries   []Entry `json:"entries"`
}

const (
	recordSep = "\x1e"
	fieldSep  = "\x1f"
	timeout   = 15 * time.Second
)

// Read returns the newest commits that touched the board in root, at most
// limit. columns are the ids of the board's columns; only files in them are
// cards. If cardPath is not empty, only the history of that one card is
// read, following it through moves and renames.
func Read(ctx context.Context, root string, columns []string, cardPath string, limit int) Log {
	log := Log{Entries: []Entry{}}
	if _, err := exec.LookPath("git"); err != nil {
		log.Reason = "git is not installed"
		return log
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if _, err := git(ctx, root, "rev-parse", "--show-toplevel"); err != nil {
		log.Reason = "the board is not in a Git repository"
		return log
	}
	log.Available = true
	if branch, err := git(ctx, root, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		log.Branch = strings.TrimSpace(branch)
	}

	args := []string{
		"log", "-n", strconv.Itoa(limit),
		// %x1e and %x1f make Git print the separators; they cannot occur in
		// names or subjects, so the output splits without guessing.
		"--format=%x1e%H%x1f%h%x1f%an%x1f%cI%x1f%s",
		"--name-status", "-M", "--relative",
	}
	if cardPath != "" {
		args = append(args, "--follow", "--", cardPath)
	} else {
		args = append(args, "--", ".")
	}
	out, err := git(ctx, root, args...)
	if err != nil {
		// A repository without any commit yet has no history; that is not
		// an error worth showing.
		return log
	}
	log.Entries = parse(out, columns)
	return log
}

// git runs one read-only Git command in dir. Paths are printed as they are,
// not escaped, and Git takes no locks it does not need.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	cmd.Env = append(cmd.Environ(), "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", errors.New(strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// parse reads the output of `git log --name-status` in the format Read asks
// for: one record per commit, a header line and then one line per file.
func parse(out string, columns []string) []Entry {
	entries := []Entry{}
	for _, record := range strings.Split(out, recordSep) {
		lines := strings.Split(strings.TrimSpace(record), "\n")
		head := strings.Split(lines[0], fieldSep)
		if len(head) != 5 {
			continue
		}
		e := Entry{Hash: head[0], Short: head[1], Author: head[2], Date: head[3], Subject: head[4], Changes: []Change{}}
		for _, line := range lines[1:] {
			if c, ok := change(strings.Split(line, "\t"), columns); ok {
				e.Changes = append(e.Changes, c)
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// place tells whether a path is a card and, if so, in which column and with
// which id. A card lies directly in a column. One level deeper is accepted
// too: boards used to keep cards in a directory per person, and their
// history should still read as cards.
func place(path string, columns []string) (column, id string, ok bool) {
	parts := strings.Split(path, "/")
	file := parts[len(parts)-1]
	if len(parts) < 2 || len(parts) > 3 || !slices.Contains(columns, parts[0]) ||
		!strings.HasSuffix(file, ".md") || strings.HasPrefix(file, ".") {
		return "", "", false
	}
	return parts[0], strings.TrimSuffix(file, ".md"), true
}

// change turns one line of `git log --name-status` into a Change.
func change(fields []string, columns []string) (Change, bool) {
	if len(fields) < 2 || fields[0] == "" {
		return Change{}, false
	}
	status, path := fields[0], fields[len(fields)-1]
	col, id, isCard := place(path, columns)

	switch status[0] {
	case 'A':
		if !isCard {
			return Change{Kind: KindOther, Path: path}, true
		}
		return Change{Kind: KindAdded, Card: id, Path: path, To: col}, true
	case 'D':
		if !isCard {
			return Change{Kind: KindOther, Path: path}, true
		}
		return Change{Kind: KindRemoved, Card: id, Path: path, From: col}, true
	case 'R', 'C':
		if len(fields) < 3 {
			return Change{}, false
		}
		oldCol, oldID, wasCard := place(fields[1], columns)
		edited := status[1:] != "100" // the similarity: 100 means identical content
		switch {
		case isCard && wasCard && oldCol != col:
			c := Change{Kind: KindMoved, Card: id, Path: path, From: oldCol, To: col, Edited: edited}
			if oldID != id {
				c.Was = oldID
			}
			return c, true
		case isCard && wasCard && oldID != id:
			return Change{Kind: KindRenamed, Card: id, Path: path, To: col, Was: oldID, Edited: edited}, true
		case isCard && wasCard:
			// Same column, same name: the file only changed its place inside
			// the column, as boards with a directory per person did.
			return Change{Kind: KindEdited, Card: id, Path: path, To: col}, true
		case isCard:
			return Change{Kind: KindAdded, Card: id, Path: path, To: col}, true
		case wasCard:
			return Change{Kind: KindRemoved, Card: oldID, Path: fields[1], From: oldCol}, true
		}
		return Change{Kind: KindOther, Path: path}, true
	default: // M, T and anything else that changes a file in place
		if !isCard {
			return Change{Kind: KindOther, Path: path}, true
		}
		return Change{Kind: KindEdited, Card: id, Path: path, To: col}, true
	}
}
