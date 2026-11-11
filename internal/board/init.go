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

	"go.yaml.in/yaml/v3"
)

// InitOptions controls Init.
type InitOptions struct {
	// ID is the board's id. Empty means: derive it from the directory name.
	ID string
	// Name is the display name. It may be empty.
	Name string
	// Sync is none, commit or push. Empty means none.
	Sync string
	// Columns names the columns in order. Empty means: take the directories
	// that already hold cards, or three usual columns if there are none.
	Columns []string
}

// InitResult says what Init did.
type InitResult struct {
	// Root is the absolute path of the new board.
	Root string `json:"root"`
	ID   string `json:"id"`
	// Columns are the columns in order.
	Columns []string `json:"columns"`
	// Adopted is true if the columns are directories that were already there.
	Adopted bool `json:"adopted"`
}

// Column names with a known place and meaning. Directories found by Init
// are put in this order; names it does not know go in the middle.
var (
	columnRank = map[string]int{
		"ideas": 0, "planning": 0, "inbox": 0,
		"backlog": 1, "todo": 1, "next": 1,
		"working_on": 3, "doing": 3, "in_progress": 3, "in-progress": 3, "wip": 3,
		"review": 4, "blocked": 5, "deferred": 5, "waiting": 5,
		"done": 6, "finished": 6, "closed": 6,
		"archive": 7,
	}
	doingNames = []string{"working_on", "doing", "in_progress", "in-progress", "wip"}
	doneNames  = []string{"done", "finished", "closed"}
)

const unknownRank = 2

// Init turns a directory into a board by writing its board.yaml. It never
// changes or moves a file that is already there: directories that hold
// Markdown files become the columns, as they are. An empty directory gets
// three usual columns. The directory is created if it does not exist.
func Init(dir string, opt InitOptions) (*InitResult, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	file := filepath.Join(root, ConfigFile)
	if _, err := os.Lstat(file); err == nil {
		return nil, fmt.Errorf("%s is a board already: it has a %s", root, ConfigFile)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	res := &InitResult{Root: root, ID: opt.ID}
	if res.ID == "" {
		// A board that lives in its project as "board" is named after the
		// project.
		name := filepath.Base(root)
		if name == "board" || name == "kanban" || name == "cards" {
			name = filepath.Base(filepath.Dir(root))
		}
		if res.ID = Slug(name); res.ID == "" {
			return nil, fmt.Errorf("cannot derive a board id from the directory name %q, give one with --id", name)
		}
	}

	switch {
	case len(opt.Columns) > 0:
		res.Columns = opt.Columns
	default:
		if res.Columns, err = cardDirs(root); err != nil {
			return nil, err
		}
		res.Adopted = len(res.Columns) > 0
		if !res.Adopted {
			res.Columns = []string{"backlog", "working_on", "done"}
		}
	}

	text := configText(res.ID, opt.Name, opt.Sync, res.Columns)
	// Check before writing: a wrong id, sync mode or column name is reported
	// in the words of the format, and nothing is left behind, not even the
	// directory.
	if _, msgs := ParseConfig([]byte(text)); len(msgs) > 0 {
		return nil, errors.New(strings.Join(msgs, "; "))
	}
	for _, col := range res.Columns {
		if err := os.MkdirAll(filepath.Join(root, col), 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return nil, err
	}
	return res, f.Close()
}

// cardDirs returns the directories in root that hold Markdown files, either
// directly or one level down (boards that keep a directory per person), in
// the order a board usually has them.
func cardDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil // Init creates the directory
	}
	if err != nil {
		return nil, err
	}
	var found []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") || !columnIDRe.MatchString(name) {
			continue
		}
		if holdsCards(filepath.Join(root, name), 1) {
			found = append(found, name)
		}
	}
	rank := func(name string) int {
		if r, ok := columnRank[name]; ok {
			return r
		}
		return unknownRank
	}
	slices.SortStableFunc(found, func(a, b string) int {
		return cmp.Or(cmp.Compare(rank(a), rank(b)), cmp.Compare(a, b))
	})
	return found, nil
}

func holdsCards(dir string, deeper int) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if isCardFile(e) || (deeper > 0 && e.IsDir() && holdsCards(filepath.Join(dir, e.Name()), deeper-1)) {
			return true
		}
	}
	return false
}

// configText writes a board.yaml the way a person would: short, in the
// order of the spec, with nothing that is not needed.
func configText(id, name, sync string, columns []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "kardboard: %d\nid: %s\n", FormatVersion, id)
	if name != "" {
		// Let YAML quote the name if it has to.
		if quoted, err := yaml.Marshal(name); err == nil {
			b.WriteString("name: " + string(quoted))
		}
	}
	if sync == "" {
		sync = "none"
	}
	fmt.Fprintf(&b, "sync: %s\n\ncolumns:\n", sync)
	doing := slices.IndexFunc(columns, func(c string) bool { return slices.Contains(doingNames, c) })
	done := slices.IndexFunc(columns, func(c string) bool { return slices.Contains(doneNames, c) })
	for i, col := range columns {
		fmt.Fprintf(&b, "  - id: %s\n", col)
		if i == doing {
			b.WriteString("    doing: true\n")
		}
		if i == done {
			b.WriteString("    done: true\n")
		}
	}
	return b.String()
}
