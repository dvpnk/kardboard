package gitsync

import (
	"path/filepath"
	"testing"

	"kardboard/internal/board"
)

func TestSideBranch(t *testing.T) {
	root := repo(t, ModeNone)
	if got := SideBranch(ctx, load(t, root)); got != nil {
		t.Fatalf("on main: %+v, want nil", got)
	}

	// A move on a feature branch says where it happened.
	run(t, root, "switch", "-q", "-c", "feature/pdf")
	res, err := Change(ctx, load(t, root), "a", board.Change{Column: "working_on"})
	if err != nil {
		t.Fatal(err)
	}
	if b := res.Branch; b == nil || b.Name != "feature/pdf" || b.Main != "main" || b.Detached {
		t.Fatalf("branch after move = %+v", b)
	}
	// Nothing changed, nothing to say.
	res, err = Change(ctx, load(t, root), "a", board.Change{Column: "working_on"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Branch != nil {
		t.Fatalf("branch without a change = %+v", res.Branch)
	}
	c, out, err := New(ctx, load(t, root), "C", board.NewOptions{})
	if err != nil || c == nil || out.Branch == nil {
		t.Fatalf("new on a branch: %v, %+v", err, out.Branch)
	}

	// A commit checked out on its own is no branch.
	run(t, root, "add", "-A")
	run(t, root, "commit", "-q", "-m", "work")
	run(t, root, "switch", "-q", "--detach", "HEAD")
	if b := SideBranch(ctx, load(t, root)); b == nil || !b.Detached || b.Main != "main" {
		t.Fatalf("detached = %+v", b)
	}

	// The board on main is as it was: the move lives on the branch.
	run(t, root, "switch", "-q", "main")
	if got := SideBranch(ctx, load(t, root)); got != nil {
		t.Fatalf("back on main: %+v", got)
	}
	if card, _ := load(t, root).Card("a"); card.Column != "backlog" {
		t.Fatalf("on main card a is in %s, want backlog", card.Column)
	}
}

func TestSideBranchFollowsOrigin(t *testing.T) {
	seed := repo(t, ModeNone)
	run(t, seed, "branch", "-q", "-m", "main", "trunk")
	clone := filepath.Join(t.TempDir(), "clone")
	run(t, seed, "clone", "-q", seed, clone)
	run(t, clone, "switch", "-q", "-c", "main") // a branch called main is not the main one here
	b := SideBranch(ctx, load(t, clone))
	if b == nil || b.Name != "main" || b.Main != "trunk" {
		t.Fatalf("got %+v, want main beside trunk", b)
	}
}

func TestSideBranchQuietWhenUnsure(t *testing.T) {
	root := repo(t, ModeNone)
	run(t, root, "branch", "-q", "-m", "main", "work")
	run(t, root, "switch", "-q", "-c", "other")
	if b := SideBranch(ctx, load(t, root)); b != nil {
		t.Fatalf("no main branch known, got %+v", b)
	}
}

func TestSideBranchOnlyForSyncNone(t *testing.T) {
	// A board that commits may live on a branch of its own on purpose.
	root := repo(t, ModeCommit)
	run(t, root, "switch", "-q", "-c", "kardboard")
	if b := SideBranch(ctx, load(t, root)); b != nil {
		t.Fatalf("sync: commit, got %+v", b)
	}
}
