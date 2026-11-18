package gitsync

import (
	"context"
	"os/exec"
	"strings"

	"github.com/dvpnk/kardboard/internal/board"
)

// Branch is the branch a board in a code repository is seen on, when that
// is not the repository's main branch. A board with sync: none lives in the
// working tree: what it shows and what kard changes in it is only on this
// branch until the branch is merged.
type Branch struct {
	// Name is the checked-out branch, or the short commit if none is.
	Name string `json:"name"`
	// Detached is true if no branch is checked out, only a commit.
	Detached bool `json:"detached,omitempty"`
	// Main is the repository's main branch, such as main.
	Main string `json:"main"`
}

// SideBranch returns the branch the board is seen on if that is not the
// repository's main branch, and nil otherwise. It only looks at boards with
// sync: none; a board that commits lives on the branch it is meant to, which
// may well be a branch of its own. If kard cannot tell which branch is the
// main one, it says nothing.
func SideBranch(ctx context.Context, b *board.Board) *Branch {
	if b.Config == nil || b.Config.Sync != ModeNone {
		return nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	ctx, cancel := local(ctx)
	defer cancel()
	main := mainBranch(ctx, b.Root)
	if main == "" {
		return nil
	}
	// symbolic-ref also knows a branch without any commit yet.
	if out, err := git(ctx, b.Root, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
		name := strings.TrimSpace(out)
		if name == main {
			return nil
		}
		return &Branch{Name: name, Main: main}
	}
	if hash := head(ctx, b.Root); hash != "" {
		return &Branch{Name: hash, Detached: true, Main: main}
	}
	return nil
}

// mainBranch guesses the repository's main branch: the one origin points
// at, else Git's default for new repositories, main or master, as far as
// such a branch exists here.
func mainBranch(ctx context.Context, root string) string {
	if out, err := git(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if name := strings.TrimPrefix(strings.TrimSpace(out), "origin/"); name != "" {
			return name
		}
	}
	candidates := []string{"main", "master"}
	if out, err := git(ctx, root, "config", "--get", "init.defaultBranch"); err == nil {
		candidates = append([]string{strings.TrimSpace(out)}, candidates...)
	}
	for _, name := range candidates {
		if name == "" {
			continue
		}
		if _, err := git(ctx, root, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); err == nil {
			return name
		}
	}
	return ""
}
