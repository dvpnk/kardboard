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

// brokenBoard is the example board plus one card that breaks two rules.
func brokenBoard(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(example)); err != nil {
		t.Fatal(err)
	}
	card := "---\nsize: XXL\n---\nno title\n"
	if err := os.WriteFile(filepath.Join(root, "backlog", "kaputt.md"), []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
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
    offline-modus                Offline-Modus
Backlog (2)
    druckvorlagen                Druckvorlagen
    export-als-pdf               Export als PDF
In Arbeit (1)
  alex:
      login-fehler-bei-umlauten  Login-Fehler bei Umlauten im Passwort
Erledigt (1)
    projekt-aufsetzen            Projekt aufsetzen
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no settings file

	t.Setenv("KARDBOARD_USER", "alex")
	code, out, _ := kb(t, "list", "--mine", "--json", "-C", example)
	var res struct {
		Board struct{ ID, Name string }
		Cards []struct{ ID, Person string }
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != ExitOK || res.Board.ID != "demo" || len(res.Cards) != 1 || res.Cards[0].Person != "alex" {
		t.Errorf("code %d, result %+v", code, res)
	}

	t.Setenv("KARDBOARD_USER", "")
	code, _, errOut := kb(t, "list", "--mine", "-C", example)
	if code != ExitUsage || !strings.Contains(errOut, "no user set") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
}

func TestShow(t *testing.T) {
	code, out, _ := kb(t, "show", "login-fehler-bei-umlauten.md", "-C", example)
	for _, want := range []string{
		"path:     working_on/alex/login-fehler-bei-umlauten.md",
		"column:   In Arbeit",
		"person:   alex",
		"tags:     bug",
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
	}
	for _, args := range tests {
		code, _, errOut := kb(t, args...)
		if code != ExitUsage || !strings.HasPrefix(errOut, "kb: ") {
			t.Errorf("kb %v: code %d, stderr %q", args, code, errOut)
		}
	}
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

func isFile(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

func TestNewCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KARDBOARD_USER", "")
	root := copyOfExample(t)

	code, out, _ := kb(t, "new", "Dunkles Design", "-C", root)
	if code != ExitOK || out != "planning/dunkles-design.md\n" {
		t.Errorf("code %d, out %q", code, out)
	}
	data, _ := os.ReadFile(filepath.Join(root, "planning", "dunkles-design.md"))
	if !strings.HasPrefix(string(data), "---\ncreated: 20") || !strings.HasSuffix(string(data), "---\n\n# Dunkles Design\n") {
		t.Errorf("file = %q", data)
	}

	// The column requires a size: refused, then accepted with --set.
	code, _, errOut := kb(t, "new", "Export als CSV", "-c", "backlog", "-C", root)
	if code != ExitViolation || !strings.Contains(errOut, "requires size") || !strings.Contains(errOut, "--force") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
	code, out, _ = kb(t, "new", "Export als CSV", "-c", "backlog", "--set", "size=S", "--set", "tags=feature,ui", "--json", "-C", root)
	var res struct {
		ID, Path string
		Fields   map[string]any
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != ExitOK || res.Path != "backlog/export-als-csv.md" || res.Fields["size"] != "S" {
		t.Errorf("code %d, result %+v", code, res)
	}

	// A person column needs to know who you are, or --as.
	code, _, errOut = kb(t, "new", "Ohne Namen", "-c", "working_on", "--set", "size=S", "-C", root)
	if code != ExitUsage || !strings.Contains(errOut, "no user set") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
	code, out, _ = kb(t, "new", "Mit Namen", "-c", "working_on", "--as", "sam", "--id", "eigener-name", "--set", "size=S", "-C", root)
	if code != ExitOK || out != "working_on/sam/eigener-name.md\n" {
		t.Errorf("code %d, out %q", code, out)
	}

	if code, out, _ = kb(t, "lint", "-C", root); code != ExitOK {
		t.Errorf("board is invalid after kb new:\n%s", out)
	}
}

func TestMoveCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KARDBOARD_USER", "sam")
	root := copyOfExample(t)

	steps := []struct {
		args     []string
		code     int
		out      string // expected stdout, or part of stderr on failure
		wantFile string
	}{
		{[]string{"take", "druckvorlagen"}, ExitOK, "druckvorlagen: backlog -> working_on/sam\n", "working_on/sam/druckvorlagen.md"},
		{[]string{"take", "druckvorlagen"}, ExitOK, "druckvorlagen: already in working_on/sam\n", "working_on/sam/druckvorlagen.md"},
		// planning cards have no size, which working_on requires
		{[]string{"take", "offline-modus"}, ExitViolation, "lacks size", "planning/offline-modus.md"},
		{[]string{"take", "offline-modus", "--force"}, ExitOK, "offline-modus: planning -> working_on/sam\n", "working_on/sam/offline-modus.md"},
		// alex's card is protected
		{[]string{"done", "login-fehler-bei-umlauten"}, ExitViolation, "in the directory of alex", "working_on/alex/login-fehler-bei-umlauten.md"},
		{[]string{"give", "login-fehler-bei-umlauten", "sam", "--force"}, ExitOK, "login-fehler-bei-umlauten: working_on/alex -> working_on/sam\n", "working_on/sam/login-fehler-bei-umlauten.md"},
		{[]string{"done", "login-fehler-bei-umlauten"}, ExitOK, "login-fehler-bei-umlauten: working_on/sam -> done\n", "done/login-fehler-bei-umlauten.md"},
		// give from a plain column goes to the first per-person column
		{[]string{"give", "export-als-pdf", "alex"}, ExitOK, "export-als-pdf: backlog -> working_on/alex\n", "working_on/alex/export-als-pdf.md"},
		{[]string{"move", "druckvorlagen", "backlog"}, ExitOK, "druckvorlagen: working_on/sam -> backlog\n", "backlog/druckvorlagen.md"},
		{[]string{"move", "druckvorlagen", "working_on", "--as", "alex"}, ExitOK, "druckvorlagen: backlog -> working_on/alex\n", "working_on/alex/druckvorlagen.md"},
		{[]string{"move", "druckvorlagen", "nirgends"}, ExitUsage, `no column "nirgends"`, "working_on/alex/druckvorlagen.md"},
		{[]string{"move", "gibt-es-nicht", "done"}, ExitUsage, `no card "gibt-es-nicht"`, ""},
		{[]string{"move", "druckvorlagen"}, ExitUsage, "accepts 2 arg(s)", ""},
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

	_, out, _ := kb(t, "move", "druckvorlagen", "done", "--force", "--json", "-C", root)
	var res struct {
		ID, From, To, Path string
		Moved              bool
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if res.From != "working_on/alex" || res.To != "done" || !res.Moved {
		t.Errorf("result = %+v", res)
	}
}

func TestTakeAndDoneNeedTheirColumns(t *testing.T) {
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
	for _, args := range [][]string{{"take", "karte"}, {"give", "karte", "sam"}, {"done", "karte"}} {
		code, _, errOut := kb(t, append(args, "-C", root)...)
		if code != ExitUsage || !strings.Contains(errOut, "has no column with") {
			t.Errorf("kb %v: code %d, stderr %q", args, code, errOut)
		}
	}
}
