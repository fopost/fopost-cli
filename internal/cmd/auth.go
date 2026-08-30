package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/fopost/fopost-cli/internal/config"
)

func init() { register(newAuthCmd) }

func newAuthCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Store, inspect, and remove the API key",
	}
	cmd.AddCommand(newAuthLoginCmd(state), newAuthStatusCmd(state), newAuthLogoutCmd(state))
	return cmd
}

type loginResult struct {
	Status     string `json:"status"`
	Key        string `json:"key"`
	ConfigPath string `json:"config_path"`
	Workspaces int    `json:"workspaces"`
	Workspace  string `json:"workspace,omitempty"`
}

func newAuthLoginCmd(state *State) *cobra.Command {
	var (
		workspace string
		baseURL   string
		noVerify  bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store an API key in the config file",
		Long: "Stores an API key at " + configPathForHelp() + " with mode 0600.\n\n" +
			"The key is read from --api-key, from standard input when it is piped, or from a\n" +
			"prompt that does not echo. Create one in the FoPost dashboard under\n" +
			"Settings → API Keys. The key is never printed back.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, err := readAPIKey(state)
			if err != nil {
				return err
			}

			file, err := state.File()
			if err != nil {
				return err
			}
			file.APIKey = key
			if baseURL != "" {
				file.BaseURL = strings.TrimRight(baseURL, "/")
			}
			if workspace != "" {
				file.Workspace = workspace
			}

			// Build the client from what is about to be saved, not from the
			// precedence chain, so login verifies the key it was handed.
			state.apiKey = key
			if file.BaseURL != "" {
				state.baseURL = file.BaseURL
			}
			state.resolved = config.Resolved{}
			state.client = nil

			result := loginResult{Status: "saved", Key: config.Mask(key), Workspace: file.Workspace}
			if !noVerify {
				client, err := state.Client()
				if err != nil {
					return err
				}
				workspaces, err := client.Workspaces.List(cmd.Context())
				if err != nil {
					return err
				}
				result.Workspaces = len(workspaces)
				if file.Workspace == "" && len(workspaces) == 1 {
					file.Workspace = workspaces[0].ID
					result.Workspace = workspaces[0].ID
				}
			}

			if err := config.Save(file); err != nil {
				return err
			}
			path, err := config.Path()
			if err != nil {
				return err
			}
			result.ConfigPath = path

			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Success("Signed in as %s", result.Key)
				printer.Line("Key saved to %s (mode 0600).", path)
				if !noVerify {
					printer.Line("Reachable workspaces: %d", result.Workspaces)
				}
				if result.Workspace != "" {
					printer.Line("Default workspace: %s", result.Workspace)
				}
			})
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace id to save as the default")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "API base URL to save alongside the key")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "save the key without calling the API to check it")
	return cmd
}

// readAPIKey takes the key from --api-key, from piped stdin, or from a prompt
// with echo off. It never writes the key back to the terminal.
func readAPIKey(state *State) (string, error) {
	if key := strings.TrimSpace(state.apiKey); key != "" {
		return key, nil
	}
	if key := strings.TrimSpace(os.Getenv(config.EnvAPIKey)); key != "" {
		return key, nil
	}
	if file, ok := state.In.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		fmt.Fprint(state.Err, "API key: ")
		raw, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(state.Err)
		if err != nil {
			return "", fmt.Errorf("reading the key: %w", err)
		}
		key := strings.TrimSpace(string(raw))
		if key == "" {
			return "", usageErrorf("no API key entered")
		}
		return key, nil
	}

	scanner := bufio.NewScanner(state.In)
	if scanner.Scan() {
		if key := strings.TrimSpace(scanner.Text()); key != "" {
			return key, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading the key: %w", err)
	}
	return "", usageErrorf("no API key given. Pass --api-key, set %s, or pipe the key on stdin", config.EnvAPIKey)
}

type statusResult struct {
	Authenticated bool   `json:"authenticated"`
	Key           string `json:"key,omitempty"`
	Source        string `json:"source"`
	BaseURL       string `json:"base_url"`
	Workspace     string `json:"workspace,omitempty"`
	ConfigPath    string `json:"config_path"`
	Verified      *bool  `json:"verified,omitempty"`
	Workspaces    *int   `json:"workspaces,omitempty"`
}

func newAuthStatusCmd(state *State) *cobra.Command {
	var noVerify bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show which key is in effect, masked",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			path, err := config.Path()
			if err != nil {
				return err
			}
			baseURL := resolved.BaseURL
			if baseURL == "" {
				baseURL = defaultBaseURL
			}
			result := statusResult{
				Authenticated: resolved.APIKey != "",
				Key:           config.Mask(resolved.APIKey),
				Source:        string(resolved.APIKeySource),
				BaseURL:       baseURL,
				Workspace:     resolved.Workspace,
				ConfigPath:    path,
			}

			var verifyErr error
			if result.Authenticated && !noVerify {
				client, err := state.Client()
				if err != nil {
					return err
				}
				workspaces, err := client.Workspaces.List(cmd.Context())
				verified := err == nil
				result.Verified = &verified
				if err != nil {
					verifyErr = err
				} else {
					count := len(workspaces)
					result.Workspaces = &count
				}
			}

			printer := state.Printer()
			if err := printer.Value(result, func() {
				if !result.Authenticated {
					printer.Line("Not signed in. Run `fopost auth login`.")
					printer.Line("Config file: %s", path)
					return
				}
				fields := [][2]string{
					{"Key", result.Key},
					{"Source", result.Source},
					{"API", result.BaseURL},
					{"Config", path},
				}
				if result.Workspace != "" {
					fields = append(fields, [2]string{"Workspace", result.Workspace})
				}
				if result.Verified != nil {
					if *result.Verified {
						fields = append(fields, [2]string{"Status", fmt.Sprintf("valid, %d workspace(s)", *result.Workspaces)})
					} else {
						fields = append(fields, [2]string{"Status", "rejected by the API"})
					}
				}
				printer.Fields(fields)
			}); err != nil {
				return err
			}

			if !result.Authenticated {
				return usageErrorf("not signed in")
			}
			return verifyErr
		},
	}
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "do not call the API to check the key")
	return cmd
}

type logoutResult struct {
	Status     string `json:"status"`
	ConfigPath string `json:"config_path"`
}

func newAuthLogoutCmd(state *State) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored API key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			file, err := state.File()
			if err != nil {
				return err
			}
			status := "removed"
			if file.APIKey == "" {
				status = "no key stored"
			}
			file.APIKey = ""
			if all {
				file.Workspace = ""
				file.BaseURL = ""
			}
			if err := config.Save(file); err != nil {
				return err
			}

			printer := state.Printer()
			return printer.Value(logoutResult{Status: status, ConfigPath: path}, func() {
				if status == "removed" {
					printer.Success("Signed out. The key was removed from %s.", path)
				} else {
					printer.Line("No key was stored in %s.", path)
				}
				if envSet(config.EnvAPIKey) {
					printer.Warn("%s is still set in this shell and will keep being used.", config.EnvAPIKey)
				}
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "also clear the saved workspace and base URL")
	return cmd
}

func envSet(name string) bool {
	value, ok := os.LookupEnv(name)
	return ok && strings.TrimSpace(value) != ""
}

func configPathForHelp() string {
	path, err := config.Path()
	if err != nil {
		return "~/.config/fopost/config.json"
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
