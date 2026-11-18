package registry

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dvpnk/kardboard/internal/userconfig"
)

// tree creates files below a fresh directory. A path ending in "/" is an
// empty directory; board.yaml files get a valid board with the given id.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	// macOS reaches the temp dir through a symlink; resolve it so paths compare.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if strings.HasSuffix(rel, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func boardYAML(id string) string {
	return "kardboard: 1\nid: " + id + "\ncolumns:\n  - id: todo\n  - id: done\n    done: true\n"
}

func rel(t *testing.T, root string, paths []string) []string {
	t.Helper()
	out := make([]string, len(paths))
	for i, p := range paths {
		r, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = filepath.ToSlash(r)
	}
	return out
}

func TestDiscover(t *testing.T) {
	root := tree(t, map[string]string{
		"alpha/board/board.yaml":                boardYAML("alpha"),
		"alpha/board/todo/a.md":                 "# A\n",
		"alpha/board/todo/nested/board.yaml":    boardYAML("inside-a-board"),
		"beta-board/board.yaml":                 boardYAML("beta"),
		"group/gamma/planning/board/board.yaml": boardYAML("gamma"),
		"alpha/node_modules/pkg/board.yaml":     boardYAML("dependency"),
		"alpha/vendor/x/board.yaml":             boardYAML("vendored"),
		".hidden/board.yaml":                    boardYAML("hidden"),
		"a/b/c/d/e/f/g/too-deep/board.yaml":     boardYAML("deep"),
		"a/b/c/d/e/just-in-reach/board.yaml":    boardYAML("reach"),
		"not-a-board/board.yaml/keep":           "board.yaml is a directory here",
		"empty/":                                "",
	})
	got := rel(t, root, Discover(root))
	want := []string{"a/b/c/d/e/just-in-reach", "alpha/board", "beta-board", "group/gamma/planning/board"}
	if !slices.Equal(got, want) {
		t.Errorf("found %v, want %v", got, want)
	}

	// The directory itself may be a board, and a missing directory is empty.
	if got := Discover(filepath.Join(root, "beta-board")); len(got) != 1 {
		t.Errorf("a board directory: %v", got)
	}
	if got := Discover(filepath.Join(root, "nope")); len(got) != 0 {
		t.Errorf("a missing directory: %v", got)
	}
}

func TestEntries(t *testing.T) {
	root := tree(t, map[string]string{
		"added/board.yaml":     boardYAML("added"),
		"repos/one/board.yaml": boardYAML("one"),
		"repos/two/board.yaml": boardYAML("two"),
		"elsewhere/board.yaml": boardYAML("here"),
	})
	cfg := &userconfig.Config{
		Boards: []string{filepath.Join(root, "added"), filepath.Join(root, "repos", "one")},
		Scan:   []string{filepath.Join(root, "repos")},
	}

	entries := Entries(cfg, filepath.Join(root, "elsewhere"))
	var got []string
	for _, e := range entries {
		r, _ := filepath.Rel(root, e.Root)
		got = append(got, filepath.ToSlash(r)+":"+string(e.Source))
	}
	// repos/one is both added and found by the scan: it is listed once, as added.
	want := []string{"added:added", "repos/one:added", "elsewhere:here", "repos/two:scan"}
	if !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}

	if got := Entries(&userconfig.Config{}, ""); len(got) != 0 {
		t.Errorf("no settings: %v", got)
	}
	// The current board is not repeated if the user added it already.
	if got := Entries(cfg, filepath.Join(root, "added")); len(got) != 3 {
		t.Errorf("here = an added board: %d entries", len(got))
	}
}

func TestLoad(t *testing.T) {
	root := tree(t, map[string]string{
		"a/board.yaml":      boardYAML("same"),
		"a/todo/x.md":       "# X\n",
		"b/board.yaml":      boardYAML("same"),
		"c/board.yaml":      boardYAML("other"),
		"broken/board.yaml": "kardboard: 1\n",
		"empty/":            "",
	})
	entry := func(name string) Entry { return Entry{Root: filepath.Join(root, name), Source: SourceAdded} }
	loaded := Load([]Entry{entry("a"), entry("b"), entry("c"), entry("broken"), entry("empty"), entry("missing")})

	if len(loaded) != 6 {
		t.Fatalf("got %d boards", len(loaded))
	}
	a, b, c, broken, empty, missing := loaded[0], loaded[1], loaded[2], loaded[3], loaded[4], loaded[5]
	if a.Err != "" || a.ID() != "same" || len(a.Board.Cards) != 1 {
		t.Errorf("a = %+v", a)
	}
	if !strings.Contains(b.Err, `id "same" is already used`) {
		t.Errorf("b.Err = %q", b.Err)
	}
	if c.Err != "" || c.ID() != "other" {
		t.Errorf("c = %+v", c)
	}
	// A board.yaml that is invalid or missing is the board's own problem
	// (rule E01), not a failure to load.
	for name, l := range map[string]*Loaded{"broken": broken, "empty": empty, "missing": missing} {
		if l.Err != "" || l.ID() != "" || l.Board == nil || len(l.Board.Problems()) == 0 {
			t.Errorf("%s = %+v", name, l)
		}
	}

	if l, ok := Find(loaded, "same"); !ok || l != a {
		t.Error("Find must return the board that keeps the id")
	}
	if _, ok := Find(loaded, "nope"); ok {
		t.Error("Find found an unknown id")
	}
	if _, ok := Find(loaded, ""); ok {
		t.Error("Find found a board without id")
	}
}
