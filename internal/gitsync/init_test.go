package gitsync

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvpnk/kardboard/internal/board"
)

// fresh makes an empty directory with git init and a new board.yaml in it,
// as kard init leaves it.
func fresh(t *testing.T, mode string, gitInit bool) *board.Board {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	root := t.TempDir()
	if gitInit {
		run(t, root, "init", "-q", "-b", "main")
		identify(t, root, "arnold")
	}
	if _, err := board.Init(root, board.InitOptions{ID: "x", Sync: mode}); err != nil {
		t.Fatal(err)
	}
	b, err := board.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestInitCommitsBoardYAML(t *testing.T) {
	b := fresh(t, ModeCommit, true)
	// Something else staged stays out of the commit.
	write(t, b.Root, "notes.txt", "mine\n")
	run(t, b.Root, "add", "notes.txt")

	out, err := Init(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if out.Commit == "" || out.Pushed {
		t.Fatalf("outcome %+v, want a commit and no push", out)
	}
	if got := run(t, b.Root, "log", "--format=%s"); got != "init board x" {
		t.Errorf("log %q", got)
	}
	if got := run(t, b.Root, "show", "--name-only", "--format=", "HEAD"); got != "board.yaml" {
		t.Errorf("commit holds %q, want only board.yaml", got)
	}
	if got := run(t, b.Root, "status", "--short"); got != "A  notes.txt" {
		t.Errorf("status %q, want notes.txt still staged", got)
	}
}

func TestInitLeavesSyncNoneAlone(t *testing.T) {
	b := fresh(t, ModeNone, true)
	out, err := Init(ctx, b)
	if err != nil || out.Commit != "" {
		t.Fatalf("Init = %+v, %v; want nothing", out, err)
	}
	if got := run(t, b.Root, "status", "--short"); got != "?? board.yaml" {
		t.Errorf("status %q", got)
	}
}

func TestInitOutsideARepository(t *testing.T) {
	b := fresh(t, ModeCommit, false)
	out, err := Init(ctx, b)
	if err == nil || !strings.Contains(err.Error(), "git init") {
		t.Fatalf("err %v, want a hint to run git init", err)
	}
	if out.Commit != "" {
		t.Errorf("commit %q outside a repository", out.Commit)
	}
}

func TestInitPushes(t *testing.T) {
	b := fresh(t, ModePush, true)

	// Without an upstream: committed, and the error says how to push.
	out, err := Init(ctx, b)
	if err == nil || !strings.Contains(err.Error(), "git push -u") || out.Commit == "" || out.Pushed {
		t.Fatalf("Init = %+v, %v; want a commit and a hint to push", out, err)
	}

	// With one: the next board's commit reaches the remote.
	remote := filepath.Join(t.TempDir(), "remote.git")
	run(t, b.Root, "init", "-q", "--bare", remote)
	run(t, b.Root, "remote", "add", "origin", remote)
	run(t, b.Root, "push", "-q", "-u", "origin", "main")
	sub := filepath.Join(b.Root, "sub")
	if _, err := board.Init(sub, board.InitOptions{ID: "sub", Sync: ModePush}); err != nil {
		t.Fatal(err)
	}
	sb, err := board.Load(sub)
	if err != nil {
		t.Fatal(err)
	}
	out, err = Init(ctx, sb)
	var pe *PushError
	if err != nil || errors.As(err, &pe) || !out.Pushed {
		t.Fatalf("Init = %+v, %v; want it pushed", out, err)
	}
	if got := run(t, remote, "log", "-1", "--format=%s", "main"); got != "init board sub" {
		t.Errorf("remote has %q", got)
	}
}
