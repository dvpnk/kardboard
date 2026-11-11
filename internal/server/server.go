// Package server is the local web server behind `kb ui`. It only reads: the
// boards are shown as they are on disk, and nothing is changed through it.
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
	"strings"
	"sync"
	"time"

	"kardboard/internal/board"
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
}

// New creates a server. here, if not empty, is a board directory that is
// shown even if the user never added it.
func New(here string) *Server {
	return &Server{here: here, subs: map[chan struct{}]struct{}{}}
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
	Error    string          `json:"error,omitempty"`
	Sync     string          `json:"sync,omitempty"`
	Columns  []columnJSON    `json:"columns"`
	Fields   []fieldJSON     `json:"fields"`
	Cards    []*board.Card   `json:"cards"`
	Problems []board.Problem `json:"problems"`
	Errors   int             `json:"errors"`
	Warnings int             `json:"warnings"`
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

func describe(l *registry.Loaded) boardJSON {
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
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.Handle("GET /", http.FileServerFS(web))
	return guard(mux)
}

// guard keeps other websites away from the boards. The server listens on
// the loopback interface only, but a page in the user's browser could still
// reach it under a made-up host name (DNS rebinding), so requests must name
// the loopback host. It also refuses everything but reading.
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			host = h
		}
		switch strings.Trim(host, "[]") {
		case "127.0.0.1", "localhost", "::1":
		default:
			http.Error(w, "kardboard answers only on localhost", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "kardboard's web UI only reads", http.StatusMethodNotAllowed)
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
		state.Boards = append(state.Boards, describe(l))
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
