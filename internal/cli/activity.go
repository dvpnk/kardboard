package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dvpnk/kardboard/internal/board"
	"github.com/dvpnk/kardboard/internal/gitsync"
	"github.com/dvpnk/kardboard/internal/people"
)

// Environment variables a runner sets for its session, so the session's
// kard calls need no names. Anyone else may set them too.
const (
	// envBoard names the board directory, as -C does.
	envBoard = "KARD_BOARD"
	// envCard names the card that kard comment and kard ask write to when
	// no card is given.
	envCard = "KARD_CARD"
)

// cardAndText sorts out the arguments of kard comment and kard ask: the
// card, unless KARD_CARD names it, and the text, unless --text gives it.
// A card given as an argument wins over KARD_CARD.
func (a *app) cardAndText(args []string, textFile string) (card, text string, err error) {
	envCardID := os.Getenv(envCard)
	if textFile != "" {
		switch len(args) {
		case 0:
			card = envCardID
		case 1:
			card = args[0]
		default:
			return "", "", errors.New("the text comes from --text; give only the card")
		}
		if card == "" {
			return "", "", fmt.Errorf("name the card, or set %s", envCard)
		}
		text, err = a.readInput(textFile)
		return card, text, err
	}
	switch {
	case len(args) == 2:
		return args[0], args[1], nil
	case len(args) == 1 && envCardID != "":
		return envCardID, args[0], nil
	case len(args) == 1:
		return "", "", fmt.Errorf("no text: give it after the card or with --text (or set %s and give only the text)", envCard)
	default:
		return "", "", fmt.Errorf("name the card and the text, or set %s", envCard)
	}
}

// addEntry writes one entry to a card's activity and prints what happened.
func (a *app) addEntry(card string, build func(b *board.Board) (board.Entry, error)) error {
	ctx := context.Background()
	b, err := a.loadValid()
	if err != nil {
		return err
	}
	// With sync: push this pulls, so the entry goes below what the others
	// have written meanwhile.
	if b, err = gitsync.Prepare(ctx, b); err != nil {
		return err
	}
	me, err := people.Me(b.Root)
	if err != nil {
		return fmt.Errorf("kard must know who writes: %w", err)
	}
	e, err := build(b)
	if err != nil {
		return err
	}
	e.Author, e.Time = me.Name, time.Now()
	res, err := gitsync.AddEntry(ctx, b, card, e)
	if res == nil {
		return err
	}
	// An entry that is committed but not pushed is still printed; the
	// error then says what is left to do.
	if a.asJSON {
		if jsonErr := a.printJSON(res); jsonErr != nil {
			return jsonErr
		}
		return err
	}
	what := res.Entry.Kind
	if len(res.Entry.To) > 0 {
		what += " to " + strings.Join(res.Entry.To, ", ")
	}
	fmt.Fprintf(a.out, "%s: %s added to %s%s\n", res.Card, what, res.Path, gitNote(res.Outcome))
	return err
}

func (a *app) commentCmd() *cobra.Command {
	var textFile string
	cmd := &cobra.Command{
		Use:   "comment [card] [text]",
		Short: "Add a comment to a card's activity",
		Long: `Add a comment to a card's activity, activity/<card>.md next to the
columns. The card's own file stays untouched, and anyone may comment on
any card. On boards with sync: commit or push the comment is one commit
"comment <card>".

The text comes after the card, or with --text from a file or, with "-",
from stdin. Without a card, KARD_CARD names it.`,
		Example: `  kard comment export-as-pdf "Keep the old CSV columns."
  kard comment export-as-pdf --text notes.md
  echo "Pull request: #42" | kard comment export-as-pdf --text -`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			card, text, err := a.cardAndText(args, textFile)
			if err != nil {
				return err
			}
			return a.addEntry(card, func(*board.Board) (board.Entry, error) {
				return board.Entry{Kind: board.KindComment, Text: text}, nil
			})
		},
	}
	cmd.Flags().StringVar(&textFile, "text", "", "the comment, from this file or from stdin with -")
	return cmd
}

func (a *app) askCmd() *cobra.Command {
	var textFile string
	var to []string
	cmd := &cobra.Command{
		Use:   "ask [card] [question]",
		Short: "Ask a question in a card's activity",
		Long: `Ask a question in a card's activity, activity/<card>.md next to the
columns. On boards with sync: commit or push it is one commit "ask <card>".

A question an agent asks makes the card wait: its state is "waiting" until
a person writes in the activity, whoever that is. --to names whom the
question is for; names are looked up as with kard assign, and the full
address is written. Without --to, whoever sees the card can answer.

The question comes after the card, or with --text from a file or, with
"-", from stdin. Without a card, KARD_CARD names it.`,
		Example: `  kard ask export-as-pdf "Keep the old CSV columns?" --to arnold
  kard ask export-as-pdf --text question.md --to arnold,sly
  KARD_CARD=export-as-pdf kard ask "Which branch?"`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			card, text, err := a.cardAndText(args, textFile)
			if err != nil {
				return err
			}
			return a.addEntry(card, func(b *board.Board) (board.Entry, error) {
				e := board.Entry{Kind: board.KindQuestion, Text: text}
				if len(to) > 0 {
					names, err := people.ResolveAll(to, people.Known(context.Background(), b))
					if err != nil {
						return e, err
					}
					for _, n := range names {
						if !b.HasPerson(e.To, n) {
							e.To = append(e.To, n)
						}
					}
				}
				return e, nil
			})
		},
	}
	cmd.Flags().StringVar(&textFile, "text", "", "the question, from this file or from stdin with -")
	cmd.Flags().StringArrayVar(&to, "to", nil, "whom the question is for, may be repeated or separated by commas")
	return cmd
}

// ── State in kard list and kard show ─────────────────────────────────────────

// stateOf returns the state of a card for JSON: nil if it has none.
func stateOf(b *board.Board, c *board.Card) (*string, []string) {
	st := b.State(c)
	if st.State == "" {
		return nil, nil
	}
	return &st.State, st.WaitingFor
}

// stateNote is the state at the end of a line of kard list.
func stateNote(b *board.Board, c *board.Card) string {
	st := b.State(c)
	switch {
	case st.State == "":
		return ""
	case len(st.WaitingFor) > 0:
		return fmt.Sprintf("  [%s for %s]", st.State, strings.Join(st.WaitingFor, ", "))
	default:
		return "  [" + st.State + "]"
	}
}

// printActivity prints a card's activity below the card, oldest first, in
// local time.
func (a *app) printActivity(entries []board.Entry) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(a.out, "\nActivity (%d)\n", len(entries))
	for _, e := range entries {
		kind := e.Kind
		if len(e.To) > 0 {
			kind += " to " + strings.Join(e.To, ", ")
		}
		fmt.Fprintf(a.out, "\n%s · %s · %s\n", e.Time.Local().Format("2006-01-02 15:04"), e.Author, kind)
		if board.IsRun(e.Kind) {
			for _, line := range strings.Split(strings.TrimRight(board.Entry{Kind: e.Kind, Values: e.Values}.Format(), "\n"), "\n")[1:] {
				fmt.Fprintln(a.out, "  "+line)
			}
			continue
		}
		for _, line := range strings.Split(e.Text, "\n") {
			fmt.Fprintln(a.out, strings.TrimRight("  "+line, " "))
		}
	}
}
