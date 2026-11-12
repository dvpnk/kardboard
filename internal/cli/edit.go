package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"kardboard/internal/board"
	"kardboard/internal/gitsync"
)

// editCmd opens a card in the user's editor and saves what comes back as
// one change: the same rules as `kb set`, and on boards that commit one
// commit "edit <card>". The editor works on a copy, so a text that cannot
// be saved is not lost and the card is never half written.
func (a *app) editCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "edit <card>",
		Short: "Open a card in your editor ($VISUAL or $EDITOR) and save it as one change",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			b, err := a.loadValid()
			if err != nil {
				return err
			}
			// With sync: push this pulls, so one edits what the others
			// have written meanwhile.
			if b, err = gitsync.Prepare(ctx, b); err != nil {
				return err
			}
			c, ok := b.Card(strings.TrimSuffix(args[0], ".md"))
			if !ok {
				return fmt.Errorf("no card %q on board %s", args[0], b.Config.ID)
			}
			// Better to say now than after the text is written.
			if err := gitsync.Check(ctx, b, c.ID); err != nil {
				return err
			}
			data, err := os.ReadFile(filepath.Join(b.Root, filepath.FromSlash(c.Path)))
			if err != nil {
				return err
			}

			dir, err := os.MkdirTemp("", "kb-edit-")
			if err != nil {
				return err
			}
			// The copy has the card's name, so the editor knows it is Markdown.
			copyPath := filepath.Join(dir, c.ID+".md")
			if err := os.WriteFile(copyPath, data, 0o600); err != nil {
				os.RemoveAll(dir)
				return err
			}
			if err := runEditor(copyPath); err != nil {
				os.RemoveAll(dir)
				return err
			}
			edited, err := os.ReadFile(copyPath)
			if err != nil {
				return err
			}
			if string(edited) == string(data) {
				os.RemoveAll(dir)
				fmt.Fprintf(a.out, "%s: nothing to change\n", c.ID)
				return nil
			}

			text := string(edited)
			err = a.change(b, c.ID, board.Change{File: &text, Base: c.Sum, Force: force})
			var notPushed *gitsync.PushError
			if err == nil || errors.As(err, &notPushed) {
				os.RemoveAll(dir)
				return err
			}
			// Whatever went wrong, what was written stays.
			fmt.Fprintf(a.errOut, "kb: your text is in %s\n", copyPath)
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, forceHelp)
	return cmd
}

// runEditor opens path in the user's editor and waits until it is closed.
// $VISUAL comes before $EDITOR; either may carry arguments, as in
// "code --wait".
func runEditor(path string) error {
	command := os.Getenv("VISUAL")
	if command == "" {
		command = os.Getenv("EDITOR")
	}
	if command == "" {
		command = "vi"
		if runtime.GOOS == "windows" {
			command = "notepad"
		}
	}
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("the editor %q ended with an error, nothing was saved: %w", command, err)
	}
	return nil
}
