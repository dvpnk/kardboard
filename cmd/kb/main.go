// Command kb is the command line of kardboard.
package main

import (
	"os"

	"kardboard/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
