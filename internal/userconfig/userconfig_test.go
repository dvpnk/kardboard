package userconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// home gives the test its own home and settings directory.
func home(t *testing.T) (homeDir, configFile string) {
	t.Helper()
	homeDir = t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, "cfg"))
	t.Setenv(EnvUser, "")
	return homeDir, filepath.Join(homeDir, "cfg", "kardboard", "config.yaml")
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

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLoad(t *testing.T) {
	_, file := home(t)

	cfg, err := Load()
	if err != nil || cfg.User != "" || len(cfg.Boards) != 0 {
		t.Fatalf("missing file: cfg %+v, err %v", cfg, err)
	}

	write(t, file, "user: arnold\nboards:\n  - ~/a\nscan:\n  - ~/Repos\nhidden: [atlas]\n")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.User != "arnold" || !slices.Equal(cfg.Boards, []string{"~/a"}) || !slices.Equal(cfg.Scan, []string{"~/Repos"}) || !slices.Equal(cfg.Hidden, []string{"atlas"}) {
		t.Errorf("cfg = %+v", cfg)
	}

	write(t, file, "user: arnold\ncolour: red\n")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "colour") {
		t.Errorf("unknown key: err = %v", err)
	}
}

func TestExpandAndAbbrev(t *testing.T) {
	homeDir, _ := home(t)
	inHome := filepath.Join(homeDir, "Repos", "x")

	if got := Expand("~/Repos/x"); got != inHome {
		t.Errorf("Expand = %q, want %q", got, inHome)
	}
	if got := Expand("~"); got != homeDir {
		t.Errorf("Expand(~) = %q", got)
	}
	if got := Abbrev(inHome); got != "~/Repos/x" {
		t.Errorf("Abbrev = %q", got)
	}
	if got := Abbrev(homeDir); got != "~" {
		t.Errorf("Abbrev(home) = %q", got)
	}
	// A directory that merely starts like the home directory is not inside it.
	if other := homeDir + "-other"; Abbrev(other) != other {
		t.Errorf("Abbrev(%q) = %q", other, Abbrev(other))
	}
	if abs := Expand("relative/dir"); !filepath.IsAbs(abs) {
		t.Errorf("Expand of a relative path = %q", abs)
	}
}

func TestEditLists(t *testing.T) {
	homeDir, file := home(t)
	a, b := filepath.Join(homeDir, "a"), filepath.Join(homeDir, "b")

	// The first edit creates the directory and the file.
	if changed, err := AddToList(KeyBoards, a); err != nil || !changed {
		t.Fatalf("changed %v, err %v", changed, err)
	}
	if got, want := read(t, file), "boards:\n  - ~/a\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	if changed, _ := AddToList(KeyBoards, a); changed {
		t.Error("adding twice changed the file")
	}
	if _, err := AddToList(KeyScan, b); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, file), "boards:\n  - ~/a\nscan:\n  - ~/b\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}

	if changed, err := RemoveFromList(KeyBoards, b); err != nil || changed {
		t.Errorf("removing what is not there: changed %v, err %v", changed, err)
	}
	if changed, err := RemoveFromList(KeyBoards, a); err != nil || !changed {
		t.Errorf("changed %v, err %v", changed, err)
	}
	cfg, err := Load()
	if err != nil || len(cfg.Boards) != 0 || !slices.Equal(cfg.Scan, []string{"~/b"}) {
		t.Errorf("cfg %+v, err %v", cfg, err)
	}
}

func TestEditKeepsCommentsAndOtherSettings(t *testing.T) {
	homeDir, file := home(t)
	// Comments, other settings and the order of keys survive. Blank lines
	// do not: the YAML library drops them when it writes the file.
	write(t, file, "# my settings\nuser: arnold # that's me\n\nboards:\n  # work\n  - ~/atlas-board\nhidden:\n  - atlas\n")

	if _, err := AddToList(KeyBoards, filepath.Join(homeDir, "demo")); err != nil {
		t.Fatal(err)
	}
	want := "# my settings\nuser: arnold # that's me\nboards:\n  # work\n  - ~/atlas-board\n  - ~/demo\nhidden:\n  - atlas\n"
	if got := read(t, file); got != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}

	// A path written in full is recognised as the same directory as ~/...
	write(t, file, "boards:\n  - "+filepath.Join(homeDir, "demo")+"\n")
	if changed, _ := AddToList(KeyBoards, filepath.Join(homeDir, "demo")); changed {
		t.Error("the same directory was added again")
	}
	if changed, _ := RemoveFromList(KeyBoards, filepath.Join(homeDir, "demo")); !changed {
		t.Error("the directory was not removed")
	}
}

func TestEditRefusesBrokenSettings(t *testing.T) {
	_, file := home(t)
	for name, content := range map[string]string{
		"not YAML":       "boards: [a\n",
		"not a mapping":  "- a\n",
		"boards is text": "boards: nope\n",
	} {
		write(t, file, content)
		if _, err := AddToList(KeyBoards, "/x"); err == nil {
			t.Errorf("%s: no error", name)
		}
		if read(t, file) != content {
			t.Errorf("%s: the file was changed", name)
		}
	}

	// An empty file and an empty list are fine.
	write(t, file, "")
	if _, err := AddToList(KeyBoards, "/x"); err != nil {
		t.Errorf("empty file: %v", err)
	}
	write(t, file, "boards:\n")
	if _, err := AddToList(KeyBoards, "/x"); err != nil {
		t.Errorf("empty list: %v", err)
	}
	if got, want := read(t, file), "boards:\n  - /x\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}
