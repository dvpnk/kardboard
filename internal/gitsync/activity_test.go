package gitsync

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dvpnk/kardboard/internal/board"
)

func comment(who, text string) board.Entry {
	return board.Entry{Author: who, Kind: board.KindComment, Text: text}
}

func TestEntryMessage(t *testing.T) {
	cases := map[string]board.Entry{
		"comment a": {Kind: board.KindComment},
		"ask a":     {Kind: board.KindQuestion},
	}
	for want, e := range cases {
		if got := EntryMessage("a", e); got != want {
			t.Errorf("EntryMessage(%s) = %q, want %q", e.Kind, got, want)
		}
	}
}

func TestEntryOnBoardWithoutSync(t *testing.T) {
	root := repo(t, ModeNone)
	res, err := AddEntry(ctx, load(t, root), "a", comment("arnold", "Hi."))
	if err != nil || res.Commit != "" || res.Path != "activity/a.md" || res.Entry.Text != "Hi." {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if !isFile(root, "activity/a.md") || len(subjects(t, root, "HEAD")) != 1 {
		t.Error("expected the file and no commit")
	}
}

func TestEntryIsOneCommitWithOnlyTheActivity(t *testing.T) {
	root := repo(t, ModeCommit)
	write(t, root, "backlog/b.md", "# B\n\nchanged\n")
	write(t, root, "notes.txt", "staged\n")
	run(t, root, "add", "notes.txt")

	for _, text := range []string{"First.", "Second."} {
		res, err := AddEntry(ctx, load(t, root), "a", comment("arnold", text))
		if err != nil || res.Commit == "" || res.Pushed {
			t.Fatalf("res %+v, err %v", res, err)
		}
	}
	if got := subjects(t, root, "HEAD"); !slices.Equal(got, []string{"comment a", "comment a", "start"}) {
		t.Errorf("history = %q", got)
	}
	if got := run(t, root, "show", "--name-only", "--format=", "HEAD"); got != "activity/a.md" {
		t.Errorf("commit holds %q", got)
	}
	if got := run(t, root, "status", "--porcelain"); got != "A  notes.txt\n M backlog/b.md" && got != "M backlog/b.md\nA  notes.txt" {
		// the order is Git's business; both must still be there
		if !strings.Contains(got, "notes.txt") || !strings.Contains(got, "backlog/b.md") {
			t.Errorf("status = %q", got)
		}
	}
}

func TestEntryRefusedWhileTheActivityHasChanges(t *testing.T) {
	root := repo(t, ModeCommit)
	write(t, root, "activity/a.md", "## 2026-11-27T14:02:31Z · arnold · comment\nNot committed.\n")
	_, err := AddEntry(ctx, load(t, root), "a", comment("arnold", "Hi."))
	if err == nil || !strings.Contains(err.Error(), "not committed yet") {
		t.Fatalf("err = %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "activity", "a.md"))
	if strings.Contains(string(data), "Hi.") {
		t.Error("the entry was written all the same")
	}
}

func TestFailedEntryCommitLeavesNoFile(t *testing.T) {
	root := repo(t, ModeCommit)
	write(t, root, ".git/hooks/pre-commit", "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(filepath.Join(root, ".git", "hooks", "pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := AddEntry(ctx, load(t, root), "a", comment("arnold", "Hi."))
	if err == nil || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("err = %v", err)
	}
	if isFile(root, "activity/a.md") {
		t.Error("the activity file was left behind")
	}
	if got := run(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("status = %q", got)
	}
}

func TestEntriesOfBothLandOnTheSameActivity(t *testing.T) {
	arnold, sly := team(t)
	ab := load(t, arnold) // read before Sly's entry arrives, and not prepared
	if _, err := AddEntry(ctx, load(t, sly), "a", comment("sly", "From Sly.")); err != nil {
		t.Fatal(err)
	}
	e := comment("arnold", "From Arnold.")
	e.Time = time.Date(2026, 11, 27, 14, 0, 0, 0, time.UTC)
	res, err := AddEntry(ctx, ab, "a", e)
	if err != nil || !res.Pushed {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if got := subjects(t, arnold, "origin/main"); !slices.Equal(got, []string{"comment a", "comment a", "start"}) {
		t.Errorf("remote = %q", got)
	}
	entries := load(t, arnold).Activity("a")
	if len(entries) != 2 || entries[0].Text != "From Sly." || entries[1].Text != "From Arnold." {
		t.Fatalf("entries = %+v", entries)
	}
	// Added again, it keeps the time it was written.
	if !entries[1].Time.Equal(e.Time) {
		t.Errorf("time = %v, want %v", entries[1].Time, e.Time)
	}
	if got := run(t, arnold, "status", "--porcelain", "--branch"); got != "## main...origin/main" {
		t.Errorf("status = %q, want clean and level with the remote", got)
	}
}

func TestEntryOnACardRenamedMeanwhile(t *testing.T) {
	arnold, sly := team(t)
	ab := load(t, arnold)
	sb := load(t, sly)
	if _, err := AddEntry(ctx, sb, "a", comment("sly", "Before the rename.")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Rename(ctx, load(t, sly), "a", "alpha", board.RenameOptions{}); err != nil {
		t.Fatal(err)
	}
	_, err := AddEntry(ctx, ab, "a", comment("arnold", "Too late."))
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Card != "a" {
		t.Fatalf("err = %v", err)
	}
	if isFile(arnold, "activity/a.md") || !isFile(arnold, "activity/alpha.md") {
		t.Error("arnold does not have sly's state")
	}
}

func TestRenameCommitsTheActivity(t *testing.T) {
	root := repo(t, ModeCommit)
	if _, err := AddEntry(ctx, load(t, root), "a", comment("arnold", "Hi.")); err != nil {
		t.Fatal(err)
	}
	res, out, err := Rename(ctx, load(t, root), "a", "alpha", board.RenameOptions{})
	if err != nil || out.Commit == "" || res.NewActivity != "activity/alpha.md" {
		t.Fatalf("res %+v, out %+v, err %v", res, out, err)
	}
	if got := run(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("status = %q, want everything in the commit", got)
	}
	if got := run(t, root, "show", "--name-status", "--format=", "HEAD"); !strings.Contains(got, "activity/a.md") || !strings.Contains(got, "activity/alpha.md") {
		t.Errorf("commit holds %q", got)
	}

	// An activity with changes stops the rename.
	write(t, root, "activity/alpha.md", "changed\n")
	if _, _, err := Rename(ctx, load(t, root), "alpha", "beta", board.RenameOptions{}); err == nil || !strings.Contains(err.Error(), "activity/alpha.md has changes") {
		t.Errorf("err = %v", err)
	}
}
