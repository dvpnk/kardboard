package board

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
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

func TestMoveRenamesWithoutTouchingTheText(t *testing.T) {
	b := load(t, map[string]string{"planning/a.md": sized})

	res, err := b.Change("a.md", Change{Column: "backlog"})
	if err != nil {
		t.Fatal(err)
	}
	want := ChangeResult{ID: "a", From: "planning", To: "backlog", Path: "backlog/a.md", Moved: true}
	if !reflect.DeepEqual(*res, want) {
		t.Errorf("result = %+v, want %+v", *res, want)
	}
	if has(t, b, "planning/a.md") || read(t, b, "backlog/a.md") != sized {
		t.Error("file was not moved unchanged")
	}
	reload(t, b)
}

func TestTakeAssignsAndMovesInOneStep(t *testing.T) {
	b := load(t, map[string]string{"backlog/a.md": sized})

	res, err := b.Change("a", Change{Column: "working_on", Set: []string{"assignee=arnold"}, User: "arnold"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Moved || res.To != "working_on" || !reflect.DeepEqual(res.Set, map[string]any{"assignee": []string{"arnold"}}) {
		t.Errorf("result = %+v", res)
	}
	if got, want := read(t, b, "working_on/a.md"), "---\nsize: S\nassignee: arnold\n---\n\n# Sized\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	if c, _ := reload(t, b).Card("a"); who(c) != "arnold" {
		t.Errorf("assignee = %q", who(c))
	}
}

func TestAssignWithoutMoving(t *testing.T) {
	b := load(t, map[string]string{"backlog/a.md": sized})

	res, err := b.Change("a", Change{Set: []string{"assignee=laura"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Moved || res.From != "backlog" || res.To != "backlog" {
		t.Errorf("result = %+v", res)
	}
	c, _ := reload(t, b).Card("a")
	if who(c) != "laura" || c.Column != "backlog" {
		t.Errorf("card = %+v", c)
	}

	// The card is laura's now. She can hand it over and drop it.
	b = reload(t, b)
	if _, err = b.Change("a", Change{Set: []string{"assignee=sly"}, User: "laura"}); err != nil {
		t.Fatal(err)
	}
	b = reload(t, b)
	res, err = b.Change("a", Change{Set: []string{"assignee="}, User: "sly"})
	if err != nil || !slices.Equal(res.Removed, []string{"assignee"}) {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if got := read(t, b, "backlog/a.md"); got != sized {
		t.Errorf("file = %q, want the original %q", got, sized)
	}
}

func TestSeveralPeopleOnOneCard(t *testing.T) {
	b := load(t, map[string]string{"backlog/a.md": sized})

	// Two people are written as a list, one as a plain name.
	res, err := b.Change("a", Change{Set: []string{"assignee=arnold, sly"}})
	if err != nil || !reflect.DeepEqual(res.Set, map[string]any{"assignee": []string{"arnold", "sly"}}) {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if got, want := read(t, b, "backlog/a.md"), "---\nsize: S\nassignee: [arnold, sly]\n---\n\n# Sized\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	b = reload(t, b)
	if c, _ := b.Card("a"); who(c) != "arnold,sly" {
		t.Errorf("assignees = %q", who(c))
	}

	// The card belongs to both of them, and to nobody else.
	if _, err = b.Change("a", Change{Column: "working_on", User: "laura"}); err == nil || !strings.Contains(err.Error(), "is assigned to arnold, sly") {
		t.Errorf("laura: err = %v", err)
	}
	if _, err = b.Change("a", Change{Column: "working_on", User: "sly"}); err != nil {
		t.Errorf("sly: %v", err)
	}

	// Down to one person, the list becomes a plain name again.
	b = reload(t, b)
	if _, err = b.Change("a", Change{Set: []string{"assignee=sly"}, User: "arnold"}); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, b, "working_on/a.md"), "---\nsize: S\nassignee: sly\n---\n\n# Sized\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}

	// The same people in the same order are no change.
	b = load(t, map[string]string{"backlog/a.md": "---\nassignee: [arnold, sly]\nsize: S\n---\n# A\n"})
	if res, err = b.Change("a", Change{Set: []string{"assignee=arnold,sly"}}); err != nil || res.Changed() {
		t.Errorf("res %+v, err %v", res, err)
	}
}

func TestNothingToChange(t *testing.T) {
	const card = "---\nassignee: arnold\nsize: S\ntags: [bug]\n---\n\n# A\n"
	b := load(t, map[string]string{"working_on/a.md": card})

	// Same column, same values: allowed for anyone, and the file stays as it is.
	for _, ch := range []Change{
		{Column: "working_on"},
		{Set: []string{"assignee=arnold"}},
		{Column: "working_on", Set: []string{"size=S", "tags=bug"}},
		{Set: []string{"owner="}}, // removing what is not there
	} {
		res, err := b.Change("a", ch)
		if err != nil || res.Changed() {
			t.Errorf("change %+v: res %+v, err %v", ch, res, err)
		}
	}
	if read(t, b, "working_on/a.md") != card {
		t.Error("file was rewritten")
	}
}

func TestChangeRules(t *testing.T) {
	files := func() map[string]string {
		return map[string]string{
			"planning/unsized.md": "# No size\n",
			"backlog/sized.md":    sized,
			"backlog/slys.md":     "---\nassignee: sly\nsize: S\n---\n\n# Sly's, not started\n",
			"working_on/busy.md":  "---\nassignee: sly\nsize: S\n---\n\n# Sly's, in progress\n",
			"planning/twice.md":   "# A\n",
			"done/twice.md":       "# A\n",
		}
	}
	tests := []struct {
		name    string
		id      string
		ch      Change
		refused bool   // want a RefusedError
		wantErr string // part of the message, "" for success
	}{
		{"unknown card", "nope", Change{Column: "done"}, false, `no card "nope"`},
		{"unknown column", "sized", Change{Column: "nope"}, false, `no column "nope"`},
		{"ambiguous card", "twice", Change{Column: "backlog"}, false, "exists 2 times"},
		{"unknown field", "sized", Change{Set: []string{"prio=1"}}, false, `unknown field "prio"`},
		{"reserved field", "sized", Change{Set: []string{"status=done"}}, false, "cannot be set"},
		{"wrong value", "sized", Change{Set: []string{"size=XXL"}}, false, "size: must be one of"},
		{"invalid person", "sized", Change{Set: []string{"assignee=Sly Smith"}}, false, `assignee: "Sly Smith" is neither a short name`},
		{"field twice", "sized", Change{Set: []string{"size=S", "size=M"}}, false, "given twice"},

		{"required field missing in target", "unsized", Change{Column: "backlog"}, true, "would lack size, required in backlog"},
		{"required field missing, forced", "unsized", Change{Column: "backlog", Force: true}, false, ""},
		{"required field supplied with the move", "unsized", Change{Column: "backlog", Set: []string{"size=M"}}, false, ""},
		{"in progress needs an assignee", "sized", Change{Column: "working_on"}, true, "would lack assignee, required in working_on"},
		{"removing a required field", "sized", Change{Set: []string{"size="}}, true, "would lack size, required in backlog"},
		{"removing a required field, forced", "sized", Change{Set: []string{"size="}, Force: true}, false, ""},
		{"removing a required field while moving where it is optional", "sized", Change{Column: "planning", Set: []string{"size="}}, false, ""},
		{"a field missing before does not block other edits", "unsized", Change{Set: []string{"owner=Einkauf"}}, false, ""},

		{"someone else's card, move", "busy", Change{Column: "done", User: "arnold"}, true, "is assigned to sly"},
		{"someone else's card, reassign", "busy", Change{Set: []string{"assignee=arnold"}, User: "arnold"}, true, "is assigned to sly"},
		{"someone else's card, edit", "busy", Change{Set: []string{"tags=bug"}, User: "arnold"}, true, "is assigned to sly"},
		{"someone else's card in the backlog", "slys", Change{Column: "working_on", Set: []string{"assignee=arnold"}, User: "arnold"}, true, "is assigned to sly"},
		{"someone else's card, forced", "busy", Change{Column: "done", User: "arnold", Force: true}, false, ""},
		{"someone else's card, user unknown", "busy", Change{Column: "done"}, false, ErrNeedUser.Error()},
		{"someone else's card, user unknown, forced", "busy", Change{Column: "done", Force: true}, false, ""},
		{"own card", "busy", Change{Column: "done", User: "sly"}, false, ""},
		{"own card handed over", "busy", Change{Set: []string{"assignee=arnold"}, User: "sly"}, false, ""},
		{"own card given up where an assignee is required", "busy", Change{Set: []string{"assignee="}, User: "sly"}, true, "would lack assignee"},
		{"nobody's card needs no user", "sized", Change{Column: "done"}, false, ""},
		{"nobody's card assigned to someone else", "sized", Change{Set: []string{"assignee=sly"}, User: "arnold"}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := load(t, files())
			oldPath, oldText := "", ""
			if c, ok := b.Card(tt.id); ok {
				oldPath, oldText = c.Path, read(t, b, c.Path)
			}

			res, err := b.Change(tt.id, tt.ch)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !res.Changed() || !has(t, b, res.Path) || (res.Moved && has(t, b, oldPath)) {
					t.Errorf("change did not happen: %+v", res)
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
			if oldPath != "" && (!has(t, b, oldPath) || read(t, b, oldPath) != oldText) {
				t.Error("a rejected change touched the card")
			}
		})
	}
}

func TestRejectedChangeLeavesNoTrace(t *testing.T) {
	// The edit is fine but the move is not: neither must happen.
	b := load(t, map[string]string{"backlog/a.md": sized})
	if err := os.MkdirAll(filepath.Join(b.Root, "done"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.Root, "done", "a.md"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := b.Change("a", Change{Column: "done", Set: []string{"tags=bug"}})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v", err)
	}
	if read(t, b, "done/a.md") != "other" || read(t, b, "backlog/a.md") != sized {
		t.Error("the board was changed")
	}
	entries, _ := os.ReadDir(filepath.Join(b.Root, "backlog"))
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

func TestChangeRefusesBrokenHeader(t *testing.T) {
	const broken = "---\nsize: [S\n---\n# A\n"
	b := load(t, map[string]string{"planning/a.md": broken})
	if _, err := b.Change("a", Change{Set: []string{"owner=x"}}); err == nil || !strings.Contains(err.Error(), "not valid YAML") {
		t.Errorf("err = %v", err)
	}
	if read(t, b, "planning/a.md") != broken {
		t.Error("file was changed")
	}
	// Moving does not read the header and still works.
	if _, err := b.Change("a", Change{Column: "done"}); err != nil {
		t.Errorf("move: %v", err)
	}
}

func TestChangeRepairsAWrongValue(t *testing.T) {
	b := load(t, map[string]string{"backlog/a.md": "---\nsize: XXL\n---\n# A\n"})
	if _, err := b.Change("a", Change{Set: []string{"size=L"}}); err != nil {
		t.Fatal(err)
	}
	reload(t, b)

	// A wrong value can also be removed, although the board never accepted it.
	b = load(t, map[string]string{"planning/a.md": "---\nsize: XXL\n---\n# A\n"})
	res, err := b.Change("a", Change{Set: []string{"size="}})
	if err != nil || !slices.Equal(res.Removed, []string{"size"}) {
		t.Fatalf("res %+v, err %v", res, err)
	}
	reload(t, b)
}

func TestEditHead(t *testing.T) {
	str := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	list := func(items ...string) *yaml.Node {
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, i := range items {
			n.Content = append(n.Content, str(i))
		}
		return n
	}
	tests := []struct {
		name  string
		in    string
		field string
		value *yaml.Node
		want  string
	}{
		{"add to a card without header", "# A\n\nText\n", "assignee", str("arnold"),
			"---\nassignee: arnold\n---\n\n# A\n\nText\n"},
		{"add to a card without header and leading blank lines", "\n\n# A\n", "assignee", str("arnold"),
			"---\nassignee: arnold\n---\n\n# A\n"},
		{"add as the last field", "---\nsize: S\n---\n\n# A\n", "assignee", str("arnold"),
			"---\nsize: S\nassignee: arnold\n---\n\n# A\n"},
		{"add to an empty header", "---\n---\n# A\n", "assignee", str("arnold"),
			"---\nassignee: arnold\n---\n# A\n"},
		{"replace in place", "---\nsize: S\nassignee: arnold\ntags: [bug]\n---\n# A\n", "assignee", str("sly"),
			"---\nsize: S\nassignee: sly\ntags: [bug]\n---\n# A\n"},
		{"replace the first field", "---\nassignee: arnold\nsize: S\n---\n# A\n", "assignee", str("sly"),
			"---\nassignee: sly\nsize: S\n---\n# A\n"},
		{"replace the last field", "---\nsize: S\nassignee: arnold\n---\n# A\n", "assignee", str("sly"),
			"---\nsize: S\nassignee: sly\n---\n# A\n"},
		{"replace a list written over several lines", "---\ntags:\n  - bug\n  - ui\nsize: S\n---\n# A\n", "tags", list("feature"),
			"---\ntags: [feature]\nsize: S\n---\n# A\n"},
		{"keep comments and blank lines of other fields",
			"---\n# who\nassignee: arnold\n\n# how big\nsize: S\n---\n# A\n", "assignee", str("sly"),
			"---\n# who\nassignee: sly\n\n# how big\nsize: S\n---\n# A\n"},
		{"keep a comment at the end of the header", "---\nassignee: arnold\n# end\n---\n# A\n", "assignee", str("sly"),
			"---\nassignee: sly\n# end\n---\n# A\n"},
		{"keep other formatting untouched", "---\nsize:    S\ntags: [ bug ,ui ]\n---\n# A\n", "assignee", str("arnold"),
			"---\nsize:    S\ntags: [ bug ,ui ]\nassignee: arnold\n---\n# A\n"},
		{"quote a value that needs it", "# A\n", "owner", str("Einkauf: Nord"),
			"---\nowner: 'Einkauf: Nord'\n---\n\n# A\n"},
		{"quote a name that looks like something else", "# A\n", "assignee", str("true"),
			"---\nassignee: \"true\"\n---\n\n# A\n"},

		{"remove one of several", "---\nsize: S\nassignee: arnold\ntags: [bug]\n---\n# A\n", "assignee", nil,
			"---\nsize: S\ntags: [bug]\n---\n# A\n"},
		{"remove a list written over several lines", "---\ntags:\n  - bug\n  - ui\nsize: S\n---\n# A\n", "tags", nil,
			"---\nsize: S\n---\n# A\n"},
		{"remove the only field drops the header", "---\nassignee: arnold\n---\n\n# A\n", "assignee", nil,
			"# A\n"},
		{"remove what is not there", "---\nsize: S\n---\n# A\n", "assignee", nil,
			"---\nsize: S\n---\n# A\n"},
		{"remove from a card without header", "# A\n", "assignee", nil, "# A\n"},

		{"windows line endings stay", "---\r\nsize: S\r\n---\r\n\r\n# A\r\n", "assignee", str("arnold"),
			"---\r\nsize: S\r\nassignee: arnold\r\n---\r\n\r\n# A\r\n"},
		{"byte order mark stays", "\uFEFF---\nsize: S\n---\n# A\n", "assignee", str("arnold"),
			"\uFEFF---\nsize: S\nassignee: arnold\n---\n# A\n"},
		{"text that looks like a header later on is left alone", "# A\n\n---\nsize: S\n---\n", "assignee", str("arnold"),
			"---\nassignee: arnold\n---\n\n# A\n\n---\nsize: S\n---\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := editHead(tt.in, []string{tt.field}, map[string]*yaml.Node{tt.field: tt.value})
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}

	for name, in := range map[string]string{
		"unclosed header":    "---\nsize: S\n# A\n",
		"header not YAML":    "---\nsize: [S\n---\n# A\n",
		"header is a list":   "---\n- a\n---\n# A\n",
		"header in one line": "---\n{size: S}\n---\n# A\n",
	} {
		if _, err := editHead(in, []string{"assignee"}, map[string]*yaml.Node{"assignee": str("arnold")}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestEditHeadSeveralFields(t *testing.T) {
	str := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	in := "---\nsize: S\nassignee: arnold\nowner: x\n---\n# A\n"
	got, err := editHead(in, []string{"owner", "assignee", "size"}, map[string]*yaml.Node{
		"owner": nil, "assignee": str("sly"), "size": str("M"),
	})
	if want := "---\nsize: M\nassignee: sly\n---\n# A\n"; err != nil || got != want {
		t.Errorf("got %q, err %v, want %q", got, err, want)
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
		Column: "working_on", Today: "2026-11-11",
		Set: []string{
			"size=M", "tags=bug, feature", "owner=Einkauf: Nord", "estimate=2.5", "blocked=true", "assignee=arnold",
			"related=[other]", "depends_on=other,groesse-aendern", "due = 2026-12-24", "created=2026-01-01", "reviewer=sly",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The header follows the order of the fields in board.yaml, not of --set.
	want := "---\n" +
		"assignee: arnold\n" +
		"size: M\n" +
		"tags: [bug, feature]\n" +
		"owner: 'Einkauf: Nord'\n" +
		"estimate: 2.5\n" +
		"blocked: true\n" +
		"related: [other]\n" +
		"reviewer: sly\n" +
		"due: 2026-12-24\n" +
		"depends_on: [other, groesse-aendern]\n" +
		"created: 2026-01-01\n" +
		"---\n\n# With everything\n"
	if got := read(t, b, "working_on/with-everything.md"); got != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}
	if who(c) != "arnold" || c.Fields["estimate"] != 2.5 || c.Fields["blocked"] != true {
		t.Errorf("card = %+v", c)
	}

	if _, err = b.New("Explicit", NewOptions{ID: "my-id", Column: "backlog", Set: []string{"size=S"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, b, "backlog/my-id.md"), "---\nsize: S\n---\n\n# Explicit\n"; got != want {
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
		{"required field missing", "A", NewOptions{Column: "backlog"}, true, "column backlog requires size"},
		{"in progress needs assignee and size", "A", NewOptions{Column: "working_on"}, true, "column working_on requires assignee, size"},
		{"set without =", "A", NewOptions{Set: []string{"size"}}, false, "expected field=value"},
		{"set unknown field", "A", NewOptions{Set: []string{"prio=1"}}, false, `unknown field "prio"`},
		{"set reserved field", "A", NewOptions{Set: []string{"status=done"}}, false, "cannot be set"},
		{"set wrong enum", "A", NewOptions{Set: []string{"size=XXL"}}, false, "size: must be one of"},
		{"set empty value", "A", NewOptions{Set: []string{"size="}}, false, "size: must be one of"},
		{"set wrong date", "A", NewOptions{Set: []string{"due=morgen"}}, false, "due: must be a date"},
		{"set wrong number", "A", NewOptions{Set: []string{"estimate=viel"}}, false, "estimate: must be a number"},
		{"set wrong list value", "A", NewOptions{Set: []string{"tags=bug,nope"}}, false, `"nope" is not one of`},
		{"set person outside the values", "A", NewOptions{Set: []string{"reviewer=laura"}}, false, `reviewer: "laura" is not one of arnold, sly`},
		{"set no person", "A", NewOptions{Set: []string{"assignee="}}, false, "assignee: must name one person"},
		{"set a person twice", "A", NewOptions{Set: []string{"assignee=arnold,sly,arnold"}}, false, `"arnold" is named twice`},
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
	if col, ok := cfg.FirstDoing(); !ok || col.ID != "working_on" {
		t.Errorf("FirstDoing = %+v, %v", col, ok)
	}
	if col, ok := cfg.FirstDone(); !ok || col.ID != "done" {
		t.Errorf("FirstDone = %+v, %v", col, ok)
	}
	plain, _ := ParseConfig([]byte("kardboard: 1\nid: x\ncolumns:\n  - id: a\n"))
	if _, ok := plain.FirstDoing(); ok {
		t.Error("FirstDoing found a column on a board without one")
	}
	if _, ok := plain.FirstDone(); ok {
		t.Error("FirstDone found a column on a board without one")
	}
}
