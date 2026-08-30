package cmd

import "github.com/spf13/cobra"

// registry holds one builder per top-level command. Each command file adds its
// own in an init, so a command is a self-contained file rather than an edit to
// the root command.
var registry []func(*State) *cobra.Command

func register(builders ...func(*State) *cobra.Command) {
	registry = append(registry, builders...)
}
