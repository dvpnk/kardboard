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
