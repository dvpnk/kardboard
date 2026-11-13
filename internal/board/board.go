// Package board reads a kardboard board from disk and checks it against the
// format described in docs/spec.md.
package board

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Severity tells errors from warnings.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Problem is one violation of a rule from the spec.
type Problem struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	// Path is relative to the board root and uses forward slashes.
	Path    string `json:"path"`
	Message string `json:"message"`
}

func severityOf(code string) Severity {
	if strings.HasPrefix(code, "W") {
		return SeverityWarning
	}
	return SeverityError
}

// Board is a board directory as found on disk.
type Board struct {
	// Root is the absolute path of the directory that holds board.yaml.
	Root string
	// Config is nil when board.yaml is missing or invalid.
	Config *Config
	Cards  []*Card
	// problems that belong to the board rather than to a single card.
	problems []Problem
	// mailmap is read on first use, see Canonical.
	mailmap mailmap
}

// Load reads the board in root. It returns an error only if the directory
// cannot be read at all; rule violations end up in Problems.
func Load(root string) (*Board, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	b := &Board{Root: abs}

	data, err := os.ReadFile(filepath.Join(abs, ConfigFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			b.failConfig("file is missing")
			return b, nil
		}
		return nil, err
	}
	cfg, msgs := ParseConfig(data)
	if len(msgs) > 0 {
		for _, m := range msgs {
			b.failConfig("%s", m)
		}
		return b, nil
	}
	b.Config = cfg

	for _, col := range cfg.Columns {
		if err := b.scanColumn(col); err != nil {
			return nil, err
		}
	}
	b.checkBoardRules()
	return b, nil
}

func (b *Board) failConfig(format string, args ...any) {
	b.problems = append(b.problems, Problem{
		Code: "E01", Severity: SeverityError, Path: ConfigFile, Message: fmt.Sprintf(format, args...),
	})
}

// scanColumn collects the cards of one column: the Markdown files directly
// in its directory. A missing directory is an empty column, because Git does
// not keep empty directories.
func (b *Board) scanColumn(col Column) error {
	entries, err := os.ReadDir(filepath.Join(b.Root, col.ID))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || !isCardFile(e) {
			continue
		}
		if _, err := b.readCard(col, e.Name()); err != nil {
			return err
		}
	}
	return nil
}

func isCardFile(e fs.DirEntry) bool {
	return e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".md")
}

func (b *Board) readCard(col Column, file string) (*Card, error) {
	rel := col.ID + "/" + file
	data, err := os.ReadFile(filepath.Join(b.Root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	card := &Card{
		ID:     strings.TrimSuffix(file, ".md"),
		Path:   rel,
		Column: col.ID,
	}
	card.parse(b.Config, string(data))
	b.Cards = append(b.Cards, card)
	return card, nil
}

// checkBoardRules applies the rules that need more than one card or the
// card's place: E06, E08, E09, E10, W01 and W02.
func (b *Board) checkBoardRules() {
	byID := map[string][]*Card{}
	for _, c := range b.Cards {
		byID[c.ID] = append(byID[c.ID], c)
	}

	for _, c := range b.Cards {
		if !slugRe.MatchString(c.ID) || len(c.ID) > maxIDLength {
			c.fail("E08", "file name must be a-z, 0-9 and - only, at most %d characters", maxIDLength)
		}
		if others := byID[c.ID]; len(others) > 1 {
			var paths []string
			for _, o := range others {
				if o != c {
					paths = append(paths, o.Path)
				}
			}
			c.fail("E09", "file name is also used by %s", strings.Join(paths, ", "))
		}
		// A field that is present but wrong (E05) is not reported as missing.
		for _, f := range b.Config.AllFields() {
			if _, has := c.Fields[f.Name]; !has && f.Required.In(c.Column) && !slices.Contains(c.rejected, f.Name) {
				c.fail("E06", "field %q is required in %s", f.Name, c.Column)
			}
		}
		for _, ref := range c.refs {
			switch {
			case strings.Contains(ref, "/"):
				c.fail("W01", "reference %q points to another board and is not checked", ref)
			case len(byID[ref]) == 0:
				c.fail("E10", "reference %q: no such card on this board", ref)
			}
		}
	}

	for _, col := range b.Config.Columns {
		if col.Limit == 0 {
			continue
		}
		n := 0
		for _, c := range b.Cards {
			if c.Column == col.ID {
				n++
			}
		}
		if n > col.Limit {
			b.problems = append(b.problems, Problem{
				Code: "W02", Severity: SeverityWarning, Path: col.ID,
				Message: fmt.Sprintf("%d cards, limit is %d", n, col.Limit),
			})
		}
	}
}

// BoardProblems returns the violations that belong to the board as a whole
// rather than to one card: a broken board.yaml, an exceeded limit.
func (b *Board) BoardProblems() []Problem {
	return slices.Clone(b.problems)
}

// Problems returns all violations of the board and its cards, sorted by path
// and code.
func (b *Board) Problems() []Problem {
	out := slices.Clone(b.problems)
	for _, c := range b.Cards {
		out = append(out, c.Problems...)
	}
	slices.SortStableFunc(out, func(x, y Problem) int {
		return cmp.Or(cmp.Compare(x.Path, y.Path), cmp.Compare(x.Code, y.Code))
	})
	return out
}

// Card finds a card by its id, with or without the ".md" suffix. With
// duplicate file names (rule E09) it returns the first one.
func (b *Board) Card(id string) (*Card, bool) {
	id = strings.TrimSuffix(id, ".md")
	for _, c := range b.Cards {
		if c.ID == id {
			return c, true
		}
	}
	return nil, false
}

// ErrNoBoard means that no board.yaml could be found.
var ErrNoBoard = errors.New("no board found")

// FindRoot locates the board for a working directory. It looks for
// board.yaml in start and its parents first. If there is none, it searches
// the Git repository that contains start and expects exactly one board there.
func FindRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	repo := ""
	for dir := abs; ; dir = filepath.Dir(dir) {
		if isFile(filepath.Join(dir, ConfigFile)) {
			return dir, nil
		}
		if repo == "" && exists(filepath.Join(dir, ".git")) {
			repo = dir
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	if repo == "" {
		return "", fmt.Errorf("%w: no %s in %s or above, and not inside a Git repository", ErrNoBoard, ConfigFile, abs)
	}

	var found []string
	err = filepath.WalkDir(repo, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable directories cannot hold our board
		}
		if d.IsDir() {
			name := d.Name()
			if path != repo && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == ConfigFile {
			found = append(found, filepath.Dir(path))
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("%w: no %s in repository %s", ErrNoBoard, ConfigFile, repo)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("%w: repository %s holds %d boards (%s), choose one with -C", ErrNoBoard, repo, len(found), strings.Join(found, ", "))
	}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
