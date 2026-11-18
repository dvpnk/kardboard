// Package gitsync changes cards the way the board's `sync` setting asks for:
// files only (none), one commit per action (commit), or pull, commit and push
// (push).
//
// The command line and the web UI change cards through this package, never
// through package board directly, so a board that promises commits gets them.
package gitsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"kardboard/internal/board"
)

// Sync modes of a board, as written in board.yaml.
const (
	ModeNone   = "none"
	ModeCommit = "commit"
	ModePush   = "push"
)

const (
	localTimeout  = 15 * time.Second
	remoteTimeout = 60 * time.Second
)

// Outcome says what Git did for one action.
type Outcome struct {
	// Commit is the short hash of the commit made for the action. Empty on
	// boards with sync: none and when nothing changed.
	Commit string `json:"commit,omitempty"`
	// Pushed is true if the commit reached the remote.
	Pushed bool `json:"pushed,omitempty"`
	// Branch is set on boards with sync: none in a code repository when
	// the change was made on a branch other than the main one: it is only
	// there until that branch is merged.
	Branch *Branch `json:"branch,omitempty"`
}

// Result is what Change did.
type Result struct {
	*board.ChangeResult
	Outcome
}

// ConflictError means someone else changed the same card in the meantime.
// The user's own change was taken back and the board now shows the others'.
type ConflictError struct {
	Card string
	// Column is where the card is now. Empty if it is gone.
	Column string
}

func (e *ConflictError) Error() string {
	where := "it is no longer on the board"
	if e.Column != "" {
		where = "it is now in " + e.Column
	}
	return fmt.Sprintf("someone changed card %s at the same time; your change was taken back, %s", e.Card, where)
}

// PushError means the change is committed on this machine but did not reach
// the remote.
type PushError struct{ Reason string }

func (e *PushError) Error() string {
	return "committed here, but not pushed: " + e.Reason + "; try `kard sync` later"
}

// ── Running Git ─────────────────────────────────────────────────────────────

// git runs one Git command in dir. Git never asks for a password here: the
// web server has no terminal to ask on.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}

func local(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, localTimeout)
}

// gitPath resolves a path inside the repository's .git directory.
func gitPath(ctx context.Context, root, name string) string {
	out, err := git(ctx, root, "rev-parse", "--git-path", name)
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(out)
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return p
}

func exists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func rebasing(ctx context.Context, root string) bool {
	return exists(gitPath(ctx, root, "rebase-merge")) || exists(gitPath(ctx, root, "rebase-apply"))
}

// Usable reports whether the board's sync mode can work where the board
// lies: Git is installed, the board is in a repository and, for push, the
// branch has an upstream. Boards with sync: none are always usable.
func Usable(ctx context.Context, b *board.Board) error {
	mode := b.Config.Sync
	if mode == ModeNone {
		return nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("board %s uses sync: %s, but git is not installed", b.Config.ID, mode)
	}
	ctx, cancel := local(ctx)
	defer cancel()
	if _, err := git(ctx, b.Root, "rev-parse", "--show-toplevel"); err != nil {
		return fmt.Errorf("board %s uses sync: %s, but is not in a Git repository", b.Config.ID, mode)
	}
	if mode == ModePush {
		if _, err := git(ctx, b.Root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err != nil {
			return fmt.Errorf("board %s uses sync: push, but the current branch has no upstream; push it once with `git push -u`", b.Config.ID)
		}
	}
	return nil
}

// ready is Usable plus: no rebase or merge is under way.
func ready(ctx context.Context, b *board.Board) error {
	if err := Usable(ctx, b); err != nil {
		return err
	}
	ctx, cancel := local(ctx)
	defer cancel()
	if rebasing(ctx, b.Root) || exists(gitPath(ctx, b.Root, "MERGE_HEAD")) {
		return errors.New("a rebase or merge is under way in the board's repository; finish or abort it first")
	}
	return nil
}

// dirty lists the files under the given paths (relative to root) that differ
// from the last commit: changed, staged, deleted or not yet known to Git.
func dirty(ctx context.Context, root string, paths ...string) ([]string, error) {
	spec := append([]string{"--"}, paths...)
	work, err := git(ctx, root, append([]string{"ls-files", "--modified", "--others", "--deleted", "--exclude-standard"}, spec...)...)
	if err != nil {
		return nil, err
	}
	staged, err := git(ctx, root, append([]string{"diff", "--cached", "--name-only", "--no-renames", "--relative"}, spec...)...)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(work+staged, "\n") {
		if line != "" && !slices.Contains(out, line) {
			out = append(out, line)
		}
	}
	slices.Sort(out)
	return out, nil
}

// pull brings in the remote's commits and puts the local ones on top. If
// that does not work because both sides changed the same thing, it restores
// the state from before and reports a conflict.
func pull(ctx context.Context, root string) (conflict bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, remoteTimeout)
	defer cancel()
	_, err = git(ctx, root, "-c", "advice.mergeConflict=false", "pull", "--rebase", "--autostash", "--quiet")
	if err == nil {
		return false, nil
	}
	if rebasing(ctx, root) {
		_, _ = git(ctx, root, "rebase", "--abort")
		return true, err
	}
	return false, err
}

func pushOnce(ctx context.Context, root string) error {
	ctx, cancel := context.WithTimeout(ctx, remoteTimeout)
	defer cancel()
	_, err := git(ctx, root, "push", "--quiet")
	return err
}

func head(ctx context.Context, root string) string {
	out, _ := git(ctx, root, "rev-parse", "--short", "HEAD")
	return strings.TrimSpace(out)
}

// commit makes one commit that holds exactly the given paths, even if other
// things are staged.
func commit(ctx context.Context, root string, messages []string, paths ...string) error {
	spec := append([]string{"--"}, paths...)
	if _, err := git(ctx, root, append([]string{"add", "-A"}, spec...)...); err != nil {
		return err
	}
	args := []string{"commit", "--quiet", "--only"}
	args = append(args, "-m", messages[0])
	if len(messages) > 1 {
		args = append(args, "-m", strings.Join(messages[1:], "\n"))
	}
	_, err := git(ctx, root, append(args, spec...)...)
	return err
}

// ── Before a change ─────────────────────────────────────────────────────────

// Prepare gets a board ready for a change. On boards with sync: push it
// pulls first and returns the board as it is after that; build the change
// from the returned board. Other boards are returned as they are.
func Prepare(ctx context.Context, b *board.Board) (*board.Board, error) {
	if b.Config == nil || b.Config.Sync != ModePush {
		return b, nil
	}
	if err := ready(ctx, b); err != nil {
		return nil, err
	}
	if conflict, err := pull(ctx, b.Root); err != nil {
		if conflict {
			return nil, errors.New("commits on this machine do not fit the remote's; nothing was changed. Resolve it by hand with `git pull --rebase`")
		}
		return nil, fmt.Errorf("cannot get the remote's state, nothing was changed: %w", err)
	}
	fresh, err := board.Load(b.Root)
	if err != nil {
		return nil, err
	}
	if fresh.Config == nil {
		return nil, errors.New("board.yaml is invalid after pulling, see `kard lint`")
	}
	return fresh, nil
}

// ── Changing ────────────────────────────────────────────────────────────────

// Messages returns the commit messages for a change, the subject first. They
// have a fixed form, so `git log` reads as the board's journal.
func Messages(res *board.ChangeResult) []string {
	var out []string
	if res.Moved {
		out = append(out, fmt.Sprintf("move %s: %s -> %s", res.ID, res.From, res.To))
	}
	edited := false
	for name, value := range res.Set {
		if name != board.FieldAssignee {
			edited = true
			continue
		}
		who, _ := value.([]string)
		out = append(out, fmt.Sprintf("assign %s: %s", res.ID, strings.Join(who, ", ")))
	}
	for _, name := range res.Removed {
		if name == board.FieldAssignee {
			out = append(out, "unassign "+res.ID)
		} else {
			edited = true
		}
	}
	if edited || res.Text {
		out = append(out, "edit "+res.ID)
	}
	return out
}

// Change applies ch to a card and does what the board's sync mode asks for.
// With sync: commit or push the change is one commit that holds only this
// card. If that commit cannot be made, the card is put back as it was.
//
// A *PushError comes together with a result: the change happened and is
// committed, it just did not reach the remote.
func Change(ctx context.Context, b *board.Board, id string, ch board.Change) (*Result, error) {
	if b.Config == nil || b.Config.Sync == ModeNone {
		res, err := b.Change(id, ch)
		if err != nil {
			return nil, err
		}
		out := &Result{ChangeResult: res}
		if res.Changed() {
			out.Branch = SideBranch(ctx, b)
		}
		return out, nil
	}
	if err := ready(ctx, b); err != nil {
		return nil, err
	}
	oldPath := ""
	if c, ok := b.Card(id); ok {
		oldPath = c.Path
		if err := clean(ctx, b.Root, c.ID, oldPath); err != nil {
			return nil, err
		}
	}
	res, err := b.Change(id, ch)
	if err != nil {
		return nil, err
	}
	out := &Result{ChangeResult: res}
	if !res.Changed() {
		return out, nil
	}

	lctx, cancel := local(ctx)
	defer cancel()
	paths := []string{oldPath}
	if res.Path != oldPath {
		paths = append(paths, res.Path)
	}
	if err := commit(lctx, b.Root, Messages(res), paths...); err != nil {
		if res.Path != oldPath {
			_, _ = git(lctx, b.Root, "reset", "--quiet", "--", res.Path)
			_ = os.Remove(filepath.Join(b.Root, filepath.FromSlash(res.Path)))
		}
		if _, undoErr := git(lctx, b.Root, "checkout", "--quiet", "HEAD", "--", oldPath); undoErr != nil {
			return nil, fmt.Errorf("git commit failed (%w), and card %s could not be put back: %v", err, res.ID, undoErr)
		}
		return nil, fmt.Errorf("nothing was changed, because git commit failed: %w", err)
	}
	out.Commit = head(lctx, b.Root)
	if b.Config.Sync == ModePush {
		if err := push(ctx, b.Root, res.ID); err != nil {
			var pe *PushError
			if errors.As(err, &pe) {
				return out, err
			}
			return nil, err
		}
		out.Commit, out.Pushed = head(lctx, b.Root), true
	}
	return out, nil
}

// Check says whether Change could change the card now, as far as Git is
// concerned: the board's sync mode works here, and the card has no changes
// waiting to be committed. `kard edit` asks before it opens the editor, so
// nobody writes a text that cannot be saved.
func Check(ctx context.Context, b *board.Board, id string) error {
	if b.Config == nil || b.Config.Sync == ModeNone {
		return nil
	}
	if err := ready(ctx, b); err != nil {
		return err
	}
	if c, ok := b.Card(id); ok {
		return clean(ctx, b.Root, c.ID, c.Path)
	}
	return nil
}

// clean insists that a card has no changes waiting to be committed. They
// would end up in the action's commit and be lost with it in a conflict.
func clean(ctx context.Context, root, id, path string) error {
	ctx, cancel := local(ctx)
	defer cancel()
	changed, err := dirty(ctx, root, path)
	if err != nil {
		return err
	}
	if len(changed) > 0 {
		return fmt.Errorf("card %s has changes that are not committed yet; commit them first", id)
	}
	return nil
}

// New creates a card and does what the board's sync mode asks for. As with
// Change, a *PushError comes together with the card.
func New(ctx context.Context, b *board.Board, title string, opt board.NewOptions) (*board.Card, Outcome, error) {
	var out Outcome
	if b.Config == nil || b.Config.Sync == ModeNone {
		c, err := b.New(title, opt)
		if err == nil {
			out.Branch = SideBranch(ctx, b)
		}
		return c, out, err
	}
	if err := ready(ctx, b); err != nil {
		return nil, out, err
	}
	c, err := b.New(title, opt)
	if err != nil {
		return nil, out, err
	}
	lctx, cancel := local(ctx)
	defer cancel()
	if err := commit(lctx, b.Root, []string{"new " + c.ID}, c.Path); err != nil {
		_, _ = git(lctx, b.Root, "reset", "--quiet", "--", c.Path)
		_ = os.Remove(filepath.Join(b.Root, filepath.FromSlash(c.Path)))
		return nil, out, fmt.Errorf("nothing was created, because git commit failed: %w", err)
	}
	out.Commit = head(lctx, b.Root)
	if b.Config.Sync == ModePush {
		if err := push(ctx, b.Root, c.ID); err != nil {
			var pe *PushError
			if errors.As(err, &pe) {
				return c, out, err
			}
			return nil, out, err
		}
		out.Commit, out.Pushed = head(lctx, b.Root), true
	}
	return c, out, nil
}

// Rename renames a card and does what the board's sync mode asks for. With
// sync: commit or push it is one commit "rename <old> -> <new>" that holds
// the card and every card whose references were updated. None of them may
// have changes waiting to be committed. If the commit cannot be made,
// everything is put back. As with Change, a *PushError comes together with
// the result.
func Rename(ctx context.Context, b *board.Board, id, newID string, opt board.RenameOptions) (*board.RenameResult, Outcome, error) {
	var out Outcome
	if b.Config == nil || b.Config.Sync == ModeNone {
		res, err := b.Rename(id, newID, opt)
		if err == nil {
			out.Branch = SideBranch(ctx, b)
		}
		return res, out, err
	}
	if err := ready(ctx, b); err != nil {
		return nil, out, err
	}
	// Every card the rename may touch must be committed: the card and
	// those that refer to anything at all, which is cheap to ask for.
	var paths []string
	for _, c := range b.Cards {
		if c.ID == strings.TrimSuffix(id, ".md") || len(c.Refs()) > 0 {
			paths = append(paths, c.Path)
		}
	}
	lctx, cancel := local(ctx)
	defer cancel()
	if len(paths) > 0 {
		changed, err := dirty(lctx, b.Root, paths...)
		if err != nil {
			return nil, out, err
		}
		if len(changed) > 0 {
			return nil, out, fmt.Errorf("%s has changes that are not committed yet; commit them first", strings.Join(changed, ", "))
		}
	}

	res, err := b.Rename(id, newID, opt)
	if err != nil {
		if res != nil {
			// Renamed, but a reference could not be written: put it all back.
			_ = os.Remove(filepath.Join(b.Root, filepath.FromSlash(res.NewPath)))
			_, _ = git(lctx, b.Root, append([]string{"checkout", "--quiet", "HEAD", "--", res.Path}, res.Updated...)...)
		}
		return nil, out, err
	}
	touched := append([]string{res.Path, res.NewPath}, res.Updated...)
	if err := commit(lctx, b.Root, []string{fmt.Sprintf("rename %s -> %s", res.ID, res.NewID)}, touched...); err != nil {
		_, _ = git(lctx, b.Root, "reset", "--quiet", "--", res.NewPath)
		_ = os.Remove(filepath.Join(b.Root, filepath.FromSlash(res.NewPath)))
		if _, undoErr := git(lctx, b.Root, append([]string{"checkout", "--quiet", "HEAD", "--", res.Path}, res.Updated...)...); undoErr != nil {
			return nil, out, fmt.Errorf("git commit failed (%w), and the cards could not be put back: %v", err, undoErr)
		}
		return nil, out, fmt.Errorf("nothing was renamed, because git commit failed: %w", err)
	}
	out.Commit = head(lctx, b.Root)
	if b.Config.Sync == ModePush {
		if err := push(ctx, b.Root, res.NewID); err != nil {
			var pe *PushError
			if errors.As(err, &pe) {
				return res, out, err
			}
			return nil, out, err
		}
		out.Commit, out.Pushed = head(lctx, b.Root), true
	}
	return res, out, nil
}

// push sends the commit just made. If the remote refuses because someone
// was faster, it pulls and tries once more. If the others changed the very
// same card, the own commit is taken back, their state is pulled in, and a
// *ConflictError says where the card is now.
func push(ctx context.Context, root, id string) error {
	first := pushOnce(ctx, root)
	if first == nil {
		return nil
	}
	conflict, err := pull(ctx, root)
	if err == nil {
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
		return fmt.Errorf("someone changed card %s at the same time, and your commit could not be taken back; check with `git status`: %w", id, err)
	}
	if _, err := pull(ctx, root); err != nil {
		return fmt.Errorf("someone changed card %s at the same time; your change was taken back, but the remote's state could not be pulled in: %w", id, err)
	}
	ce := &ConflictError{Card: id}
	if fresh, err := board.Load(root); err == nil && fresh.Config != nil {
		if c, ok := fresh.Card(id); ok {
			ce.Column = c.Column
		}
	}
	return ce
}

func lastLine(err error) string {
	lines := strings.Split(strings.TrimSpace(err.Error()), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// ── Status and kard sync ──────────────────────────────────────────────────────

// Status compares the board's repository with its remote, as far as the
// last fetch knows it.
type Status struct {
	Branch string `json:"branch,omitempty"`
	// Ahead counts commits that are not pushed yet, Behind those that are
	// not pulled yet. Both are 0 if the branch has no upstream.
	Ahead  int `json:"ahead"`
	Behind int `json:"behind"`
	// Dirty lists the ids of cards with changes that are not committed.
	Dirty []string `json:"dirty"`
}

// ReadStatus reads the status without talking to the remote.
func ReadStatus(ctx context.Context, b *board.Board) (Status, error) {
	st := Status{Dirty: []string{}}
	if err := Usable(ctx, b); err != nil {
		return st, err
	}
	ctx, cancel := local(ctx)
	defer cancel()
	if out, err := git(ctx, b.Root, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		st.Branch = strings.TrimSpace(out)
	}
	if out, err := git(ctx, b.Root, "rev-list", "--left-right", "--count", "HEAD...@{u}"); err == nil {
		if parts := strings.Fields(out); len(parts) == 2 {
			st.Ahead, _ = strconv.Atoi(parts[0])
			st.Behind, _ = strconv.Atoi(parts[1])
		}
	}
	columns := make([]string, 0, len(b.Config.Columns))
	for _, col := range b.Config.Columns {
		// Git complains about a pathspec that matches nothing it knows.
		if exists(filepath.Join(b.Root, col.ID)) {
			columns = append(columns, col.ID)
		}
	}
	if len(columns) > 0 {
		files, err := dirty(ctx, b.Root, columns...)
		if err != nil {
			return st, err
		}
		for _, f := range files {
			parts := strings.Split(f, "/")
			if len(parts) == 2 && strings.HasSuffix(parts[1], ".md") {
				if id := strings.TrimSuffix(parts[1], ".md"); !slices.Contains(st.Dirty, id) {
					st.Dirty = append(st.Dirty, id)
				}
			}
		}
	}
	return st, nil
}

// Fetch asks the remote what is new, without changing any file.
func Fetch(ctx context.Context, b *board.Board) error {
	if err := Usable(ctx, b); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, remoteTimeout)
	defer cancel()
	_, err := git(ctx, b.Root, "fetch", "--quiet")
	return err
}

// SyncResult says what Sync moved.
type SyncResult struct {
	Pulled int `json:"pulled"`
	Pushed int `json:"pushed"`
}

// Sync pulls what is new on the remote and pushes what is new here. It is
// for boards with sync: push only.
func Sync(ctx context.Context, b *board.Board) (SyncResult, error) {
	var res SyncResult
	if b.Config.Sync != ModePush {
		return res, fmt.Errorf("board %s uses sync: %s; only boards with sync: push have a remote to sync with", b.Config.ID, b.Config.Sync)
	}
	if err := ready(ctx, b); err != nil {
		return res, err
	}
	if err := Fetch(ctx, b); err != nil {
		return res, fmt.Errorf("cannot reach the remote: %s", lastLine(err))
	}
	st, err := ReadStatus(ctx, b)
	if err != nil {
		return res, err
	}
	if st.Behind > 0 {
		if conflict, err := pull(ctx, b.Root); err != nil {
			if conflict {
				return res, errors.New("commits on this machine do not fit the remote's; resolve it by hand with `git pull --rebase`")
			}
			return res, fmt.Errorf("cannot pull: %s", lastLine(err))
		}
		res.Pulled = st.Behind
	}
	if st.Ahead > 0 {
		if err := pushOnce(ctx, b.Root); err != nil {
			return res, fmt.Errorf("cannot push: %s", lastLine(err))
		}
		res.Pushed = st.Ahead
	}
	return res, nil
}
