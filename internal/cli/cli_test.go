package cli

import (
	"bytes"
	"encoding/json"
	"os"
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
		res.Cards[0].ID != "export-als-pdf" || res.Cards[0].Column != "backlog" || res.Cards[0].Fields["assignee"] != "sam" {
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
	if code != ExitOK || res.Path != "backlog/export-als-csv.md" || res.Fields["size"] != "S" || res.Fields["assignee"] != "sam" {
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
