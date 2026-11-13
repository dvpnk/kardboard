package cli

import "github.com/spf13/cobra"

// examples are shown by `kard <command> --help`. Two or three lines each,
// with the cards of examples/board, so people and agents see a call that
// works before they write their own. Commands that set their own Example
// keep it.
var examples = map[string]string{
	"init":     "  kard init\n  kard init ~/Repos/notizen --columns backlog,doing,done --sync commit",
	"lint":     "  kard lint\n  kard lint --all --json",
	"schema":   "  kard schema > card.schema.json\n  kard schema working_on",
	"list":     "  kard list\n  kard list working_on --json\n  kard list --mine --all",
	"show":     "  kard show export-als-pdf\n  kard show export-als-pdf --json   # fields, body and sum",
	"log":      "  kard log\n  kard log export-als-pdf -n 5",
	"move":     "  kard move export-als-pdf working_on",
	"take":     "  kard take export-als-pdf\n  kard take export-als-pdf --add   # join the people on it",
	"done":     "  kard done export-als-pdf",
	"assign":   "  kard assign export-als-pdf sam\n  kard assign export-als-pdf alex,sam\n  kard assign export-als-pdf alex --add",
	"unassign": "  kard unassign export-als-pdf\n  kard unassign export-als-pdf sam",
	"set":      "  kard set export-als-pdf size=M due=2026-12-01\n  kard set export-als-pdf tags=feature,ui\n  kard set export-als-pdf due=   # remove the field",
	"rename":   "  kard rename druckvorlagen print-templates",
	"boards":   "  kard boards\n  kard boards --json",
	"add":      "  kard add\n  kard add ~/Repos/notizen",
	"remove":   "  kard remove notizen",
	"scan":     "  kard scan ~/Repos\n  kard scan ~/Repos --remove",
	"sync":     "  kard sync",
	"ui":       "  kard ui\n  kard ui --port 4800 --no-open",
}

// addExamples puts the examples on the commands that have none yet.
func addExamples(root *cobra.Command) {
	for _, cmd := range root.Commands() {
		if ex, ok := examples[cmd.Name()]; ok && cmd.Example == "" {
			cmd.Example = ex
		}
	}
}
