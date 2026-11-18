// Command kard is the command line of kardboard.
package main

import (
	"os"

	"github.com/dvpnk/kardboard/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
