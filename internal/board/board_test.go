package board

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// testConfig declares one field of every type.
const testConfig = `kardboard: 1
id: test
columns:
  - id: planning
  - id: backlog
  - id: working_on
    doing: true
    limit: 2
  - id: done
    done: true
fields:
  assignee:
    type: person
    required: [working_on]
  size:
    type: enum
    values: [S, M, L]
    required: [backlog, working_on]
  tags:
    type: list
    values: [bug, feature]
  owner:
    type: text
  estimate:
    type: number
  blocked:
    type: bool
  related:
    type: cards
  reviewer:
    type: person
    values: [arnold, sly]
`

// writeBoard creates a board directory from a map of relative paths to file
// contents. Without a board.yaml entry it uses testConfig.
func writeBoard(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if _, ok := files[ConfigFile]; !ok {
		files[ConfigFile] = testConfig
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func load(t *testing.T, files map[string]string) *Board {
	t.Helper()
	b, err := Load(writeBoard(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// who returns the people on a card as "arnold,sly", or "" for nobody.
func who(c *Card) string {
	return strings.Join(c.Assignees(), ",")
}

func codes(b *Board) []string {
	var out []string
	for _, p := range b.Problems() {
		out = append(out, p.Code)
	}
	return out
}

func TestExampleBoardIsValid(t *testing.T) {
	b, err := Load(filepath.Join("..", "..", "examples", "board"))
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Problems(); len(got) != 0 {
		t.Fatalf("example board has problems: %+v", got)
	}
	if len(b.Cards) != 5 {
		t.Fatalf("got %d cards, want 5", len(b.Cards))
	}

	c, ok := b.Card("export-as-pdf.md")
	if !ok {
		t.Fatal("card export-as-pdf not found")
	}
	want := map[string]any{
		"assignee":   []string{"sly"},
		"size":       "M",
		"milestone":  "release",
		"tags":       []string{"feature"},
		"due":        "2026-12-01",
		"depends_on": []string{"print-templates"},
		"created":    "2026-11-11",
	}
	if !reflect.DeepEqual(c.Fields, want) {
		t.Errorf("fields = %#v, want %#v", c.Fields, want)
	}
	if c.Title != "Export as PDF" || c.Column != "backlog" || who(c) != "sly" {
		t.Errorf("card = %+v", c)
	}

	c, _ = b.Card("login-fails-with-umlauts")
	if who(c) != "arnold" || c.Path != "working_on/login-fails-with-umlauts.md" {
		t.Errorf("assigned card = %+v", c)
	}
	if c, _ = b.Card("offline-mode"); who(c) != "" {
		t.Errorf("unassigned card has assignee %q", who(c))
	}
}

func TestCardRules(t *testing.T) {
	const ok = "---\nsize: S\n---\n\n# Fine\n"
	const taken = "---\nassignee: arnold\nsize: S\n---\n\n# Fine\n"
	tests := []struct {
		name  string
		files map[string]string
		want  []string // codes of all problems, sorted by path then code
	}{
		{"no header where nothing is required", map[string]string{"planning/a.md": "# A\n"}, nil},
		{"empty header", map[string]string{"planning/a.md": "---\n---\n# A\n"}, nil},
		{"blank lines before the title", map[string]string{"planning/a.md": "\n\n# A\n"}, nil},
		{"windows line endings and BOM", map[string]string{"backlog/a.md": "\uFEFF---\r\nsize: M\r\n---\r\n\r\n# A\r\n"}, nil},
		{"every type accepted", map[string]string{
			"backlog/a.md": "---\nassignee: arnold\nreviewer: sly\nsize: L\ntags: [bug, feature]\nowner: Einkauf\nestimate: 2.5\nblocked: false\n" +
				"related: [b]\ndue: 2026-12-01\ncreated: 2026-01-31\ndepends_on: [b]\n---\n# A\n",
			"backlog/b.md": ok,
		}, nil},

		{"E02 header not closed", map[string]string{"planning/a.md": "---\nsize: S\n\n# A\n"}, []string{"E02", "E07"}},
		{"E02 header is not YAML", map[string]string{"planning/a.md": "---\nsize: [S\n---\n# A\n"}, []string{"E02"}},
		{"E02 header is a list", map[string]string{"planning/a.md": "---\n- size\n---\n# A\n"}, []string{"E02"}},
		{"E02 key twice", map[string]string{"planning/a.md": "---\nsize: S\nsize: M\n---\n# A\n"}, []string{"E02"}},

		{"E03 unknown field", map[string]string{"planning/a.md": "---\nprio: 1\n---\n# A\n"}, []string{"E03"}},
		{"E04 reserved field", map[string]string{"planning/a.md": "---\nstatus: done\ncolumn: backlog\n---\n# A\n"}, []string{"E04", "E04"}},
		{"assigned in any column", map[string]string{"planning/a.md": "---\nassignee: arnold\n---\n# A\n", "done/b.md": "---\nassignee: sly-2\n---\n# B\n"}, nil},
		{"in progress and assigned", map[string]string{"working_on/a.md": taken}, nil},
		{"several people on one card", map[string]string{
			"working_on/a.md": "---\nassignee: [arnold, sly]\nsize: S\n---\n# A\n",
			"backlog/b.md":    "---\nassignee:\n  - arnold\n  - sly\nreviewer: [sly, arnold]\nsize: S\n---\n# B\n",
			"done/c.md":       "---\nassignee: [arnold]\n---\n# C\n",
		}, nil},

		{"E05 enum", map[string]string{"planning/a.md": "---\nsize: XXL\n---\n# A\n"}, []string{"E05"}},
		{"E05 enum given a list", map[string]string{"planning/a.md": "---\nsize: [S]\n---\n# A\n"}, []string{"E05"}},
		{"E05 list value not allowed", map[string]string{"planning/a.md": "---\ntags: [bug, nope]\n---\n# A\n"}, []string{"E05"}},
		{"E05 list given a scalar", map[string]string{"planning/a.md": "---\ntags: bug\n---\n# A\n"}, []string{"E05"}},
		{"E05 date in wrong form", map[string]string{"planning/a.md": "---\ndue: 01.12.2026\n---\n# A\n"}, []string{"E05"}},
		{"E05 date that does not exist", map[string]string{"planning/a.md": "---\ndue: 2026-02-30\n---\n# A\n"}, []string{"E05"}},
		{"E05 number", map[string]string{"planning/a.md": "---\nestimate: viel\n---\n# A\n"}, []string{"E05"}},
		{"E05 bool", map[string]string{"planning/a.md": "---\nblocked: vielleicht\n---\n# A\n"}, []string{"E05"}},
		{"E05 text over several lines", map[string]string{"planning/a.md": "---\nowner: |\n  a\n  b\n---\n# A\n"}, []string{"E05"}},
		{"E05 empty value", map[string]string{"planning/a.md": "---\nowner:\n---\n# A\n"}, []string{"E05"}},
		{"E05 person is not a short name", map[string]string{"planning/a.md": "---\nassignee: Arnold Smith\n---\n# A\n"}, []string{"E05"}},
		{"E05 person in a list is not a short name", map[string]string{"planning/a.md": "---\nassignee: [arnold, Sly Smith]\n---\n# A\n"}, []string{"E05"}},
		{"E05 person named twice", map[string]string{"planning/a.md": "---\nassignee: [arnold, sly, arnold]\n---\n# A\n"}, []string{"E05"}},
		{"E05 empty list of people", map[string]string{"planning/a.md": "---\nassignee: []\n---\n# A\n"}, []string{"E05"}},
		{"E05 people as a mapping", map[string]string{"planning/a.md": "---\nassignee: {arnold: true}\n---\n# A\n"}, []string{"E05"}},
		{"E05 person not among the values", map[string]string{"planning/a.md": "---\nreviewer: laura\n---\n# A\n"}, []string{"E05"}},
		{"E05 one of several people not among the values", map[string]string{"planning/a.md": "---\nreviewer: [arnold, laura]\n---\n# A\n"}, []string{"E05"}},
		{"E05 empty list where an assignee is required", map[string]string{"working_on/a.md": "---\nassignee: []\nsize: S\n---\n# A\n"}, []string{"E05"}},
		{"E05 card reference", map[string]string{"planning/a.md": "---\nrelated: [Not_A_Card]\n---\n# A\n"}, []string{"E05"}},

		{"E06 required here", map[string]string{"backlog/a.md": "# A\n"}, []string{"E06"}},
		{"E06 in progress without assignee and size", map[string]string{"working_on/a.md": "# A\n"}, []string{"E06", "E06"}},
		{"E06 in progress without assignee", map[string]string{"working_on/a.md": ok}, []string{"E06"}},
		{"E06 not reported for a wrong value", map[string]string{"backlog/a.md": "---\nsize: XXL\n---\n# A\n"}, []string{"E05"}},

		{"E07 no title", map[string]string{"planning/a.md": "Just text\n"}, []string{"E07"}},
		{"E07 empty file", map[string]string{"planning/a.md": ""}, []string{"E07"}},
		{"E07 second-level heading first", map[string]string{"planning/a.md": "## A\n\n# B\n"}, []string{"E07"}},
		{"E07 hash without space", map[string]string{"planning/a.md": "#A\n"}, []string{"E07"}},

		{"E08 upper case and underscore", map[string]string{"planning/Bad_Name.md": "# A\n"}, []string{"E08"}},
		{"E08 too long", map[string]string{"planning/" + strings.Repeat("a", 81) + ".md": "# A\n"}, []string{"E08"}},

		{"E09 same name in two columns", map[string]string{"planning/a.md": "# A\n", "done/a.md": "# A\n"}, []string{"E09", "E09"}},

		{"E10 missing target", map[string]string{"planning/a.md": "---\ndepends_on: [gone]\n---\n# A\n"}, []string{"E10"}},
		{"E10 in a declared cards field", map[string]string{"planning/a.md": "---\nrelated: [gone]\n---\n# A\n"}, []string{"E10"}},

		{"W01 other board", map[string]string{"planning/a.md": "---\ndepends_on: [atlas/x]\n---\n# A\n"}, []string{"W01"}},
		{"W02 limit of the column", map[string]string{
			"working_on/a.md": taken, "working_on/b.md": taken, "working_on/c.md": taken,
		}, []string{"W02"}},
		{"limit reached but not exceeded", map[string]string{"working_on/a.md": taken, "working_on/b.md": taken}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := load(t, tt.files)
			if got := codes(b); !slices.Equal(got, tt.want) {
				t.Errorf("codes = %v, want %v\nproblems: %+v", got, tt.want, b.Problems())
			}
		})
	}
}

func TestOnlyCardsInColumnsAreRead(t *testing.T) {
	b := load(t, map[string]string{
		"planning/a.md":          "# A\n",
		"planning/.hidden.md":    "not a card",
		"planning/notes.txt":     "not a card",
		"planning/sub/deep.md":   "ignored: columns have no subdirectories",
		"working_on/arnold/y.md": "ignored: columns have no subdirectories",
		"working_on/.keep/z.md":  "ignored: hidden directory",
		"reference/mock.md":      "ignored: not a column",
		"README.md":              "ignored: not in a column",
		"done/.gitkeep":          "",
	})
	if got := codes(b); len(got) != 0 {
		t.Fatalf("problems: %+v", b.Problems())
	}
	if len(b.Cards) != 1 || b.Cards[0].ID != "a" {
		t.Fatalf("cards = %+v", b.Cards)
	}
}

func TestInvalidCardIsStillListed(t *testing.T) {
	b := load(t, map[string]string{"backlog/a.md": "---\nprio: 1\n---\nno title\n"})
	c, ok := b.Card("a")
	if !ok {
		t.Fatal("invalid card is missing from the board")
	}
	if c.Valid() {
		t.Error("card should be invalid")
	}
	if c.Title != "a" {
		t.Errorf("title = %q, want the id as fallback", c.Title)
	}
}

func TestWarningsDoNotInvalidateACard(t *testing.T) {
	b := load(t, map[string]string{"planning/a.md": "---\ndepends_on: [other/x]\n---\n# A\n"})
	if c, _ := b.Card("a"); !c.Valid() {
		t.Error("a warning must not make the card invalid")
	}
}

func TestConfigRules(t *testing.T) {
	const cols = "columns:\n  - id: backlog\n"
	tests := []struct {
		name string
		yaml string
		want string // part of one message
	}{
		{"empty file", "", "empty"},
		{"not YAML", "kardboard: [1\n", "line 1"},
		{"version missing", "id: x\n" + cols, "format version is missing"},
		{"version unknown", "kardboard: 2\nid: x\n" + cols, "version 2 is not supported"},
		{"id missing", "kardboard: 1\n" + cols, "not a valid board id"},
		{"id invalid", "kardboard: 1\nid: Atlas Board\n" + cols, "not a valid board id"},
		{"sync invalid", "kardboard: 1\nid: x\nsync: always\n" + cols, "not one of none, commit, push"},
		{"no columns", "kardboard: 1\nid: x\n", "at least one column"},
		{"column id invalid", "kardboard: 1\nid: x\ncolumns:\n  - id: In Arbeit\n", "not a valid directory name"},
		{"column twice", "kardboard: 1\nid: x\ncolumns:\n  - id: a\n  - id: a\n", "used twice"},
		{"unknown top-level key", "kardboard: 1\nid: x\ncolour: red\n" + cols, "colour"},
		{"unknown column key", "kardboard: 1\nid: x\ncolumns:\n  - id: a\n    wip: 3\n", "wip"},
		{"unknown field key", "kardboard: 1\nid: x\n" + cols + "fields:\n  size:\n    type: text\n    colour: red\n", `unknown key "colour"`},
		{"field name invalid", "kardboard: 1\nid: x\n" + cols + "fields:\n  Size:\n    type: text\n", "not a valid field name"},
		{"field name reserved", "kardboard: 1\nid: x\n" + cols + "fields:\n  status:\n    type: text\n", "reserved"},
		{"field twice", "kardboard: 1\nid: x\n" + cols + "fields:\n  a:\n    type: text\n  a:\n    type: text\n", "declared twice"},
		{"type missing", "kardboard: 1\nid: x\n" + cols + "fields:\n  a:\n    title: A\n", "type is missing"},
		{"type unknown", "kardboard: 1\nid: x\n" + cols + "fields:\n  a:\n    type: string\n", `type "string" is not one of`},
		{"enum without values", "kardboard: 1\nid: x\n" + cols + "fields:\n  a:\n    type: enum\n", "enum needs values"},
		{"values on a date", "kardboard: 1\nid: x\n" + cols + "fields:\n  a:\n    type: date\n    values: [x]\n", "only allowed for enum, list and person"},
		{"old key per_person", "kardboard: 1\nid: x\ncolumns:\n  - id: a\n    per_person: true\n", "per_person"},
		{"required unknown column", "kardboard: 1\nid: x\n" + cols + "fields:\n  a:\n    type: text\n    required: [nope]\n", `unknown column "nope"`},
		{"required not a bool", "kardboard: 1\nid: x\n" + cols + "fields:\n  a:\n    type: text\n    required: maybe\n", "required must be"},
		{"built-in with other type", "kardboard: 1\nid: x\n" + cols + "fields:\n  due:\n    type: text\n", "must keep type date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, msgs := ParseConfig([]byte(tt.yaml))
			if cfg != nil {
				t.Fatal("config accepted")
			}
			if !slices.ContainsFunc(msgs, func(m string) bool { return strings.Contains(m, tt.want) }) {
				t.Errorf("messages %q do not mention %q", msgs, tt.want)
			}
		})
	}
}

func TestConfigDefaultsAndOrder(t *testing.T) {
	cfg, msgs := ParseConfig([]byte(testConfig))
	if len(msgs) != 0 {
		t.Fatal(msgs)
	}
	if cfg.Sync != "none" {
		t.Errorf("sync = %q, want none", cfg.Sync)
	}
	var names []string
	for _, f := range cfg.AllFields() {
		names = append(names, f.Name)
	}
	// declared fields in file order; assignee and tags are redeclared, so
	// they are not repeated among the built-in ones
	want := []string{"assignee", "size", "tags", "owner", "estimate", "blocked", "related", "reviewer", "due", "depends_on", "created"}
	if !slices.Equal(names, want) {
		t.Errorf("fields = %v, want %v", names, want)
	}
	if f, _ := cfg.Field("size"); !f.Required.In("backlog") || f.Required.In("planning") {
		t.Errorf("size required = %+v", f.Required)
	}
}

func TestBrokenConfigStopsTheBoard(t *testing.T) {
	b := load(t, map[string]string{ConfigFile: "kardboard: 1\nid: x\n", "backlog/a.md": "# A\n"})
	if got := codes(b); !slices.Equal(got, []string{"E01"}) {
		t.Errorf("codes = %v, want [E01]", got)
	}
	if b.Config != nil || len(b.Cards) != 0 {
		t.Error("a board with a broken board.yaml must not read cards")
	}
}

func TestMissingConfig(t *testing.T) {
	b, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := codes(b); !slices.Equal(got, []string{"E01"}) {
		t.Errorf("codes = %v, want [E01]", got)
	}
}

func TestFindRoot(t *testing.T) {
	mk := func(t *testing.T, root string, paths ...string) {
		t.Helper()
		for _, p := range paths {
			full := filepath.Join(root, filepath.FromSlash(p))
			if strings.HasSuffix(p, "/") {
				if err := os.MkdirAll(full, 0o755); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	tmp := func(t *testing.T) string {
		t.Helper()
		// macOS reaches the temp dir through a symlink; resolve it so paths compare.
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return dir
	}

	t.Run("from inside the board", func(t *testing.T) {
		root := tmp(t)
		mk(t, root, "board/board.yaml", "board/working_on/")
		got, err := FindRoot(filepath.Join(root, "board", "working_on"))
		if err != nil || got != filepath.Join(root, "board") {
			t.Errorf("got %q, %v", got, err)
		}
	})
	t.Run("from elsewhere in the repository", func(t *testing.T) {
		root := tmp(t)
		mk(t, root, ".git/", "src/app/", "planning/board/board.yaml", "node_modules/pkg/board.yaml", ".cache/board.yaml")
		got, err := FindRoot(filepath.Join(root, "src", "app"))
		if err != nil || got != filepath.Join(root, "planning", "board") {
			t.Errorf("got %q, %v", got, err)
		}
	})
	t.Run("two boards in one repository", func(t *testing.T) {
		root := tmp(t)
		mk(t, root, ".git/", "a/board.yaml", "b/board.yaml")
		if _, err := FindRoot(root); !errors.Is(err, ErrNoBoard) || !strings.Contains(err.Error(), "2 boards") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("repository without a board", func(t *testing.T) {
		root := tmp(t)
		mk(t, root, ".git/", "src/")
		if _, err := FindRoot(filepath.Join(root, "src")); !errors.Is(err, ErrNoBoard) {
			t.Errorf("err = %v", err)
		}
	})
}
