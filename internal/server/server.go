// Package server is the local web server behind `kb ui`. It shows the boards
// as they are on disk and changes cards through the same actions as the
// command line.
package server

import (
	"context"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"kardboard/internal/board"
	"kardboard/internal/gitlog"
	"kardboard/internal/gitsync"
	"kardboard/internal/registry"
	"kardboard/internal/userconfig"
)

//go:embed web
var webFS embed.FS

const (
	// rescanEvery is how often the directories under `scan` are searched
	// again for new boards. Searching walks whole directory trees, so it
	// runs far less often than the check for changed cards.
	rescanEvery = 30 * time.Second
	// DefaultPoll is how often the known boards are checked for changes.
	DefaultPoll = time.Second
	// fetchEvery is how often boards with sync: push ask their remote what
	// is new.
	fetchEvery = time.Minute
)

// Server serves the boards known on this machine.
type Server struct {
	// here is the board of the directory kb was started in, or "".
	here string

	mu      sync.Mutex
	entries []registry.Entry
	scanned time.Time
	cfgMod  time.Time

	subMu sync.Mutex
	subs  map[chan struct{}]struct{}

	// writeMu lets one change happen at a time. Fetching holds it too, so
	// Git never runs twice in one repository.
	writeMu sync.Mutex

	// unreachable holds, per board directory, why the last fetch failed.
	remoteMu    sync.Mutex
	unreachable map[string]string
}

// New creates a server. here, if not empty, is a board directory that is
// shown even if the user never added it.
func New(here string) *Server {
	return &Server{here: here, subs: map[chan struct{}]struct{}{}, unreachable: map[string]string{}}
}

// known returns the board directories, searching the scan directories again
// if the last search is old, the settings changed, or force is set.
func (s *Server) known(force bool) []registry.Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	var mod time.Time
	if path, err := userconfig.Path(); err == nil {
		if info, err := os.Stat(path); err == nil {
			mod = info.ModTime()
		}
	}
	if force || s.entries == nil || time.Since(s.scanned) > rescanEvery || !mod.Equal(s.cfgMod) {
		cfg, err := userconfig.Load()
		if err != nil {
			cfg = &userconfig.Config{} // a broken settings file must not take the boards down
		}
		s.entries = registry.Entries(cfg, s.here)
		if s.entries == nil {
			s.entries = []registry.Entry{}
		}
		s.scanned, s.cfgMod = time.Now(), mod
	}
	return s.entries
}

// ── JSON ────────────────────────────────────────────────────────────────────

type stateJSON struct {
	// User is the user's own name, or "" if kb does not know it.
	User string `json:"user"`
	// Hidden lists the ids of boards the settings hide by default.
	Hidden []string    `json:"hidden"`
	Boards []boardJSON `json:"boards"`
}

type boardJSON struct {
	// Key identifies the board in this response even if it has no usable id.
	Key    string `json:"key"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Root   string `json:"root"`
	Source string `json:"source"`
	// Error says why the board cannot be shown. Columns and cards are
	// empty then.
	Error string `json:"error,omitempty"`
	Sync  string `json:"sync,omitempty"`
	// Writable says whether cards can be changed here. A board that wants
	// Git commits but is not in a repository cannot; Git.Error says why.
	Writable bool `json:"writable"`
	// Git is set for boards with sync: commit or push.
	Git      *gitJSON        `json:"git,omitempty"`
	Columns  []columnJSON    `json:"columns"`
	Fields   []fieldJSON     `json:"fields"`
	Cards    []*board.Card   `json:"cards"`
	Problems []board.Problem `json:"problems"`
	Errors   int             `json:"errors"`
	Warnings int             `json:"warnings"`
}

// gitJSON says how a board's repository stands against its remote.
type gitJSON struct {
	gitsync.Status
	// Unreachable is true if the last attempt to ask the remote failed.
	// Ahead and Behind are then as old as the last attempt that worked.
	Unreachable bool `json:"unreachable,omitempty"`
	// Error says why the board's sync mode cannot work here.
	Error string `json:"error,omitempty"`
}

type columnJSON struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Doing bool   `json:"doing,omitempty"`
	Done  bool   `json:"done,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type fieldJSON struct {
	Name   string   `json:"name"`
	Title  string   `json:"title"`
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
}

type cardJSON struct {
	Board string `json:"board"`
	*board.Card
	Body string `json:"body"`
}

func (s *Server) describe(ctx context.Context, l *registry.Loaded) boardJSON {
	out := boardJSON{
		Key:      l.Root,
		Name:     filepath.Base(l.Root),
		Root:     userconfig.Abbrev(l.Root),
		Source:   string(l.Source),
		Columns:  []columnJSON{},
		Fields:   []fieldJSON{},
		Cards:    []*board.Card{},
		Problems: []board.Problem{},
	}
	if l.Board == nil {
		out.Error = l.Err
		return out
	}
	b := l.Board
	if b.Config == nil {
		var msgs []string
		for _, p := range b.BoardProblems() {
			msgs = append(msgs, p.Message)
		}
		out.Error = "board.yaml: " + strings.Join(msgs, "; ")
		return out
	}
	out.ID, out.Sync = b.Config.ID, b.Config.Sync
	out.Writable = true
	if b.Config.Sync != gitsync.ModeNone {
		st, err := gitsync.ReadStatus(ctx, b)
		out.Git = &gitJSON{Status: st}
		if err != nil {
			out.Git.Error, out.Writable = err.Error(), false
		}
		s.remoteMu.Lock()
		out.Git.Unreachable = s.unreachable[b.Root] != ""
		s.remoteMu.Unlock()
	}
	if b.Config.Name != "" {
		out.Name = b.Config.Name
	} else {
		out.Name = b.Config.ID
	}
	if l.Err != "" {
		out.Error = l.Err
		return out
	}
	for _, c := range b.Config.Columns {
		out.Columns = append(out.Columns, columnJSON{ID: c.ID, Title: c.Label(), Doing: c.Doing, Done: c.Done, Limit: c.Limit})
	}
	for _, f := range b.Config.AllFields() {
		out.Fields = append(out.Fields, fieldJSON{Name: f.Name, Title: f.Label(), Type: string(f.Type), Values: f.Values})
	}
	out.Cards = append(out.Cards, b.Cards...)
	out.Problems = append(out.Problems, b.BoardProblems()...)
	for _, p := range b.Problems() {
		if p.Severity == board.SeverityError {
			out.Errors++
		} else {
			out.Warnings++
		}
	}
	return out
}

// ── Handlers ────────────────────────────────────────────────────────────────

// Handler returns the HTTP handler: the API under /api/ and the web UI
// everywhere else.
func (s *Server) Handler() http.Handler {
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/card", s.handleCard)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("POST /api/action", s.handleAction)
	mux.HandleFunc("POST /api/sync", s.handleSync)
	mux.Handle("GET /", http.FileServerFS(web))
	return guard(mux)
}

// ActionHeader must be present on every changing request, see guard.
const ActionHeader = "X-Kardboard"

func isLoopback(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	switch strings.Trim(host, "[]") {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// guard keeps other websites away from the boards. The server listens on
// the loopback interface only, but every page in the user's browser can
// still send requests to it, so three things are checked.
//
// Reading: the request must name the loopback host. That stops a page that
// reaches the port under a made-up host name (DNS rebinding).
//
// Changing: the request must also come from kardboard's own page (same
// origin) and carry a header that browsers only let a page send to its own
// origin. A form or script on another site can do neither.
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r.Host) {
			http.Error(w, "kardboard answers only on localhost", http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead:
		case http.MethodPost:
			origin := r.Header.Get("Origin")
			if origin != "http://"+r.Host || r.Header.Get(ActionHeader) != "1" ||
				!strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				http.Error(w, "changes are only accepted from kardboard's own page", http.StatusForbidden)
				return
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	state := stateJSON{Hidden: []string{}, Boards: []boardJSON{}}
	state.User, _ = userconfig.User()
	if cfg, err := userconfig.Load(); err == nil && cfg.Hidden != nil {
		state.Hidden = cfg.Hidden
	}
	for _, l := range registry.Load(s.known(r.URL.Query().Has("rescan"))) {
		state.Boards = append(state.Boards, s.describe(r.Context(), l))
	}
	writeJSON(w, state)
}

func (s *Server) handleCard(w http.ResponseWriter, r *http.Request) {
	boardID, cardID := r.URL.Query().Get("board"), r.URL.Query().Get("id")
	l, ok := registry.Find(registry.Load(s.known(false)), boardID)
	if !ok {
		http.Error(w, fmt.Sprintf("no board %q", boardID), http.StatusNotFound)
		return
	}
	c, ok := l.Board.Card(cardID)
	if !ok {
		http.Error(w, fmt.Sprintf("no card %q on board %s", cardID, boardID), http.StatusNotFound)
		return
	}
	writeJSON(w, cardJSON{Board: boardID, Card: c, Body: c.Body})
}

// handleHistory returns the Git history of a board, or of one of its cards.
// A board outside a Git repository has none; the answer then says so.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	l, ok := registry.Find(registry.Load(s.known(false)), q.Get("board"))
	if !ok {
		http.Error(w, fmt.Sprintf("no board %q", q.Get("board")), http.StatusNotFound)
		return
	}
	limit := 60
	if n, err := strconv.Atoi(q.Get("limit")); err == nil {
		limit = min(max(n, 1), 200)
	}
	cardPath := ""
	if id := q.Get("card"); id != "" {
		c, ok := l.Board.Card(id)
		if !ok {
			http.Error(w, fmt.Sprintf("no card %q on board %s", id, q.Get("board")), http.StatusNotFound)
			return
		}
		cardPath = c.Path
	}
	columns := make([]string, len(l.Board.Config.Columns))
	for i, c := range l.Board.Config.Columns {
		columns[i] = c.ID
	}
	writeJSON(w, gitlog.Read(r.Context(), l.Board.Root, columns, cardPath, limit))
}

// ── Changing ────────────────────────────────────────────────────────────────

// actionRequest names one action on one card. The actions are those of the
// command line and do the same.
type actionRequest struct {
	Board string `json:"board"`
	Card  string `json:"card"`
	// Action is move, take, done, assign, unassign or set.
	Action string `json:"action"`
	// Column is the target of move.
	Column string `json:"column"`
	// People are the names for assign and unassign.
	People []string `json:"people"`
	// Add makes take and assign keep the people already on the card.
	Add bool `json:"add"`
	// Set holds "field=value" for set.
	Set []string `json:"set"`
	// Force overrides the rules that protect other people's cards and
	// required fields, like --force.
	Force bool `json:"force"`
}

type actionError struct {
	Error string `json:"error"`
	// Refused is true if a rule of the board forbids the action and Force
	// would do it anyway.
	Refused bool `json:"refused,omitempty"`
}

func fail(w http.ResponseWriter, status int, err error, refused bool) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(actionError{Error: err.Error(), Refused: refused})
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	var req actionRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("cannot read the request: %w", err), false)
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Read the board afresh: the change must see the files as they are now.
	l, ok := registry.Find(registry.Load(s.known(false)), req.Board)
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("no board %q", req.Board), false)
		return
	}
	// With sync: push this pulls, so the action is built from what the
	// others have done meanwhile.
	b, err := gitsync.Prepare(r.Context(), l.Board)
	if err != nil {
		s.broadcast() // pulling may have changed files even though it failed later
		fail(w, http.StatusBadRequest, err, false)
		return
	}
	user, userErr := userconfig.User()

	var ch board.Change
	switch req.Action {
	case "move":
		if req.Column == "" {
			err = errors.New("move needs a column")
		}
		ch = board.Change{Column: req.Column}
	case "take":
		if userErr != nil {
			err = userErr
		} else {
			ch = b.TakeChange(req.Card, user, req.Add)
		}
	case "done":
		ch, err = b.DoneChange()
	case "assign":
		ch, err = b.AssignChange(req.Card, req.People, req.Add)
	case "unassign":
		ch = b.UnassignChange(req.Card, req.People)
	case "set":
		if len(req.Set) == 0 {
			err = errors.New("set needs at least one field=value")
		}
		ch = board.Change{Set: req.Set}
	default:
		err = fmt.Errorf("unknown action %q", req.Action)
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err, false)
		return
	}
	ch.User, ch.Force = user, req.Force

	res, err := gitsync.Change(r.Context(), b, req.Card, ch)
	var refused *board.RefusedError
	var conflict *gitsync.ConflictError
	switch {
	case res != nil:
		// The change happened. If it could not be pushed, the answer says so.
		out := actionResult{Result: res}
		if err != nil {
			out.Warning = err.Error()
		}
		s.broadcast()
		writeJSON(w, out)
	case errors.As(err, &conflict):
		s.broadcast() // the card is now where the others put it
		fail(w, http.StatusConflict, err, false)
	case errors.As(err, &refused):
		fail(w, http.StatusConflict, err, true)
	case errors.Is(err, board.ErrNeedUser):
		if userErr == nil {
			userErr = err
		}
		fail(w, http.StatusBadRequest, fmt.Errorf("the card is assigned to someone, so kb must know who you are: %w", userErr), false)
	default:
		fail(w, http.StatusBadRequest, err, false)
	}
}

// actionResult is the answer to an action that happened.
type actionResult struct {
	*gitsync.Result
	// Warning is set if the change is committed but could not be pushed.
	Warning string `json:"warning,omitempty"`
}

// handleSync pulls and pushes one board with sync: push, like `kb sync`.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Board string `json:"board"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("cannot read the request: %w", err), false)
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	l, ok := registry.Find(registry.Load(s.known(false)), req.Board)
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("no board %q", req.Board), false)
		return
	}
	res, err := gitsync.Sync(r.Context(), l.Board)
	s.setReachable(l.Board.Root, nil)
	s.broadcast()
	if err != nil {
		fail(w, http.StatusBadRequest, err, false)
		return
	}
	writeJSON(w, res)
}

func (s *Server) setReachable(root string, err error) {
	s.remoteMu.Lock()
	defer s.remoteMu.Unlock()
	if err != nil {
		s.unreachable[root] = err.Error()
	} else {
		delete(s.unreachable, root)
	}
}

// fetchAll asks the remote of every board with sync: push what is new and
// tells the open pages if the answer changed anything they show.
func (s *Server) fetchAll(ctx context.Context) {
	changed := false
	for _, l := range registry.Load(s.known(false)) {
		if ctx.Err() != nil {
			return
		}
		b := l.Board
		if b == nil || b.Config == nil || b.Config.Sync != gitsync.ModePush || gitsync.Usable(ctx, b) != nil {
			continue
		}
		s.remoteMu.Lock()
		was := s.unreachable[b.Root] != ""
		s.remoteMu.Unlock()
		before, _ := gitsync.ReadStatus(ctx, b)

		s.writeMu.Lock()
		err := gitsync.Fetch(ctx, b)
		s.writeMu.Unlock()
		s.setReachable(b.Root, err)

		after, _ := gitsync.ReadStatus(ctx, b)
		if before.Ahead != after.Ahead || before.Behind != after.Behind || was != (err != nil) {
			changed = true
		}
	}
	if changed {
		s.broadcast()
	}
}

// handleEvents keeps a connection open and sends one event whenever a board
// changed on disk. The page then asks for the state again.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	ch := make(chan struct{}, 1)
	s.subMu.Lock()
	s.subs[ch] = struct{}{}
	s.subMu.Unlock()
	defer func() {
		s.subMu.Lock()
		delete(s.subs, ch)
		s.subMu.Unlock()
	}()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprint(w, "event: change\ndata: {}\n\n")
		case <-keepAlive.C:
			fmt.Fprint(w, ": still here\n\n")
		}
		flusher.Flush()
	}
}

func (s *Server) broadcast() {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default: // an event is already waiting for this page
		}
	}
}

// ── Watching ────────────────────────────────────────────────────────────────

// Watch checks the boards for changes until ctx ends and tells the open
// pages about each one. It compares names, sizes and modification times, so
// it needs no support from the operating system and works on every platform.
func (s *Server) Watch(ctx context.Context, every time.Duration) {
	last := s.fingerprint()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	// Asking the remotes can take a while, so it runs beside the watching.
	go func() {
		fetch := time.NewTicker(fetchEvery)
		defer fetch.Stop()
		for {
			s.fetchAll(ctx)
			select {
			case <-ctx.Done():
				return
			case <-fetch.C:
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if now := s.fingerprint(); now != last {
				last = now
				s.broadcast()
			}
		}
	}
}

// fingerprint sums up what is on disk: which boards are known, and for each
// one its board.yaml and the files of its directories one level down.
func (s *Server) fingerprint() string {
	h := sha1.New()
	note := func(path string, info fs.FileInfo) {
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, info.Size(), info.ModTime().UnixNano())
	}
	if path, err := userconfig.Path(); err == nil {
		if info, err := os.Stat(path); err == nil {
			note(path, info)
		}
	}
	for _, e := range s.known(false) {
		fmt.Fprintf(h, "board %s\n", e.Root)
		top, err := os.ReadDir(e.Root)
		if err != nil {
			continue
		}
		for _, d := range top {
			if strings.HasPrefix(d.Name(), ".") {
				continue
			}
			path := filepath.Join(e.Root, d.Name())
			if !d.IsDir() {
				if info, err := d.Info(); err == nil {
					note(path, info)
				}
				continue
			}
			files, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			for _, f := range files {
				if f.IsDir() || strings.HasPrefix(f.Name(), ".") {
					continue
				}
				if info, err := f.Info(); err == nil {
					note(filepath.Join(path, f.Name()), info)
				}
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ── Running ─────────────────────────────────────────────────────────────────

// Run serves on the loopback interface until ctx ends. ready is called with
// the address once the server accepts connections.
func (s *Server) Run(ctx context.Context, port int, ready func(url string)) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d (is kb ui already running? try --port): %w", port, err)
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.Watch(ctx, DefaultPoll)
	go func() {
		<-ctx.Done()
		// Open event streams never finish on their own, so do not wait for them.
		_ = srv.Close()
	}()

	if ready != nil {
		ready(fmt.Sprintf("http://%s/", ln.Addr()))
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
