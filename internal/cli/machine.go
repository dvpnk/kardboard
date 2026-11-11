package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"kardboard/internal/board"
	"kardboard/internal/gitsync"
	"kardboard/internal/registry"
	"kardboard/internal/server"
	"kardboard/internal/userconfig"
)

// known loads every board known on this machine. here, if not empty, is a
// board directory to include even if the user never added it.
func (a *app) known(here string) ([]*registry.Loaded, error) {
	cfg, err := userconfig.Load()
	if err != nil {
		return nil, err
	}
	return registry.Load(registry.Entries(cfg, here)), nil
}

// here returns the board of the current directory or of -C, or "" if there
// is none. Commands that work on all boards do not need one.
func (a *app) here() string {
	if a.dir != "" {
		return userconfig.Expand(a.dir)
	}
	if root, err := board.FindRoot("."); err == nil {
		return root
	}
	return ""
}

// ── kb boards ───────────────────────────────────────────────────────────────

type boardRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Source string `json:"source"`
	Cards  int    `json:"cards"`
	Errors int    `json:"errors"`
	// Error says why the board is unusable, if it is.
	Error string `json:"error,omitempty"`
}

func row(l *registry.Loaded) boardRow {
	r := boardRow{Path: userconfig.Abbrev(l.Root), Source: string(l.Source), Error: l.Err}
	if l.Board == nil {
		return r
	}
	for _, p := range l.Board.Problems() {
		if p.Severity == board.SeverityError {
			r.Errors++
		}
	}
	if cfg := l.Board.Config; cfg != nil {
		r.ID, r.Name, r.Cards = cfg.ID, cfg.Name, len(l.Board.Cards)
		if r.Name == "" {
			r.Name = cfg.ID
		}
	} else if r.Error == "" {
		r.Error = "board.yaml is missing or invalid, see `kb lint -C " + r.Path + "`"
	}
	return r
}

func (a *app) boardsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "boards",
		Short: "List the boards known on this machine",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			loaded, err := a.known("")
			if err != nil {
				return err
			}
			rows := make([]boardRow, len(loaded))
			for i, l := range loaded {
				rows[i] = row(l)
			}
			if a.asJSON {
				return a.printJSON(rows)
			}
			if len(rows) == 0 {
				fmt.Fprintln(a.out, "No boards yet. Add one with `kb add <dir>` or a whole directory of projects with `kb scan <dir>`.")
				return nil
			}
			width := 2
			for _, r := range rows {
				width = max(width, len(r.ID))
			}
			fmt.Fprintf(a.out, "%-*s  %5s  %-6s  %s\n", width, "ID", "CARDS", "SOURCE", "PATH")
			for _, r := range rows {
				id, note := r.ID, ""
				if id == "" {
					id = "-"
				}
				switch {
				case r.Error != "":
					note = "  ! " + r.Error
				case r.Errors > 0:
					note = "  ! " + plural(r.Errors, "error")
				}
				fmt.Fprintf(a.out, "%-*s  %5d  %-6s  %s%s\n", width, id, r.Cards, r.Source, r.Path, note)
			}
			return nil
		},
	}
}

// ── kb init ─────────────────────────────────────────────────────────────────

type initResult struct {
	*board.InitResult
	Cards  int `json:"cards"`
	Errors int `json:"errors"`
	// Added says whether the board is now known on this machine.
	Added bool `json:"added"`
}

func (a *app) initCmd() *cobra.Command {
	var opt board.InitOptions
	var columns string
	var noAdd bool
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Turn a directory into a board",
		Long:  "Write a board.yaml into a directory. Directories in it that already hold Markdown files become the columns, and no file is changed or moved. An empty directory gets the columns backlog, working_on and done. The board is then made known on this machine, like `kb add`.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir := "."
			switch {
			case len(args) == 1:
				dir = userconfig.Expand(args[0])
			case a.dir != "":
				dir = userconfig.Expand(a.dir)
			}
			for _, c := range strings.Split(columns, ",") {
				if c = strings.TrimSpace(c); c != "" {
					opt.Columns = append(opt.Columns, c)
				}
			}
			made, err := board.Init(dir, opt)
			if err != nil {
				return err
			}
			res := initResult{InitResult: made}
			b, err := board.Load(made.Root)
			if err != nil {
				return err
			}
			res.Cards = len(b.Cards)
			for _, p := range b.Problems() {
				if p.Severity == board.SeverityError {
					res.Errors++
				}
			}

			// Not being able to add the board is no reason to fail: the
			// board exists, and `kb add` can be repeated.
			var addErr error
			if !noAdd {
				addErr = a.addKnown(made.Root, made.ID)
				res.Added = addErr == nil
			}
			if a.asJSON {
				return a.printJSON(res)
			}
			how := "created"
			if made.Adopted {
				how = "found"
			}
			fmt.Fprintf(a.out, "%s: board in %s\n", made.ID, userconfig.Abbrev(made.Root))
			fmt.Fprintf(a.out, "  columns %s: %s\n", how, strings.Join(made.Columns, ", "))
			if res.Cards > 0 {
				fmt.Fprintf(a.out, "  %s", plural(res.Cards, "card"))
				if res.Errors > 0 {
					fmt.Fprintf(a.out, ", %s; see `kb lint -C %s`", plural(res.Errors, "error"), userconfig.Abbrev(made.Root))
				}
				fmt.Fprintln(a.out)
			}
			switch {
			case addErr != nil:
				fmt.Fprintf(a.errOut, "kb: the board is not added to this machine: %s\n", addErr)
			case res.Added:
				fmt.Fprintln(a.out, "  added to this machine, `kb ui` shows it")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opt.ID, "id", "", "id of the board (default: derived from the directory name)")
	cmd.Flags().StringVar(&opt.Name, "name", "", "display name of the board")
	cmd.Flags().StringVar(&opt.Sync, "sync", "", "none, commit or push (default: none)")
	cmd.Flags().StringVar(&columns, "columns", "", "columns in order, separated by commas (default: the directories that hold cards)")
	cmd.Flags().BoolVar(&noAdd, "no-add", false, "do not make the board known on this machine")
	return cmd
}

// addKnown makes the board in root known on this machine, unless another
// known board already uses its id.
func (a *app) addKnown(root, id string) error {
	loaded, err := a.known("")
	if err != nil {
		return err
	}
	for _, l := range loaded {
		if l.ID() == id && l.Root != root {
			return fmt.Errorf("id %q is already used by the board in %s; ids must differ", id, userconfig.Abbrev(l.Root))
		}
	}
	_, err = userconfig.AddToList(userconfig.KeyBoards, root)
	return err
}

// ── kb add, remove, scan ────────────────────────────────────────────────────

func (a *app) addCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [dir]",
		Short: "Make a board known on this machine",
		Long:  "Make a board known on this machine. dir is the board directory or any directory inside its project; without it, the board of the current directory is added.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			start := "."
			if len(args) == 1 {
				start = args[0]
			}
			root, err := board.FindRoot(start)
			if err != nil {
				return err
			}
			b, err := board.Load(root)
			if err != nil {
				return err
			}
			if b.Config == nil {
				for _, p := range b.Problems() {
					fmt.Fprintln(a.errOut, formatProblem(p))
				}
				return errViolation
			}
			loaded, err := a.known("")
			if err != nil {
				return err
			}
			for _, l := range loaded {
				if l.ID() == b.Config.ID && l.Root != root {
					return fmt.Errorf("id %q is already used by the board in %s; ids must differ", b.Config.ID, userconfig.Abbrev(l.Root))
				}
			}
			changed, err := userconfig.AddToList(userconfig.KeyBoards, root)
			if err != nil {
				return err
			}
			if !changed {
				fmt.Fprintf(a.out, "%s: already added (%s)\n", b.Config.ID, userconfig.Abbrev(root))
				return nil
			}
			fmt.Fprintf(a.out, "%s: added (%s)\n", b.Config.ID, userconfig.Abbrev(root))
			return nil
		},
	}
}

func (a *app) removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id|dir>",
		Short: "Forget a board on this machine; deletes nothing",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			loaded, err := a.known("")
			if err != nil {
				return err
			}
			// An id names the board; anything else is taken as its directory.
			root, viaScan := userconfig.Expand(args[0]), false
			for _, l := range loaded {
				if l.ID() == args[0] || l.Root == root {
					root, viaScan = l.Root, l.Source == registry.SourceScan
					break
				}
			}
			changed, err := userconfig.RemoveFromList(userconfig.KeyBoards, root)
			if err != nil {
				return err
			}
			switch {
			case changed:
				fmt.Fprintf(a.out, "removed %s; the files are untouched\n", userconfig.Abbrev(root))
				return nil
			case viaScan:
				return fmt.Errorf("%s was not added by itself, it is found through a directory under `scan`; hide it in the UI or stop scanning that directory with `kb scan --remove <dir>`", userconfig.Abbrev(root))
			default:
				return fmt.Errorf("no added board %q, see `kb boards`", args[0])
			}
		},
	}
}

func (a *app) scanCmd() *cobra.Command {
	var remove bool
	cmd := &cobra.Command{
		Use:   "scan <dir>",
		Short: "Look for boards below a directory, now and from now on",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir := userconfig.Expand(args[0])
			if remove {
				changed, err := userconfig.RemoveFromList(userconfig.KeyScan, dir)
				if err != nil {
					return err
				}
				if !changed {
					return fmt.Errorf("%s is not scanned", userconfig.Abbrev(dir))
				}
				fmt.Fprintf(a.out, "no longer scanning %s\n", userconfig.Abbrev(dir))
				return nil
			}
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				return fmt.Errorf("%s is not a directory", args[0])
			}
			changed, err := userconfig.AddToList(userconfig.KeyScan, dir)
			if err != nil {
				return err
			}
			found := registry.Discover(dir)
			state := "scanning"
			if !changed {
				state = "already scanning"
			}
			fmt.Fprintf(a.out, "%s %s: %s found\n", state, userconfig.Abbrev(dir), plural(len(found), "board"))
			for _, root := range found {
				fmt.Fprintf(a.out, "  %s\n", userconfig.Abbrev(root))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&remove, "remove", false, "stop scanning the directory")
	return cmd
}

// ── kb sync ─────────────────────────────────────────────────────────────────

func (a *app) syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Pull and push a board with sync: push",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			b, err := a.loadValid()
			if err != nil {
				return err
			}
			res, err := gitsync.Sync(context.Background(), b)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(res)
			}
			if res.Pulled == 0 && res.Pushed == 0 {
				fmt.Fprintf(a.out, "%s: up to date\n", b.Config.ID)
				return nil
			}
			fmt.Fprintf(a.out, "%s: %s pulled, %s pushed\n", b.Config.ID, plural(res.Pulled, "commit"), plural(res.Pushed, "commit"))
			return nil
		},
	}
}

// ── kb ui ───────────────────────────────────────────────────────────────────

// DefaultPort is where `kb ui` listens unless told otherwise.
const DefaultPort = 4712

func (a *app) uiCmd() *cobra.Command {
	var port int
	var noOpen bool
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Show all known boards in the browser",
		Long:  "Start a web server on this machine only and show all known boards in the browser. The page follows changes on disk. Cards can be moved, taken and assigned there; each of these is the same action as the command of that name.",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return server.New(a.here()).Run(ctx, port, func(url string) {
				fmt.Fprintf(a.out, "kardboard is at %s  (Ctrl+C stops it)\n", url)
				if !noOpen {
					if err := openBrowser(url); err != nil {
						fmt.Fprintf(a.errOut, "kb: could not open the browser: %s\n", err)
					}
				}
			})
		},
	}
	cmd.Flags().IntVar(&port, "port", DefaultPort, "port to listen on")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open the browser")
	return cmd
}

func openBrowser(url string) error {
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		return errors.New("refusing to open an address that is not this machine")
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
