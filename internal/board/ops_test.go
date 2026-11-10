package board

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const sized = "---\nsize: S\n---\n\n# Sized\n"

func has(t *testing.T, b *Board, rel string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(b.Root, filepath.FromSlash(rel)))
	return err == nil
}

func read(t *testing.T, b *Board, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(b.Root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// reload reads the board again from disk and fails on any problem.
func reload(t *testing.T, b *Board) *Board {
	t.Helper()
	fresh, err := Load(b.Root)
	if err != nil {
		t.Fatal(err)
	}
	if got := fresh.Problems(); len(got) != 0 {
		t.Fatalf("board has problems after the change: %+v", got)
	}
	return fresh
}

func TestMove(t *testing.T) {
	b := load(t, map[string]string{"backlog/a.md": sized})

	res, err := b.Move("a.md", "working_on", MoveOptions{User: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	want := MoveResult{ID: "a", From: "backlog", To: "working_on/alex", Path: "working_on/alex/a.md", Moved: true}
	if *res != want {
		t.Errorf("result = %+v, want %+v", *res, want)
	}
	if has(t, b, "backlog/a.md") || read(t, b, "working_on/alex/a.md") != sized {
		t.Error("file was not moved unchanged")
	}

	b = reload(t, b)
	if res, err = b.Move("a", "done", MoveOptions{User: "alex"}); err != nil || res.To != "done" {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if has(t, b, "working_on/alex") {
		t.Error("empty person directory was left behind")
	}
	reload(t, b)
}

func TestMoveToSamePlaceDoesNothing(t *testing.T) {
	b := load(t, map[string]string{"working_on/alex/a.md": sized})
	res, err := b.Move("a", "working_on", MoveOptions{User: "alex"})
	if err != nil || res.Moved || res.From != res.To {
		t.Errorf("res %+v, err %v", res, err)
	}
	if !has(t, b, "working_on/alex/a.md") {
		t.Error("card is gone")
	}
}

func TestMoveKeepsPersonDirectoryWithOtherCards(t *testing.T) {
	b := load(t, map[string]string{"working_on/alex/a.md": sized, "working_on/alex/b.md": sized})
	if _, err := b.Move("a", "done", MoveOptions{User: "alex"}); err != nil {
		t.Fatal(err)
	}
	if !has(t, b, "working_on/alex/b.md") {
		t.Error("other card is gone")
	}
}

func TestMoveRules(t *testing.T) {
	files := func() map[string]string {
		return map[string]string{
			"planning/unsized.md":    "# No size\n",
			"backlog/sized.md":       sized,
			"working_on/sam/sams.md": sized,
			"planning/twice.md":      "# A\n",
			"done/twice.md":          "# A\n",
		}
	}
	tests := []struct {
		name    string
		id, col string
		opt     MoveOptions
		refused bool   // want a RefusedError
		wantErr string // part of the message, "" for success
	}{
		{"unknown card", "nope", "done", MoveOptions{}, false, `no card "nope"`},
		{"unknown column", "sized", "nope", MoveOptions{}, false, `no column "nope"`},
		{"ambiguous card", "twice", "backlog", MoveOptions{}, false, "exists 2 times"},
		{"person for a plain column", "sized", "done", MoveOptions{Person: "sam"}, false, "no directories per person"},
		{"invalid person", "sized", "working_on", MoveOptions{Person: "Sam Smith"}, false, "not a valid person name"},
		{"person column without a name", "sized", "working_on", MoveOptions{}, false, ErrNeedUser.Error()},

		{"required field missing", "unsized", "backlog", MoveOptions{}, true, "lacks size, required in backlog"},
		{"required field missing, forced", "unsized", "backlog", MoveOptions{Force: true}, false, ""},

		{"someone else's card", "sams", "done", MoveOptions{User: "alex"}, true, "in the directory of sam"},
		{"someone else's card, forced", "sams", "done", MoveOptions{User: "alex", Force: true}, false, ""},
		{"someone else's card, user unknown", "sams", "done", MoveOptions{}, false, ErrNeedUser.Error()},
		{"own card", "sams", "done", MoveOptions{User: "sam"}, false, ""},
		{"own card handed over", "sams", "working_on", MoveOptions{User: "sam", Person: "alex"}, false, ""},
		{"unassigned card for someone else", "sized", "working_on", MoveOptions{User: "alex", Person: "sam"}, false, ""},
		{"plain move needs no user", "sized", "done", MoveOptions{}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := load(t, files())
			before, _ := b.Card(tt.id)
			oldPath := ""
			if before != nil {
				oldPath = before.Path
			}

			res, err := b.Move(tt.id, tt.col, tt.opt)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !res.Moved || !has(t, b, res.Path) || has(t, b, oldPath) {
					t.Errorf("card did not move: %+v", res)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.wantErr)
			}
			var refused *RefusedError
			if errors.As(err, &refused) != tt.refused {
				t.Errorf("RefusedError = %v, want %v", !tt.refused, tt.refused)
			}
			if oldPath != "" && !has(t, b, oldPath) {
				t.Error("a rejected move changed the board")
			}
		})
	}
}

func TestMoveDoesNotOverwrite(t *testing.T) {
	// A file that appeared at the target after the board was read must
	// survive.
	b := load(t, map[string]string{"backlog/a.md": sized})
	if err := os.MkdirAll(filepath.Join(b.Root, "done"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.Root, "done", "a.md"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Move("a", "done", MoveOptions{}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v", err)
	}
	if read(t, b, "done/a.md") != "other" || !has(t, b, "backlog/a.md") {
		t.Error("existing file was overwritten")
	}
}

func TestChangesNeedSyncNone(t *testing.T) {
	for _, sync := range []string{"commit", "push"} {
		b := load(t, map[string]string{
			ConfigFile:      "kardboard: 1\nid: x\nsync: " + sync + "\ncolumns:\n  - id: backlog\n  - id: done\n",
			"backlog/a.md":  "# A\n",
			"done/.gitkeep": "",
		})
		if _, err := b.Move("a", "done", MoveOptions{}); err == nil || !strings.Contains(err.Error(), "sync: "+sync) {
			t.Errorf("move with sync %s: err = %v", sync, err)
		}
		if _, err := b.New("B", NewOptions{}); err == nil || !strings.Contains(err.Error(), "sync: "+sync) {
			t.Errorf("new with sync %s: err = %v", sync, err)
		}
		if !has(t, b, "backlog/a.md") || has(t, b, "backlog/b.md") {
			t.Error("board was changed")
		}
	}
}

func TestNew(t *testing.T) {
	b := load(t, map[string]string{"backlog/other.md": sized})

	c, err := b.New("  Größe ändern  ", NewOptions{Today: "2026-11-11"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "groesse-aendern" || c.Path != "planning/groesse-aendern.md" || c.Title != "Größe ändern" {
		t.Errorf("card = %+v", c)
	}
	if got, want := read(t, b, c.Path), "---\ncreated: 2026-11-11\n---\n\n# Größe ändern\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}

	c, err = b.New("With everything", NewOptions{
		Column: "working_on", User: "alex", Today: "2026-11-11",
		Set: []string{
			"size=M", "tags=bug, feature", "owner=Einkauf: Nord", "estimate=2.5", "blocked=true",
			"related=[other]", "depends_on=other,groesse-aendern", "due = 2026-12-24", "created=2026-01-01",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\n" +
		"size: M\n" +
		"tags: [bug, feature]\n" +
		"owner: 'Einkauf: Nord'\n" +
		"estimate: 2.5\n" +
		"blocked: true\n" +
		"related: [other]\n" +
		"due: 2026-12-24\n" +
		"depends_on: [other, groesse-aendern]\n" +
		"created: 2026-01-01\n" +
		"---\n\n# With everything\n"
	if got := read(t, b, "working_on/alex/with-everything.md"); got != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}
	if c.Person != "alex" || c.Fields["estimate"] != 2.5 || c.Fields["blocked"] != true {
		t.Errorf("card = %+v", c)
	}

	if _, err = b.New("Explicit", NewOptions{ID: "my-id", Person: "sam", Column: "working_on", Set: []string{"size=S"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, b, "working_on/sam/my-id.md"), "---\nsize: S\n---\n\n# Explicit\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}

	// Everything New wrote must be a valid board.
	if fresh := reload(t, b); len(fresh.Cards) != 4 {
		t.Errorf("got %d cards, want 4", len(fresh.Cards))
	}
}

func TestNewWithoutAnyField(t *testing.T) {
	b := load(t, map[string]string{})
	c, err := b.New("Plain", NewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, b, c.Path); got != "# Plain\n" {
		t.Errorf("file = %q", got)
	}
}

func TestNewRules(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		opt     NewOptions
		refused bool
		wantErr string
	}{
		{"empty title", "  ", NewOptions{}, false, "one line"},
		{"title over two lines", "a\nb", NewOptions{}, false, "one line"},
		{"title without letters", "???", NewOptions{}, false, "cannot derive a file name"},
		{"invalid id", "A", NewOptions{ID: "Not_Valid"}, false, "not a valid card id"},
		{"id taken", "Other", NewOptions{}, false, "already exists at backlog/other.md"},
		{"id taken, given explicitly", "A", NewOptions{ID: "other"}, false, "already exists"},
		{"unknown column", "A", NewOptions{Column: "nope"}, false, `no column "nope"`},
		{"person column without a name", "A", NewOptions{Column: "working_on"}, false, ErrNeedUser.Error()},
		{"required field missing", "A", NewOptions{Column: "backlog"}, true, "column backlog requires size"},
		{"set without =", "A", NewOptions{Set: []string{"size"}}, false, "expected field=value"},
		{"set unknown field", "A", NewOptions{Set: []string{"prio=1"}}, false, `unknown field "prio"`},
		{"set reserved field", "A", NewOptions{Set: []string{"status=done"}}, false, "cannot be set"},
		{"set wrong enum", "A", NewOptions{Set: []string{"size=XXL"}}, false, "size: must be one of"},
		{"set wrong date", "A", NewOptions{Set: []string{"due=morgen"}}, false, "due: must be a date"},
		{"set wrong number", "A", NewOptions{Set: []string{"estimate=viel"}}, false, "estimate: must be a number"},
		{"set wrong list value", "A", NewOptions{Set: []string{"tags=bug,nope"}}, false, `"nope" is not one of`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := load(t, map[string]string{"backlog/other.md": sized})
			_, err := b.New(tt.title, tt.opt)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.wantErr)
			}
			var refused *RefusedError
			if errors.As(err, &refused) != tt.refused {
				t.Errorf("RefusedError = %v, want %v", !tt.refused, tt.refused)
			}
			if fresh := reload(t, b); len(fresh.Cards) != 1 {
				t.Errorf("a rejected card was written: %d cards", len(fresh.Cards))
			}
		})
	}

	t.Run("required field missing, forced", func(t *testing.T) {
		b := load(t, map[string]string{})
		if _, err := b.New("A", NewOptions{Column: "backlog", Force: true}); err != nil {
			t.Fatal(err)
		}
		fresh, _ := Load(b.Root)
		if got := codes(fresh); !slices.Equal(got, []string{"E06"}) {
			t.Errorf("codes = %v, want [E06]", got)
		}
	})
}

func TestSlug(t *testing.T) {
	long := strings.Repeat("wort ", 30) // 150 characters
	tests := []struct{ title, want string }{
		{"Export als PDF", "export-als-pdf"},
		{"Größe ändern: Überschrift & Fußzeile (v2)", "groesse-aendern-ueberschrift-fusszeile-v2"},
		{"  --Hello,   World!--  ", "hello-world"},
		{"Atlas ↔ Nova", "atlas-nova"},
		{"Café Señor Łódź", "cafe-senor-lodz"},
		{"„Zitat“ — mit Gedankenstrich", "zitat-mit-gedankenstrich"},
		{"100% fertig", "100-fertig"},
		{"日本語", ""},
		{"???", ""},
		{long, strings.TrimSuffix(strings.Repeat("wort-", 16), "-")},
		{strings.Repeat("a", 100), strings.Repeat("a", 80)},
		{strings.Repeat("a", 80) + " b", strings.Repeat("a", 80)},
	}
	for _, tt := range tests {
		got := Slug(tt.title)
		if got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.title, got, tt.want)
		}
		if got != "" && (!slugRe.MatchString(got) || len(got) > maxIDLength) {
			t.Errorf("Slug(%q) = %q is not a valid id", tt.title, got)
		}
	}
}

func TestFirstColumns(t *testing.T) {
	cfg, _ := ParseConfig([]byte(testConfig))
	if col, ok := cfg.FirstPerPerson(); !ok || col.ID != "working_on" {
		t.Errorf("FirstPerPerson = %+v, %v", col, ok)
	}
	if col, ok := cfg.FirstDone(); !ok || col.ID != "done" {
		t.Errorf("FirstDone = %+v, %v", col, ok)
	}
	plain, _ := ParseConfig([]byte("kardboard: 1\nid: x\ncolumns:\n  - id: a\n"))
	if _, ok := plain.FirstPerPerson(); ok {
		t.Error("FirstPerPerson found a column on a board without one")
	}
	if _, ok := plain.FirstDone(); ok {
		t.Error("FirstDone found a column on a board without one")
	}
}
