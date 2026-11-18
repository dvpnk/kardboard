package server

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dvpnk/kardboard/internal/people"
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
	// Neither Git nor the operating system tells kard who the user is.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	lookup := people.LookupAccount
	people.LookupAccount = func() (*user.User, error) { return nil, errors.New("no account") }
	t.Cleanup(func() { people.LookupAccount = lookup })
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
	Hidden []string
	Boards []struct {
		Key, ID, Name, Root, Source, Error, Sync string
		User                                     *struct{ Name, Source string }
		UserError                                string
		Me                                       []string
		People                                   []struct{ ID, Name string }
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
		Writable         bool
		Git              *struct {
			Branch        string
			Ahead, Behind int
			Dirty         []string
			Unreachable   bool
			Error         string
		}
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
	settings(t, "user: sly\nhidden: [atlas]\n")
	root := copyOfExample(t)
	st := getState(t, New(root), "")

	if len(st.Hidden) != 1 || st.Hidden[0] != "atlas" {
		t.Errorf("hidden %v", st.Hidden)
	}
	if b := st.Boards[0]; b.User == nil || b.User.Name != "sly" || b.User.Source != "user" || fmt.Sprint(b.Me) != "[sly]" {
		t.Errorf("user %+v, me %v", b.User, b.Me)
	}
	if len(st.Boards) != 1 {
		t.Fatalf("got %d boards", len(st.Boards))
	}
	b := st.Boards[0]
	if b.ID != "demo" || b.Name != "Demo project" || b.Source != "here" || b.Key != root || b.Error != "" || b.Sync != "none" {
		t.Errorf("board = %+v", b)
	}
	if len(b.Columns) != 4 || b.Columns[2].ID != "working_on" || !b.Columns[2].Doing || b.Columns[2].Limit != 3 ||
		b.Columns[2].Title != "In progress" || !b.Columns[3].Done {
		t.Errorf("columns = %+v", b.Columns)
	}
	if len(b.Fields) == 0 || b.Fields[0].Name != "assignee" || b.Fields[0].Type != "person" || b.Fields[1].Title != "Size" {
		t.Errorf("fields = %+v", b.Fields)
	}
	if len(b.Cards) != 5 || b.Errors != 0 || b.Warnings != 0 {
		t.Fatalf("%d cards, %d errors, %d warnings", len(b.Cards), b.Errors, b.Warnings)
	}
	for _, c := range b.Cards {
		if c.ID == "export-as-pdf" {
			who, _ := c.Fields["assignee"].([]any)
			if c.Column != "backlog" || c.Title != "Export as PDF" || len(who) != 1 || who[0] != "sly" {
				t.Errorf("card = %+v", c)
			}
			return
		}
	}
	t.Error("card export-as-pdf is missing")
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
	if got := strings.TrimSpace(rec.Body.String()); got != `{"hidden":[],"boards":[]}` {
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

	rec := get(t, s, "/api/card?board=demo&id=export-as-pdf")
	var card struct {
		Board, ID, Title, Body string
		Fields                 map[string]any
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &card); err != nil {
		t.Fatalf("%v\n%s", err, rec.Body)
	}
	if card.Board != "demo" || card.ID != "export-as-pdf" || card.Fields["size"] != "M" ||
		!strings.Contains(card.Body, "# Export as PDF") || !strings.Contains(card.Body, "## Done when") {
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
		"/":                     `<script type="module" src="app.js">`,
		"/theme.js":             "kb.theme",
		"/app.js":               "kardboard web UI",
		"/preact.js":            "export const html",
		"/state.js":             "export const state",
		"/api.js":               "export async function act",
		"/texts.js":             "export const t",
		"/parts.js":             "export function Icon",
		"/top.js":               "export function TopBar",
		"/board.js":             "export function Main",
		"/panels.js":            "export function Detail",
		"/fields.js":            "export function Fields",
		"/md.js":                "export function render",
		"/vendor/preact.mjs":    "createElement",
		"/vendor/hooks.mjs":     "useState",
		"/vendor/htm.module.js": "export default",
		"/app.css":              "--mono:",
		"/favicon.svg":          "<svg",
		"/favicon-32.png":       "PNG",
	} {
		rec := get(t, s, target)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: status %d, body lacks %q", target, rec.Code, want)
		}
	}
	// The fonts are part of the program, with their licence next to them.
	for _, name := range []string{"JetBrainsMono-Regular.woff2", "JetBrainsMono-Medium.woff2"} {
		rec := get(t, s, "/fonts/"+name)
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "wOF2") || rec.Header().Get("Content-Type") != "font/woff2" {
			t.Errorf("%s: status %d, type %q", name, rec.Code, rec.Header().Get("Content-Type"))
		}
		if css := get(t, s, "/app.css").Body.String(); !strings.Contains(css, `url("fonts/`+name+`")`) {
			t.Errorf("app.css does not use %s", name)
		}
	}
	if rec := get(t, s, "/fonts/OFL.txt"); !strings.Contains(rec.Body.String(), "SIL Open Font License, Version 1.1") {
		t.Error("the font licence is not shipped with the fonts")
	}

	// The theme is set by a script file of its own: the page allows no
	// inline scripts, and the choice must apply before the first paint.
	if page := get(t, s, "/").Body.String(); !strings.Contains(page, `<script src="theme.js"></script>`) || strings.Contains(page, "<script>") {
		t.Error("index.html must load theme.js and contain no inline script")
	}
	if css := get(t, s, "/app.css").Body.String(); !strings.Contains(css, `:root[data-theme="dark"]`) || !strings.Contains(css, `:root:not([data-theme="light"])`) {
		t.Error("app.css must let the chosen theme win over the system's")
	}

	rec := get(t, s, "/")
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("Content-Security-Policy = %q", csp)
	}
	if rec := get(t, s, "/nope.js"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown file: status %d", rec.Code)
	}
}

// The page is a set of ES modules that import each other by relative path,
// with no bundler in between. A module the browser cannot find, or that is
// not served as JavaScript, stops the whole page, so every import must name
// a file that is there.
func TestWebModulesResolve(t *testing.T) {
	settings(t, "")
	s := New("")
	importRE := regexp.MustCompile(`(?:import|export)\s*(?:[\w*{}\s,]*?from\s*)?"([^"]+)"`)
	todo := []string{"app.js"}
	seen := map[string]bool{}
	for len(todo) > 0 {
		name := todo[0]
		todo = todo[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		rec := get(t, s, "/"+name)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", name, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
			t.Errorf("%s: Content-Type %q, a module must be served as JavaScript", name, ct)
		}
		for _, m := range importRE.FindAllStringSubmatch(rec.Body.String(), -1) {
			spec := m[1]
			if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
				t.Errorf("%s imports %q: the browser cannot resolve a bare name", name, spec)
				continue
			}
			todo = append(todo, path.Join(path.Dir(name), spec))
		}
	}
	for _, want := range []string{"preact.js", "vendor/preact.mjs", "vendor/hooks.mjs", "vendor/htm.module.js", "md.js"} {
		if !seen[want] {
			t.Errorf("%s is not reached from app.js", want)
		}
	}
}

// The files under web/vendor come from other projects. Their checksums are
// listed in web/vendor/README.md; a file that changes without the list
// changing is noticed here.
func TestVendorFilesAreUnchanged(t *testing.T) {
	want := map[string]string{
		"preact.mjs":         "3a3201e9ccef7d30ef1273427f1b17ef8323e464b2c8d73b4fd5636af730433e",
		"hooks.mjs":          "df45d3c5b39fc012b2397a64d4056ef06d3fbde676c9567a7c12d4d783b6bda3",
		"htm.module.js":      "ab33dd3f38059b9be4d5f5350128eefb2356639c4e0bbe9d9e8b3ba75847e9e4",
		"LICENSE-preact.txt": "1fe6958409c8c257a70c587a18b6f7f412b179b456630790d30b2ec9a8e4b7d4",
		"LICENSE-htm.txt":    "740725f7252e750af735d0028cc534970772f513331e9f68150fede8fb3ce00f",
	}
	entries, err := fs.ReadDir(webFS, "web/vendor")
	if err != nil {
		t.Fatal(err)
	}
	readme, err := fs.ReadFile(webFS, "web/vendor/README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		sum, ok := want[e.Name()]
		if !ok {
			t.Errorf("vendor/%s is not listed here and in vendor/README.md", e.Name())
			continue
		}
		data, err := fs.ReadFile(webFS, "web/vendor/"+e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != sum {
			t.Errorf("vendor/%s has SHA-256 %s, want %s", e.Name(), got, sum)
		}
		if !strings.Contains(string(readme), sum) {
			t.Errorf("vendor/README.md does not list the SHA-256 of %s", e.Name())
		}
		delete(want, e.Name())
	}
	for name := range want {
		t.Errorf("vendor/%s is missing", name)
	}
}

func TestOnlyLocalhost(t *testing.T) {
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
	// Only /api/action changes anything, and only by POST.
	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		for _, target := range []string{"/api/state", "/api/action", "/"} {
			if code := do(method, "127.0.0.1:4712", target); code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status %d, want 405", method, target, code)
			}
		}
	}
}

// post sends an action the way kardboard's own page does.
func post(t *testing.T, s *Server, body string, change func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	return postTo(t, s, "/api/action", body, change)
}

func postTo(t *testing.T, s *Server, target, body string, change func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:4712"+target, strings.NewReader(body))
	req.Header.Set("Origin", "http://127.0.0.1:4712")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ActionHeader, "1")
	if change != nil {
		change(req)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func isFile(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestActions(t *testing.T) {
	settings(t, "user: sly\n")
	root := copyOfExample(t)
	s := New(root)

	steps := []struct {
		body   string
		status int
		want   string // part of the response
		file   string // a file that must exist afterwards
	}{
		{`{"board":"demo","card":"print-templates","action":"take"}`, 200, `"to":"working_on"`, "working_on/print-templates.md"},
		{`{"board":"demo","card":"print-templates","action":"take"}`, 200, `"moved":false`, "working_on/print-templates.md"},
		{`{"board":"demo","card":"print-templates","action":"assign","people":["arnold"],"add":true}`, 200, `"assignee":["sly","arnold"]`, ""},
		{`{"board":"demo","card":"print-templates","action":"unassign","people":["sly"]}`, 200, `"assignee":["arnold"]`, ""},
		// now it is arnold's card: sly is refused, and may force it
		{`{"board":"demo","card":"print-templates","action":"done"}`, 409, `"refused":true`, "working_on/print-templates.md"},
		{`{"board":"demo","card":"print-templates","action":"done","force":true}`, 200, `"to":"done"`, "done/print-templates.md"},
		{`{"board":"demo","card":"offline-mode","action":"move","column":"backlog"}`, 409, "would lack size", "planning/offline-mode.md"},
		{`{"board":"demo","card":"offline-mode","action":"set","set":["size=S","tags=ui"]}`, 200, `"size":"S"`, ""},
		// a text read before the file changed is not saved; one without a base is
		{`{"board":"demo","card":"offline-mode","action":"set","text":{"title":"Offline mode","body":"Neu."},"base":"0123"}`, 409, "changed since it was opened", ""},
		{`{"board":"demo","card":"offline-mode","action":"set","text":{"title":"Offline mode","body":"Neu."}}`, 200, `"text":true`, ""},
		{`{"board":"demo","card":"offline-mode","action":"move","column":"backlog"}`, 200, `"to":"backlog"`, "backlog/offline-mode.md"},

		{`{"board":"demo","card":"offline-mode","action":"move"}`, 400, "move needs a column", ""},
		{`{"board":"demo","card":"offline-mode","action":"move","column":"nope"}`, 400, "no column", ""},
		{`{"board":"demo","card":"offline-mode","action":"set","set":["size=XXL"]}`, 400, "size: must be one of", ""},
		{`{"board":"demo","card":"offline-mode","action":"set"}`, 400, "set needs", ""},
		{`{"board":"demo","card":"offline-mode","action":"assign"}`, 400, "no person named", ""},
		{`{"board":"demo","card":"offline-mode","action":"delete"}`, 400, "unknown action", ""},
		{`{"board":"demo","card":"nope","action":"done"}`, 400, "no card", ""},
		{`{"board":"nope","card":"x","action":"done"}`, 404, "no board", ""},
		{`{"board":"demo","card":"x","action":"done","extra":1}`, 400, "cannot read the request", ""},
		{`not json`, 400, "cannot read the request", ""},
	}
	for _, st := range steps {
		rec := post(t, s, st.body, nil)
		if rec.Code != st.status || !strings.Contains(rec.Body.String(), st.want) {
			t.Fatalf("%s\n  status %d, want %d\n  body %s  want it to contain %s", st.body, rec.Code, st.status, rec.Body, st.want)
		}
		if st.file != "" && !isFile(root, st.file) {
			t.Errorf("%s: %s is missing", st.body, st.file)
		}
	}

	if got, want := readFile(t, root, "done/print-templates.md"), "assignee: arnold\n"; !strings.Contains(got, want) {
		t.Errorf("file = %q", got)
	}
	if got := readFile(t, root, "backlog/offline-mode.md"); !strings.HasSuffix(got, "# Offline mode\n\nNeu.\n") {
		t.Errorf("text = %q", got)
	}
	// Everything the page did leaves a valid board.
	if st := getState(t, s, ""); st.Boards[0].Errors != 0 {
		t.Errorf("the board has %d errors after the actions", st.Boards[0].Errors)
	}
}

func TestActionsNeedTheUserOnlyWhenItMatters(t *testing.T) {
	settings(t, "")
	root := copyOfExample(t)
	s := New(root)

	// Nobody's card moves without kard knowing who asks.
	if rec := post(t, s, `{"board":"demo","card":"print-templates","action":"move","column":"done"}`, nil); rec.Code != 200 {
		t.Errorf("move: status %d, %s", rec.Code, rec.Body)
	}
	for body, want := range map[string]string{
		`{"board":"demo","card":"offline-mode","action":"take"}`:             "does not know who you are",
		`{"board":"demo","card":"login-fails-with-umlauts","action":"done"}`: "must know who you are",
	} {
		rec := post(t, s, body, nil)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: status %d, body %s", body, rec.Code, rec.Body)
		}
	}
	if !isFile(root, "working_on/login-fails-with-umlauts.md") || !isFile(root, "planning/offline-mode.md") {
		t.Error("a rejected action changed the board")
	}
}

// git runs Git in dir with settings of its own, not those of whoever runs
// the tests.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=arnold", "-c", "user.email=arnold@example.com"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// syncBoard is a copy of the example with the given sync mode.
func syncBoard(t *testing.T, mode string) string {
	t.Helper()
	root := copyOfExample(t)
	yaml := strings.Replace(readFile(t, root, "board.yaml"), "sync: none", "sync: "+mode, 1)
	writeFile(t, filepath.Join(root, "board.yaml"), yaml)
	return root
}

func commitAll(t *testing.T, root string) {
	t.Helper()
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "user.name", "arnold")
	git(t, root, "config", "user.email", "arnold@example.com")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "start")
}

func TestSyncBoardOutsideARepositoryIsReadOnly(t *testing.T) {
	settings(t, "")
	root := syncBoard(t, "commit")
	s := New(root)

	b := getState(t, s, "").Boards[0]
	if b.Writable || b.Sync != "commit" || b.Git == nil || !strings.Contains(b.Git.Error, "not in a Git repository") {
		t.Errorf("board = writable %v, sync %q, git %+v", b.Writable, b.Sync, b.Git)
	}
	rec := post(t, s, `{"board":"demo","card":"print-templates","action":"move","column":"done"}`, nil)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "not in a Git repository") || !isFile(root, "backlog/print-templates.md") {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
}

func TestActionOnACommitBoardMakesACommit(t *testing.T) {
	settings(t, "")
	root := syncBoard(t, "commit")
	commitAll(t, root)
	s := New(root)

	if b := getState(t, s, "").Boards[0]; !b.Writable || b.Git == nil || b.Git.Branch != "main" || b.Git.Error != "" {
		t.Fatalf("board = writable %v, git %+v", b.Writable, b.Git)
	}
	rec := post(t, s, `{"board":"demo","card":"print-templates","action":"move","column":"done","force":true}`, nil)
	var res struct {
		Moved           bool
		Commit, Warning string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || rec.Code != 200 || !res.Moved || res.Commit == "" || res.Warning != "" {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if got := git(t, root, "log", "-1", "--format=%s"); got != "move print-templates: backlog -> done" {
		t.Errorf("subject = %q", got)
	}

	// A card with changes that are not committed is listed and left alone.
	writeFile(t, filepath.Join(root, "planning", "offline-mode.md"), readFile(t, root, "planning/offline-mode.md")+"\nmehr\n")
	if b := getState(t, s, "").Boards[0]; len(b.Git.Dirty) != 1 || b.Git.Dirty[0] != "offline-mode" {
		t.Errorf("dirty = %v", b.Git.Dirty)
	}
	rec = post(t, s, `{"board":"demo","card":"offline-mode","action":"move","column":"done","force":true}`, nil)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "not committed") {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
}

func TestPushBoardFollowsItsRemote(t *testing.T) {
	settings(t, "")
	seed := syncBoard(t, "push")
	commitAll(t, seed)
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, seed, "clone", "-q", "--bare", seed, remote)
	clone := func(name string) string {
		dir := filepath.Join(t.TempDir(), name)
		git(t, seed, "clone", "-q", remote, dir)
		git(t, dir, "config", "user.name", name)
		git(t, dir, "config", "user.email", name+"@example.com")
		real, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatal(err)
		}
		return real
	}
	mine, theirs := clone("arnold"), clone("sly")
	s := New(mine)

	// An action is pushed.
	rec := post(t, s, `{"board":"demo","card":"print-templates","action":"move","column":"done","force":true}`, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"pushed":true`) {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if got := git(t, remote, "log", "-1", "--format=%s"); got != "move print-templates: backlog -> done" {
		t.Errorf("remote subject = %q", got)
	}

	// Someone else pushes. The page learns of it with the next fetch and
	// gets it with a sync.
	git(t, theirs, "pull", "-q", "--rebase")
	git(t, theirs, "mv", "planning/offline-mode.md", "backlog/offline-mode.md")
	git(t, theirs, "commit", "-q", "-m", "theirs")
	git(t, theirs, "push", "-q")

	if b := getState(t, s, "").Boards[0]; b.Git.Behind != 0 {
		t.Errorf("behind = %d before any fetch", b.Git.Behind)
	}
	s.fetchAll(context.Background())
	if b := getState(t, s, "").Boards[0]; b.Git.Behind != 1 || b.Git.Ahead != 0 || b.Git.Unreachable {
		t.Errorf("git = %+v", b.Git)
	}
	rec = postTo(t, s, "/api/sync", `{"board":"demo"}`, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"pulled":1`) || !isFile(mine, "backlog/offline-mode.md") {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}

	// A remote that is gone is reported, and the action is refused whole.
	git(t, mine, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	s.fetchAll(context.Background())
	if b := getState(t, s, "").Boards[0]; !b.Git.Unreachable {
		t.Errorf("git = %+v, want unreachable", b.Git)
	}
	rec = post(t, s, `{"board":"demo","card":"offline-mode","action":"move","column":"done","force":true}`, nil)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "nothing was changed") || !isFile(mine, "backlog/offline-mode.md") {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
}

// A page on another website can make the browser send requests to the
// port. None of them may change a card.
func TestOtherSitesCannotChangeCards(t *testing.T) {
	settings(t, "user: sly\n")
	root := copyOfExample(t)
	s := New(root)
	const body = `{"board":"demo","card":"print-templates","action":"move","column":"done"}`

	attacks := map[string]func(*http.Request){
		"another site as origin":           func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") },
		"no origin at all":                 func(r *http.Request) { r.Header.Del("Origin") },
		"origin null (sandboxed frame)":    func(r *http.Request) { r.Header.Set("Origin", "null") },
		"another port on this machine":     func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:8080") },
		"localhost under another name":     func(r *http.Request) { r.Header.Set("Origin", "http://localhost:4712") },
		"https instead of http":            func(r *http.Request) { r.Header.Set("Origin", "https://127.0.0.1:4712") },
		"origin that only starts the same": func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:4712.evil.example") },
		"a plain form: no custom header":   func(r *http.Request) { r.Header.Del(ActionHeader) },
		"a plain form: form content type": func(r *http.Request) {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		},
		"a plain form: text content type": func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
		"a made-up host name (DNS rebinding)": func(r *http.Request) {
			r.Host = "evil.example:4712"
			r.Header.Set("Origin", "http://evil.example:4712")
		},
	}
	for name, attack := range attacks {
		rec := post(t, s, body, attack)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, rec.Code)
		}
	}
	if !isFile(root, "backlog/print-templates.md") {
		t.Fatal("an attack moved the card")
	}

	// A browser asks first before it lets another site send the custom
	// header. The server does not grant that.
	req := httptest.NewRequest(http.MethodOptions, "http://127.0.0.1:4712/api/action", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "x-kardboard,content-type")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code < 400 || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("preflight: status %d, allow-origin %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// The page's own request goes through, also when it was opened as localhost.
	rec = post(t, s, body, func(r *http.Request) {
		r.Host = "localhost:4712"
		r.Header.Set("Origin", "http://localhost:4712")
	})
	if rec.Code != http.StatusOK || !isFile(root, "done/print-templates.md") {
		t.Errorf("own page: status %d, body %s", rec.Code, rec.Body)
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

func TestHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	settings(t, "")
	root := copyOfExample(t)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-C", root, "-c", "user.name=Arnold Example", "-c", "user.email=arnold@example.com",
			"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null",
		}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	s := New(root)

	type history struct {
		Available bool
		Reason    string
		Branch    string
		Entries   []struct {
			Short, Author, Date, Subject string
			Changes                      []struct{ Kind, Card, From, To string }
		}
	}
	read := func(query string) (int, history) {
		t.Helper()
		rec := get(t, s, "/api/history?"+query)
		var h history
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &h); err != nil {
				t.Fatalf("%v\n%s", err, rec.Body)
			}
		}
		return rec.Code, h
	}

	// A board outside a repository has no history, and says so.
	code, h := read("board=demo")
	if code != http.StatusOK || h.Available || h.Reason == "" || h.Entries == nil {
		t.Errorf("without Git: code %d, %+v", code, h)
	}

	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-q", "-m", "Board anlegen")
	// A move through the page is a rename on disk; the user commits it.
	if rec := post(t, s, `{"board":"demo","card":"print-templates","action":"move","column":"done"}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("move: %d %s", rec.Code, rec.Body)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "Print templates fertig")

	code, h = read("board=demo")
	if code != http.StatusOK || !h.Available || h.Branch != "main" || len(h.Entries) != 2 {
		t.Fatalf("board history: code %d, %+v", code, h)
	}
	top := h.Entries[0]
	if top.Subject != "Print templates fertig" || top.Author != "Arnold Example" || len(top.Changes) != 1 ||
		top.Changes[0].Kind != "moved" || top.Changes[0].Card != "print-templates" || top.Changes[0].From != "backlog" || top.Changes[0].To != "done" {
		t.Errorf("newest entry = %+v", top)
	}

	code, h = read("board=demo&card=print-templates")
	if code != http.StatusOK || len(h.Entries) != 2 || h.Entries[1].Changes[0].Kind != "added" {
		t.Errorf("card history: code %d, %+v", code, h)
	}
	if code, h = read("board=demo&limit=1"); len(h.Entries) != 1 {
		t.Errorf("limit: code %d, %d entries", code, len(h.Entries))
	}
	// A limit that is no number, zero or huge must not break the request.
	for _, q := range []string{"board=demo&limit=abc", "board=demo&limit=0", "board=demo&limit=999999"} {
		if code, _ = read(q); code != http.StatusOK {
			t.Errorf("%s: code %d", q, code)
		}
	}
	for _, q := range []string{"board=nope", "board=demo&card=nope", ""} {
		if code, _ = read(q); code != http.StatusNotFound {
			t.Errorf("%q: code %d, want 404", q, code)
		}
	}
}
