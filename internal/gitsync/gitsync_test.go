package gitsync

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dvpnk/kardboard/internal/board"
)

var ctx = context.Background()

const columns = "columns:\n  - id: backlog\n  - id: working_on\n    doing: true\n  - id: done\n    done: true\n"

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func isFile(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

func identify(t *testing.T, dir, name string) {
	run(t, dir, "config", "user.name", name)
	run(t, dir, "config", "user.email", name+"@example.com")
	run(t, dir, "config", "commit.gpgsign", "false")
}

// repo makes a repository that holds a board with the given sync mode and
// two cards in the backlog.
func repo(t *testing.T, mode string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	// Tests must not depend on, or change, the settings of whoever runs them.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	root := t.TempDir()
	run(t, root, "init", "-q", "-b", "main")
	identify(t, root, "arnold")
	write(t, root, "board.yaml", "kardboard: 1\nid: x\nsync: "+mode+"\n"+columns)
	write(t, root, "backlog/a.md", "# A\n")
	write(t, root, "backlog/b.md", "# B\n")
	run(t, root, "add", "-A")
	run(t, root, "commit", "-q", "-m", "start")
	return root
}

// team makes a remote and two clones of it, for two people on one board.
func team(t *testing.T) (arnold, sly string) {
	t.Helper()
	seed := repo(t, ModePush)
	remote := filepath.Join(t.TempDir(), "remote.git")
	run(t, seed, "clone", "-q", "--bare", seed, remote)
	clone := func(name string) string {
		dir := filepath.Join(t.TempDir(), name)
		run(t, seed, "clone", "-q", remote, dir)
		identify(t, dir, name)
		return dir
	}
	return clone("arnold"), clone("sly")
}

func load(t *testing.T, root string) *board.Board {
	t.Helper()
	b, err := board.Load(root)
	if err != nil || b.Config == nil {
		t.Fatalf("load: %v, %v", err, b.Problems())
	}
	return b
}

func subjects(t *testing.T, root string, rev string) []string {
	return strings.Split(run(t, root, "log", "--format=%s", rev), "\n")
}

func TestMessages(t *testing.T) {
	cases := []struct {
		res  board.ChangeResult
		want []string
	}{
		{board.ChangeResult{ID: "a", From: "backlog", To: "done", Moved: true}, []string{"move a: backlog -> done"}},
		{board.ChangeResult{ID: "a", Set: map[string]any{"assignee": []string{"arnold", "sly"}}}, []string{"assign a: arnold, sly"}},
		{board.ChangeResult{ID: "a", Removed: []string{"assignee"}}, []string{"unassign a"}},
		{board.ChangeResult{ID: "a", Set: map[string]any{"size": "M"}, Removed: []string{"due"}}, []string{"edit a"}},
		{board.ChangeResult{ID: "a", Text: true}, []string{"edit a"}},
		{board.ChangeResult{ID: "a", Set: map[string]any{"size": "M"}, Text: true}, []string{"edit a"}},
		{board.ChangeResult{ID: "a", From: "backlog", To: "working_on", Moved: true, Set: map[string]any{"assignee": []string{"arnold"}}},
			[]string{"move a: backlog -> working_on", "assign a: arnold"}},
	}
	for _, c := range cases {
		if got := Messages(&c.res); !slices.Equal(got, c.want) {
			t.Errorf("Messages(%+v) = %q, want %q", c.res, got, c.want)
		}
	}
}

func TestNoneOnlyChangesFiles(t *testing.T) {
	root := repo(t, ModeNone)
	res, err := Change(ctx, load(t, root), "a", board.Change{Column: "done"})
	if err != nil || res.Commit != "" || res.Pushed {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if got := subjects(t, root, "HEAD"); len(got) != 1 {
		t.Errorf("commits = %q, want only the first", got)
	}
}

func TestCommitMakesOneCommitWithOnlyThisCard(t *testing.T) {
	root := repo(t, ModeCommit)
	// Something else is staged and something else is changed: neither may
	// end up in the card's commit.
	write(t, root, "notes.txt", "staged\n")
	run(t, root, "add", "notes.txt")
	write(t, root, "backlog/b.md", "# B\n\nchanged\n")

	b := load(t, root)
	res, err := Change(ctx, b, "a", b.TakeChange("a", "arnold", false))
	if err != nil {
		t.Fatal(err)
	}
	if res.Commit == "" || res.Pushed || !res.Moved {
		t.Errorf("res = %+v", res)
	}
	if got := run(t, root, "log", "-1", "--format=%B"); got != "move a: backlog -> working_on\n\nassign a: arnold" {
		t.Errorf("message = %q", got)
	}
	if got := run(t, root, "show", "--format=", "--name-only", "--no-renames", "HEAD"); got != "backlog/a.md\nworking_on/a.md" {
		t.Errorf("commit holds %q, want only the moved card", got)
	}
	if got := run(t, root, "status", "--porcelain"); got != "M backlog/b.md\nA  notes.txt" && got != " M backlog/b.md\nA  notes.txt" {
		t.Errorf("status = %q, want the other changes untouched", got)
	}
}

func TestCommitForFieldsAndNew(t *testing.T) {
	root := repo(t, ModeCommit)
	b := load(t, root)
	if _, err := Change(ctx, b, "a", board.Change{Set: []string{"assignee=sly"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Change(ctx, b, "a", board.Change{Set: []string{"due=2026-12-01"}, Force: true}); err != nil {
		t.Fatal(err)
	}
	c, done, err := New(ctx, b, "Neue Karte", board.NewOptions{})
	if err != nil || c.ID != "neue-karte" || done.Commit == "" {
		t.Fatalf("card %+v, outcome %+v, err %v", c, done, err)
	}
	want := []string{"new neue-karte", "edit a", "assign a: sly", "start"}
	if got := subjects(t, root, "HEAD"); !slices.Equal(got, want) {
		t.Errorf("subjects = %q, want %q", got, want)
	}
	if got := run(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("status = %q, want clean", got)
	}

	// Nothing to change means no commit.
	res, err := Change(ctx, b, "a", board.Change{Set: []string{"assignee=sly"}, Force: true})
	if err != nil || res.Commit != "" || res.Changed() {
		t.Errorf("res %+v, err %v", res, err)
	}
}

func TestCardWithUncommittedChangesIsRefused(t *testing.T) {
	root := repo(t, ModeCommit)
	write(t, root, "backlog/a.md", "# A\n\nnot committed\n")
	write(t, root, "backlog/c.md", "# C\n") // unknown to Git
	b := load(t, root)
	for _, id := range []string{"a", "c"} {
		if _, err := Change(ctx, b, id, board.Change{Column: "done"}); err == nil || !strings.Contains(err.Error(), "not committed") {
			t.Errorf("%s: err = %v", id, err)
		}
		if !isFile(root, "backlog/"+id+".md") {
			t.Errorf("%s was moved", id)
		}
	}
}

func TestFailedCommitPutsTheCardBack(t *testing.T) {
	root := repo(t, ModeCommit)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho no >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := load(t, root)
	_, err := Change(ctx, b, "a", b.TakeChange("a", "arnold", false))
	if err == nil || !strings.Contains(err.Error(), "nothing was changed") {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := New(ctx, b, "C", board.NewOptions{}); err == nil || !strings.Contains(err.Error(), "nothing was created") {
		t.Fatalf("new: err = %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "backlog/a.md"))
	if string(data) != "# A\n" || isFile(root, "working_on/a.md") || isFile(root, "backlog/c.md") {
		t.Errorf("card was not put back: %q", data)
	}
	if got := run(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("status = %q, want clean", got)
	}
}

func TestSyncNeedsARepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	write(t, root, "board.yaml", "kardboard: 1\nid: x\nsync: commit\n"+columns)
	write(t, root, "backlog/a.md", "# A\n")
	b := load(t, root)
	if _, err := Change(ctx, b, "a", board.Change{Column: "done"}); err == nil || !strings.Contains(err.Error(), "not in a Git repository") {
		t.Errorf("err = %v", err)
	}
	if !isFile(root, "backlog/a.md") {
		t.Error("card was moved")
	}

	// Push also needs somewhere to push to.
	root = repo(t, ModePush)
	if _, err := Prepare(ctx, load(t, root)); err == nil || !strings.Contains(err.Error(), "no upstream") {
		t.Errorf("err = %v", err)
	}
}

func TestPushReachesTheRemote(t *testing.T) {
	arnold, sly := team(t)
	b, err := Prepare(ctx, load(t, arnold))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Change(ctx, b, "a", board.Change{Column: "done"})
	if err != nil || !res.Pushed || res.Commit == "" {
		t.Fatalf("res %+v, err %v", res, err)
	}
	// Sly sees it after preparing a change of their own.
	sb, err := Prepare(ctx, load(t, sly))
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := sb.Card("a"); c.Column != "done" {
		t.Errorf("sly sees a in %s", c.Column)
	}
}

func TestPushRetriesWhenSomeoneWasFaster(t *testing.T) {
	arnold, sly := team(t)
	ab := load(t, arnold) // read before Sly's change arrives, and not prepared
	if _, err := Change(ctx, load(t, sly), "b", board.Change{Column: "done"}); err != nil {
		t.Fatal(err)
	}
	res, err := Change(ctx, ab, "a", board.Change{Column: "working_on"})
	if err != nil || !res.Pushed {
		t.Fatalf("res %+v, err %v", res, err)
	}
	want := []string{"move a: backlog -> working_on", "move b: backlog -> done", "start"}
	if got := subjects(t, arnold, "origin/main"); !slices.Equal(got, want) {
		t.Errorf("remote = %q, want %q", got, want)
	}
	if !isFile(arnold, "done/b.md") || !isFile(arnold, "working_on/a.md") {
		t.Error("arnold does not have both changes")
	}
}

func TestSameCardMovedByBothIsTakenBack(t *testing.T) {
	arnold, sly := team(t)
	ab := load(t, arnold)
	if _, err := Change(ctx, load(t, sly), "a", board.Change{Column: "done"}); err != nil {
		t.Fatal(err)
	}
	res, err := Change(ctx, ab, "a", board.Change{Column: "working_on"})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || res != nil {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if conflict.Card != "a" || conflict.Column != "done" {
		t.Errorf("conflict = %+v", conflict)
	}
	if !isFile(arnold, "done/a.md") || isFile(arnold, "working_on/a.md") || isFile(arnold, "backlog/a.md") {
		t.Error("arnold does not have sly's state")
	}
	if got := run(t, arnold, "status", "--porcelain", "--branch"); got != "## main...origin/main" {
		t.Errorf("status = %q, want clean and level with the remote", got)
	}
	if got := subjects(t, arnold, "HEAD"); !slices.Equal(got, []string{"move a: backlog -> done", "start"}) {
		t.Errorf("history = %q", got)
	}
}

func TestEditAndMoveOfTheSameCardBothLand(t *testing.T) {
	arnold, sly := team(t)
	ab := load(t, arnold)
	if _, err := Change(ctx, load(t, sly), "a", board.Change{Column: "done"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Change(ctx, ab, "a", board.Change{Set: []string{"assignee=arnold"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(arnold, "done/a.md"))
	if err != nil || !strings.Contains(string(data), "assignee: arnold") {
		t.Errorf("done/a.md = %q, %v", data, err)
	}
}

func TestUnreachableRemoteKeepsTheCommit(t *testing.T) {
	arnold, _ := team(t)
	b := load(t, arnold)
	run(t, arnold, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))

	if _, err := Prepare(ctx, b); err == nil || !strings.Contains(err.Error(), "nothing was changed") {
		t.Errorf("prepare: err = %v", err)
	}
	res, err := Change(ctx, b, "a", board.Change{Column: "done"})
	var notPushed *PushError
	if !errors.As(err, &notPushed) || res == nil || res.Pushed || res.Commit == "" {
		t.Fatalf("res %+v, err %v", res, err)
	}
	if !isFile(arnold, "done/a.md") || subjects(t, arnold, "HEAD")[0] != "move a: backlog -> done" {
		t.Error("the change is not committed here")
	}
	st, err := ReadStatus(ctx, load(t, arnold))
	if err != nil || st.Ahead != 1 || st.Behind != 0 {
		t.Errorf("status %+v, err %v", st, err)
	}
}

func TestStatusAndSync(t *testing.T) {
	arnold, sly := team(t)
	if _, err := Change(ctx, load(t, sly), "b", board.Change{Column: "done"}); err != nil {
		t.Fatal(err)
	}
	// Arnold has a commit that is not pushed and a card that is not committed.
	write(t, arnold, "notes.txt", "x\n")
	run(t, arnold, "add", "notes.txt")
	run(t, arnold, "commit", "-q", "-m", "notes")
	write(t, arnold, "backlog/a.md", "# A\n\nmore\n")

	b := load(t, arnold)
	if st, _ := ReadStatus(ctx, b); st.Behind != 0 || st.Ahead != 1 {
		t.Errorf("before fetch: %+v", st)
	}
	if err := Fetch(ctx, b); err != nil {
		t.Fatal(err)
	}
	st, err := ReadStatus(ctx, b)
	if err != nil || st.Branch != "main" || st.Ahead != 1 || st.Behind != 1 || !slices.Equal(st.Dirty, []string{"a"}) {
		t.Errorf("status %+v, err %v", st, err)
	}

	res, err := Sync(ctx, b)
	if err != nil || res.Pulled != 1 || res.Pushed != 1 {
		t.Fatalf("sync %+v, err %v", res, err)
	}
	st, _ = ReadStatus(ctx, load(t, arnold))
	if st.Ahead != 0 || st.Behind != 0 || !slices.Equal(st.Dirty, []string{"a"}) {
		t.Errorf("after sync: %+v", st)
	}
	if !isFile(arnold, "done/b.md") {
		t.Error("sly's change did not arrive")
	}
	if res, err := Sync(ctx, load(t, arnold)); err != nil || res.Pulled+res.Pushed != 0 {
		t.Errorf("second sync %+v, err %v", res, err)
	}

	if _, err := Sync(ctx, load(t, repo(t, ModeCommit))); err == nil || !strings.Contains(err.Error(), "sync: commit") {
		t.Errorf("sync on a commit board: err = %v", err)
	}
}
