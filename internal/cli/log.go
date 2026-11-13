package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"kardboard/internal/gitlog"
)

// logCmd prints the Git history of the board or of one card, one line per
// commit. What a commit did is read from what it contains, as the web UI
// does, not from its message.
func (a *app) logCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "log [card]",
		Short: "Show which commits changed the board, or one card",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if limit < 1 {
				return errors.New("-n must be at least 1")
			}
			b, err := a.loadValid()
			if err != nil {
				return err
			}
			cardPath := ""
			if len(args) == 1 {
				c, ok := b.Card(strings.TrimSuffix(args[0], ".md"))
				if !ok {
					return fmt.Errorf("no card %q on board %s", args[0], b.Config.ID)
				}
				cardPath = c.Path
			}
			columns := make([]string, len(b.Config.Columns))
			for i, c := range b.Config.Columns {
				columns[i] = c.ID
			}
			log := gitlog.Read(context.Background(), b.Root, columns, cardPath, limit)
			if a.asJSON {
				return a.printJSON(log)
			}
			if !log.Available {
				return errors.New(log.Reason + "; it has no history")
			}
			if len(log.Entries) == 0 {
				fmt.Fprintln(a.out, "No commits yet.")
				return nil
			}
			// Names may hold letters of more than one byte: pad by letters.
			width := 0
			for _, e := range log.Entries {
				width = max(width, utf8.RuneCountInString(e.Author))
			}
			for _, e := range log.Entries {
				pad := strings.Repeat(" ", width-utf8.RuneCountInString(e.Author))
				fmt.Fprintf(a.out, "%s  %s  %s%s  %s\n", e.Short, when(e.Date), e.Author, pad, describe(e))
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "number", "n", 20, "show at most this many commits")
	return cmd
}

// when writes a commit's date in local time, to the minute.
func when(date string) string {
	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return date
	}
	return t.Local().Format("2006-01-02 15:04")
}

// maxParts is how many cards one line names before it counts the rest.
const maxParts = 3

// describe says in a few words what a commit did to the cards. A commit
// that touched no card, e.g. one that changed board.yaml, is told by its
// subject.
func describe(e gitlog.Entry) string {
	var parts []string
	others := 0
	for _, c := range e.Changes {
		var s string
		switch c.Kind {
		case gitlog.KindAdded:
			s = fmt.Sprintf("new %s in %s", c.Card, c.To)
		case gitlog.KindMoved:
			s = fmt.Sprintf("%s: %s -> %s", c.Card, c.From, c.To)
		case gitlog.KindEdited:
			s = "edited " + c.Card
		case gitlog.KindRenamed:
			s = fmt.Sprintf("renamed %s -> %s", c.Was, c.Card)
		case gitlog.KindRemoved:
			s = fmt.Sprintf("removed %s from %s", c.Card, c.From)
		default:
			others++
			continue
		}
		if c.Edited {
			s += ", edited"
		}
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return e.Subject
	}
	// A commit that touched many cards gets one line all the same.
	if len(parts) > maxParts {
		parts = append(parts[:maxParts], fmt.Sprintf("and %d more", len(parts)-maxParts))
	}
	if others > 0 {
		parts = append(parts, plural(others, "other file"))
	}
	return strings.Join(parts, "; ")
}
