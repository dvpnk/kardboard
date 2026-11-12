// Package cli implements the kb command.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kardboard/internal/board"
	"kardboard/internal/gitsync"
	"kardboard/internal/registry"
	"kardboard/internal/userconfig"
)

// Exit codes of kb.
const (
	ExitOK        = 0
	ExitViolation = 1 // the board breaks a rule of the format, or a rule forbids the change
	ExitUsage     = 2 // wrong call, or nothing to work on
)

// errViolation ends a command with ExitViolation. The command has already
// printed what is wrong.
var errViolation = errors.New("board violates the format")

type app struct {
	out, errOut io.Writer
	dir         string // -C: the board directory
	boardID     string // -b: the id of a board known on this machine
	asJSON      bool
}

// Execute runs kb with the process arguments and returns the exit code.
func Execute() int {
	return Run(os.Args[1:], os.Stdout, os.Stderr)
}

// Run runs kb with the given arguments and returns the exit code.
func Run(args []string, out, errOut io.Writer) int {
	a := &app{out: out, errOut: errOut}
	root := &cobra.Command{
		Use:           "kb",
		Short:         "Boards made of Markdown cards that live in your projects",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)
	root.PersistentFlags().StringVarP(&a.dir, "dir", "C", "", "board directory, instead of looking it up from the current directory")
	root.PersistentFlags().StringVarP(&a.boardID, "board", "b", "", "id of a board known on this machine, see `kb boards`")
	root.PersistentFlags().BoolVar(&a.asJSON, "json", false, "print JSON")
	root.MarkFlagsMutuallyExclusive("dir", "board")
	root.AddCommand(a.initCmd(), a.lintCmd(), a.listCmd(), a.showCmd())
	root.AddCommand(a.newCmd(), a.moveCmd(), a.takeCmd(), a.doneCmd(), a.assignCmd(), a.unassignCmd(), a.setCmd(), a.editCmd(), a.renameCmd())
	root.AddCommand(a.boardsCmd(), a.addCmd(), a.removeCmd(), a.scanCmd(), a.syncCmd(), a.uiCmd())

	err := root.Execute()
	var refused *board.RefusedError
	var conflict *gitsync.ConflictError
	var notPushed *gitsync.PushError
	var stale *board.StaleError
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, errViolation):
		return ExitViolation
	case errors.As(err, &refused):
		fmt.Fprintf(errOut, "kb: %s (--force does it anyway)\n", refused.Reason)
		return ExitViolation
	case errors.As(err, &conflict), errors.As(err, &notPushed), errors.As(err, &stale):
		fmt.Fprintf(errOut, "kb: %s\n", err)
		return ExitViolation
	default:
		fmt.Fprintf(errOut, "kb: %s\n", err)
		return ExitUsage
	}
}

// load finds and reads the board the command works on.
func (a *app) load() (*board.Board, error) {
	if a.boardID != "" {
		loaded, err := a.known("")
		if err != nil {
			return nil, err
		}
		l, ok := registry.Find(loaded, a.boardID)
		if !ok {
			return nil, fmt.Errorf("no board %q on this machine, see `kb boards`", a.boardID)
		}
		return l.Board, nil
	}
	root := a.dir
	if root == "" {
		found, err := board.FindRoot(".")
		if err != nil {
			return nil, err
		}
		root = found
	}
	return board.Load(root)
}

// loadValid is load for commands that need a usable board.yaml.
func (a *app) loadValid() (*board.Board, error) {
	b, err := a.load()
	if err != nil {
		return nil, err
	}
	if b.Config == nil {
		for _, p := range b.Problems() {
			fmt.Fprintln(a.errOut, formatProblem(p))
		}
		return nil, errViolation
	}
	return b, nil
}

func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func formatProblem(p board.Problem) string {
	return fmt.Sprintf("%s: %s %s", p.Path, p.Code, p.Message)
}

func formatValue(v any) string {
	if list, ok := v.([]string); ok {
		return strings.Join(list, ", ")
	}
	return fmt.Sprint(v)
}

// ── kb lint ─────────────────────────────────────────────────────────────────

type lintResult struct {
	Board    string          `json:"board"`
	Cards    int             `json:"cards"`
	Errors   int             `json:"errors"`
	Warnings int             `json:"warnings"`
	Problems []board.Problem `json:"problems"`
}

func (a *app) lintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Check the board against the format",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			b, err := a.load()
			if err != nil {
				return err
			}
			res := lintResult{Board: b.Root, Cards: len(b.Cards), Problems: b.Problems()}
			for _, p := range res.Problems {
				if p.Severity == board.SeverityError {
					res.Errors++
				} else {
					res.Warnings++
				}
			}
			if a.asJSON {
				if res.Problems == nil {
					res.Problems = []board.Problem{}
				}
				if err := a.printJSON(res); err != nil {
					return err
				}
			} else {
				for _, p := range res.Problems {
					fmt.Fprintln(a.out, formatProblem(p))
				}
				fmt.Fprintf(a.out, "%s, %s, %s\n",
					plural(res.Cards, "card"), plural(res.Errors, "error"), plural(res.Warnings, "warning"))
			}
			if res.Errors > 0 {
				return errViolation
			}
			return nil
		},
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// ── kb list ─────────────────────────────────────────────────────────────────

type boardInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Root string `json:"root"`
}

type listResult struct {
	Board boardInfo     `json:"board"`
	Cards []*board.Card `json:"cards"`
}

func info(b *board.Board) boardInfo {
	name := b.Config.Name
	if name == "" {
		name = b.Config.ID
	}
	return boardInfo{ID: b.Config.ID, Name: name, Root: b.Root}
}

func (a *app) listCmd() *cobra.Command {
	var mine bool
	cmd := &cobra.Command{
		Use:   "list [column]",
		Short: "List the cards of the board",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			b, err := a.loadValid()
			if err != nil {
				return err
			}
			columns := b.Config.Columns
			if len(args) == 1 {
				col, ok := b.Config.Column(args[0])
				if !ok {
					return fmt.Errorf("no column %q, the board has %s", args[0], columnIDs(b.Config))
				}
				columns = []board.Column{col}
			}
			me := ""
			if mine {
				if me, err = userconfig.User(); err != nil {
					return err
				}
			}

			cards := []*board.Card{}
			for _, col := range columns {
				for _, c := range b.Cards {
					if c.Column == col.ID && (!mine || slices.Contains(c.Assignees(), me)) {
						cards = append(cards, c)
					}
				}
			}
			if a.asJSON {
				return a.printJSON(listResult{Board: info(b), Cards: cards})
			}
			a.printList(columns, cards)
			return nil
		},
	}
	cmd.Flags().BoolVar(&mine, "mine", false, "only cards assigned to you")
	return cmd
}

func columnIDs(cfg *board.Config) string {
	ids := make([]string, len(cfg.Columns))
	for i, c := range cfg.Columns {
		ids[i] = c.ID
	}
	return strings.Join(ids, ", ")
}

// printList prints the cards grouped by column. A card that breaks a rule is
// marked with "!", an assigned card ends with "@name" for each person.
func (a *app) printList(columns []board.Column, cards []*board.Card) {
	width := 0
	for _, c := range cards {
		width = max(width, len(c.ID))
	}
	width = min(width, 40)

	for _, col := range columns {
		var inCol []*board.Card
		for _, c := range cards {
			if c.Column == col.ID {
				inCol = append(inCol, c)
			}
		}
		fmt.Fprintf(a.out, "%s (%d)\n", col.Label(), len(inCol))
		for _, c := range inCol {
			mark := " "
			if !c.Valid() {
				mark = "!"
			}
			line := fmt.Sprintf("  %s %-*s  %s", mark, width, c.ID, c.Title)
			if who := c.Assignees(); len(who) > 0 {
				line += "  @" + strings.Join(who, " @")
			}
			fmt.Fprintln(a.out, line)
		}
	}
}

// ── kb show ─────────────────────────────────────────────────────────────────

type showResult struct {
	Board boardInfo `json:"board"`
	*board.Card
	Body string `json:"body"`
}

func (a *app) showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <card>",
		Short: "Show one card",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			b, err := a.loadValid()
			if err != nil {
				return err
			}
			c, ok := b.Card(args[0])
			if !ok {
				return fmt.Errorf("no card %q on board %s", args[0], b.Config.ID)
			}
			if a.asJSON {
				return a.printJSON(showResult{Board: info(b), Card: c, Body: c.Body})
			}

			col, _ := b.Config.Column(c.Column)
			rows := [][2]string{{"path", c.Path}, {"column", col.Label()}}
			for _, f := range b.Config.AllFields() {
				if v, has := c.Fields[f.Name]; has {
					rows = append(rows, [2]string{f.Name, formatValue(v)})
				}
			}
			width := 0
			for _, r := range rows {
				width = max(width, len(r[0]))
			}
			for _, r := range rows {
				fmt.Fprintf(a.out, "%-*s  %s\n", width+1, r[0]+":", r[1])
			}
			for _, p := range c.Problems {
				fmt.Fprintf(a.out, "! %s %s\n", p.Code, p.Message)
			}
			fmt.Fprintf(a.out, "\n%s\n", strings.Trim(c.Body, "\n"))
			return nil
		},
	}
}

// ── kb new ──────────────────────────────────────────────────────────────────

func (a *app) newCmd() *cobra.Command {
	var opt board.NewOptions
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a card",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			b, err := a.loadValid()
			if err != nil {
				return err
			}
			ctx := context.Background()
			if b, err = gitsync.Prepare(ctx, b); err != nil {
				return err
			}
			opt.Today = time.Now().Format("2006-01-02")
			c, done, err := gitsync.New(ctx, b, args[0], opt)
			if c == nil {
				return err
			}
			// A card that was created but not pushed is still printed; the
			// error then says what is left to do.
			if a.asJSON {
				if jsonErr := a.printJSON(showResult{Board: info(b), Card: c, Body: c.Body}); jsonErr != nil {
					return jsonErr
				}
				return err
			}
			fmt.Fprintln(a.out, c.Path+gitNote(done))
			return err
		},
	}
	cmd.Flags().StringVarP(&opt.Column, "column", "c", "", "column to put the card in (default: the first one)")
	cmd.Flags().StringArrayVar(&opt.Set, "set", nil, "header value as field=value, may be repeated")
	cmd.Flags().StringVar(&opt.ID, "id", "", "file name without .md (default: derived from the title)")
	cmd.Flags().BoolVar(&opt.Force, "force", false, "create the card even if the column requires a field it lacks")
	return cmd
}

// ── kb move, take, done, assign, unassign, set ──────────────────────────────

const forceHelp = "also change a card assigned to someone else, or leave a required field empty"

// change applies one change to a card and prints what happened.
//
// The user's name is looked up here but only reported as missing when the
// board needs it: a board where nobody is assigned works without any setup.
func (a *app) change(b *board.Board, id string, ch board.Change) error {
	var userErr error
	if ch.User == "" {
		ch.User, userErr = userconfig.User()
	}
	res, err := gitsync.Change(context.Background(), b, id, ch)
	if res == nil {
		if errors.Is(err, board.ErrNeedUser) && userErr != nil {
			return fmt.Errorf("card %s is assigned to someone, so kb must know who you are: %w", id, userErr)
		}
		return err
	}
	// A change that is committed but not pushed is still printed; the error
	// then says what is left to do.
	if a.asJSON {
		if jsonErr := a.printJSON(res); jsonErr != nil {
			return jsonErr
		}
		return err
	}

	var parts []string
	if res.Moved {
		parts = append(parts, fmt.Sprintf("%s -> %s", res.From, res.To))
	}
	for _, name := range slices.Sorted(maps.Keys(res.Set)) {
		parts = append(parts, fmt.Sprintf("%s = %s", name, formatValue(res.Set[name])))
	}
	for _, name := range res.Removed {
		parts = append(parts, name+" removed")
	}
	if res.Text {
		parts = append(parts, "text changed")
	}
	if len(parts) == 0 {
		parts = []string{"nothing to change"}
	}
	fmt.Fprintf(a.out, "%s: %s%s\n", res.ID, strings.Join(parts, ", "), gitNote(res.Outcome))
	return err
}

// gitNote says what Git did for an action, for boards that sync.
func gitNote(o gitsync.Outcome) string {
	switch {
	case o.Commit == "":
		return ""
	case o.Pushed:
		return fmt.Sprintf("  (commit %s, pushed)", o.Commit)
	default:
		return fmt.Sprintf("  (commit %s)", o.Commit)
	}
}

// changeCmd builds a command that loads the board and applies one change.
func (a *app) changeCmd(use, short string, nargs cobra.PositionalArgs, build func(b *board.Board, args []string) (board.Change, error)) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  nargs,
		RunE: func(_ *cobra.Command, args []string) error {
			b, err := a.loadValid()
			if err != nil {
				return err
			}
			// With sync: push this pulls, so the change is built from what
			// the others have done meanwhile.
			if b, err = gitsync.Prepare(context.Background(), b); err != nil {
				return err
			}
			ch, err := build(b, args)
			if err != nil {
				return err
			}
			ch.Force = force
			return a.change(b, args[0], ch)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, forceHelp)
	return cmd
}

func (a *app) moveCmd() *cobra.Command {
	return a.changeCmd("move <card> <column>", "Move a card to a column", cobra.ExactArgs(2),
		func(_ *board.Board, args []string) (board.Change, error) {
			return board.Change{Column: args[1]}, nil
		})
}

func (a *app) takeCmd() *cobra.Command {
	var add bool
	cmd := a.changeCmd("take <card>", "Start working on a card: assign it to you and move it to the doing column", cobra.ExactArgs(1),
		func(b *board.Board, args []string) (board.Change, error) {
			me, err := userconfig.User()
			if err != nil {
				return board.Change{}, err
			}
			return b.TakeChange(args[0], me, add), nil
		})
	cmd.Flags().BoolVar(&add, "add", false, "join the people already on the card instead of replacing them")
	return cmd
}

func (a *app) doneCmd() *cobra.Command {
	return a.changeCmd("done <card>", "Move a card to the column for finished work", cobra.ExactArgs(1),
		func(b *board.Board, _ []string) (board.Change, error) {
			return b.DoneChange()
		})
}

func (a *app) assignCmd() *cobra.Command {
	var add bool
	cmd := a.changeCmd("assign <card> <person>...", "Say whose card it is, without moving it", cobra.MinimumNArgs(2),
		func(b *board.Board, args []string) (board.Change, error) {
			return b.AssignChange(args[0], args[1:], add)
		})
	cmd.Aliases = []string{"give"}
	cmd.Flags().BoolVar(&add, "add", false, "add to the people already on the card instead of replacing them")
	return cmd
}

func (a *app) unassignCmd() *cobra.Command {
	return a.changeCmd("unassign <card> [person]...", "Make a card nobody's, or take single people off it", cobra.MinimumNArgs(1),
		func(b *board.Board, args []string) (board.Change, error) {
			return b.UnassignChange(args[0], args[1:]), nil
		})
}

func (a *app) setCmd() *cobra.Command {
	return a.changeCmd("set <card> <field=value>...", "Change header fields; an empty value removes the field", cobra.MinimumNArgs(2),
		func(_ *board.Board, args []string) (board.Change, error) {
			return board.Change{Set: args[1:]}, nil
		})
}
