package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dvpnk/kardboard/internal/people"
	"github.com/dvpnk/kardboard/internal/userconfig"
)

type whoamiResult struct {
	Board string `json:"board"`
	people.Identity
	// Error says why kard does not know who you are on this board.
	Error string `json:"error,omitempty"`
}

// whoamiCmd says who the user is on a board and where kard learnt it.
func (a *app) whoamiCmd() *cobra.Command {
	var all bool
	var set string
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Say who you are on the board, and where kard got the name from",
		Long: `Say who you are on the board, and where kard got the name from. kard
takes the first of:

  1. KARDBOARD_USER
  2. user in the nearest .kard/config.yaml, from the board up to the top
     of its Git repository
  3. user in ~/.config/kardboard/config.yaml
  4. git config user.email, as Git sees it in the board's directory
  5. the operating system's account name

--set writes a name to .kard/config.yaml at the top of the repository, or
in the board's directory outside one. The .kard directory ignores itself,
so the name never gets committed.`,
		Example: `  kard whoami
  kard whoami --all
  kard whoami --set arnold-s`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if all {
				if cmd.Flags().Changed("set") {
					return fmt.Errorf("--set names one board, it does not go with --all")
				}
				return a.whoamiAll()
			}
			b, err := a.load()
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("set") {
				path, err := people.SetProject(b.Root, set)
				if err != nil {
					return err
				}
				if !a.asJSON {
					fmt.Fprintf(a.out, "wrote %s\n", userconfig.Abbrev(path))
				}
			}
			id, err := people.Me(b.Root)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(whoamiResult{Board: b.Root, Identity: id})
			}
			fmt.Fprintf(a.out, "%s  (%s)\n", id.Name, id.Describe())
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "say it for every board kard boards shows")
	cmd.Flags().StringVar(&set, "set", "", "write this `name` to the project's .kard/config.yaml")
	return cmd
}

func (a *app) whoamiAll() error {
	loaded, err := a.everyBoard()
	if err != nil {
		return err
	}
	results := []whoamiResult{}
	for _, l := range loaded {
		res := whoamiResult{Board: l.Root}
		if id, err := people.Me(l.Root); err != nil {
			res.Error = err.Error()
		} else {
			res.Identity = id
		}
		if a.asJSON {
			results = append(results, res)
			continue
		}
		if res.Error != "" {
			fmt.Fprintf(a.out, "%s: %s\n", heading(l), res.Error)
		} else {
			fmt.Fprintf(a.out, "%s: %s  (%s)\n", heading(l), res.Name, res.Describe())
		}
	}
	if a.asJSON {
		return a.printJSON(results)
	}
	return nil
}
