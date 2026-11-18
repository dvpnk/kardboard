package board

import (
	"errors"
	"reflect"
	"testing"
)

func TestRenameMovesTheFileAndItsReferences(t *testing.T) {
	b := load(t, map[string]string{
		"backlog/a.md":   "---\nsize: S\n---\n\n# A\n\nText.\n",
		"planning/b.md":  "---\nrelated: [a, c]\n---\n\n# B\n",
		"planning/c.md":  "---\nrelated: [test/a]\nowner: x\n---\n\n# C\n",
		"done/d.md":      "---\nrelated: [other/a]\n---\n\n# D\n",
		"planning/ab.md": "# Ab\n",
	})
	res, err := b.Rename("a", "alpha", RenameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := RenameResult{ID: "a", NewID: "alpha", Path: "backlog/a.md", NewPath: "backlog/alpha.md", Updated: []string{"planning/b.md", "planning/c.md"}}
	if !reflect.DeepEqual(*res, want) {
		t.Errorf("result = %+v, want %+v", *res, want)
	}
	if has(t, b, "backlog/a.md") || read(t, b, "backlog/alpha.md") != "---\nsize: S\n---\n\n# A\n\nText.\n" {
		t.Error("card was not renamed unchanged")
	}
	for path, text := range map[string]string{
		"planning/b.md": "---\nrelated: [alpha, c]\n---\n\n# B\n",
		"planning/c.md": "---\nrelated: [test/alpha]\nowner: x\n---\n\n# C\n",
		// another board's card of the same name is another card
		"done/d.md": "---\nrelated: [other/a]\n---\n\n# D\n",
	} {
		if got := read(t, b, path); got != text {
			t.Errorf("%s = %q, want %q", path, got, text)
		}
	}
	fresh, err := Load(b.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range fresh.Problems() {
		if p.Code == "E10" {
			t.Errorf("a reference was left behind: %+v", p)
		}
	}
}

func TestRenameRules(t *testing.T) {
	b := load(t, map[string]string{
		"backlog/a.md":    "---\nsize: S\n---\n\n# A\n",
		"backlog/b.md":    "---\nsize: S\n---\n\n# B\n",
		"working_on/s.md": "---\nassignee: sly\nsize: S\n---\n\n# S\n",
	})
	for name, newID := range map[string]string{
		"taken":   "b",
		"same":    "a",
		"invalid": "Not Valid",
	} {
		if _, err := b.Rename("a", newID, RenameOptions{}); err == nil {
			t.Errorf("%s: renamed to %q", name, newID)
		}
	}
	if _, err := b.Rename("nope", "x", RenameOptions{}); err == nil {
		t.Error("renamed a card that does not exist")
	}
	var refused *RefusedError
	if _, err := b.Rename("s", "x", RenameOptions{User: "arnold"}); !errors.As(err, &refused) {
		t.Errorf("someone else's card: err = %v, want refused", err)
	}
	if _, err := b.Rename("s", "x", RenameOptions{}); !errors.Is(err, ErrNeedUser) {
		t.Errorf("unknown user: err = %v", err)
	}
	if _, err := b.Rename("s", "x", RenameOptions{User: "sly"}); err != nil {
		t.Errorf("own card: %v", err)
	}
	if !has(t, b, "backlog/a.md") || !has(t, b, "working_on/x.md") {
		t.Error("wrong files after the refusals")
	}
}
