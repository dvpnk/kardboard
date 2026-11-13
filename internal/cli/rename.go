package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"kardboard/internal/board"
	"kardboard/internal/gitsync"
	"kardboard/internal/people"
)

// renameCmd gives a card a new name and makes the references to it on the
// same board follow.
func (a *app) renameCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rename <card> <new>",
		Short: "Give a card a new file name and update the references to it",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			b, err := a.loadValid()
			if err != nil {
				return err
			}
			if b, err = gitsync.Prepare(ctx, b); err != nil {
				return err
			}
			// Who one is matters only for a card that belongs to someone.
			me, userErr := people.Me(b.Root)
			res, out, err := gitsync.Rename(ctx, b, args[0], args[1], board.RenameOptions{User: me.Name, Force: force})
			if res == nil {
				if errors.Is(err, board.ErrNeedUser) && userErr != nil {
					return fmt.Errorf("card %s is assigned to someone, so kard must know who you are: %w", args[0], userErr)
				}
				return err
			}
			if a.asJSON {
				if jsonErr := a.printJSON(struct {
					*board.RenameResult
					gitsync.Outcome
				}{res, out}); jsonErr != nil {
					return jsonErr
				}
				return err
			}
			fmt.Fprintf(a.out, "%s -> %s%s\n", res.Path, res.NewPath, gitNote(out))
			if len(res.Updated) > 0 {
				fmt.Fprintf(a.out, "references updated in %s\n", strings.Join(res.Updated, ", "))
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "also rename a card assigned to someone else")
	return cmd
}
