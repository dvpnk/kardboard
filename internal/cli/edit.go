package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
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
// one change: the same rules as `kard set`, and on boards that commit one
// commit "edit <card>". The editor works on a copy, so a text that cannot
// be saved is not lost and the card is never half written.
//
// With --file, the new content comes from a file or from stdin instead of
// an editor, for scripts and agents. Everything else stays the same.
func (a *app) editCmd() *cobra.Command {
	var force bool
	var file, base string
	cmd := &cobra.Command{
		Use:   "edit <card>",
		Short: "Open a card in your editor ($VISUAL or $EDITOR) and save it as one change",
		Long: `Open a card in your editor ($VISUAL or $EDITOR) and save it as one change.

The editor gets a copy. When it is closed, the copy replaces the card with
the same rules as kard set: a card assigned to someone else needs --force, a
required field must not go away, and on boards with sync: commit or push it
is one commit. If the copy has new errors or the card changed meanwhile,
nothing is saved and kard says where the copy is.

With --file the new content, header included, comes from a file or, with
"-", from stdin, without an editor. --base gives the card's sum as read
before (kard show --json prints it): if the card changed since, nothing is
saved.`,
		Example: `  kard edit export-als-pdf
  kard edit export-als-pdf --file new.md
  cat new.md | kard edit export-als-pdf --file - --base 3f9c0a1b2c3d4e5f60718293`,
		Args: cobra.ExactArgs(1),
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
			if file != "" {
				text, err := a.readInput(file)
				if err != nil {
					return err
				}
				// --base is the sum of the card when the caller read it;
				// without it, the card as it is now.
				if base == "" {
					base = c.Sum
				}
				return a.change(b, c.ID, board.Change{File: &text, Base: base, Force: force})
			}
			if base != "" {
				return errors.New("--base goes with --file")
			}

			dir, err := os.MkdirTemp("", "kard-edit-")
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
			fmt.Fprintf(a.errOut, "kard: your text is in %s\n", copyPath)
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, forceHelp)
	cmd.Flags().StringVar(&file, "file", "", "take the new content from this file, or from stdin with -, instead of an editor")
	cmd.Flags().StringVar(&base, "base", "", "with --file: the card's sum when it was read; refuse if it changed since")
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

// readInput reads the text a flag names: a file, or with "-" what is piped
// into kard. It is how programs and agents hand kard a text without an editor.
func (a *app) readInput(source string) (string, error) {
	if source == "-" {
		data, err := io.ReadAll(a.in)
		return string(data), err
	}
	data, err := os.ReadFile(source)
	return string(data), err
}
