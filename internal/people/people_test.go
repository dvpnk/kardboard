package people

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"kardboard/internal/board"
)

// clean gives the test empty settings, no Git identity and no account.
func clean(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv(EnvUser, "")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	lookup := LookupAccount
	LookupAccount = func() (*user.User, error) { return nil, errors.New("no account") }
	t.Cleanup(func() { LookupAccount = lookup })
	return dir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// repo makes dir a Git repository with a board in board/.
func repo(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "init", "-q")
	b := filepath.Join(dir, "board")
	write(t, filepath.Join(b, "board.yaml"), "kardboard: 1\nid: demo\ncolumns:\n  - id: todo\n")
	return b
}

func TestMeTakesTheFirstSource(t *testing.T) {
	dir := clean(t)
	b := repo(t, dir)

	if _, err := Me(b); err == nil || !strings.Contains(err.Error(), "does not know who you are") {
		t.Fatalf("nothing set: err = %v", err)
	}

	LookupAccount = func() (*user.User, error) { return &user.User{Username: `FIRMA\Lambert`}, nil }
	check(t, b, "lambert", FromAccount)

	run(t, dir, "config", "user.email", "Angelique.Lambert@Firma.de")
	check(t, b, "angelique.lambert@firma.de", FromGit)

	write(t, filepath.Join(dir, "config", "kardboard", "config.yaml"), "user: arnold\n")
	check(t, b, "arnold", FromUser)

	write(t, filepath.Join(dir, ".kard", "config.yaml"), "user: angelique-l\n")
	check(t, b, "angelique-l", FromProject)

	// The nearest .kard wins: the board's own over the repository's.
	write(t, filepath.Join(b, ".kard", "config.yaml"), "user: board-only\n")
	check(t, b, "board-only", FromProject)

	t.Setenv(EnvUser, "sly")
	check(t, b, "sly", FromEnv)

	t.Setenv(EnvUser, "Sly Smith")
	if _, err := Me(b); err == nil || !strings.Contains(err.Error(), "neither a short name") {
		t.Errorf("invalid name: err = %v", err)
	}
}

func check(t *testing.T, dir, name string, source Source) {
	t.Helper()
	id, err := Me(dir)
	if err != nil || id.Name != name || id.Source != source {
		t.Errorf("Me = %+v, %v; want %s from %s", id, err, name, source)
	}
}

func TestKardAboveTheRepositoryIsNotRead(t *testing.T) {
	dir := clean(t)
	write(t, filepath.Join(dir, ".kard", "config.yaml"), "user: outside\n")
	inner := filepath.Join(dir, "projekt")
	b := repo(t, inner)
	if id, err := Me(b); err == nil {
		t.Errorf("Me = %+v: a .kard outside the repository must not count", id)
	}
}

func TestProjectFileRejectsOtherKeys(t *testing.T) {
	dir := clean(t)
	write(t, filepath.Join(dir, ".kard", "config.yaml"), "user: arnold\ncolour: red\n")
	if _, err := Me(dir); err == nil || !strings.Contains(err.Error(), "colour") {
		t.Errorf("err = %v", err)
	}
}

func TestSetProject(t *testing.T) {
	dir := clean(t)
	b := repo(t, dir)

	path, err := SetProject(b, "Angelique-L")
	if err == nil {
		t.Fatalf("an invalid name was written to %s", path)
	}
	path, err = SetProject(b, "angelique-l")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, ".kard", "config.yaml"); path != want {
		t.Errorf("path = %s, want the top of the repository", path)
	}
	check(t, b, "angelique-l", FromProject)

	// The directory ignores itself, so adding everything leaves it out.
	run(t, dir, "add", "-A")
	out, _ := exec.Command("git", "-C", dir, "ls-files").Output()
	if strings.Contains(string(out), ".kard") {
		t.Errorf("git tracks .kard:\n%s", out)
	}

	// Forced in anyway, lint calls it an error.
	run(t, dir, "add", "-f", ".kard/config.yaml")
	bd, err := board.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	problems := TrackedProject(context.Background(), bd)
	if len(problems) != 1 || problems[0].Code != "E11" || problems[0].Path != "../.kard/config.yaml" {
		t.Errorf("problems = %+v", problems)
	}
}

func TestSetProjectOutsideARepository(t *testing.T) {
	dir := clean(t)
	path, err := SetProject(dir, "arnold@privat.de")
	if err != nil || path != filepath.Join(dir, ".kard", "config.yaml") {
		t.Fatalf("path %s, err %v", path, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".kard", ".gitignore")); err != nil {
		t.Error(err)
	}
	check(t, dir, "arnold@privat.de", FromProject)
}

func TestKnownFromHistoryCardsAndMailmap(t *testing.T) {
	dir := clean(t)
	b := repo(t, dir)
	write(t, filepath.Join(b, "board.yaml"), "kardboard: 1\nid: demo\ncolumns:\n  - id: todo\n")
	write(t, filepath.Join(b, "todo", "eins.md"), "---\nassignee: [sly, alt@privat.de]\n---\n\n# Eins\n")
	run(t, dir, "add", "-A")
	run(t, dir, "-c", "user.name=Angélique Lambert", "-c", "user.email=angelique@firma.de", "commit", "-q", "-m", "eins")
	write(t, filepath.Join(b, "todo", "zwei.md"), "# Zwei\n")
	run(t, dir, "add", "-A")
	run(t, dir, "-c", "user.name=Anna Lang", "-c", "user.email=anna@firma.de", "commit", "-q", "-m", "zwei")
	// Angélique's old address is the same person.
	write(t, filepath.Join(dir, ".mailmap"), "Angélique Lambert <angelique@firma.de> <alt@privat.de>\n")

	bd, err := board.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	known := Known(context.Background(), bd)
	got := []string{}
	for _, p := range known {
		got = append(got, p.ID+"="+p.Name)
	}
	if want := "angelique@firma.de=Angélique Lambert anna@firma.de=Anna Lang sly="; strings.Join(got, " ") != want {
		t.Errorf("known = %v, want %s", got, want)
	}
	if !bd.SamePerson("alt@privat.de", "Angelique@Firma.de") {
		t.Error("the .mailmap must join the two addresses")
	}
	c, _ := bd.Card("eins")
	if !bd.HasPerson(c.Assignees(), "angelique@firma.de") {
		t.Error("the card under the old address must count as Angélique's")
	}

	for name, want := range map[string]string{
		"sly":             "sly",
		"Angelique":       "angelique@firma.de", // name in Git, without the accent
		"lang":            "anna@firma.de",      // any word of the name
		"anna@":           "anna@firma.de",
		"neu":             "neu", // nobody: a new short name
		"Neu@Example.org": "neu@example.org",
	} {
		if id, err := Resolve(name, known); err != nil || id != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", name, id, err, want)
		}
	}
	var amb *AmbiguousError
	if _, err := Resolve("an", known); !errors.As(err, &amb) || len(amb.Candidates) != 2 {
		t.Errorf("Resolve(an): err = %v", err)
	}
	if _, err := Resolve("Sly Smith", known); err == nil {
		t.Error("Resolve(Sly Smith) must fail")
	}

	set, err := ResolveSet(bd.Config, []string{"assignee=sly,lang", "tags=bug", "assignee="}, known)
	if err != nil || strings.Join(set, " ") != "assignee=sly,anna@firma.de tags=bug assignee=" {
		t.Errorf("ResolveSet = %v, %v", set, err)
	}
}
