package cmd

import (
	"runtime"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/buildinfo"
)

type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	SDK       string `json:"sdk_version"`
	Go        string `json:"go_version"`
	Platform  string `json:"platform"`
}

func init() { register(newVersionCmd) }

func newVersionCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version and build details",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := versionInfo{
				Version:   buildinfo.Version,
				Commit:    buildinfo.Revision(),
				BuildDate: buildinfo.BuildDate(),
				SDK:       fopost.Version,
				Go:        runtime.Version(),
				Platform:  runtime.GOOS + "/" + runtime.GOARCH,
			}
			printer := state.Printer()
			return printer.Value(info, func() {
				printer.Fields([][2]string{
					{"Version", info.Version},
					{"Commit", info.Commit},
					{"Built", info.BuildDate},
					{"SDK", "fopost-go " + info.SDK},
					{"Go", info.Go},
					{"Platform", info.Platform},
				})
			})
		},
	}
}
