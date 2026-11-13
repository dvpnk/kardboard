package cli

import (
	"errors"
	"fmt"

	"kardboard/internal/board"
	"kardboard/internal/people"
	"kardboard/internal/registry"
	"kardboard/internal/userconfig"
)

const allHelp = "every board known on this machine, see `kard boards`"

// errNoBoards ends a command with --all when no board is known at all.
var errNoBoards = errors.New("no boards known on this machine; add one with `kard add <dir>` or a whole directory of projects with `kard scan <dir>`")

// everyBoard returns the boards --all works on: those `kard boards` lists,
// with the one the command was started in. -C and -b name one board, so
// they do not go with --all.
func (a *app) everyBoard() ([]*registry.Loaded, error) {
	if a.dir != "" || a.boardID != "" {
		return nil, errors.New("--all works on every board; it does not go with -C or -b")
	}
	loaded, err := a.known(a.here())
	if err != nil {
		return nil, err
	}
	if len(loaded) == 0 {
		return nil, errNoBoards
	}
	return loaded, nil
}

// heading names a board above its part of the output.
func heading(l *registry.Loaded) string {
	r := row(l)
	if r.ID == "" {
		return r.Path
	}
	if r.Name != r.ID {
		return fmt.Sprintf("%s (%s, %s)", r.Name, r.ID, r.Path)
	}
	return fmt.Sprintf("%s (%s)", r.ID, r.Path)
}

// unusable says why a board cannot be worked on at all, or "".
func unusable(l *registry.Loaded) string {
	if l.Err != "" {
		return l.Err
	}
	if l.Board == nil {
		return "cannot be read"
	}
	return ""
}

// lintAll checks every known board, each under its own heading, and sums up
// at the end. It fails if any board has an error or cannot be checked.
func (a *app) lintAll() error {
	loaded, err := a.everyBoard()
	if err != nil {
		return err
	}
	results := make([]lintResult, len(loaded))
	var cards, errs, warnings int
	for i, l := range loaded {
		if why := unusable(l); why != "" {
			results[i] = lintResult{Board: l.Root, Problems: []board.Problem{}, Error: why, Errors: 1}
		} else {
			results[i] = lint(l.Board)
		}
		cards += results[i].Cards
		errs += results[i].Errors
		warnings += results[i].Warnings
	}

	if a.asJSON {
		if err := a.printJSON(results); err != nil {
			return err
		}
	} else {
		for i, l := range loaded {
			res := results[i]
			if i > 0 {
				fmt.Fprintln(a.out)
			}
			if res.Error != "" {
				fmt.Fprintf(a.out, "%s: %s\n", heading(l), res.Error)
				continue
			}
			fmt.Fprintf(a.out, "%s: %s, %s, %s\n", heading(l),
				plural(res.Cards, "card"), plural(res.Errors, "error"), plural(res.Warnings, "warning"))
			for _, p := range res.Problems {
				fmt.Fprintln(a.out, "  "+formatProblem(p))
			}
		}
		fmt.Fprintf(a.out, "\n%s, %s, %s, %s\n", plural(len(loaded), "board"),
			plural(cards, "card"), plural(errs, "error"), plural(warnings, "warning"))
	}
	if errs > 0 {
		return errViolation
	}
	return nil
}

// listAll lists the cards of every known board, board by board. A column
// narrows each board to the column of that name; boards without one are
// left out. Boards that cannot be read are named on stderr.
func (a *app) listAll(column string, mine bool) error {
	loaded, err := a.everyBoard()
	if err != nil {
		return err
	}
	results := []listResult{}
	found := false
	first := true
	for _, l := range loaded {
		if why := unusable(l); why != "" || l.Board.Config == nil {
			if why == "" {
				why = "board.yaml is missing or invalid, see `kard lint -C " + userconfig.Abbrev(l.Root) + "`"
			}
			fmt.Fprintf(a.errOut, "kard: %s left out: %s\n", userconfig.Abbrev(l.Root), why)
			continue
		}
		b := l.Board
		columns := b.Config.Columns
		if column != "" {
			col, ok := b.Config.Column(column)
			if !ok {
				continue
			}
			columns = []board.Column{col}
		}
		found = true
		me := ""
		if mine {
			id, err := people.Me(b.Root)
			if err != nil {
				fmt.Fprintf(a.errOut, "kard: %s left out: %s\n", userconfig.Abbrev(l.Root), err)
				continue
			}
			me = id.Name
		}
		cards := listed(b, columns, me)
		if a.asJSON {
			results = append(results, listResult{Board: info(b), Cards: cards})
			continue
		}
		if !first {
			fmt.Fprintln(a.out)
		}
		first = false
		fmt.Fprintf(a.out, "== %s\n", heading(l))
		a.printList(columns, cards)
	}
	if column != "" && !found {
		return fmt.Errorf("no board has a column %q", column)
	}
	if a.asJSON {
		return a.printJSON(results)
	}
	return nil
}
