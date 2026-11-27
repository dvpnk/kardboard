package gitsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dvpnk/kardboard/internal/board"
)

// EntryMessage returns the commit message for an entry in a card's
// activity: `comment <card>`, `ask <card>`, `run <card>: started in <column>`.
func EntryMessage(id string, e board.Entry) string {
	switch e.Kind {
	case board.KindQuestion:
		return "ask " + id
	case board.KindRunStarted:
		if col := e.Values["column"]; col != "" {
			return fmt.Sprintf("run %s: started in %s", id, col)
		}
		return fmt.Sprintf("run %s: started", id)
	case board.KindRunPaused, board.KindRunFinished, board.KindRunFailed:
		return fmt.Sprintf("run %s: %s", id, strings.TrimPrefix(e.Kind, "run "))
	default:
		return e.Kind + " " + id
	}
}

// EntryResult is what AddEntry did.
type EntryResult struct {
	Card string `json:"card"`
	// Path is the activity file, relative to the board root.
	Path  string      `json:"path"`
	Entry board.Entry `json:"entry"`
	Outcome
}

// AddEntry adds an entry to the activity of a card and does what the
// board's sync mode asks for. With sync: commit or push the entry is one
// commit that holds only the activity file.
//
// Entries are only added, so with sync: push an entry never gets lost in a
// conflict: if someone else added to the same file in the meantime, the
// own commit is taken back, the others' state is pulled in, and the entry
// is added again below theirs. As with Change, a *PushError comes together
// with a result.
func AddEntry(ctx context.Context, b *board.Board, id string, e board.Entry) (*EntryResult, error) {
	if b.Config == nil || b.Config.Sync == ModeNone {
		path, err := b.AddEntry(id, e)
		if err != nil {
			return nil, err
		}
		res := result(b, id, path)
		res.Branch = SideBranch(ctx, b)
		return res, nil
	}
	if err := ready(ctx, b); err != nil {
		return nil, err
	}
	res, err := commitEntry(ctx, b, id, e)
	if err != nil {
		return nil, err
	}
	if b.Config.Sync != ModePush {
		return res, nil
	}

	err = pushEntry(ctx, b.Root, res.Card)
	if errors.Is(err, errEntryConflict) {
		// Someone added to the same activity meanwhile. Their entries come
		// first, ours is added below them, once more.
		fresh, loadErr := board.Load(b.Root)
		if loadErr != nil {
			return nil, loadErr
		}
		if fresh.Config == nil {
			return nil, errors.New("board.yaml is invalid after pulling, see `kard lint`; the entry was not written")
		}
		if _, ok := fresh.Card(id); !ok {
			return nil, &ConflictError{Card: res.Card}
		}
		if res, err = commitEntry(ctx, fresh, id, res.Entry); err != nil {
			return nil, err
		}
		if err := pushOnce(ctx, b.Root); err != nil {
			return res, &PushError{Reason: "the remote refused twice (" + lastLine(err) + ")"}
		}
	} else if err != nil {
		var pe *PushError
		if errors.As(err, &pe) {
			return res, err
		}
		return nil, err
	}
	lctx, cancel := local(ctx)
	defer cancel()
	res.Commit, res.Pushed = head(lctx, b.Root), true
	return res, nil
}

func result(b *board.Board, id, path string) *EntryResult {
	entries := b.Activity(id)
	c, _ := b.Card(id)
	return &EntryResult{Card: c.ID, Path: path, Entry: entries[len(entries)-1]}
}

// commitEntry adds the entry and commits the activity file alone. If the
// commit fails, the file is put back as it was.
func commitEntry(ctx context.Context, b *board.Board, id string, e board.Entry) (*EntryResult, error) {
	c, ok := b.Card(strings.TrimSuffix(id, ".md"))
	if !ok {
		// AddEntry says why: no such card, or more than one.
		_, err := b.AddEntry(id, e)
		return nil, err
	}
	rel := board.ActivityPath(c.ID)
	lctx, cancel := local(ctx)
	defer cancel()
	changed, err := dirty(lctx, b.Root, rel)
	if err != nil {
		return nil, err
	}
	if len(changed) > 0 {
		return nil, fmt.Errorf("the activity of card %s (%s) has changes that are not committed yet; commit them first", c.ID, rel)
	}
	existed := b.HasActivity(c.ID)

	path, err := b.AddEntry(c.ID, e)
	if err != nil {
		return nil, err
	}
	res := result(b, c.ID, path)
	if err := commit(lctx, b.Root, []string{EntryMessage(c.ID, res.Entry)}, rel); err != nil {
		var undoErr error
		if existed {
			_, undoErr = git(lctx, b.Root, "checkout", "--quiet", "HEAD", "--", rel)
		} else {
			_, _ = git(lctx, b.Root, "reset", "--quiet", "--", rel)
			undoErr = os.Remove(filepath.Join(b.Root, filepath.FromSlash(rel)))
		}
		if undoErr != nil {
			return nil, fmt.Errorf("git commit failed (%w), and %s could not be put back: %v", err, rel, undoErr)
		}
		return nil, fmt.Errorf("nothing was written, because git commit failed: %w", err)
	}
	res.Commit = head(lctx, b.Root)
	return res, nil
}

// errEntryConflict means the remote holds entries of the same activity file
// that are not here yet. The own commit is taken back and the remote's
// state pulled in; the entry is to be added again.
var errEntryConflict = errors.New("the activity changed on the remote")

// pushEntry sends the commit just made. If the remote refuses because
// someone was faster, it pulls and tries once more. If the pull does not
// fit, which for an activity file means both sides added to its end, the
// own commit is taken back and errEntryConflict returned.
//
// If the card is gone after the pull, renamed or removed by someone else,
// the own commit is taken back too: an entry without its card would only
// be found by kard lint.
func pushEntry(ctx context.Context, root, id string) error {
	first := pushOnce(ctx, root)
	if first == nil {
		return nil
	}
	conflict, err := pull(ctx, root)
	if err == nil {
		if fresh, loadErr := board.Load(root); loadErr == nil && fresh.Config != nil {
			if _, ok := fresh.Card(id); !ok {
				lctx, cancel := local(ctx)
				defer cancel()
				if _, err := git(lctx, root, "reset", "--quiet", "--keep", "HEAD~1"); err != nil {
					return fmt.Errorf("card %s is no longer on the board, and your entry could not be taken back; check with `git status`: %w", id, err)
				}
				return &ConflictError{Card: id}
			}
		}
		if err := pushOnce(ctx, root); err != nil {
			return &PushError{Reason: "the remote refused twice (" + lastLine(err) + ")"}
		}
		return nil
	}
	if !conflict {
		return &PushError{Reason: lastLine(first)}
	}
	lctx, cancel := local(ctx)
	defer cancel()
	if _, err := git(lctx, root, "reset", "--quiet", "--keep", "HEAD~1"); err != nil {
		return fmt.Errorf("someone wrote in the same activity at the same time, and your commit could not be taken back; check with `git status`: %w", err)
	}
	if _, err := pull(ctx, root); err != nil {
		return fmt.Errorf("someone wrote in the same activity at the same time; your entry was taken back, but the remote's state could not be pulled in: %w", err)
	}
	return errEntryConflict
}
