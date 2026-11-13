package cli

import "github.com/spf13/cobra"

// examples are shown by `kb <command> --help`. Two or three lines each,
// with the cards of examples/board, so people and agents see a call that
// works before they write their own. Commands that set their own Example
// keep it.
var examples = map[string]string{
	"init":     "  kb init\n  kb init ~/Repos/notizen --columns backlog,doing,done --sync commit",
	"lint":     "  kb lint\n  kb lint --all --json",
	"schema":   "  kb schema > card.schema.json\n  kb schema working_on",
	"list":     "  kb list\n  kb list working_on --json\n  kb list --mine --all",
	"show":     "  kb show export-als-pdf\n  kb show export-als-pdf --json   # fields, body and sum",
	"log":      "  kb log\n  kb log export-als-pdf -n 5",
	"move":     "  kb move export-als-pdf working_on",
	"take":     "  kb take export-als-pdf\n  kb take export-als-pdf --add   # join the people on it",
	"done":     "  kb done export-als-pdf",
	"assign":   "  kb assign export-als-pdf sam\n  kb assign export-als-pdf alex,sam\n  kb assign export-als-pdf alex --add",
	"unassign": "  kb unassign export-als-pdf\n  kb unassign export-als-pdf sam",
	"set":      "  kb set export-als-pdf size=M due=2026-12-01\n  kb set export-als-pdf tags=feature,ui\n  kb set export-als-pdf due=   # remove the field",
	"rename":   "  kb rename druckvorlagen print-templates",
	"boards":   "  kb boards\n  kb boards --json",
	"add":      "  kb add\n  kb add ~/Repos/notizen",
	"remove":   "  kb remove notizen",
	"scan":     "  kb scan ~/Repos\n  kb scan ~/Repos --remove",
	"sync":     "  kb sync",
	"ui":       "  kb ui\n  kb ui --port 4800 --no-open",
}

// addExamples puts the examples on the commands that have none yet.
func addExamples(root *cobra.Command) {
	for _, cmd := range root.Commands() {
		if ex, ok := examples[cmd.Name()]; ok && cmd.Example == "" {
			cmd.Example = ex
		}
	}
}
