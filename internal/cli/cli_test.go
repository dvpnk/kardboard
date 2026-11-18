package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"kardboard/internal/people"
)

var example = filepath.Join("..", "..", "examples", "board")

// kard runs the command and returns exit code, stdout and stderr.
func kard(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// asUser makes kard see the given user and no settings file. An empty name
// means: nobody has told kard who they are, and neither Git nor the
// operating system can say it.
func asUser(t *testing.T, name string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KARDBOARD_USER", name)
	if name == "" {
		nobody(t)
	}
}

// nobody hides the user's Git address and account name from kard.
func nobody(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	lookup := people.LookupAccount
	people.LookupAccount = func() (*user.User, error) { return nil, errors.New("no account") }
	t.Cleanup(func() { people.LookupAccount = lookup })
}

// copyOfExample is a board the test may change.
func copyOfExample(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(example)); err != nil {
		t.Fatal(err)
	}
	return root
}

// brokenBoard is the example board plus one card that breaks two rules.
func brokenBoard(t *testing.T) string {
	t.Helper()
	root := copyOfExample(t)
	card := "---\nsize: XXL\n---\nno title\n"
	if err := os.WriteFile(filepath.Join(root, "backlog", "kaputt.md"), []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func isFile(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

func TestLint(t *testing.T) {
	code, out, _ := kard(t, "lint", "-C", example)
	if code != ExitOK || out != "5 cards, 0 errors, 0 warnings\n" {
		t.Errorf("code %d, out %q", code, out)
	}

	code, out, _ = kard(t, "lint", "-C", brokenBoard(t))
	if code != ExitViolation {
		t.Errorf("code %d, want %d", code, ExitViolation)
	}
	for _, want := range []string{
		"backlog/kaputt.md: E05 size: must be one of XS, S, M, L, XL",
		"backlog/kaputt.md: E07 ",
		"6 cards, 2 errors, 0 warnings",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestLintJSON(t *testing.T) {
	code, out, _ := kard(t, "lint", "--json", "-C", brokenBoard(t))
	if code != ExitViolation {
		t.Errorf("code %d", code)
	}
	var res struct {
		Cards, Errors, Warnings int
		Problems                []struct{ Code, Severity, Path, Message string }
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if res.Cards != 6 || res.Errors != 2 || len(res.Problems) != 2 || res.Problems[0].Severity != "error" {
		t.Errorf("result = %+v", res)
	}

	_, out, _ = kard(t, "lint", "--json", "-C", example)
	if !strings.Contains(out, `"problems": []`) {
		t.Errorf("a clean board must print an empty list, not null:\n%s", out)
	}
}

func TestLintBrokenConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "board.yaml"), []byte("kardboard: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := kard(t, "lint", "-C", root)
	if code != ExitViolation || !strings.Contains(out, "board.yaml: E01 ") {
		t.Errorf("code %d, out %q", code, out)
	}
	// A directory without board.yaml is the same violation.
	code, out, _ = kard(t, "lint", "-C", t.TempDir())
	if code != ExitViolation || !strings.Contains(out, "board.yaml: E01 file is missing") {
		t.Errorf("code %d, out %q", code, out)
	}
	// Other commands cannot work without a valid board.yaml.
	code, _, errOut := kard(t, "list", "-C", root)
	if code != ExitViolation || !strings.Contains(errOut, "board.yaml: E01 ") {
		t.Errorf("list: code %d, stderr %q", code, errOut)
	}
}

func TestList(t *testing.T) {
	code, out, _ := kard(t, "list", "-C", example)
	want := `Planning (1)
    offline-mode              Offline mode
Backlog (2)
    export-as-pdf             Export as PDF  @sly
    print-templates           Print templates
In progress (1)
    login-fails-with-umlauts  Login fails with umlauts in the password  @arnold
Done (1)
    set-up-project            Set up the project
`
	if code != ExitOK || out != want {
		t.Errorf("code %d, out:\n%s", code, out)
	}

	code, out, _ = kard(t, "list", "backlog", "-C", brokenBoard(t))
	if code != ExitOK || !strings.Contains(out, "  ! kaputt") || strings.Contains(out, "Planning") {
		t.Errorf("code %d, out:\n%s", code, out)
	}
}

func TestListMine(t *testing.T) {
	// sly's card is still in the backlog: --mine finds it in any column.
	asUser(t, "sly")
	code, out, _ := kard(t, "list", "--mine", "--json", "-C", example)
	var res struct {
		Board struct{ ID, Name string }
		Cards []struct {
			ID, Column string
			Fields     map[string]any
		}
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != ExitOK || res.Board.ID != "demo" || len(res.Cards) != 1 ||
		res.Cards[0].ID != "export-as-pdf" || res.Cards[0].Column != "backlog" || fmt.Sprint(res.Cards[0].Fields["assignee"]) != "[sly]" {
		t.Errorf("code %d, result %+v", code, res)
	}

	// A copy outside this repository, whose own Git config may name someone.
	asUser(t, "")
	code, _, errOut := kard(t, "list", "--mine", "-C", copyOfExample(t))
	if code != ExitUsage || !strings.Contains(errOut, "does not know who you are") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
}

func TestShow(t *testing.T) {
	code, out, _ := kard(t, "show", "login-fails-with-umlauts.md", "-C", example)
	for _, want := range []string{
		"path:      working_on/login-fails-with-umlauts.md",
		"column:    In progress",
		"assignee:  arnold",
		"tags:      bug",
		"# Login fails with umlauts in the password",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if code != ExitOK {
		t.Errorf("code %d", code)
	}

	_, out, _ = kard(t, "show", "export-as-pdf", "--json", "-C", example)
	var res struct {
		ID, Title, Body string
		Fields          map[string]any
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if res.ID != "export-as-pdf" || res.Fields["size"] != "M" || !strings.Contains(res.Body, "## Done when") {
		t.Errorf("result = %+v", res)
	}
}

func TestUsageErrors(t *testing.T) {
	tests := [][]string{
		{"nope"},
		{"show", "-C", example},
		{"show", "gibt-es-nicht", "-C", example},
		{"list", "nope", "-C", example},
		{"lint", "extra", "-C", example},
		{"move", "print-templates", "-C", example},
		{"assign", "print-templates", "-C", example},
		{"set", "print-templates", "-C", example},
	}
	for _, args := range tests {
		code, _, errOut := kard(t, args...)
		if code != ExitUsage || !strings.HasPrefix(errOut, "kard: ") {
			t.Errorf("kard %v: code %d, stderr %q", args, code, errOut)
		}
	}
}

func TestNewCommand(t *testing.T) {
	asUser(t, "")
	root := copyOfExample(t)

	code, out, _ := kard(t, "new", "Dunkles Design", "-C", root)
	if code != ExitOK || out != "planning/dunkles-design.md\n" {
		t.Errorf("code %d, out %q", code, out)
	}
	text := readFile(t, root, "planning/dunkles-design.md")
	if !strings.HasPrefix(text, "---\ncreated: 20") || !strings.HasSuffix(text, "---\n\n# Dunkles Design\n") {
		t.Errorf("file = %q", text)
	}

	// The column requires a size: refused, then accepted with --set.
	code, _, errOut := kard(t, "new", "Export als CSV", "-c", "backlog", "-C", root)
	if code != ExitViolation || !strings.Contains(errOut, "requires size") || !strings.Contains(errOut, "--force") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
	code, out, _ = kard(t, "new", "Export als CSV", "-c", "backlog",
		"--set", "size=S", "--set", "tags=feature,ui", "--set", "assignee=sly", "--json", "-C", root)
	var res struct {
		ID, Path string
		Fields   map[string]any
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != ExitOK || res.Path != "backlog/export-als-csv.md" || res.Fields["size"] != "S" || fmt.Sprint(res.Fields["assignee"]) != "[sly]" {
		t.Errorf("code %d, result %+v", code, res)
	}

	code, out, _ = kard(t, "new", "Mit Namen", "-c", "working_on", "--id", "eigener-name", "--set", "size=S", "--set", "assignee=sly", "-C", root)
	if code != ExitOK || out != "working_on/eigener-name.md\n" {
		t.Errorf("code %d, out %q", code, out)
	}

	if code, out, _ = kard(t, "lint", "-C", root); code != ExitOK {
		t.Errorf("board is invalid after kard new:\n%s", out)
	}
}

func TestChangeCommands(t *testing.T) {
	asUser(t, "sly")
	root := copyOfExample(t)

	steps := []struct {
		args     []string
		code     int
		out      string // expected stdout, or part of stderr on failure
		wantFile string
	}{
		// take assigns and moves in one step
		{[]string{"take", "print-templates"}, ExitOK, "print-templates: backlog -> working_on, assignee = sly\n", "working_on/print-templates.md"},
		{[]string{"take", "print-templates"}, ExitOK, "print-templates: nothing to change\n", "working_on/print-templates.md"},
		// planning cards have no size, which working_on requires
		{[]string{"take", "offline-mode"}, ExitViolation, "would lack size", "planning/offline-mode.md"},
		{[]string{"take", "offline-mode", "--force"}, ExitOK, "offline-mode: planning -> working_on, assignee = sly\n", "working_on/offline-mode.md"},
		{[]string{"set", "offline-mode", "size=S", "tags=feature,ui"}, ExitOK, "offline-mode: size = S, tags = feature, ui\n", ""},
		// sly's own card from the backlog: only the column changes
		{[]string{"take", "export-as-pdf"}, ExitOK, "export-as-pdf: backlog -> working_on\n", "working_on/export-as-pdf.md"},
		// arnold's card is protected
		{[]string{"done", "login-fails-with-umlauts"}, ExitViolation, "is assigned to arnold", "working_on/login-fails-with-umlauts.md"},
		{[]string{"take", "login-fails-with-umlauts"}, ExitViolation, "is assigned to arnold", ""},
		{[]string{"take", "login-fails-with-umlauts", "--force"}, ExitOK, "login-fails-with-umlauts: assignee = sly\n", ""},
		{[]string{"done", "login-fails-with-umlauts"}, ExitOK, "login-fails-with-umlauts: working_on -> done\n", "done/login-fails-with-umlauts.md"},
		// assigning does not move; give is the same command
		{[]string{"assign", "print-templates", "arnold"}, ExitOK, "print-templates: assignee = arnold\n", "working_on/print-templates.md"},
		{[]string{"give", "print-templates", "sly"}, ExitViolation, "is assigned to arnold", ""},
		{[]string{"give", "export-as-pdf", "arnold"}, ExitOK, "export-as-pdf: assignee = arnold\n", "working_on/export-as-pdf.md"},
		// a card in progress needs someone
		{[]string{"unassign", "offline-mode"}, ExitViolation, "would lack assignee", ""},
		{[]string{"move", "offline-mode", "backlog"}, ExitOK, "offline-mode: working_on -> backlog\n", "backlog/offline-mode.md"},
		{[]string{"unassign", "offline-mode"}, ExitOK, "offline-mode: assignee removed\n", ""},
		{[]string{"unassign", "offline-mode"}, ExitOK, "offline-mode: nothing to change\n", ""},
		{[]string{"set", "offline-mode", "tags="}, ExitOK, "offline-mode: tags removed\n", ""},
		{[]string{"set", "offline-mode", "size=XXL"}, ExitUsage, "size: must be one of", ""},
		{[]string{"move", "offline-mode", "nirgends"}, ExitUsage, `no column "nirgends"`, "backlog/offline-mode.md"},
		{[]string{"move", "gibt-es-nicht", "done"}, ExitUsage, `no card "gibt-es-nicht"`, ""},
	}
	for _, s := range steps {
		code, out, errOut := kard(t, append(s.args, "-C", root)...)
		if code != s.code {
			t.Fatalf("kard %v: code %d, want %d\nstdout: %s\nstderr: %s", s.args, code, s.code, out, errOut)
		}
		if code == ExitOK && out != s.out {
			t.Errorf("kard %v: out %q, want %q", s.args, out, s.out)
		}
		if code != ExitOK && !strings.Contains(errOut, s.out) {
			t.Errorf("kard %v: stderr %q lacks %q", s.args, errOut, s.out)
		}
		if s.wantFile != "" && !isFile(root, s.wantFile) {
			t.Errorf("kard %v: %s is missing", s.args, s.wantFile)
		}
	}

	if got, want := readFile(t, root, "backlog/offline-mode.md"), "---\nsize: S\n---\n\n# Offline mode\n"; !strings.HasPrefix(got, want) {
		t.Errorf("offline-mode = %q, want it to start with %q", got, want)
	}
	if code, out, _ := kard(t, "lint", "-C", root); code != ExitOK {
		t.Errorf("board is invalid after the changes:\n%s", out)
	}

	_, out, _ := kard(t, "move", "print-templates", "done", "--force", "--json", "-C", root)
	var res struct {
		ID, From, To, Path string
		Moved              bool
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if res.From != "working_on" || res.To != "done" || res.Path != "done/print-templates.md" || !res.Moved {
		t.Errorf("result = %+v", res)
	}
}

func TestSeveralPeople(t *testing.T) {
	asUser(t, "sly")
	root := copyOfExample(t)
	const card = "print-templates"

	steps := []struct {
		args []string
		code int
		out  string // expected stdout, or part of stderr on failure
		file string // expected header line, "" for no assignee
	}{
		{[]string{"assign", card, "arnold", "laura"}, ExitOK, "print-templates: assignee = arnold, laura\n", "assignee: [arnold, laura]"},
		// sly is not on the card: joining it is a change to someone else's card
		{[]string{"take", card, "--add"}, ExitViolation, "is assigned to arnold, laura", "assignee: [arnold, laura]"},
		{[]string{"take", card, "--add", "--force"}, ExitOK, "print-templates: backlog -> working_on, assignee = arnold, laura, sly\n", "assignee: [arnold, laura, sly]"},
		// now it is sly's card too
		{[]string{"take", card, "--add"}, ExitOK, "print-templates: nothing to change\n", "assignee: [arnold, laura, sly]"},
		{[]string{"unassign", card, "laura"}, ExitOK, "print-templates: assignee = arnold, sly\n", "assignee: [arnold, sly]"},
		{[]string{"unassign", card, "laura"}, ExitOK, "print-templates: nothing to change\n", "assignee: [arnold, sly]"},
		{[]string{"assign", card, "--add", "laura", "sly"}, ExitOK, "print-templates: assignee = arnold, sly, laura\n", "assignee: [arnold, sly, laura]"},
		{[]string{"unassign", card, "arnold", "laura"}, ExitOK, "print-templates: assignee = sly\n", "assignee: sly"},
		// in progress, the last person cannot leave
		{[]string{"unassign", card, "sly"}, ExitViolation, "would lack assignee", "assignee: sly"},
		// without --add, take and assign replace everyone
		{[]string{"assign", card, "arnold,laura"}, ExitOK, "print-templates: assignee = arnold, laura\n", "assignee: [arnold, laura]"},
		{[]string{"take", card, "--force"}, ExitOK, "print-templates: assignee = sly\n", "assignee: sly"},
		{[]string{"set", card, "assignee=sly,arnold"}, ExitOK, "print-templates: assignee = sly, arnold\n", "assignee: [sly, arnold]"},
		{[]string{"assign", card, "Sly Smith"}, ExitUsage, "is neither a short name", "assignee: [sly, arnold]"},
		{[]string{"assign", card, ""}, ExitUsage, "no person named", "assignee: [sly, arnold]"},
	}
	for _, s := range steps {
		code, out, errOut := kard(t, append(s.args, "-C", root)...)
		if code != s.code {
			t.Fatalf("kard %v: code %d, want %d\nstdout: %s\nstderr: %s", s.args, code, s.code, out, errOut)
		}
		if code == ExitOK && out != s.out {
			t.Errorf("kard %v: out %q, want %q", s.args, out, s.out)
		}
		if code != ExitOK && !strings.Contains(errOut, s.out) {
			t.Errorf("kard %v: stderr %q lacks %q", s.args, errOut, s.out)
		}
		path := "backlog/print-templates.md"
		if !isFile(root, path) {
			path = "working_on/print-templates.md"
		}
		if text := readFile(t, root, path); !strings.Contains(text, "\n"+s.file+"\n") {
			t.Errorf("kard %v: file lacks %q:\n%s", s.args, s.file, text)
		}
	}

	// Both people see the card as theirs.
	_, out, _ := kard(t, "list", "working_on", "-C", root)
	if !strings.Contains(out, "Print templates  @sly @arnold\n") {
		t.Errorf("list:\n%s", out)
	}
	for _, name := range []string{"sly", "arnold"} {
		asUser(t, name)
		if _, out, _ := kard(t, "list", "working_on", "--mine", "-C", root); !strings.Contains(out, "print-templates") {
			t.Errorf("--mine for %s:\n%s", name, out)
		}
	}
	asUser(t, "laura")
	if _, out, _ := kard(t, "list", "working_on", "--mine", "-C", root); strings.Contains(out, "print-templates") {
		t.Errorf("--mine for laura:\n%s", out)
	}
	if code, out, _ := kard(t, "lint", "-C", root); code != ExitOK {
		t.Errorf("board is invalid after the changes:\n%s", out)
	}
}

func TestWhoYouAreMattersOnlyForAssignedCards(t *testing.T) {
	asUser(t, "")
	root := copyOfExample(t)

	// Nobody's cards can be moved and assigned without any setup.
	if code, _, errOut := kard(t, "move", "print-templates", "done", "-C", root); code != ExitOK {
		t.Errorf("move: code %d, stderr %q", code, errOut)
	}
	if code, _, errOut := kard(t, "assign", "offline-mode", "laura", "-C", root); code != ExitOK {
		t.Errorf("assign: code %d, stderr %q", code, errOut)
	}
	// An assigned card and `take` need to know who is asking.
	code, _, errOut := kard(t, "done", "login-fails-with-umlauts", "-C", root)
	if code != ExitUsage || !strings.Contains(errOut, "must know who you are") || !strings.Contains(errOut, "does not know who you are") {
		t.Errorf("done: code %d, stderr %q", code, errOut)
	}
	code, _, errOut = kard(t, "take", "set-up-project", "-C", root)
	if code != ExitUsage || !strings.Contains(errOut, "does not know who you are") {
		t.Errorf("take: code %d, stderr %q", code, errOut)
	}
}

func TestBoardWithoutSpecialColumns(t *testing.T) {
	asUser(t, "sly")
	root := t.TempDir()
	files := map[string]string{
		"board.yaml":    "kardboard: 1\nid: plain\ncolumns:\n  - id: todo\n",
		"todo/karte.md": "# Karte\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Without a doing column, taking a card only assigns it.
	code, out, _ := kard(t, "take", "karte", "-C", root)
	if code != ExitOK || out != "karte: assignee = sly\n" {
		t.Errorf("take: code %d, out %q", code, out)
	}
	if got, want := readFile(t, root, "todo/karte.md"), "---\nassignee: sly\n---\n\n# Karte\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	code, _, errOut := kard(t, "done", "karte", "-C", root)
	if code != ExitUsage || !strings.Contains(errOut, "has no column with done: true") {
		t.Errorf("done: code %d, stderr %q", code, errOut)
	}
}

// machine gives the test its own home directory and settings, and returns
// the path of the settings file.
func machine(t *testing.T) (home, configFile string) {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("KARDBOARD_USER", "")
	return home, filepath.Join(home, ".config", "kardboard", "config.yaml")
}

// makeBoard creates a small valid board and returns its directory.
func makeBoard(t *testing.T, dir, id string) string {
	t.Helper()
	files := map[string]string{
		"board.yaml":   "kardboard: 1\nid: " + id + "\ncolumns:\n  - id: todo\n  - id: done\n    done: true\n",
		"todo/eins.md": "# Eins\n",
	}
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAddRemoveAndBoards(t *testing.T) {
	home, configFile := machine(t)
	alpha := makeBoard(t, filepath.Join(home, "repos", "alpha", "board"), "alpha")
	beta := makeBoard(t, filepath.Join(home, "repos", "beta"), "beta")
	twin := makeBoard(t, filepath.Join(home, "elsewhere", "twin"), "alpha")

	code, out, _ := kard(t, "boards")
	if code != ExitOK || !strings.Contains(out, "No boards yet") {
		t.Errorf("empty machine: code %d, out %q", code, out)
	}

	// A directory inside the board works as well as the board itself.
	code, out, _ = kard(t, "add", filepath.Join(alpha, "todo"))
	if code != ExitOK || out != "alpha: added (~/repos/alpha/board)\n" {
		t.Errorf("add: code %d, out %q", code, out)
	}
	if got, want := readFile(t, filepath.Dir(configFile), "config.yaml"), "boards:\n  - ~/repos/alpha/board\n"; got != want {
		t.Errorf("settings = %q, want %q", got, want)
	}
	code, out, _ = kard(t, "add", alpha)
	if code != ExitOK || !strings.Contains(out, "already added") {
		t.Errorf("add twice: code %d, out %q", code, out)
	}
	if code, out, _ = kard(t, "add", beta); code != ExitOK {
		t.Errorf("add beta: code %d, out %q", code, out)
	}

	// Two boards with the same id cannot both be known.
	code, _, errOut := kard(t, "add", twin)
	if code != ExitUsage || !strings.Contains(errOut, `id "alpha" is already used by the board in ~/repos/alpha/board`) {
		t.Errorf("add twin: code %d, stderr %q", code, errOut)
	}
	if code, _, errOut = kard(t, "add", filepath.Join(home, "repos")); code != ExitUsage || !strings.Contains(errOut, "no board found") {
		t.Errorf("add without board: code %d, stderr %q", code, errOut)
	}

	code, out, _ = kard(t, "boards")
	want := "ID     CARDS  SOURCE  PATH\n" +
		"alpha      1  added   ~/repos/alpha/board\n" +
		"beta       1  added   ~/repos/beta\n"
	if code != ExitOK || out != want {
		t.Errorf("boards: code %d, out:\n%s", code, out)
	}

	// -b reaches a known board from anywhere.
	code, out, _ = kard(t, "list", "-b", "beta")
	if code != ExitOK || !strings.Contains(out, "eins") {
		t.Errorf("list -b: code %d, out %q", code, out)
	}
	if code, _, errOut = kard(t, "list", "-b", "nope"); code != ExitUsage || !strings.Contains(errOut, `no board "nope"`) {
		t.Errorf("list -b nope: code %d, stderr %q", code, errOut)
	}
	if code, _, _ = kard(t, "list", "-b", "beta", "-C", alpha); code != ExitUsage {
		t.Errorf("-b together with -C: code %d", code)
	}

	// Removing forgets the board and leaves its files alone.
	code, out, _ = kard(t, "remove", "alpha")
	if code != ExitOK || !strings.Contains(out, "removed ~/repos/alpha/board") || !isFile(alpha, "todo/eins.md") {
		t.Errorf("remove by id: code %d, out %q", code, out)
	}
	if code, _, _ = kard(t, "remove", beta); code != ExitOK {
		t.Errorf("remove by directory: code %d", code)
	}
	if code, _, errOut = kard(t, "remove", "alpha"); code != ExitUsage || !strings.Contains(errOut, "no added board") {
		t.Errorf("remove again: code %d, stderr %q", code, errOut)
	}
	if got := readFile(t, filepath.Dir(configFile), "config.yaml"); got != "boards: []\n" {
		t.Errorf("settings = %q", got)
	}
}

func TestScan(t *testing.T) {
	home, _ := machine(t)
	makeBoard(t, filepath.Join(home, "repos", "alpha", "board"), "alpha")
	makeBoard(t, filepath.Join(home, "repos", "beta"), "beta")
	broken := filepath.Join(home, "repos", "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "board.yaml"), []byte("kardboard: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, _ := kard(t, "scan", filepath.Join(home, "repos"))
	want := "scanning ~/repos: 3 boards found\n  ~/repos/alpha/board\n  ~/repos/beta\n  ~/repos/broken\n"
	if code != ExitOK || out != want {
		t.Errorf("scan: code %d, out:\n%s", code, out)
	}
	if code, out, _ = kard(t, "scan", filepath.Join(home, "repos")); code != ExitOK || !strings.HasPrefix(out, "already scanning") {
		t.Errorf("scan twice: code %d, out %q", code, out)
	}
	if code, _, errOut := kard(t, "scan", filepath.Join(home, "nope")); code != ExitUsage || !strings.Contains(errOut, "not a directory") {
		t.Errorf("scan nothing: code %d, stderr %q", code, errOut)
	}

	code, out, _ = kard(t, "boards")
	for _, line := range []string{
		"alpha      1  scan    ~/repos/alpha/board\n",
		"beta       1  scan    ~/repos/beta\n",
		"-          0  scan    ~/repos/broken  ! board.yaml is missing or invalid",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("boards lacks %q:\n%s", line, out)
		}
	}
	if code != ExitOK {
		t.Errorf("boards: code %d", code)
	}

	_, out, _ = kard(t, "boards", "--json")
	var rows []struct {
		ID, Path, Source, Error string
		Cards, Errors           int
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(rows) != 3 || rows[0].ID != "alpha" || rows[0].Source != "scan" || rows[2].Error == "" || rows[2].Errors == 0 {
		t.Errorf("rows = %+v", rows)
	}

	// A board found by a scan cannot be removed by itself.
	code, _, errOut := kard(t, "remove", "alpha")
	if code != ExitUsage || !strings.Contains(errOut, "found through a directory under `scan`") {
		t.Errorf("remove a scanned board: code %d, stderr %q", code, errOut)
	}
	if code, out, _ = kard(t, "scan", "--remove", filepath.Join(home, "repos")); code != ExitOK || out != "no longer scanning ~/repos\n" {
		t.Errorf("scan --remove: code %d, out %q", code, out)
	}
	if code, _, _ = kard(t, "scan", "--remove", filepath.Join(home, "repos")); code != ExitUsage {
		t.Errorf("scan --remove twice: code %d", code)
	}
	if _, out, _ = kard(t, "boards"); !strings.Contains(out, "No boards yet") {
		t.Errorf("boards after --remove:\n%s", out)
	}
}

func TestAddRefusesABrokenBoard(t *testing.T) {
	home, configFile := machine(t)
	dir := filepath.Join(home, "broken")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "board.yaml"), []byte("kardboard: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := kard(t, "add", dir)
	if code != ExitViolation || !strings.Contains(errOut, "board.yaml: E01 ") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
	if isFile(filepath.Dir(configFile), "config.yaml") {
		t.Error("the settings were written")
	}
}

// ── sync: commit and push ───────────────────────────────────────────────────

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// syncRepo is a repository with the example board in the given sync mode.
func syncRepo(t *testing.T, mode string) string {
	t.Helper()
	root := copyOfExample(t)
	yaml := strings.Replace(readFile(t, root, "board.yaml"), "sync: none", "sync: "+mode, 1)
	if err := os.WriteFile(filepath.Join(root, "board.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.name", "arnold")
	gitIn(t, root, "config", "user.email", "arnold@example.com")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "start")
	return root
}

func TestChangeOnASideBranch(t *testing.T) {
	asUser(t, "arnold")
	root := syncRepo(t, "none")

	code, out, _ := kard(t, "move", "print-templates", "planning", "-C", root)
	if code != ExitOK || strings.Contains(out, "branch") {
		t.Fatalf("on main: code %d, out %q", code, out)
	}
	gitIn(t, root, "switch", "-q", "-c", "feature/pdf")
	code, out, _ = kard(t, "move", "print-templates", "backlog", "-C", root)
	if code != ExitOK || !strings.HasSuffix(out, "  (on branch feature/pdf: main gets this when it is merged)\n") {
		t.Fatalf("on a branch: code %d, out %q", code, out)
	}
	code, out, _ = kard(t, "move", "print-templates", "planning", "-C", root, "--json")
	if code != ExitOK || !strings.Contains(out, `"branch": {`) || !strings.Contains(out, `"main": "main"`) {
		t.Errorf("json: code %d, out %q", code, out)
	}
}

func TestCommitBoard(t *testing.T) {
	asUser(t, "arnold")
	root := syncRepo(t, "commit")

	code, out, errOut := kard(t, "new", "Dunkles Design", "-c", "planning", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "planning/dunkles-design.md  (commit ") {
		t.Fatalf("new: code %d, out %q, err %q", code, out, errOut)
	}
	code, out, errOut = kard(t, "assign", "dunkles-design", "sly", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "dunkles-design: assignee = sly  (commit ") {
		t.Fatalf("assign: code %d, out %q, err %q", code, out, errOut)
	}
	code, out, _ = kard(t, "assign", "dunkles-design", "sly", "-C", root, "--force")
	if code != ExitOK || out != "dunkles-design: nothing to change\n" {
		t.Errorf("assign again: code %d, out %q", code, out)
	}
	if got := gitIn(t, root, "log", "--format=%s"); got != "assign dunkles-design: sly\nnew dunkles-design\nstart" {
		t.Errorf("log = %q", got)
	}

	code, _, errOut = kard(t, "sync", "-C", root)
	if code != ExitUsage || !strings.Contains(errOut, "sync: commit") {
		t.Errorf("sync: code %d, err %q", code, errOut)
	}
}

func TestPushBoard(t *testing.T) {
	asUser(t, "arnold")
	seed := syncRepo(t, "push")
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitIn(t, seed, "clone", "-q", "--bare", seed, remote)
	clone := func(name string) string {
		dir := filepath.Join(t.TempDir(), name)
		gitIn(t, seed, "clone", "-q", remote, dir)
		gitIn(t, dir, "config", "user.name", name)
		gitIn(t, dir, "config", "user.email", name+"@example.com")
		return dir
	}
	arnold, sly := clone("arnold"), clone("sly")

	code, out, errOut := kard(t, "take", "print-templates", "-C", arnold, "--force")
	if code != ExitOK || !strings.Contains(out, "backlog -> working_on") || !strings.HasSuffix(out, ", pushed)\n") {
		t.Fatalf("take: code %d, out %q, err %q", code, out, errOut)
	}

	// Sly's command pulls first, so it sees that the card is Arnold's.
	t.Setenv("KARDBOARD_USER", "sly")
	code, _, errOut = kard(t, "done", "print-templates", "-C", sly)
	if code != ExitViolation || !strings.Contains(errOut, "assigned to arnold") {
		t.Errorf("done: code %d, err %q", code, errOut)
	}
	if !isFile(sly, "working_on/print-templates.md") {
		t.Error("sly did not get arnold's change")
	}

	// A commit made by hand goes out with kard sync.
	gitIn(t, sly, "commit", "-q", "--allow-empty", "-m", "by hand")
	code, out, errOut = kard(t, "sync", "-C", sly)
	if code != ExitOK || out != "demo: 0 commits pulled, 1 commit pushed\n" {
		t.Errorf("sync: code %d, out %q, err %q", code, out, errOut)
	}
	if code, out, _ = kard(t, "sync", "-C", sly); code != ExitOK || out != "demo: up to date\n" {
		t.Errorf("sync again: code %d, out %q", code, out)
	}

	// Without the remote the change stays here, and kard says so.
	gitIn(t, sly, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	code, _, errOut = kard(t, "move", "offline-mode", "backlog", "-C", sly, "--force")
	if code != ExitUsage || !strings.Contains(errOut, "nothing was changed") || !isFile(sly, "planning/offline-mode.md") {
		t.Errorf("move: code %d, err %q", code, errOut)
	}
}

// ── kard init ─────────────────────────────────────────────────────────────────

func put(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# "+rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInitInAnEmptyDirectory(t *testing.T) {
	home, configFile := machine(t)
	root := filepath.Join(home, "Mein Projekt", "board") // does not exist yet

	code, out, errOut := kard(t, "init", root)
	if code != ExitOK || errOut != "" {
		t.Fatalf("code %d, out %q, err %q", code, out, errOut)
	}
	want := "kardboard: 1\nid: mein-projekt\nsync: none\n\ncolumns:\n  - id: backlog\n  - id: working_on\n    doing: true\n  - id: done\n    done: true\n"
	if got := readFile(t, root, "board.yaml"); got != want {
		t.Errorf("board.yaml = %q", got)
	}
	for _, line := range []string{"mein-projekt: board in ~/Mein Projekt/board", "columns created: backlog, working_on, done", "added to this machine"} {
		if !strings.Contains(out, line) {
			t.Errorf("out %q lacks %q", out, line)
		}
	}
	if cfg := readFile(t, filepath.Dir(configFile), "config.yaml"); !strings.Contains(cfg, "~/Mein Projekt/board") {
		t.Errorf("settings = %q, want the board in them", cfg)
	}

	// The new board works right away.
	if code, out, _ := kard(t, "lint", "-C", root); code != ExitOK || out != "0 cards, 0 errors, 0 warnings\n" {
		t.Errorf("lint: code %d, out %q", code, out)
	}
	if code, out, errOut := kard(t, "new", "Erste Karte", "-b", "mein-projekt"); code != ExitOK || out != "backlog/erste-karte.md\n" {
		t.Errorf("new: code %d, out %q, err %q", code, out, errOut)
	}

	// A board is never written over.
	code, _, errOut = kard(t, "init", root, "--id", "anders")
	if code != ExitUsage || !strings.Contains(errOut, "is a board already") || readFile(t, root, "board.yaml") != want {
		t.Errorf("second init: code %d, err %q", code, errOut)
	}
}

func TestInitAdoptsDirectoriesWithCards(t *testing.T) {
	home, _ := machine(t)
	root := filepath.Join(home, "zettel")
	put(t, root,
		"done/a.md", "backlog/b.md", "backlog/c.md", "planning/d.md",
		"working_on/sly/e.md",      // a directory per person still makes a column
		"reference/notes.md",       // unknown name: goes in the middle
		"bin/tool.sh", "README.md", // no cards: not columns
		".github/x.md", "Mit Leerzeichen/y.md", // not usable as column ids
	)
	code, out, errOut := kard(t, "init", "-C", root, "--name", "Zettel: alle", "--no-add")
	if code != ExitOK || errOut != "" {
		t.Fatalf("code %d, out %q, err %q", code, out, errOut)
	}
	want := "kardboard: 1\nid: zettel\nname: 'Zettel: alle'\nsync: none\n\ncolumns:\n  - id: planning\n  - id: backlog\n  - id: reference\n  - id: working_on\n    doing: true\n  - id: done\n    done: true\n"
	if got := readFile(t, root, "board.yaml"); got != want {
		t.Errorf("board.yaml = %q", got)
	}
	if !strings.Contains(out, "columns found: planning, backlog, reference, working_on, done") || !strings.Contains(out, "5 cards") || strings.Contains(out, "added") {
		t.Errorf("out = %q", out)
	}
	// Nothing was moved or changed.
	if readFile(t, root, "working_on/sly/e.md") != "# working_on/sly/e.md\n" || isFile(root, "bin/board.yaml") {
		t.Error("init touched the files")
	}
	if code, out, _ := kard(t, "boards"); code != ExitOK || strings.Contains(out, "zettel") {
		t.Errorf("boards: code %d, out %q, want the board not added", code, out)
	}
}

func TestInitOptions(t *testing.T) {
	home, _ := machine(t)
	root := filepath.Join(home, "x")

	// A wrong value leaves nothing behind.
	for _, args := range [][]string{{"--sync", "always"}, {"--id", "Nicht Gut"}, {"--columns", "a,a"}, {"--columns", "Spalte Eins"}} {
		code, _, errOut := kard(t, append([]string{"init", root}, args...)...)
		if _, err := os.Stat(root); code != ExitUsage || errOut == "" || err == nil {
			t.Errorf("%v: code %d, err %q", args, code, errOut)
		}
	}

	code, out, errOut := kard(t, "init", root, "--id", "team", "--sync", "push", "--columns", "todo, doing ,closed", "--json")
	if code != ExitOK {
		t.Fatalf("code %d, out %q, err %q", code, out, errOut)
	}
	var res struct {
		ID      string
		Columns []string
		Adopted bool
		Added   bool
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.ID != "team" || len(res.Columns) != 3 || res.Adopted || !res.Added {
		t.Errorf("json %s, err %v", out, err)
	}
	want := "kardboard: 1\nid: team\nsync: push\n\ncolumns:\n  - id: todo\n  - id: doing\n    doing: true\n  - id: closed\n    done: true\n"
	if got := readFile(t, root, "board.yaml"); got != want {
		t.Errorf("board.yaml = %q", got)
	}

	// A second board with the same id is created, but not added.
	other := filepath.Join(home, "y")
	code, out, errOut = kard(t, "init", other, "--id", "team")
	if code != ExitOK || !isFile(other, "board.yaml") || !strings.Contains(errOut, "already used") || strings.Contains(out, "added") {
		t.Errorf("same id: code %d, out %q, err %q", code, out, errOut)
	}
}

// editorWrites makes kard edit see an editor that replaces the card with text.
// It is `cp`, so the test needs no shell script.
func editorWrites(t *testing.T, text string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "edited.md")
	if err := os.WriteFile(src, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "cp "+src)
}

func TestEditCommand(t *testing.T) {
	asUser(t, "arnold")
	root := syncRepo(t, "commit")
	before := readFile(t, root, "backlog/print-templates.md")

	// The editor closes without a change: nothing happens.
	t.Setenv("VISUAL", "true")
	code, out, errOut := kard(t, "edit", "print-templates", "-C", root)
	if code != ExitOK || out != "print-templates: nothing to change\n" {
		t.Fatalf("unchanged: code %d, out %q, err %q", code, out, errOut)
	}

	// Text and a field changed in the editor: one commit "edit".
	edited := strings.Replace(before, "size: S", "size: M", 1) + "\nDazu ein Satz.\n"
	editorWrites(t, edited)
	code, out, errOut = kard(t, "edit", "print-templates", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "print-templates: size = M, text changed  (commit ") {
		t.Fatalf("edit: code %d, out %q, err %q", code, out, errOut)
	}
	if got := readFile(t, root, "backlog/print-templates.md"); got != edited {
		t.Errorf("file = %q", got)
	}
	if got := gitIn(t, root, "log", "-1", "--format=%s"); got != "edit print-templates" {
		t.Errorf("commit = %q", got)
	}

	// A card the editor breaks is not saved, and the text is kept.
	editorWrites(t, "---\nsize: [\n---\n\n# Print templates\n")
	code, _, errOut = kard(t, "edit", "print-templates", "-C", root)
	if code == ExitOK || !strings.Contains(errOut, "would have errors") || !strings.Contains(errOut, "your text is in ") {
		t.Errorf("broken: code %d, err %q", code, errOut)
	}
	if readFile(t, root, "backlog/print-templates.md") != edited {
		t.Error("broken text was saved")
	}

	// A card with changes that are not committed is refused before the
	// editor opens.
	if err := os.WriteFile(filepath.Join(root, "backlog", "print-templates.md"), []byte(edited+"x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", "false")
	code, _, errOut = kard(t, "edit", "print-templates", "-C", root)
	if code == ExitOK || !strings.Contains(errOut, "not committed") {
		t.Errorf("dirty: code %d, err %q", code, errOut)
	}
}

func TestRenameCommand(t *testing.T) {
	asUser(t, "arnold")
	root := syncRepo(t, "commit")

	code, out, errOut := kard(t, "rename", "print-templates", "pdf-templates", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "backlog/print-templates.md -> backlog/pdf-templates.md  (commit ") ||
		!strings.Contains(out, "references updated in backlog/export-as-pdf.md") {
		t.Fatalf("rename: code %d, out %q, err %q", code, out, errOut)
	}
	if isFile(root, "backlog/print-templates.md") || !isFile(root, "backlog/pdf-templates.md") {
		t.Error("file was not renamed")
	}
	if got := readFile(t, root, "backlog/export-as-pdf.md"); !strings.Contains(got, "depends_on: [pdf-templates]") {
		t.Errorf("reference not updated:\n%s", got)
	}
	// One commit with the card and the reference, nothing left over.
	if got := gitIn(t, root, "log", "-1", "--format=%s"); got != "rename print-templates -> pdf-templates" {
		t.Errorf("commit = %q", got)
	}
	if got := gitIn(t, root, "show", "--name-only", "--no-renames", "--format=", "HEAD"); got != "backlog/export-as-pdf.md\nbacklog/pdf-templates.md\nbacklog/print-templates.md" {
		t.Errorf("files in the commit = %q", got)
	}
	if got := gitIn(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("left over: %q", got)
	}
	code, _, _ = kard(t, "lint", "-C", root)
	if code != ExitOK {
		t.Error("board is not valid after the rename")
	}

	// A referring card with changes that are not committed stops it.
	path := filepath.Join(root, "backlog", "export-as-pdf.md")
	if err := os.WriteFile(path, []byte(readFile(t, root, "backlog/export-as-pdf.md")+"x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = kard(t, "rename", "pdf-templates", "print", "-C", root)
	if code == ExitOK || !strings.Contains(errOut, "not committed") || !isFile(root, "backlog/pdf-templates.md") {
		t.Errorf("dirty reference: code %d, err %q", code, errOut)
	}
}

func TestSchemaCommand(t *testing.T) {
	code, out, errOut := kard(t, "schema", "working_on", "-C", example)
	var schema struct {
		Title    string         `json:"title"`
		Required []string       `json:"required"`
		Props    map[string]any `json:"properties"`
	}
	if code != ExitOK || json.Unmarshal([]byte(out), &schema) != nil {
		t.Fatalf("schema: code %d, err %q, out %q", code, errOut, out)
	}
	if schema.Title != "Card header, board demo, column working_on" || strings.Join(schema.Required, ",") != "assignee,size" || schema.Props["due"] == nil {
		t.Errorf("schema = %+v", schema)
	}
	if code, _, errOut = kard(t, "schema", "nope", "-C", example); code != ExitUsage || !strings.Contains(errOut, "no column") {
		t.Errorf("unknown column: code %d, err %q", code, errOut)
	}
}

func TestAllBoards(t *testing.T) {
	home, _ := machine(t)
	t.Chdir(home) // no board here, and no repository with one
	t.Setenv("KARDBOARD_USER", "arnold")

	if code, _, errOut := kard(t, "lint", "--all"); code != ExitUsage || !strings.Contains(errOut, "no boards known") {
		t.Errorf("no boards: code %d, err %q", code, errOut)
	}

	alpha := makeBoard(t, filepath.Join(home, "repos", "alpha"), "alpha")
	beta := makeBoard(t, filepath.Join(home, "repos", "beta"), "beta")
	for rel, text := range map[string]string{"todo/zwei.md": "---\nassignee: arnold\n---\n\n# Zwei\n", "done/kaputt.md": "no title\n"} {
		path := filepath.Join(beta, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{alpha, beta} {
		if code, _, errOut := kard(t, "add", dir); code != ExitOK {
			t.Fatalf("add %s: %s", dir, errOut)
		}
	}

	code, out, _ := kard(t, "lint", "--all")
	want := "alpha (~/repos/alpha): 1 card, 0 errors, 0 warnings\n\n" +
		"beta (~/repos/beta): 3 cards, 1 error, 0 warnings\n" +
		"  done/kaputt.md: E07 first line must be the title `# ...`, found \"no title\"\n\n" +
		"2 boards, 4 cards, 1 error, 0 warnings\n"
	if code != ExitViolation || out != want {
		t.Errorf("lint --all: code %d, out\n%s\nwant\n%s", code, out, want)
	}
	code, out, _ = kard(t, "lint", "--all", "--json")
	var results []lintResult
	if code != ExitViolation || json.Unmarshal([]byte(out), &results) != nil || len(results) != 2 || results[1].Errors != 1 {
		t.Errorf("lint --all --json: code %d, out %s", code, out)
	}

	code, out, _ = kard(t, "list", "--all", "--mine")
	want = "== alpha (~/repos/alpha)\ntodo (0)\ndone (0)\n\n" +
		"== beta (~/repos/beta)\ntodo (1)\n    zwei  Zwei  @arnold\ndone (0)\n"
	if code != ExitOK || out != want {
		t.Errorf("list --all --mine: code %d, out\n%s\nwant\n%s", code, out, want)
	}
	code, out, _ = kard(t, "list", "done", "--all", "--json")
	var lists []listResult
	if code != ExitOK || json.Unmarshal([]byte(out), &lists) != nil || len(lists) != 2 || len(lists[1].Cards) != 1 {
		t.Errorf("list done --all --json: code %d, out %s", code, out)
	}
	if code, _, errOut := kard(t, "list", "nope", "--all"); code != ExitUsage || !strings.Contains(errOut, `no board has a column "nope"`) {
		t.Errorf("unknown column: code %d, err %q", code, errOut)
	}
	if code, _, errOut := kard(t, "list", "--all", "-C", alpha); code != ExitUsage || !strings.Contains(errOut, "does not go with -C") {
		t.Errorf("--all with -C: code %d, err %q", code, errOut)
	}
}

func TestLogCommand(t *testing.T) {
	asUser(t, "arnold")
	root := syncRepo(t, "commit")
	for _, args := range [][]string{
		{"move", "print-templates", "working_on", "--force"},
		{"rename", "print-templates", "pdf-templates"},
		{"set", "pdf-templates", "size=M"},
	} {
		if code, _, errOut := kard(t, append(args, "-C", root)...); code != ExitOK {
			t.Fatalf("%v: %s", args, errOut)
		}
	}

	// Newest first, read from what the commits contain.
	code, out, errOut := kard(t, "log", "-C", root)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var what []string
	for _, line := range lines {
		// hash, date, time, author: the rest says what happened
		fields := strings.Fields(line)
		what = append(what, strings.Join(fields[4:], " "))
	}
	if code != ExitOK || len(lines) != 4 ||
		what[0] != "edited pdf-templates" ||
		what[1] != "edited export-as-pdf; renamed print-templates -> pdf-templates" ||
		what[2] != "print-templates: backlog -> working_on" ||
		!strings.HasPrefix(what[3], "new ") || !strings.HasSuffix(what[3], "and 2 more; 1 other file") {
		t.Errorf("log: code %d, err %q, out\n%s", code, errOut, out)
	}

	// One card, followed through its move and its rename.
	code, out, _ = kard(t, "log", "pdf-templates", "-C", root)
	if n := strings.Count(out, "\n"); code != ExitOK || n != 4 {
		t.Errorf("log of a card: code %d, %d lines\n%s", code, n, out)
	}
	if code, out, _ = kard(t, "log", "-n", "1", "-C", root); code != ExitOK || strings.Count(out, "\n") != 1 {
		t.Errorf("-n 1: code %d, out %q", code, out)
	}
	code, out, _ = kard(t, "log", "--json", "-C", root)
	var log struct {
		Available bool `json:"available"`
		Entries   []struct {
			Subject string `json:"subject"`
		} `json:"entries"`
	}
	if code != ExitOK || json.Unmarshal([]byte(out), &log) != nil || !log.Available || log.Entries[0].Subject != "edit pdf-templates" {
		t.Errorf("--json: code %d, out %s", code, out)
	}

	if code, _, errOut = kard(t, "log", "nope", "-C", root); code != ExitUsage || !strings.Contains(errOut, "no card") {
		t.Errorf("unknown card: code %d, err %q", code, errOut)
	}
	if code, _, errOut = kard(t, "log", "-C", copyOfExample(t)); code != ExitUsage || !strings.Contains(errOut, "not in a Git repository") {
		t.Errorf("no repository: code %d, err %q", code, errOut)
	}
}

// kardIn runs kard with stdin.
func kardIn(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

// words splits an example line like a shell would, for the simple lines
// the examples have: quotes, a comment at the end, a pipe into kard.
func words(line string) []string {
	if i := strings.Index(line, "| kard "); i >= 0 {
		line = line[i+2:]
	}
	var out []string
	var cur strings.Builder
	quoted, has := false, false
	for _, r := range line {
		switch {
		case r == '"':
			quoted, has = !quoted, true
		case r == '#' && !quoted:
			if has {
				out = append(out, cur.String())
			}
			return out
		case r == ' ' && !quoted:
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		out = append(out, cur.String())
	}
	return out
}

func TestHelpExamplesAreValidCalls(t *testing.T) {
	n := 0
	for name := range examples {
		code, help, _ := kard(t, name, "--help")
		if code != ExitOK || !strings.Contains(help, "Examples:") {
			t.Errorf("kard %s --help: code %d, no examples", name, code)
			continue
		}
		ex := help[strings.Index(help, "Examples:"):]
		ex = ex[:strings.Index(ex, "\n\n")]
		for _, line := range strings.Split(ex, "\n")[1:] {
			args := words(strings.TrimSpace(line))
			if len(args) == 0 || args[0] != "kard" {
				t.Errorf("kard %s: example %q does not call kard", name, line)
				continue
			}
			// --help after the call: cobra checks the flags, runs nothing.
			if code, _, errOut := kard(t, append(args[1:], "--help")...); code != ExitOK {
				t.Errorf("example %q: code %d, %s", line, code, errOut)
			}
			n++
		}
	}
	for _, cmd := range []string{"edit", "new"} {
		_, help, _ := kard(t, cmd, "--help")
		ex := help[strings.Index(help, "Examples:"):]
		ex = ex[:strings.Index(ex, "\n\n")]
		for _, line := range strings.Split(ex, "\n")[1:] {
			if code, _, errOut := kard(t, append(words(strings.TrimSpace(line))[1:], "--help")...); code != ExitOK {
				t.Errorf("example %q: code %d, %s", line, code, errOut)
			}
			n++
		}
	}
	if n < 30 {
		t.Errorf("only %d examples checked", n)
	}
}

func TestAgentFriendlyCalls(t *testing.T) {
	asUser(t, "arnold")
	root := syncRepo(t, "commit")

	// A card with text, from stdin.
	code, out, errOut := kardIn(t, "Erste Zeile.\n\n- Punkt\n", "new", "Mit Text", "--text", "-", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "planning/mit-text.md  (commit ") {
		t.Fatalf("new --text: code %d, out %q, err %q", code, out, errOut)
	}
	if got := readFile(t, root, "planning/mit-text.md"); !strings.HasSuffix(got, "# Mit Text\n\nErste Zeile.\n\n- Punkt\n") {
		t.Errorf("card = %q", got)
	}

	// Read with its sum, change without an editor, one commit.
	code, out, _ = kard(t, "show", "mit-text", "--json", "-C", root)
	var shown struct {
		Sum string `json:"sum"`
	}
	if code != ExitOK || json.Unmarshal([]byte(out), &shown) != nil || shown.Sum == "" {
		t.Fatalf("show --json: code %d, out %s", code, out)
	}
	t.Setenv("VISUAL", "false") // no editor may run
	changed := strings.Replace(readFile(t, root, "planning/mit-text.md"), "Erste Zeile.", "Neue Zeile.", 1)
	code, out, errOut = kardIn(t, changed, "edit", "mit-text", "--file", "-", "--base", shown.Sum, "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "mit-text: text changed  (commit ") {
		t.Fatalf("edit --file: code %d, out %q, err %q", code, out, errOut)
	}
	if got := gitIn(t, root, "log", "-1", "--format=%s"); got != "edit mit-text" {
		t.Errorf("commit = %q", got)
	}

	// The same old sum again: the card changed since, nothing is saved, and
	// with --json the error says so as an object on stderr.
	code, out, errOut = kardIn(t, "# Anders\n", "edit", "mit-text", "--file", "-", "--base", shown.Sum, "--json", "-C", root)
	var failure errorJSON
	if code != ExitViolation || out != "" || json.Unmarshal([]byte(errOut), &failure) != nil || failure.Kind != "stale" {
		t.Errorf("stale: code %d, out %q, err %q", code, out, errOut)
	}
	if !strings.Contains(readFile(t, root, "planning/mit-text.md"), "Neue Zeile.") {
		t.Error("the card was overwritten")
	}

	// A refusal says that --force helps.
	code, _, errOut = kard(t, "done", "export-as-pdf", "--json", "-C", root)
	if code != ExitViolation || json.Unmarshal([]byte(errOut), &failure) != nil || failure.Kind != "refused" || !failure.Force {
		t.Errorf("refused: code %d, err %q", code, errOut)
	}
	failure = errorJSON{}
	code, _, errOut = kard(t, "show", "nope", "--json", "-C", root)
	if code != ExitUsage || json.Unmarshal([]byte(errOut), &failure) != nil || failure.Kind != "usage" || failure.Force {
		t.Errorf("usage: code %d, err %q", code, errOut)
	}
	if code, _, errOut = kard(t, "edit", "mit-text", "--base", "abc", "-C", root); code != ExitUsage || !strings.Contains(errOut, "--base goes with --file") {
		t.Errorf("--base alone: code %d, err %q", code, errOut)
	}
}

func TestAssignFindsPeopleByTheBeginningOfTheirName(t *testing.T) {
	asUser(t, "arnold")
	root := copyOfExample(t)

	// sly is on a card already, so "sl" means sly.
	if code, _, errOut := kard(t, "assign", "print-templates", "sl", "-C", root); code != ExitOK {
		t.Fatalf("assign sl: code %d, %s", code, errOut)
	}
	if got := readFile(t, root, "backlog/print-templates.md"); !strings.Contains(got, "assignee: sly\n") {
		t.Errorf("card = %q", got)
	}
	// A new person is taken as given; now "sl" fits two.
	if code, _, errOut := kard(t, "assign", "offline-mode", "Slim@Example.org", "-C", root); code != ExitOK {
		t.Fatalf("assign slim: code %d, %s", code, errOut)
	}
	if got := readFile(t, root, "planning/offline-mode.md"); !strings.Contains(got, "assignee: slim@example.org\n") {
		t.Errorf("addresses are written in lower case: %q", got)
	}
	code, _, errOut := kard(t, "assign", "print-templates", "sl", "--json", "-C", root)
	var res struct {
		Kind       string
		Candidates []struct{ ID string }
	}
	if err := json.Unmarshal([]byte(errOut), &res); err != nil {
		t.Fatalf("%v: %s", err, errOut)
	}
	if code != ExitUsage || res.Kind != "ambiguous" || len(res.Candidates) != 2 {
		t.Errorf("code %d, error %+v", code, res)
	}
}

func TestWhoami(t *testing.T) {
	asUser(t, "")
	root := copyOfExample(t)

	if code, _, errOut := kard(t, "whoami", "-C", root); code != ExitUsage || !strings.Contains(errOut, "does not know who you are") {
		t.Errorf("nobody: code %d, %s", code, errOut)
	}
	if code, out, errOut := kard(t, "whoami", "--set", "angelique-l", "-C", root); code != ExitOK || !strings.Contains(out, "angelique-l  (from `user` in ") {
		t.Errorf("--set: code %d, out %q, err %q", code, out, errOut)
	}
	code, out, _ := kard(t, "whoami", "--json", "-C", root)
	var id struct{ Name, Source, Where string }
	if err := json.Unmarshal([]byte(out), &id); err != nil {
		t.Fatal(err)
	}
	if code != ExitOK || id.Name != "angelique-l" || id.Source != "project" || id.Where != filepath.Join(root, ".kard", "config.yaml") {
		t.Errorf("code %d, %+v", code, id)
	}
	// The name is now the user's on this board: their card can be taken.
	if code, _, errOut := kard(t, "take", "offline-mode", "--force", "-C", root); code != ExitOK {
		t.Errorf("take: code %d, %s", code, errOut)
	}
	if code, out, _ := kard(t, "list", "--mine", "-C", root); code != ExitOK || !strings.Contains(out, "offline-mode") {
		t.Errorf("--mine: code %d\n%s", code, out)
	}
}
