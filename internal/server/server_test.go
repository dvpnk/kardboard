package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var example = filepath.Join("..", "..", "examples", "board")

// settings gives the test its own settings file with the given content.
func settings(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("KARDBOARD_USER", "")
	if content == "" {
		return
	}
	path := filepath.Join(dir, "kardboard", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// copyOfExample is a board the test may change.
func copyOfExample(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(root, os.DirFS(example)); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// get asks the handler directly, as a browser on this machine would.
func get(t *testing.T, s *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:4712"+target, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

type state struct {
	User   string
	Hidden []string
	Boards []struct {
		Key, ID, Name, Root, Source, Error, Sync string
		Columns                                  []struct {
			ID, Title   string
			Doing, Done bool
			Limit       int
		}
		Fields []struct {
			Name, Title, Type string
			Values            []string
		}
		Cards []struct {
			ID, Path, Column, Title string
			Fields                  map[string]any
			Problems                []struct{ Code, Severity, Message string }
		}
		Problems         []struct{ Code, Path string }
		Errors, Warnings int
	}
}

func getState(t *testing.T, s *Server, query string) state {
	t.Helper()
	rec := get(t, s, "/api/state"+query)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var st state
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("%v\n%s", err, rec.Body)
	}
	return st
}

func TestStateOfOneBoard(t *testing.T) {
	settings(t, "user: sam\nhidden: [atlas]\n")
	root := copyOfExample(t)
	st := getState(t, New(root), "")

	if st.User != "sam" || len(st.Hidden) != 1 || st.Hidden[0] != "atlas" {
		t.Errorf("user %q, hidden %v", st.User, st.Hidden)
	}
	if len(st.Boards) != 1 {
		t.Fatalf("got %d boards", len(st.Boards))
	}
	b := st.Boards[0]
	if b.ID != "demo" || b.Name != "Demo-Projekt" || b.Source != "here" || b.Key != root || b.Error != "" || b.Sync != "none" {
		t.Errorf("board = %+v", b)
	}
	if len(b.Columns) != 4 || b.Columns[2].ID != "working_on" || !b.Columns[2].Doing || b.Columns[2].Limit != 3 ||
		b.Columns[2].Title != "In Arbeit" || !b.Columns[3].Done {
		t.Errorf("columns = %+v", b.Columns)
	}
	if len(b.Fields) == 0 || b.Fields[0].Name != "assignee" || b.Fields[0].Type != "person" || b.Fields[1].Title != "Größe" {
		t.Errorf("fields = %+v", b.Fields)
	}
	if len(b.Cards) != 5 || b.Errors != 0 || b.Warnings != 0 {
		t.Fatalf("%d cards, %d errors, %d warnings", len(b.Cards), b.Errors, b.Warnings)
	}
	for _, c := range b.Cards {
		if c.ID == "export-als-pdf" {
			who, _ := c.Fields["assignee"].([]any)
			if c.Column != "backlog" || c.Title != "Export als PDF" || len(who) != 1 || who[0] != "sam" {
				t.Errorf("card = %+v", c)
			}
			return
		}
	}
	t.Error("card export-als-pdf is missing")
}

func TestStateShowsBrokenBoardsAndCards(t *testing.T) {
	settings(t, "")
	dir := t.TempDir()
	good, twin, broken := filepath.Join(dir, "good"), filepath.Join(dir, "twin"), filepath.Join(dir, "broken")
	const yaml = "kardboard: 1\nid: same\ncolumns:\n  - id: todo\n    limit: 1\n"
	writeFile(t, filepath.Join(good, "board.yaml"), yaml)
	writeFile(t, filepath.Join(good, "todo", "ok.md"), "# Fine\n")
	writeFile(t, filepath.Join(good, "todo", "bad.md"), "---\nprio: 1\n---\nno title\n")
	writeFile(t, filepath.Join(twin, "board.yaml"), yaml)
	writeFile(t, filepath.Join(broken, "board.yaml"), "kardboard: 1\n")
	settings(t, "boards:\n  - "+good+"\n  - "+twin+"\n  - "+broken+"\n  - "+filepath.Join(dir, "gone")+"\n")

	st := getState(t, New(""), "")
	if len(st.Boards) != 4 {
		t.Fatalf("got %d boards", len(st.Boards))
	}
	g, tw, br, gone := st.Boards[0], st.Boards[1], st.Boards[2], st.Boards[3]

	// A faulty card is delivered with its problems, never left out.
	if g.Error != "" || len(g.Cards) != 2 || g.Errors != 2 || g.Warnings != 1 {
		t.Errorf("good = %+v", g)
	}
	if len(g.Problems) != 1 || g.Problems[0].Code != "W02" {
		t.Errorf("board problems = %+v", g.Problems)
	}
	for _, c := range g.Cards {
		if c.ID == "bad" && len(c.Problems) != 2 {
			t.Errorf("bad card problems = %+v", c.Problems)
		}
	}
	if !strings.Contains(tw.Error, `id "same" is already used`) || len(tw.Cards) != 0 {
		t.Errorf("twin = %+v", tw)
	}
	if !strings.Contains(br.Error, "board.yaml:") || br.ID != "" || br.Name != "broken" {
		t.Errorf("broken = %+v", br)
	}
	if !strings.Contains(gone.Error, "file is missing") {
		t.Errorf("gone = %+v", gone)
	}
	for _, b := range st.Boards {
		if b.Columns == nil || b.Cards == nil || b.Fields == nil || b.Problems == nil {
			t.Errorf("%s: lists must be [] in JSON, never null", b.Name)
		}
	}
}

func TestStateWithoutBoards(t *testing.T) {
	settings(t, "")
	rec := get(t, New(""), "/api/state")
	if got := strings.TrimSpace(rec.Body.String()); got != `{"user":"","hidden":[],"boards":[]}` {
		t.Errorf("body = %s", got)
	}
}

func TestScanFindsNewBoards(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	settings(t, "scan:\n  - "+dir+"\n")
	s := New("")
	if st := getState(t, s, ""); len(st.Boards) != 0 {
		t.Fatalf("got %d boards", len(st.Boards))
	}

	writeFile(t, filepath.Join(dir, "proj", "board", "board.yaml"), "kardboard: 1\nid: proj\ncolumns:\n  - id: todo\n")
	// The directories are not searched on every request ...
	if st := getState(t, s, ""); len(st.Boards) != 0 {
		t.Errorf("the scan ran again without being asked: %d boards", len(st.Boards))
	}
	// ... but on demand.
	st := getState(t, s, "?rescan")
	if len(st.Boards) != 1 || st.Boards[0].ID != "proj" || st.Boards[0].Source != "scan" {
		t.Errorf("boards = %+v", st.Boards)
	}
}

func TestCard(t *testing.T) {
	settings(t, "")
	s := New(copyOfExample(t))

	rec := get(t, s, "/api/card?board=demo&id=export-als-pdf")
	var card struct {
		Board, ID, Title, Body string
		Fields                 map[string]any
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &card); err != nil {
		t.Fatalf("%v\n%s", err, rec.Body)
	}
	if card.Board != "demo" || card.ID != "export-als-pdf" || card.Fields["size"] != "M" ||
		!strings.Contains(card.Body, "# Export als PDF") || !strings.Contains(card.Body, "## Fertig, wenn") {
		t.Errorf("card = %+v", card)
	}

	for _, target := range []string{"/api/card?board=demo&id=nope", "/api/card?board=nope&id=x", "/api/card"} {
		if rec := get(t, s, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d", target, rec.Code)
		}
	}
}

func TestWebUIIsServed(t *testing.T) {
	settings(t, "")
	s := New("")
	for target, want := range map[string]string{
		"/":         `<script type="module" src="app.js">`,
		"/app.js":   "kardboard web UI",
		"/md.js":    "export function render",
		"/app.css":  "--mono:",
		"/icon.svg": "<svg",
	} {
		rec := get(t, s, target)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: status %d, body lacks %q", target, rec.Code, want)
		}
	}
	rec := get(t, s, "/")
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("Content-Security-Policy = %q", csp)
	}
	if rec := get(t, s, "/nope.js"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown file: status %d", rec.Code)
	}
}

func TestOnlyLocalhostAndOnlyReading(t *testing.T) {
	settings(t, "")
	h := New(copyOfExample(t)).Handler()
	do := func(method, host, target string) int {
		req := httptest.NewRequest(method, "http://"+host+target, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	for _, host := range []string{"127.0.0.1:4712", "localhost:4712", "[::1]:4712", "localhost"} {
		if code := do(http.MethodGet, host, "/api/state"); code != http.StatusOK {
			t.Errorf("host %s: status %d", host, code)
		}
	}
	// A page on another site may reach the port under its own name.
	for _, host := range []string{"evil.example:4712", "127.0.0.1.evil.example:4712", "192.168.1.5:4712", "kardboard.dev"} {
		if code := do(http.MethodGet, host, "/api/state"); code != http.StatusForbidden {
			t.Errorf("host %s: status %d, want 403", host, code)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		for _, target := range []string{"/api/state", "/api/card?board=demo&id=export-als-pdf", "/"} {
			if code := do(method, "127.0.0.1:4712", target); code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status %d, want 405", method, target, code)
			}
		}
	}
}

func TestFingerprintFollowsTheBoard(t *testing.T) {
	settings(t, "")
	root := copyOfExample(t)
	s := New(root)

	before := s.fingerprint()
	if again := s.fingerprint(); again != before {
		t.Fatal("the fingerprint changes although nothing happened")
	}
	changes := []struct {
		name string
		do   func()
	}{
		{"a new card", func() { writeFile(t, filepath.Join(root, "backlog", "neu.md"), "# Neu\n") }},
		{"an edited card", func() { writeFile(t, filepath.Join(root, "backlog", "neu.md"), "# Neu, geändert\n") }},
		{"a moved card", func() {
			if err := os.Rename(filepath.Join(root, "backlog", "neu.md"), filepath.Join(root, "done", "neu.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"a removed card", func() {
			if err := os.Remove(filepath.Join(root, "done", "neu.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"a changed board.yaml", func() {
			data, _ := os.ReadFile(filepath.Join(root, "board.yaml"))
			writeFile(t, filepath.Join(root, "board.yaml"), string(data)+"\n# note\n")
		}},
	}
	for _, c := range changes {
		c.do()
		after := s.fingerprint()
		if after == before {
			t.Errorf("%s: the fingerprint did not change", c.name)
		}
		before = after
	}

	// Hidden files are not part of the board.
	writeFile(t, filepath.Join(root, "backlog", ".DS_Store"), "x")
	if s.fingerprint() != before {
		t.Error("a hidden file changed the fingerprint")
	}
}

func TestEventsAnnounceChanges(t *testing.T) {
	settings(t, "")
	root := copyOfExample(t)
	s := New(root)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Watch(ctx, 10*time.Millisecond)

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	lines := make(chan string)
	go func() {
		defer close(lines)
		r := bufio.NewReader(res.Body)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			lines <- strings.TrimSpace(line)
		}
	}()
	wait := func(want string) bool {
		timeout := time.After(3 * time.Second)
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					return false
				}
				if line == want {
					return true
				}
			case <-timeout:
				return false
			}
		}
	}

	if !wait(": connected") {
		t.Fatal("the stream did not open")
	}
	writeFile(t, filepath.Join(root, "backlog", "neu.md"), "# Neu\n")
	if !wait("event: change") {
		t.Fatal("no event after a new card")
	}
	if err := os.Rename(filepath.Join(root, "backlog", "neu.md"), filepath.Join(root, "done", "neu.md")); err != nil {
		t.Fatal(err)
	}
	if !wait("event: change") {
		t.Fatal("no event after a moved card")
	}
}

func TestRunServesUntilCancelled(t *testing.T) {
	settings(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	urls := make(chan string, 1)
	root := copyOfExample(t)
	go func() {
		done <- New(root).Run(ctx, 0, func(url string) { urls <- url })
	}()

	var url string
	select {
	case url = <-urls:
	case err := <-done:
		t.Fatalf("server ended early: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("server did not start")
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("url = %q, the server must listen on the loopback interface only", url)
	}
	res, err := http.Get(url + "api/state")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"id":"demo"`) {
		t.Errorf("status %d, body %s", res.StatusCode, body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}
