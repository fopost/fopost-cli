// Command fopost is the FoPost command-line interface.
package main

import (
	"os"

	"github.com/fopost/fopost-cli/internal/cmd"
)

func main() { os.Exit(cmd.Main()) }
