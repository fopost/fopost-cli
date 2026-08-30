// Package cmd wires the fopost command tree. Every command is a thin UX layer
// over github.com/fopost/fopost-go — no HTTP, retries, or error handling of
// its own.
package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/buildinfo"
	"github.com/fopost/fopost-cli/internal/config"
	"github.com/fopost/fopost-cli/internal/output"
)

// defaultBaseURL is what the SDK talks to when nothing overrides it.
const defaultBaseURL = fopost.DefaultBaseURL

// State is the shared context every command reads: the global flags, the
// resolved credentials, and the lazily built SDK client.
type State struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer

	apiKey    string
	baseURL   string
	workspace string
	timeout   time.Duration
	asJSON    bool
	quiet     bool
	noColor   bool

	file     *config.File
	resolved config.Resolved
	printer  *output.Printer
	client   *fopost.Client
}

// NewState builds the state a root command hangs off.
func NewState(in io.Reader, out, errOut io.Writer) *State {
	return &State{In: in, Out: out, Err: errOut, timeout: 30 * time.Second}
}

// Printer renders results in the shape the global flags asked for.
func (s *State) Printer() *output.Printer {
	if s.printer == nil {
		s.printer = output.New(s.Out, s.Err, s.asJSON, s.quiet, s.noColor)
	}
	return s.printer
}

// File is the config file as it is on disk.
func (s *State) File() (*config.File, error) {
	if s.file == nil {
		file, err := config.Load()
		if err != nil {
			return nil, err
		}
		s.file = file
	}
	return s.file, nil
}

// Resolved applies flag-over-environment-over-file precedence once and caches it.
func (s *State) Resolved() (config.Resolved, error) {
	if s.resolved.APIKeySource != "" {
		return s.resolved, nil
	}
	file, err := s.File()
	if err != nil {
		return config.Resolved{}, err
	}
	s.resolved = config.Resolve(config.Flags{
		APIKey:    s.apiKey,
		BaseURL:   s.baseURL,
		Workspace: s.workspace,
	}, file)
	return s.resolved, nil
}

// Client builds the SDK client from the resolved credentials.
func (s *State) Client() (*fopost.Client, error) {
	if s.client != nil {
		return s.client, nil
	}
	resolved, err := s.Resolved()
	if err != nil {
		return nil, err
	}
	if resolved.APIKey == "" {
		return nil, usageErrorf("no API key found. Run `fopost auth login`, set %s, or pass --api-key", config.EnvAPIKey)
	}
	opts := []fopost.Option{
		fopost.WithUserAgent(buildinfo.UserAgent()),
		fopost.WithTimeout(s.timeout),
	}
	if resolved.BaseURL != "" {
		opts = append(opts, fopost.WithBaseURL(resolved.BaseURL))
	}
	client, err := fopost.New(resolved.APIKey, opts...)
	if err != nil {
		return nil, err
	}
	s.client = client
	return client, nil
}

// WorkspaceID is the workspace a command should act on: the one it was given,
// else the saved default.
func (s *State) WorkspaceID(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	resolved, err := s.Resolved()
	if err != nil {
		return "", err
	}
	if resolved.Workspace == "" {
		return "", usageErrorf("a workspace is required. Pass --workspace, or save a default with `fopost auth login --workspace <id>`")
	}
	return resolved.Workspace, nil
}

// NewRootCmd builds the command tree.
func NewRootCmd(state *State) *cobra.Command {
	root := &cobra.Command{
		Use:   "fopost",
		Short: "Schedule and publish social media content from your terminal",
		Long: "fopost is the command-line interface for FoPost (https://fopost.com).\n\n" +
			"Every command speaks to the FoPost API with the key stored by `fopost auth login`.\n" +
			"Add --json to any command to get machine-readable output for a script.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
		Version:           buildinfo.Version,
	}
	root.SetIn(state.In)
	root.SetOut(state.Out)
	root.SetErr(state.Err)

	flags := root.PersistentFlags()
	flags.StringVar(&state.apiKey, "api-key", "", "API key to use for this call (overrides the environment and the config file)")
	flags.StringVar(&state.baseURL, "base-url", "", "API base URL (overrides "+config.EnvBaseURL+")")
	flags.StringVar(&state.workspace, "workspace", "", "workspace id to act on (overrides the saved default)")
	flags.DurationVar(&state.timeout, "timeout", 30*time.Second, "per-request timeout")
	flags.BoolVar(&state.asJSON, "json", false, "print JSON instead of a table")
	flags.BoolVar(&state.quiet, "quiet", false, "suppress human-facing output")
	flags.BoolVar(&state.noColor, "no-color", false, "disable color (also honours NO_COLOR)")

	for _, build := range registry {
		root.AddCommand(build(state))
	}
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute(args []string, in io.Reader, out, errOut io.Writer) int {
	state := NewState(in, out, errOut)
	root := NewRootCmd(state)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(errOut, "Error: %s\n", Explain(err))
		return ExitCode(err)
	}
	return ExitOK
}

// Main is the entry point the binary calls.
func Main() int {
	return Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}
