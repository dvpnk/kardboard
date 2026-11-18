package board

import (
	"errors"
	"reflect"
	"testing"
)

func TestTextReplacesTitleAndTextOnly(t *testing.T) {
	const before = "---\nsize: S # keep this comment\n---\n\n# Old\n\nOld text.\n"
	b := load(t, map[string]string{"planning/a.md": before})
	c, _ := b.Card("a")

	res, err := b.Change("a", Change{Base: c.Sum, Text: &Text{Title: " New ", Body: "\n\nLine one\r\n\nLine two  \n\n"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Text || len(res.Set) > 0 || res.Moved {
		t.Errorf("result = %+v", res)
	}
	want := "---\nsize: S # keep this comment\n---\n\n# New\n\nLine one\n\nLine two\n"
	if got := read(t, b, "planning/a.md"); got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	if c, _ := reload(t, b).Card("a"); c.Title != "New" {
		t.Errorf("title = %q", c.Title)
	}
}

func TestTextWithoutHeaderOrBody(t *testing.T) {
	b := load(t, map[string]string{"planning/a.md": "# A\n\nSome text.\n"})
	if _, err := b.Change("a", Change{Text: &Text{Title: "A", Body: "  \n"}}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, b, "planning/a.md"); got != "# A\n" {
		t.Errorf("file = %q", got)
	}
}

func TestTextAndFieldsInOneStep(t *testing.T) {
	b := load(t, map[string]string{"planning/a.md": sized})
	c, _ := b.Card("a")
	res, err := b.Change("a", Change{Base: c.Sum, Set: []string{"size=M"}, Text: &Text{Title: "Sized", Body: "More."}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Text || !reflect.DeepEqual(res.Set, map[string]any{"size": "M"}) {
		t.Errorf("result = %+v", res)
	}
	if got, want := read(t, b, "planning/a.md"), "---\nsize: M\n---\n\n# Sized\n\nMore.\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

func TestUnchangedTextIsNoChange(t *testing.T) {
	b := load(t, map[string]string{"planning/a.md": "---\nsize: S\n---\n\n# Sized\n\nText.\n"})
	res, err := b.Change("a", Change{Text: &Text{Title: "Sized", Body: "Text.\n"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed() {
		t.Errorf("result = %+v, want no change", res)
	}
}

func TestTextRefusesAStaleBase(t *testing.T) {
	b := load(t, map[string]string{"planning/a.md": sized})
	_, err := b.Change("a", Change{Base: "0123", Text: &Text{Title: "Other"}})
	var stale *StaleError
	if !errors.As(err, &stale) {
		t.Fatalf("err = %v, want a StaleError", err)
	}
	if read(t, b, "planning/a.md") != sized {
		t.Error("file changed")
	}
}

func TestTextRules(t *testing.T) {
	b := load(t, map[string]string{
		"working_on/mine.md": "---\nassignee: sly\nsize: S\n---\n\n# Mine\n",
		"planning/open.md":   "---\nsize: S\n# no end\n# Open\n",
	})
	// Someone else's card: refused, unless forced.
	var refused *RefusedError
	if _, err := b.Change("mine", Change{User: "arnold", Text: &Text{Title: "Mine", Body: "x"}}); !errors.As(err, &refused) {
		t.Errorf("other's card: err = %v, want refused", err)
	}
	if _, err := b.Change("mine", Change{User: "arnold", Force: true, Text: &Text{Title: "Mine", Body: "x"}}); err != nil {
		t.Errorf("forced: %v", err)
	}
	// A title must be one line, and a broken header stays untouched.
	if _, err := b.Change("mine", Change{User: "sly", Text: &Text{Title: " "}}); err == nil {
		t.Error("empty title was accepted")
	}
	if _, err := b.Change("open", Change{Text: &Text{Title: "Open", Body: "x"}}); err == nil {
		t.Error("text below an unclosed header was written")
	}
}

func TestFileReplacesEverythingAndNamesTheFields(t *testing.T) {
	b := load(t, map[string]string{"backlog/a.md": "---\nsize: S\ntags: [bug]\n---\n\n# A\n\nText.\n"})
	c, _ := b.Card("a")
	file := "---\nsize: M\nassignee: arnold\n---\n\n# A\n\nNew text.\n"
	res, err := b.Change("a", Change{Base: c.Sum, File: &file})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"size": "M", "assignee": []string{"arnold"}}
	if !reflect.DeepEqual(res.Set, want) || !reflect.DeepEqual(res.Removed, []string{"tags"}) || !res.Text {
		t.Errorf("result = %+v", res)
	}
	if got := read(t, b, "backlog/a.md"); got != file {
		t.Errorf("file = %q", got)
	}
	reload(t, b)
}

func TestFileRules(t *testing.T) {
	const a = "---\nsize: S\n---\n\n# A\n"
	b := load(t, map[string]string{"backlog/a.md": a})
	for name, file := range map[string]string{
		"broken header":   "---\nsize: [\n---\n\n# A\n",
		"no title":        "---\nsize: S\n---\n\nJust text.\n",
		"unknown field":   "---\nsize: S\ncolour: red\n---\n\n# A\n",
		"value not known": "---\nsize: XXL\n---\n\n# A\n",
	} {
		if _, err := b.Change("a", Change{File: &file}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// size is required in backlog: taking it away is refused like kard set.
	without := "# A\n"
	var refused *RefusedError
	if _, err := b.Change("a", Change{File: &without}); !errors.As(err, &refused) {
		t.Errorf("required field removed: err = %v, want refused", err)
	}
	// A file goes alone.
	if _, err := b.Change("a", Change{File: &without, Column: "done"}); err == nil {
		t.Error("file and move together were accepted")
	}
	if read(t, b, "backlog/a.md") != a {
		t.Error("file changed")
	}
}
