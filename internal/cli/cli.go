// Package cli implements the kb command.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"kardboard/internal/board"
	"kardboard/internal/userconfig"
)

// Exit codes of kb.
const (
	ExitOK        = 0
	ExitViolation = 1 // the board breaks a rule of the format
	ExitUsage     = 2 // wrong call, or nothing to work on
)

// errViolation ends a command with ExitViolation. The command has already
// printed what is wrong.
var errViolation = errors.New("board violates the format")

type app struct {
	out, errOut io.Writer
	dir         string
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
	root.PersistentFlags().BoolVar(&a.asJSON, "json", false, "print JSON")
	root.AddCommand(a.lintCmd(), a.listCmd(), a.showCmd())

	err := root.Execute()
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, errViolation):
		return ExitViolation
	default:
		fmt.Fprintf(errOut, "kb: %s\n", err)
		return ExitUsage
	}
}

// load finds and reads the board the command works on.
func (a *app) load() (*board.Board, error) {
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
			person := ""
			if mine {
				if person, err = userconfig.User(); err != nil {
					return err
				}
			}

			cards := []*board.Card{}
			for _, col := range columns {
				for _, c := range b.Cards {
					if c.Column == col.ID && (!mine || c.Person == person) {
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
	cmd.Flags().BoolVar(&mine, "mine", false, "only cards in your own directory")
	return cmd
}

func columnIDs(cfg *board.Config) string {
	ids := make([]string, len(cfg.Columns))
	for i, c := range cfg.Columns {
		ids[i] = c.ID
	}
	return strings.Join(ids, ", ")
}

// printList prints the cards grouped by column and, in per-person columns,
// by person. A card that breaks a rule is marked with "!".
func (a *app) printList(columns []board.Column, cards []*board.Card) {
	// Cards below a person are indented two more columns. The id column is
	// as wide as the longest id including that indent, so titles line up.
	extra := func(c *board.Card) int {
		if c.Person != "" {
			return 2
		}
		return 0
	}
	width := 0
	for _, c := range cards {
		width = max(width, len(c.ID)+extra(c))
	}
	width = min(width, 42)

	for _, col := range columns {
		var inCol []*board.Card
		for _, c := range cards {
			if c.Column == col.ID {
				inCol = append(inCol, c)
			}
		}
		fmt.Fprintf(a.out, "%s (%d)\n", col.Label(), len(inCol))
		sort.SliceStable(inCol, func(i, j int) bool { return inCol[i].Person < inCol[j].Person })
		person := ""
		for _, c := range inCol {
			if c.Person != person {
				person = c.Person
				fmt.Fprintf(a.out, "  %s:\n", person)
			}
			mark := " "
			if !c.Valid() {
				mark = "!"
			}
			indent := strings.Repeat(" ", 2+extra(c))
			fmt.Fprintf(a.out, "%s%s %-*s  %s\n", indent, mark, width-extra(c), c.ID, c.Title)
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
			if c.Person != "" {
				rows = append(rows, [2]string{"person", c.Person})
			}
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

func formatValue(v any) string {
	if list, ok := v.([]string); ok {
		return strings.Join(list, ", ")
	}
	return fmt.Sprint(v)
}
