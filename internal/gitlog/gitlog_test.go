package gitlog

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var columns = []string{"backlog", "working_on", "done"}

// repo is a Git repository with a board in the subdirectory "board".
type repo struct {
	t    *testing.T
	root string // the repository
	dir  string // the board inside it
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	r := &repo{t: t, root: root, dir: filepath.Join(root, "board")}
	r.git("init", "-q", "-b", "main")
	return r
}

func (r *repo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{
		"-C", r.root,
		"-c", "user.name=Arnold Example", "-c", "user.email=arnold@example.com",
		"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null",
	}, args...)...)
	// Keep the user's own Git settings out of the test.
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// write puts a file into the repository; rel is relative to the repository.
func (r *repo) write(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// move renames a file; both paths are relative to the repository. The next
// commit records it, and Git recognises the rename by the content.
func (r *repo) move(from, to string) {
	r.t.Helper()
	dst := filepath.Join(r.root, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(r.root, filepath.FromSlash(from)), dst); err != nil {
		r.t.Fatal(err)
	}
}

func (r *repo) commit(subject string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "-m", subject)
}

// card is long enough that Git still recognises it after a small edit.
func card(title string) string {
	return "# " + title + "\n\n" + strings.Repeat("Eine Zeile Text, die sich nicht ändert.\n", 12)
}

// show renders the entries compactly: "subject: kind card from>to".
func show(entries []Entry) []string {
	var out []string
	for _, e := range entries {
		var parts []string
		for _, c := range e.Changes {
			s := c.Kind
			if c.Card != "" {
				s += " " + c.Card
			} else {
				s += " " + c.Path
			}
			if c.From != "" || c.To != "" {
				s += " " + c.From + ">" + c.To
			}
			if c.Was != "" {
				s += " was " + c.Was
			}
			if c.Edited {
				s += " +edit"
			}
			parts = append(parts, s)
		}
		out = append(out, e.Subject+": "+strings.Join(parts, ", "))
	}
	return out
}

func history(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	r.write("board/board.yaml", "kardboard: 1\nid: demo\n")
	r.write("board/backlog/a.md", card("A"))
	r.write("board/backlog/b.md", card("B"))
	r.write("board/README.md", "about the board\n")
	r.write("src/main.go", "package main\n")
	r.commit("Board anlegen")

	r.move("board/backlog/a.md", "board/working_on/a.md")
	r.commit("a nehmen")

	r.write("board/backlog/b.md", card("B")+"Eine neue Zeile.\n")
	r.commit("b überarbeiten")

	r.write("src/main.go", "package main\n\nfunc main() {}\n")
	r.commit("Code, nicht das Board")

	r.move("board/working_on/a.md", "board/done/a.md")
	r.write("board/done/a.md", card("A")+"Erledigt am Dienstag.\n")
	r.commit("a fertig, mit Notiz")

	r.move("board/backlog/b.md", "board/backlog/c.md")
	r.commit("b heißt jetzt c")

	r.git("rm", "-q", "board/backlog/c.md")
	r.write("board/board.yaml", "kardboard: 1\nid: demo\nname: Demo\n")
	r.commit("c verworfen, Board benannt")
	return r
}

func TestBoardHistory(t *testing.T) {
	r := history(t)
	log := Read(context.Background(), r.dir, columns, "", 50)
	if !log.Available || log.Branch != "main" {
		t.Fatalf("log = %+v", log)
	}
	want := []string{
		"c verworfen, Board benannt: removed c backlog>, other board.yaml",
		"b heißt jetzt c: renamed c >backlog was b",
		"a fertig, mit Notiz: moved a working_on>done +edit",
		// "Code, nicht das Board" touched nothing in the board and is left out
		"b überarbeiten: edited b >backlog",
		"a nehmen: moved a backlog>working_on",
		"Board anlegen: other README.md, added a >backlog, added b >backlog, other board.yaml",
	}
	got := show(log.Entries)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("history:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	e := log.Entries[0]
	if e.Author != "Arnold Example" || len(e.Hash) != 40 || !strings.HasPrefix(e.Hash, e.Short) || len(e.Date) < 20 {
		t.Errorf("entry = %+v", e)
	}
	// Paths are relative to the board, not to the repository.
	if p := log.Entries[2].Changes[0].Path; p != "done/a.md" {
		t.Errorf("path = %q", p)
	}
}

func TestLimit(t *testing.T) {
	r := history(t)
	log := Read(context.Background(), r.dir, columns, "", 2)
	if got := show(log.Entries); len(got) != 2 || !strings.HasPrefix(got[1], "b heißt jetzt c") {
		t.Errorf("entries = %v", got)
	}
}

func TestCardHistoryFollowsMoves(t *testing.T) {
	r := history(t)
	log := Read(context.Background(), r.dir, columns, "done/a.md", 50)
	want := []string{
		"a fertig, mit Notiz: moved a working_on>done +edit",
		"a nehmen: moved a backlog>working_on",
		"Board anlegen: added a >backlog",
	}
	if got := show(log.Entries); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("history:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Boards used to keep the cards in progress in a directory per person. That
// history must still read as cards and columns.
func TestHistoryWithDirectoriesPerPerson(t *testing.T) {
	r := newRepo(t)
	r.write("board/backlog/x.md", card("X"))
	r.commit("anlegen")
	r.move("board/backlog/x.md", "board/working_on/laura/x.md")
	r.commit("laura nimmt x")
	r.move("board/working_on/laura/x.md", "board/working_on/angelique/x.md")
	r.commit("an angelique")
	r.move("board/working_on/angelique/x.md", "board/done/x.md")
	r.commit("fertig")

	log := Read(context.Background(), r.dir, columns, "", 50)
	want := []string{
		"fertig: moved x working_on>done",
		"an angelique: edited x >working_on",
		"laura nimmt x: moved x backlog>working_on",
		"anlegen: added x >backlog",
	}
	if got := show(log.Entries); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("history:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestNamesWithSpecialCharacters(t *testing.T) {
	r := newRepo(t)
	r.write("board/backlog/über-größe.md", card("Ü"))
	r.commit("Titel mit \"Anführung\", Tab\tund | Strich")
	log := Read(context.Background(), r.dir, columns, "", 5)
	if len(log.Entries) != 1 || log.Entries[0].Changes[0].Card != "über-größe" ||
		log.Entries[0].Subject != "Titel mit \"Anführung\", Tab\tund | Strich" {
		t.Errorf("entries = %+v", log.Entries)
	}
}

func TestWithoutHistory(t *testing.T) {
	// A directory that is not in a repository.
	plain := t.TempDir()
	log := Read(context.Background(), plain, columns, "", 10)
	if log.Available || !strings.Contains(log.Reason, "not in a Git repository") || log.Entries == nil {
		t.Errorf("plain directory: %+v", log)
	}

	// A repository without a commit, and one whose commits never touched the board.
	r := newRepo(t)
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if log := Read(context.Background(), r.dir, columns, "", 10); !log.Available || len(log.Entries) != 0 {
		t.Errorf("empty repository: %+v", log)
	}
	r.write("src/main.go", "package main\n")
	r.commit("nur Code")
	if log := Read(context.Background(), r.dir, columns, "", 10); !log.Available || len(log.Entries) != 0 || log.Branch != "main" {
		t.Errorf("untouched board: %+v", log)
	}
}

func TestPlace(t *testing.T) {
	tests := []struct {
		path   string
		column string
		id     string
		ok     bool
	}{
		{"backlog/a.md", "backlog", "a", true},
		{"working_on/laura/a.md", "working_on", "a", true},
		{"working_on/laura/deep/a.md", "", "", false},
		{"reference/a.md", "", "", false},
		{"backlog/notes.txt", "", "", false},
		{"backlog/.hidden.md", "", "", false},
		{"board.yaml", "", "", false},
		{"a.md", "", "", false},
	}
	for _, tt := range tests {
		col, id, ok := place(tt.path, columns)
		if col != tt.column || id != tt.id || ok != tt.ok {
			t.Errorf("place(%q) = %q, %q, %v", tt.path, col, id, ok)
		}
	}
}

func TestParseIgnoresWhatItDoesNotKnow(t *testing.T) {
	out := fmt.Sprintf("%sabc%sab%sAlex%s2026-11-11T10:00:00+01:00%sSubject\n\nM\tbacklog/a.md\n\nnot a status line\n%sbroken record\n",
		recordSep, fieldSep, fieldSep, fieldSep, fieldSep, recordSep)
	entries := parse(out, columns)
	if len(entries) != 1 || len(entries[0].Changes) != 1 || entries[0].Changes[0].Kind != KindEdited {
		t.Errorf("entries = %+v", entries)
	}
}
