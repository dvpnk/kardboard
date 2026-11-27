package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// agentBoard is the example board with the agent vps-1, which works cards
// in planning.
func agentBoard(t *testing.T) string {
	t.Helper()
	root := copyOfExample(t)
	yaml := readFile(t, root, "board.yaml")
	yaml = strings.Replace(yaml, "columns:\n  - id: planning\n    title: Planning\n",
		"agents: [vps-1]\n\nagent:\n  prompt: Read CLAUDE.md first.\n\ncolumns:\n  - id: planning\n    title: Planning\n    agent:\n      next: done\n      prompt: Write a plan into the card.\n", 1)
	if err := os.WriteFile(filepath.Join(root, "board.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

type shown struct {
	ID         string
	State      *string
	WaitingFor []string `json:"waiting_for"`
	Agent      *struct {
		Prompt       string
		ColumnPrompt string `json:"column_prompt"`
		Next         string
	}
	Activity []struct {
		Author, Kind, Text string
		To                 []string
	}
}

func show(t *testing.T, root, card string) shown {
	t.Helper()
	code, out, errOut := kard(t, "show", card, "--json", "-C", root)
	if code != ExitOK {
		t.Fatalf("show: code %d, err %q", code, errOut)
	}
	var res shown
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return res
}

func state(s shown) string {
	if s.State == nil {
		return "none"
	}
	return *s.State
}

// A person and a script hold a conversation on a card, without a runner.
func TestConversationOnACard(t *testing.T) {
	asUser(t, "arnold")
	root := agentBoard(t)

	if s := show(t, root, "offline-mode"); state(s) != "none" || len(s.Activity) != 0 || s.Activity == nil {
		t.Fatalf("before: %+v", s)
	}
	if code, _, errOut := kard(t, "assign", "offline-mode", "vps-1", "-C", root); code != ExitOK {
		t.Fatalf("assign: %q", errOut)
	}
	if s := show(t, root, "offline-mode"); state(s) != "queued" {
		t.Errorf("assigned to the agent in its column: state %q", state(s))
	}

	// The script is the agent: whoever starts it sets these.
	t.Setenv("KARDBOARD_USER", "vps-1")
	t.Setenv("KARD_BOARD", root)
	t.Setenv("KARD_CARD", "offline-mode")
	// It finds its card, and with it what the board and the column ask.
	code, out, errOut := kard(t, "list", "--state", "queued", "--mine")
	if code != ExitOK || out != "Planning (1)\n    offline-mode  Offline mode  @vps-1  [queued]\nBacklog (0)\nIn progress (0)\nDone (0)\n" {
		t.Fatalf("list: code %d, out %q, err %q", code, out, errOut)
	}
	if s := show(t, root, "offline-mode"); s.Agent == nil || s.Agent.Prompt != "Read CLAUDE.md first." || s.Agent.ColumnPrompt != "Write a plan into the card." || s.Agent.Next != "done" {
		t.Fatalf("agent = %+v", s.Agent)
	}
	if s := show(t, root, "export-as-pdf"); s.Agent != nil {
		t.Errorf("a column of people hands out %+v", s.Agent)
	}
	code, out, errOut = kard(t, "comment", "Read the code. Writing is the open point.")
	if code != ExitOK || out != "offline-mode: comment added to activity/offline-mode.md\n" {
		t.Fatalf("comment: code %d, out %q, err %q", code, out, errOut)
	}
	code, out, errOut = kard(t, "ask", "Should the app write without a connection?", "--to", "arnold")
	if code != ExitOK || out != "offline-mode: question to arnold added to activity/offline-mode.md\n" {
		t.Fatalf("ask: code %d, out %q, err %q", code, out, errOut)
	}
	s := show(t, root, "offline-mode")
	if state(s) != "waiting" || strings.Join(s.WaitingFor, ",") != "arnold" || len(s.Activity) != 2 || s.Activity[1].Kind != "question" {
		t.Errorf("after the question: %+v", s)
	}
	code, out, _ = kard(t, "list", "planning", "-C", root)
	if code != ExitOK || !strings.Contains(out, "offline-mode") || !strings.HasSuffix(out, "@vps-1  [waiting for arnold]\n") {
		t.Errorf("list: %q", out)
	}

	if code, out, _ = kard(t, "list", "--state", "queued", "-C", root, "--json"); code != ExitOK || strings.Contains(out, "offline-mode") {
		t.Errorf("a waiting card is listed as queued: %s", out)
	}

	// The person answers, from a file; it is the agent's turn again.
	t.Setenv("KARDBOARD_USER", "arnold")
	t.Setenv("KARD_CARD", "")
	t.Setenv("KARD_BOARD", "")
	answer := filepath.Join(t.TempDir(), "answer.md")
	if err := os.WriteFile(answer, []byte("Only read.\n\nWriting comes later.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut = kard(t, "comment", "offline-mode", "--text", answer, "-C", root, "--json")
	if code != ExitOK {
		t.Fatalf("answer: code %d, err %q", code, errOut)
	}
	var res struct {
		Card, Path string
		Entry      struct{ Author, Kind, Text string }
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if res.Card != "offline-mode" || res.Path != "activity/offline-mode.md" || res.Entry.Author != "arnold" || res.Entry.Text != "Only read.\n\nWriting comes later." {
		t.Errorf("json = %+v", res)
	}
	if s := show(t, root, "offline-mode"); state(s) != "queued" || len(s.Activity) != 3 {
		t.Errorf("after the answer: %+v", s)
	}

	code, out, _ = kard(t, "show", "offline-mode", "-C", root)
	for _, want := range []string{"state:     queued", "Activity (3)", " · vps-1 · question to arnold\n  Should the app write without a connection?", "  Only read.\n\n  Writing comes later.\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}
	code, out, _ = kard(t, "list", "--json", "-C", root)
	if code != ExitOK || !strings.Contains(out, `"state": "queued"`) || !strings.Contains(out, `"state": null`) {
		t.Errorf("list --json: %s", out)
	}
	if code, out, _ := kard(t, "lint", "-C", root); code != ExitOK {
		t.Errorf("lint: %s", out)
	}
	// The card's own file stayed as it was.
	if got, want := readFile(t, root, "planning/offline-mode.md"), readFile(t, example, "planning/offline-mode.md"); !strings.HasSuffix(got, want) {
		t.Errorf("card = %q", got)
	}

	// The agent ends its work by moving the card on.
	t.Setenv("KARDBOARD_USER", "vps-1")
	if code, _, errOut := kard(t, "move", "offline-mode", "done", "-C", root); code != ExitOK {
		t.Fatalf("move: %q", errOut)
	}
	if s := show(t, root, "offline-mode"); state(s) != "none" || s.Agent != nil {
		t.Errorf("moved on: %+v", s)
	}
	if code, _, errOut := kard(t, "list", "--state", "running", "-C", root); code != ExitUsage || !strings.Contains(errOut, `no state "running"`) {
		t.Errorf("unknown state: code %d, err %q", code, errOut)
	}
}

func TestCommentFromStdin(t *testing.T) {
	asUser(t, "arnold")
	root := agentBoard(t)
	var out, errOut strings.Builder
	code := run([]string{"comment", "export-as-pdf", "--text", "-", "-C", root}, strings.NewReader("From a pipe.\n"), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("code %d, err %q", code, errOut.String())
	}
	if got := readFile(t, root, "activity/export-as-pdf.md"); !strings.HasSuffix(got, " · arnold · comment\nFrom a pipe.\n") {
		t.Errorf("file = %q", got)
	}
}

func TestCommentAndAskErrors(t *testing.T) {
	asUser(t, "arnold")
	root := agentBoard(t)
	// Two people the board knows whose names begin alike.
	if err := os.WriteFile(filepath.Join(root, "planning", "pair.md"),
		[]byte("---\nassignee: [arnold.a@example.com, arnold.b@example.com]\n---\n\n# Pair\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"comment", "export-as-pdf"}, ExitUsage, "no text"},
		{[]string{"comment"}, ExitUsage, "name the card and the text"},
		{[]string{"comment", "--text", "x.md"}, ExitUsage, "name the card"},
		{[]string{"comment", "nope", "Hi."}, ExitUsage, `no card "nope"`},
		{[]string{"comment", "export-as-pdf", "  "}, ExitUsage, "the comment is empty"},
		{[]string{"ask", "export-as-pdf", "Who?", "--to", "arnold."}, ExitUsage, "fits 2 people"},
	}
	for _, c := range cases {
		code, _, errOut := kard(t, append(c.args, "-C", root)...)
		if code != c.code || !strings.Contains(errOut, c.want) {
			t.Errorf("kard %v: code %d, err %q, want %q", c.args, code, errOut, c.want)
		}
	}
	if isFile(root, "activity/export-as-pdf.md") {
		t.Error("a refused entry was written")
	}
	_, _, errOut := kard(t, "ask", "export-as-pdf", "Who?", "--to", "arnold.", "-C", root, "--json")
	if !strings.Contains(errOut, `"kind":"ambiguous"`) || !strings.Contains(errOut, "arnold.b@example.com") {
		t.Errorf("json error = %q", errOut)
	}
	// A unique beginning finds the full address.
	code, out, errOut := kard(t, "ask", "export-as-pdf", "Which one?", "--to", "arnold.b", "-C", root)
	if code != ExitOK || !strings.Contains(out, "question to arnold.b@example.com") {
		t.Errorf("ask: code %d, out %q, err %q", code, out, errOut)
	}
}

func TestEntriesOnACommitBoard(t *testing.T) {
	asUser(t, "arnold")
	root := syncRepo(t, "commit")
	code, out, errOut := kard(t, "comment", "export-as-pdf", "Hi.", "-C", root)
	if code != ExitOK || !strings.HasPrefix(out, "export-as-pdf: comment added to activity/export-as-pdf.md  (commit ") {
		t.Fatalf("comment: code %d, out %q, err %q", code, out, errOut)
	}
	if code, _, errOut = kard(t, "ask", "export-as-pdf", "Why?", "-C", root); code != ExitOK {
		t.Fatalf("ask: %q", errOut)
	}
	code, out, errOut = kard(t, "rename", "export-as-pdf", "pdf-export", "-C", root, "--force")
	if code != ExitOK {
		t.Fatalf("rename: code %d, err %q", code, errOut)
	}
	if got := gitIn(t, root, "log", "--format=%s"); got != "rename export-as-pdf -> pdf-export\nask export-as-pdf\ncomment export-as-pdf\nstart" {
		t.Errorf("log = %q", got)
	}
	if got := gitIn(t, root, "status", "--porcelain"); got != "" {
		t.Errorf("status = %q", got)
	}
	if !isFile(root, "activity/pdf-export.md") || isFile(root, "activity/export-as-pdf.md") {
		t.Error("the activity did not follow the card")
	}
}

func TestActivityIsNoColumn(t *testing.T) {
	root := copyOfExample(t)
	yaml := strings.Replace(readFile(t, root, "board.yaml"), "  - id: done\n", "  - id: activity\n  - id: done\n", 1)
	if err := os.WriteFile(filepath.Join(root, "board.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := kard(t, "lint", "-C", root)
	if code != ExitViolation || !strings.Contains(out, `board.yaml: E01 columns[3]: id "activity" is reserved`) {
		t.Errorf("code %d, out %q", code, out)
	}
}
