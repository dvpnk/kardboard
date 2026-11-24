package gitsync

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/dvpnk/kardboard/internal/board"
)

// Init records a new board.yaml the way the board's sync mode asks for:
// nothing with sync: none, one commit "init board <id>" with commit, and that
// commit pushed with push, if the branch has an upstream already.
//
// The board exists either way, so Init never takes it back. When Git cannot
// do its part, the returned error says what is missing and the Outcome what
// was done: a commit without a push comes with a *PushError, as elsewhere.
func Init(ctx context.Context, b *board.Board) (Outcome, error) {
	var out Outcome
	mode := b.Config.Sync
	if mode == ModeNone {
		return out, nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return out, fmt.Errorf("board.yaml is not committed: sync: %s needs git, and it is not installed", mode)
	}
	lctx, cancel := local(ctx)
	defer cancel()
	if _, err := git(lctx, b.Root, "rev-parse", "--show-toplevel"); err != nil {
		return out, fmt.Errorf("board.yaml is not committed: sync: %s needs a Git repository; run `git init` there, then commit board.yaml yourself", mode)
	}
	if rebasing(lctx, b.Root) || exists(gitPath(lctx, b.Root, "MERGE_HEAD")) {
		return out, errors.New("board.yaml is not committed: a rebase or merge is under way in the repository")
	}
	if err := commit(lctx, b.Root, []string{"init board " + b.Config.ID}, board.ConfigFile); err != nil {
		_, _ = git(lctx, b.Root, "reset", "--quiet", "--", board.ConfigFile)
		return out, fmt.Errorf("board.yaml is not committed, because git commit failed: %w", err)
	}
	out.Commit = head(lctx, b.Root)
	if mode != ModePush {
		return out, nil
	}
	if _, err := git(lctx, b.Root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err != nil {
		return out, errors.New("board.yaml is committed, but not pushed: the branch has no upstream yet; push it once with `git push -u`")
	}
	if err := push(ctx, b.Root, board.ConfigFile); err != nil {
		var pe *PushError
		if errors.As(err, &pe) {
			return out, err
		}
		return out, &PushError{Reason: lastLine(err)}
	}
	out.Commit, out.Pushed = head(lctx, b.Root), true
	return out, nil
}
