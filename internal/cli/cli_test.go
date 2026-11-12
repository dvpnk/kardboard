package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var example = filepath.Join("..", "..", "examples", "board")

// kb runs the command and returns exit code, stdout and stderr.
func kb(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// asUser makes kb see the given user and no settings file. An empty name
// means: nobody has told kb who they are.
func asUser(t *testing.T, name string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KARDBOARD_USER", name)
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
	code, out, _ := kb(t, "lint", "-C", example)
	if code != ExitOK || out != "5 cards, 0 errors, 0 warnings\n" {
		t.Errorf("code %d, out %q", code, out)
	}

	code, out, _ = kb(t, "lint", "-C", brokenBoard(t))
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
	code, out, _ := kb(t, "lint", "--json", "-C", brokenBoard(t))
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

	_, out, _ = kb(t, "lint", "--json", "-C", example)
	if !strings.Contains(out, `"problems": []`) {
		t.Errorf("a clean board must print an empty list, not null:\n%s", out)
	}
}

func TestLintBrokenConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "board.yaml"), []byte("kardboard: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := kb(t, "lint", "-C", root)
	if code != ExitViolation || !strings.Contains(out, "board.yaml: E01 ") {
		t.Errorf("code %d, out %q", code, out)
	}
	// A directory without board.yaml is the same violation.
	code, out, _ = kb(t, "lint", "-C", t.TempDir())
	if code != ExitViolation || !strings.Contains(out, "board.yaml: E01 file is missing") {
		t.Errorf("code %d, out %q", code, out)
	}
	// Other commands cannot work without a valid board.yaml.
	code, _, errOut := kb(t, "list", "-C", root)
	if code != ExitViolation || !strings.Contains(errOut, "board.yaml: E01 ") {
		t.Errorf("list: code %d, stderr %q", code, errOut)
	}
}

func TestList(t *testing.T) {
	code, out, _ := kb(t, "list", "-C", example)
	want := `Planung (1)
    offline-modus              Offline-Modus
Backlog (2)
    druckvorlagen              Druckvorlagen
    export-als-pdf             Export als PDF  @sam
In Arbeit (1)
    login-fehler-bei-umlauten  Login-Fehler bei Umlauten im Passwort  @alex
Erledigt (1)
    projekt-aufsetzen          Projekt aufsetzen
`
	if code != ExitOK || out != want {
		t.Errorf("code %d, out:\n%s", code, out)
	}

	code, out, _ = kb(t, "list", "backlog", "-C", brokenBoard(t))
	if code != ExitOK || !strings.Contains(out, "  ! kaputt") || strings.Contains(out, "Planung") {
		t.Errorf("code %d, out:\n%s", code, out)
	}
}

func TestListMine(t *testing.T) {
	// sam's card is still in the backlog: --mine finds it in any column.
	asUser(t, "sam")
	code, out, _ := kb(t, "list", "--mine", "--json", "-C", example)
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
		res.Cards[0].ID != "export-als-pdf" || res.Cards[0].Column != "backlog" || fmt.Sprint(res.Cards[0].Fields["assignee"]) != "[sam]" {
		t.Errorf("code %d, result %+v", code, res)
	}

	asUser(t, "")
	code, _, errOut := kb(t, "list", "--mine", "-C", example)
	if code != ExitUsage || !strings.Contains(errOut, "no user set") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
}

func TestShow(t *testing.T) {
	code, out, _ := kb(t, "show", "login-fehler-bei-umlauten.md", "-C", example)
	for _, want := range []string{
		"path:      working_on/login-fehler-bei-umlauten.md",
		"column:    In Arbeit",
		"assignee:  alex",
		"tags:      bug",
		"# Login-Fehler bei Umlauten im Passwort",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if code != ExitOK {
		t.Errorf("code %d", code)
	}

	_, out, _ = kb(t, "show", "export-als-pdf", "--json", "-C", example)
	var res struct {
		ID, Title, Body string
		Fields          map[string]any
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if res.ID != "export-als-pdf" || res.Fields["size"] != "M" || !strings.Contains(res.Body, "## Fertig, wenn") {
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
		{"move", "druckvorlagen", "-C", example},
		{"assign", "druckvorlagen", "-C", example},
		{"set", "druckvorlagen", "-C", example},
	}
	for _, args := range tests {
		code, _, errOut := kb(t, args...)
		if code != ExitUsage || !strings.HasPrefix(errOut, "kb: ") {
			t.Errorf("kb %v: code %d, stderr %q", args, code, errOut)
		}
	}
}

func TestNewCommand(t *testing.T) {
	asUser(t, "")
	root := copyOfExample(t)

	code, out, _ := kb(t, "new", "Dunkles Design", "-C", root)
	if code != ExitOK || out != "planning/dunkles-design.md\n" {
		t.Errorf("code %d, out %q", code, out)
	}
	text := readFile(t, root, "planning/dunkles-design.md")
	if !strings.HasPrefix(text, "---\ncreated: 20") || !strings.HasSuffix(text, "---\n\n# Dunkles Design\n") {
		t.Errorf("file = %q", text)
	}

	// The column requires a size: refused, then accepted with --set.
	code, _, errOut := kb(t, "new", "Export als CSV", "-c", "backlog", "-C", root)
	if code != ExitViolation || !strings.Contains(errOut, "requires size") || !strings.Contains(errOut, "--force") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
	code, out, _ = kb(t, "new", "Export als CSV", "-c", "backlog",
		"--set", "size=S", "--set", "tags=feature,ui", "--set", "assignee=sam", "--json", "-C", root)
	var res struct {
		ID, Path string
		Fields   map[string]any
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != ExitOK || res.Path != "backlog/export-als-csv.md" || res.Fields["size"] != "S" || fmt.Sprint(res.Fields["assignee"]) != "[sam]" {
		t.Errorf("code %d, result %+v", code, res)
	}

	code, out, _ = kb(t, "new", "Mit Namen", "-c", "working_on", "--id", "eigener-name", "--set", "size=S", "--set", "assignee=sam", "-C", root)
	if code != ExitOK || out != "working_on/eigener-name.md\n" {
		t.Errorf("code %d, out %q", code, out)
	}

	if code, out, _ = kb(t, "lint", "-C", root); code != ExitOK {
		t.Errorf("board is invalid after kb new:\n%s", out)
	}
}

func TestChangeCommands(t *testing.T) {
	asUser(t, "sam")
	root := copyOfExample(t)

	steps := []struct {
		args     []string
		code     int
		out      string // expected stdout, or part of stderr on failure
		wantFile string
	}{
		// take assigns and moves in one step
		{[]string{"take", "druckvorlagen"}, ExitOK, "druckvorlagen: backlog -> working_on, assignee = sam\n", "working_on/druckvorlagen.md"},
		{[]string{"take", "druckvorlagen"}, ExitOK, "druckvorlagen: nothing to change\n", "working_on/druckvorlagen.md"},
		// planning cards have no size, which working_on requires
		{[]string{"take", "offline-modus"}, ExitViolation, "would lack size", "planning/offline-modus.md"},
		{[]string{"take", "offline-modus", "--force"}, ExitOK, "offline-modus: planning -> working_on, assignee = sam\n", "working_on/offline-modus.md"},
		{[]string{"set", "offline-modus", "size=S", "tags=feature,ui"}, ExitOK, "offline-modus: size = S, tags = feature, ui\n", ""},
		// sam's own card from the backlog: only the column changes
		{[]string{"take", "export-als-pdf"}, ExitOK, "export-als-pdf: backlog -> working_on\n", "working_on/export-als-pdf.md"},
		// alex's card is protected
		{[]string{"done", "login-fehler-bei-umlauten"}, ExitViolation, "is assigned to alex", "working_on/login-fehler-bei-umlauten.md"},
		{[]string{"take", "login-fehler-bei-umlauten"}, ExitViolation, "is assigned to alex", ""},
		{[]string{"take", "login-fehler-bei-umlauten", "--force"}, ExitOK, "login-fehler-bei-umlauten: assignee = sam\n", ""},
		{[]string{"done", "login-fehler-bei-umlauten"}, ExitOK, "login-fehler-bei-umlauten: working_on -> done\n", "done/login-fehler-bei-umlauten.md"},
		// assigning does not move; give is the same command
		{[]string{"assign", "druckvorlagen", "alex"}, ExitOK, "druckvorlagen: assignee = alex\n", "working_on/druckvorlagen.md"},
		{[]string{"give", "druckvorlagen", "sam"}, ExitViolation, "is assigned to alex", ""},
		{[]string{"give", "export-als-pdf", "alex"}, ExitOK, "export-als-pdf: assignee = alex\n", "working_on/export-als-pdf.md"},
		// a card in progress needs someone
		{[]string{"unassign", "offline-modus"}, ExitViolation, "would lack assignee", ""},
		{[]string{"move", "offline-modus", "backlog"}, ExitOK, "offline-modus: working_on -> backlog\n", "backlog/offline-modus.md"},
		{[]string{"unassign", "offline-modus"}, ExitOK, "offline-modus: assignee removed\n", ""},
		{[]string{"unassign", "offline-modus"}, ExitOK, "offline-modus: nothing to change\n", ""},
		{[]string{"set", "offline-modus", "tags="}, ExitOK, "offline-modus: tags removed\n", ""},
		{[]string{"set", "offline-modus", "size=XXL"}, ExitUsage, "size: must be one of", ""},
		{[]string{"move", "offline-modus", "nirgends"}, ExitUsage, `no column "nirgends"`, "backlog/offline-modus.md"},
		{[]string{"move", "gibt-es-nicht", "done"}, ExitUsage, `no card "gibt-es-nicht"`, ""},
	}
	for _, s := range steps {
		code, out, errOut := kb(t, append(s.args, "-C", root)...)
		if code != s.code {
			t.Fatalf("kb %v: code %d, want %d\nstdout: %s\nstderr: %s", s.args, code, s.code, out, errOut)
		}
		if code == ExitOK && out != s.out {
			t.Errorf("kb %v: out %q, want %q", s.args, out, s.out)
		}
		if code != ExitOK && !strings.Contains(errOut, s.out) {
			t.Errorf("kb %v: stderr %q lacks %q", s.args, errOut, s.out)
		}
		if s.wantFile != "" && !isFile(root, s.wantFile) {
			t.Errorf("kb %v: %s is missing", s.args, s.wantFile)
		}
	}

	if got, want := readFile(t, root, "backlog/offline-modus.md"), "---\nsize: S\n---\n\n# Offline-Modus\n"; !strings.HasPrefix(got, want) {
		t.Errorf("offline-modus = %q, want it to start with %q", got, want)
	}
	if code, out, _ := kb(t, "lint", "-C", root); code != ExitOK {
		t.Errorf("board is invalid after the changes:\n%s", out)
	}

	_, out, _ := kb(t, "move", "druckvorlagen", "done", "--force", "--json", "-C", root)
	var res struct {
		ID, From, To, Path string
		Moved              bool
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if res.From != "working_on" || res.To != "done" || res.Path != "done/druckvorlagen.md" || !res.Moved {
		t.Errorf("result = %+v", res)
	}
}

func TestSeveralPeople(t *testing.T) {
	asUser(t, "sam")
	root := copyOfExample(t)
	const card = "druckvorlagen"

	steps := []struct {
		args []string
		code int
		out  string // expected stdout, or part of stderr on failure
		file string // expected header line, "" for no assignee
	}{
		{[]string{"assign", card, "alex", "laura"}, ExitOK, "druckvorlagen: assignee = alex, laura\n", "assignee: [alex, laura]"},
		// sam is not on the card: joining it is a change to someone else's card
		{[]string{"take", card, "--add"}, ExitViolation, "is assigned to alex, laura", "assignee: [alex, laura]"},
		{[]string{"take", card, "--add", "--force"}, ExitOK, "druckvorlagen: backlog -> working_on, assignee = alex, laura, sam\n", "assignee: [alex, laura, sam]"},
		// now it is sam's card too
		{[]string{"take", card, "--add"}, ExitOK, "druckvorlagen: nothing to change\n", "assignee: [alex, laura, sam]"},
		{[]string{"unassign", card, "laura"}, ExitOK, "druckvorlagen: assignee = alex, sam\n", "assignee: [alex, sam]"},
		{[]string{"unassign", card, "laura"}, ExitOK, "druckvorlagen: nothing to change\n", "assignee: [alex, sam]"},
		{[]string{"assign", card, "--add", "laura", "sam"}, ExitOK, "druckvorlagen: assignee = alex, sam, laura\n", "assignee: [alex, sam, laura]"},
		{[]string{"unassign", card, "alex", "laura"}, ExitOK, "druckvorlagen: assignee = sam\n", "assignee: sam"},
		// in progress, the last person cannot leave
		{[]string{"unassign", card, "sam"}, ExitViolation, "would lack assignee", "assignee: sam"},
		// without --add, take and assign replace everyone
		{[]string{"assign", card, "alex,laura"}, ExitOK, "druckvorlagen: assignee = alex, laura\n", "assignee: [alex, laura]"},
		{[]string{"take", card, "--force"}, ExitOK, "druckvorlagen: assignee = sam\n", "assignee: sam"},
		{[]string{"set", card, "assignee=sam,alex"}, ExitOK, "druckvorlagen: assignee = sam, alex\n", "assignee: [sam, alex]"},
		{[]string{"assign", card, "Sam Smith"}, ExitUsage, "is not a person's short name", "assignee: [sam, alex]"},
		{[]string{"assign", card, ""}, ExitUsage, "no person named", "assignee: [sam, alex]"},
	}
	for _, s := range steps {
		code, out, errOut := kb(t, append(s.args, "-C", root)...)
		if code != s.code {
			t.Fatalf("kb %v: code %d, want %d\nstdout: %s\nstderr: %s", s.args, code, s.code, out, errOut)
		}
		if code == ExitOK && out != s.out {
			t.Errorf("kb %v: out %q, want %q", s.args, out, s.out)
		}
		if code != ExitOK && !strings.Contains(errOut, s.out) {
			t.Errorf("kb %v: stderr %q lacks %q", s.args, errOut, s.out)
		}
		path := "backlog/druckvorlagen.md"
		if !isFile(root, path) {
			path = "working_on/druckvorlagen.md"
		}
		if text := readFile(t, root, path); !strings.Contains(text, "\n"+s.file+"\n") {
			t.Errorf("kb %v: file lacks %q:\n%s", s.args, s.file, text)
		}
	}

	// Both people see the card as theirs.
	_, out, _ := kb(t, "list", "working_on", "-C", root)
	if !strings.Contains(out, "Druckvorlagen  @sam @alex\n") {
		t.Errorf("list:\n%s", out)
	}
	for _, name := range []string{"sam", "alex"} {
		asUser(t, name)
		if _, out, _ := kb(t, "list", "working_on", "--mine", "-C", root); !strings.Contains(out, "druckvorlagen") {
			t.Errorf("--mine for %s:\n%s", name, out)
		}
	}
	asUser(t, "laura")
	if _, out, _ := kb(t, "list", "working_on", "--mine", "-C", root); strings.Contains(out, "druckvorlagen") {
		t.Errorf("--mine for laura:\n%s", out)
	}
	if code, out, _ := kb(t, "lint", "-C", root); code != ExitOK {
		t.Errorf("board is invalid after the changes:\n%s", out)
	}
}

func TestWhoYouAreMattersOnlyForAssignedCards(t *testing.T) {
	asUser(t, "")
	root := copyOfExample(t)

	// Nobody's cards can be moved and assigned without any setup.
	if code, _, errOut := kb(t, "move", "druckvorlagen", "done", "-C", root); code != ExitOK {
		t.Errorf("move: code %d, stderr %q", code, errOut)
	}
	if code, _, errOut := kb(t, "assign", "offline-modus", "laura", "-C", root); code != ExitOK {
		t.Errorf("assign: code %d, stderr %q", code, errOut)
	}
	// An assigned card and `take` need to know who is asking.
	code, _, errOut := kb(t, "done", "login-fehler-bei-umlauten", "-C", root)
	if code != ExitUsage || !strings.Contains(errOut, "must know who you are") || !strings.Contains(errOut, "no user set") {
		t.Errorf("done: code %d, stderr %q", code, errOut)
	}
	code, _, errOut = kb(t, "take", "projekt-aufsetzen", "-C", root)
	if code != ExitUsage || !strings.Contains(errOut, "no user set") {
		t.Errorf("take: code %d, stderr %q", code, errOut)
	}
}

func TestBoardWithoutSpecialColumns(t *testing.T) {
	asUser(t, "sam")
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
	code, out, _ := kb(t, "take", "karte", "-C", root)
	if code != ExitOK || out != "karte: assignee = sam\n" {
		t.Errorf("take: code %d, out %q", code, out)
	}
	if got, want := readFile(t, root, "todo/karte.md"), "---\nassignee: sam\n---\n\n# Karte\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	code, _, errOut := kb(t, "done", "karte", "-C", root)
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

	code, out, _ := kb(t, "boards")
	if code != ExitOK || !strings.Contains(out, "No boards yet") {
		t.Errorf("empty machine: code %d, out %q", code, out)
	}

	// A directory inside the board works as well as the board itself.
	code, out, _ = kb(t, "add", filepath.Join(alpha, "todo"))
	if code != ExitOK || out != "alpha: added (~/repos/alpha/board)\n" {
		t.Errorf("add: code %d, out %q", code, out)
	}
	if got, want := readFile(t, filepath.Dir(configFile), "config.yaml"), "boards:\n  - ~/repos/alpha/board\n"; got != want {
		t.Errorf("settings = %q, want %q", got, want)
	}
	code, out, _ = kb(t, "add", alpha)
	if code != ExitOK || !strings.Contains(out, "already added") {
		t.Errorf("add twice: code %d, out %q", code, out)
	}
	if code, out, _ = kb(t, "add", beta); code != ExitOK {
		t.Errorf("add beta: code %d, out %q", code, out)
	}

	// Two boards with the same id cannot both be known.
	code, _, errOut := kb(t, "add", twin)
	if code != ExitUsage || !strings.Contains(errOut, `id "alpha" is already used by the board in ~/repos/alpha/board`) {
		t.Errorf("add twin: code %d, stderr %q", code, errOut)
	}
	if code, _, errOut = kb(t, "add", filepath.Join(home, "repos")); code != ExitUsage || !strings.Contains(errOut, "no board found") {
		t.Errorf("add without board: code %d, stderr %q", code, errOut)
	}

	code, out, _ = kb(t, "boards")
	want := "ID     CARDS  SOURCE  PATH\n" +
		"alpha      1  added   ~/repos/alpha/board\n" +
		"beta       1  added   ~/repos/beta\n"
	if code != ExitOK || out != want {
		t.Errorf("boards: code %d, out:\n%s", code, out)
	}

	// -b reaches a known board from anywhere.
	code, out, _ = kb(t, "list", "-b", "beta")
	if code != ExitOK || !strings.Contains(out, "eins") {
		t.Errorf("list -b: code %d, out %q", code, out)
	}
	if code, _, errOut = kb(t, "list", "-b", "nope"); code != ExitUsage || !strings.Contains(errOut, `no board "nope"`) {
		t.Errorf("list -b nope: code %d, stderr %q", code, errOut)
	}
	if code, _, _ = kb(t, "list", "-b", "beta", "-C", alpha); code != ExitUsage {
		t.Errorf("-b together with -C: code %d", code)
	}

	// Removing forgets the board and leaves its files alone.
	code, out, _ = kb(t, "remove", "alpha")
	if code != ExitOK || !strings.Contains(out, "removed ~/repos/alpha/board") || !isFile(alpha, "todo/eins.md") {
		t.Errorf("remove by id: code %d, out %q", code, out)
	}
	if code, _, _ = kb(t, "remove", beta); code != ExitOK {
		t.Errorf("remove by directory: code %d", code)
	}
	if code, _, errOut = kb(t, "remove", "alpha"); code != ExitUsage || !strings.Contains(errOut, "no added board") {
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

	code, out, _ := kb(t, "scan", filepath.Join(home, "repos"))
	want := "scanning ~/repos: 3 boards found\n  ~/repos/alpha/board\n  ~/repos/beta\n  ~/repos/broken\n"
	if code != ExitOK || out != want {
		t.Errorf("scan: code %d, out:\n%s", code, out)
	}
	if code, out, _ = kb(t, "scan", filepath.Join(home, "repos")); code != ExitOK || !strings.HasPrefix(out, "already scanning") {
		t.Errorf("scan twice: code %d, out %q", code, out)
	}
	if code, _, errOut := kb(t, "scan", filepath.Join(home, "nope")); code != ExitUsage || !strings.Contains(errOut, "not a directory") {
		t.Errorf("scan nothing: code %d, stderr %q", code, errOut)
	}

	code, out, _ = kb(t, "boards")
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

	_, out, _ = kb(t, "boards", "--json")
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
	code, _, errOut := kb(t, "remove", "alpha")
	if code != ExitUsage || !strings.Contains(errOut, "found through a directory under `scan`") {
		t.Errorf("remove a scanned board: code %d, stderr %q", code, errOut)
	}
	if code, out, _ = kb(t, "scan", "--remove", filepath.Join(home, "repos")); code != ExitOK || out != "no longer scanning ~/repos\n" {
		t.Errorf("scan --remove: code %d, out %q", code, out)
	}
	if code, _, _ = kb(t, "scan", "--remove", filepath.Join(home, "repos")); code != ExitUsage {
		t.Errorf("scan --remove twice: code %d", code)
	}
	if _, out, _ = kb(t, "boards"); !strings.Contains(out, "No boards yet") {
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
	code, _, errOut := kb(t, "add", dir)
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
	gitIn(t, root, "config", "user.name", "alex")
	gitIn(t, root, "config", "user.email", "alex@example.com")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "start")
	return root
}

func TestCommitBoard(t *testing.T) {
	asUser(t, "alex")
	root := syncRepo(t, "commit")

	code, out, errOut := kb(t, "new", "Dunkles Design", "-c", "planning", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "planning/dunkles-design.md  (commit ") {
		t.Fatalf("new: code %d, out %q, err %q", code, out, errOut)
	}
	code, out, errOut = kb(t, "assign", "dunkles-design", "sam", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "dunkles-design: assignee = sam  (commit ") {
		t.Fatalf("assign: code %d, out %q, err %q", code, out, errOut)
	}
	code, out, _ = kb(t, "assign", "dunkles-design", "sam", "-C", root, "--force")
	if code != ExitOK || out != "dunkles-design: nothing to change\n" {
		t.Errorf("assign again: code %d, out %q", code, out)
	}
	if got := gitIn(t, root, "log", "--format=%s"); got != "assign dunkles-design: sam\nnew dunkles-design\nstart" {
		t.Errorf("log = %q", got)
	}

	code, _, errOut = kb(t, "sync", "-C", root)
	if code != ExitUsage || !strings.Contains(errOut, "sync: commit") {
		t.Errorf("sync: code %d, err %q", code, errOut)
	}
}

func TestPushBoard(t *testing.T) {
	asUser(t, "alex")
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
	alex, sam := clone("alex"), clone("sam")

	code, out, errOut := kb(t, "take", "druckvorlagen", "-C", alex, "--force")
	if code != ExitOK || !strings.Contains(out, "backlog -> working_on") || !strings.HasSuffix(out, ", pushed)\n") {
		t.Fatalf("take: code %d, out %q, err %q", code, out, errOut)
	}

	// Sam's command pulls first, so it sees that the card is Alex's.
	t.Setenv("KARDBOARD_USER", "sam")
	code, _, errOut = kb(t, "done", "druckvorlagen", "-C", sam)
	if code != ExitViolation || !strings.Contains(errOut, "assigned to alex") {
		t.Errorf("done: code %d, err %q", code, errOut)
	}
	if !isFile(sam, "working_on/druckvorlagen.md") {
		t.Error("sam did not get alex's change")
	}

	// A commit made by hand goes out with kb sync.
	gitIn(t, sam, "commit", "-q", "--allow-empty", "-m", "by hand")
	code, out, errOut = kb(t, "sync", "-C", sam)
	if code != ExitOK || out != "demo: 0 commits pulled, 1 commit pushed\n" {
		t.Errorf("sync: code %d, out %q, err %q", code, out, errOut)
	}
	if code, out, _ = kb(t, "sync", "-C", sam); code != ExitOK || out != "demo: up to date\n" {
		t.Errorf("sync again: code %d, out %q", code, out)
	}

	// Without the remote the change stays here, and kb says so.
	gitIn(t, sam, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	code, _, errOut = kb(t, "move", "offline-modus", "backlog", "-C", sam, "--force")
	if code != ExitUsage || !strings.Contains(errOut, "nothing was changed") || !isFile(sam, "planning/offline-modus.md") {
		t.Errorf("move: code %d, err %q", code, errOut)
	}
}

// ── kb init ─────────────────────────────────────────────────────────────────

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

	code, out, errOut := kb(t, "init", root)
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
	if code, out, _ := kb(t, "lint", "-C", root); code != ExitOK || out != "0 cards, 0 errors, 0 warnings\n" {
		t.Errorf("lint: code %d, out %q", code, out)
	}
	if code, out, errOut := kb(t, "new", "Erste Karte", "-b", "mein-projekt"); code != ExitOK || out != "backlog/erste-karte.md\n" {
		t.Errorf("new: code %d, out %q, err %q", code, out, errOut)
	}

	// A board is never written over.
	code, _, errOut = kb(t, "init", root, "--id", "anders")
	if code != ExitUsage || !strings.Contains(errOut, "is a board already") || readFile(t, root, "board.yaml") != want {
		t.Errorf("second init: code %d, err %q", code, errOut)
	}
}

func TestInitAdoptsDirectoriesWithCards(t *testing.T) {
	home, _ := machine(t)
	root := filepath.Join(home, "zettel")
	put(t, root,
		"done/a.md", "backlog/b.md", "backlog/c.md", "planning/d.md",
		"working_on/sam/e.md",      // a directory per person still makes a column
		"reference/notes.md",       // unknown name: goes in the middle
		"bin/tool.sh", "README.md", // no cards: not columns
		".github/x.md", "Mit Leerzeichen/y.md", // not usable as column ids
	)
	code, out, errOut := kb(t, "init", "-C", root, "--name", "Zettel: alle", "--no-add")
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
	if readFile(t, root, "working_on/sam/e.md") != "# working_on/sam/e.md\n" || isFile(root, "bin/board.yaml") {
		t.Error("init touched the files")
	}
	if code, out, _ := kb(t, "boards"); code != ExitOK || strings.Contains(out, "zettel") {
		t.Errorf("boards: code %d, out %q, want the board not added", code, out)
	}
}

func TestInitOptions(t *testing.T) {
	home, _ := machine(t)
	root := filepath.Join(home, "x")

	// A wrong value leaves nothing behind.
	for _, args := range [][]string{{"--sync", "always"}, {"--id", "Nicht Gut"}, {"--columns", "a,a"}, {"--columns", "Spalte Eins"}} {
		code, _, errOut := kb(t, append([]string{"init", root}, args...)...)
		if _, err := os.Stat(root); code != ExitUsage || errOut == "" || err == nil {
			t.Errorf("%v: code %d, err %q", args, code, errOut)
		}
	}

	code, out, errOut := kb(t, "init", root, "--id", "team", "--sync", "push", "--columns", "todo, doing ,closed", "--json")
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
	code, out, errOut = kb(t, "init", other, "--id", "team")
	if code != ExitOK || !isFile(other, "board.yaml") || !strings.Contains(errOut, "already used") || strings.Contains(out, "added") {
		t.Errorf("same id: code %d, out %q, err %q", code, out, errOut)
	}
}

// editorWrites makes kb edit see an editor that replaces the card with text.
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
	asUser(t, "alex")
	root := syncRepo(t, "commit")
	before := readFile(t, root, "backlog/druckvorlagen.md")

	// The editor closes without a change: nothing happens.
	t.Setenv("VISUAL", "true")
	code, out, errOut := kb(t, "edit", "druckvorlagen", "-C", root)
	if code != ExitOK || out != "druckvorlagen: nothing to change\n" {
		t.Fatalf("unchanged: code %d, out %q, err %q", code, out, errOut)
	}

	// Text and a field changed in the editor: one commit "edit".
	edited := strings.Replace(before, "size: S", "size: M", 1) + "\nDazu ein Satz.\n"
	editorWrites(t, edited)
	code, out, errOut = kb(t, "edit", "druckvorlagen", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "druckvorlagen: size = M, text changed  (commit ") {
		t.Fatalf("edit: code %d, out %q, err %q", code, out, errOut)
	}
	if got := readFile(t, root, "backlog/druckvorlagen.md"); got != edited {
		t.Errorf("file = %q", got)
	}
	if got := gitIn(t, root, "log", "-1", "--format=%s"); got != "edit druckvorlagen" {
		t.Errorf("commit = %q", got)
	}

	// A card the editor breaks is not saved, and the text is kept.
	editorWrites(t, "---\nsize: [\n---\n\n# Druckvorlagen\n")
	code, _, errOut = kb(t, "edit", "druckvorlagen", "-C", root)
	if code == ExitOK || !strings.Contains(errOut, "would have errors") || !strings.Contains(errOut, "your text is in ") {
		t.Errorf("broken: code %d, err %q", code, errOut)
	}
	if readFile(t, root, "backlog/druckvorlagen.md") != edited {
		t.Error("broken text was saved")
	}

	// A card with changes that are not committed is refused before the
	// editor opens.
	if err := os.WriteFile(filepath.Join(root, "backlog", "druckvorlagen.md"), []byte(edited+"x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", "false")
	code, _, errOut = kb(t, "edit", "druckvorlagen", "-C", root)
	if code == ExitOK || !strings.Contains(errOut, "not committed") {
		t.Errorf("dirty: code %d, err %q", code, errOut)
	}
}
