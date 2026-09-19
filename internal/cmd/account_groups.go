package cmd

import (
	"fmt"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newAccountGroupsCmd) }

func newAccountGroupsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account-groups",
		Aliases: []string{"account-group"},
		Short:   "Manage named groups of connected accounts",
	}
	cmd.AddCommand(
		newAccountGroupsListCmd(state),
		newAccountGroupsGetCmd(state),
		newAccountGroupsCreateCmd(state),
		newAccountGroupsRenameCmd(state),
		newAccountGroupsSetMembersCmd(state),
		newAccountGroupsDeleteCmd(state),
	)
	return cmd
}

func printAccountGroup(printer *output.Printer, group *fopost.AccountGroup) {
	printer.Fields([][2]string{
		{"ID", group.ID},
		{"Name", group.Name},
		{"Accounts", output.Dash(strings.Join(group.AccountIDs, ", "))},
		{"Created", output.Stamp(group.CreatedAt.Time, group.CreatedAt.Raw)},
	})
}

func newAccountGroupsListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List account groups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			resolved, err := state.Resolved()
			if err != nil {
				return err
			}
			groups, err := client.AccountGroups.List(cmd.Context(), resolved.Workspace)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(groups, func() {
				rows := make([][]string, 0, len(groups))
				for _, group := range groups {
					rows = append(rows, []string{group.ID, output.Truncate(group.Name, 28), itoa(len(group.AccountIDs))})
				}
				printer.Table([]string{"id", "name", "accounts"}, rows)
			})
		},
	}
}

func newAccountGroupsGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <group-id>",
		Short: "Show one account group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			group, err := client.AccountGroups.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(group, func() { printAccountGroup(printer, group) })
		},
	}
}

func newAccountGroupsCreateCmd(state *State) *cobra.Command {
	var name string
	var accounts []string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an account group",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return usageErrorf("--name is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			workspaceID, err := state.WorkspaceID("")
			if err != nil {
				return err
			}
			group, err := client.AccountGroups.Create(cmd.Context(), &fopost.CreateAccountGroupRequest{
				WorkspaceID: workspaceID,
				Name:        name,
				AccountIDs:  accounts,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(group, func() {
				printer.Success("Created account group %s (%s).", group.Name, group.ID)
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "group name (required)")
	cmd.Flags().StringArrayVar(&accounts, "account", nil, "account id to add (repeatable)")
	return cmd
}

func newAccountGroupsRenameCmd(state *State) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "rename <group-id>",
		Short: "Rename an account group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return usageErrorf("--name is required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			group, err := client.AccountGroups.Update(cmd.Context(), args[0], &fopost.UpdateAccountGroupRequest{Name: name})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(group, func() {
				printer.Success("Renamed account group %s to %s.", group.ID, group.Name)
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new group name (required)")
	return cmd
}

func newAccountGroupsSetMembersCmd(state *State) *cobra.Command {
	var accounts []string
	var clear bool
	cmd := &cobra.Command{
		Use:   "set-members <group-id>",
		Short: "Replace an account group's members",
		Example: strings.Join([]string{
			"  fopost account-groups set-members grp_1 --account acc_1 --account acc_2",
			"  fopost account-groups set-members grp_1 --clear",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear && len(accounts) > 0 {
				return usageErrorf("--account and --clear are alternatives; pass one")
			}
			if !clear && len(accounts) == 0 {
				return usageErrorf("at least one --account is required, or --clear to empty the group")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			group, err := client.AccountGroups.SetMembers(cmd.Context(), args[0], accounts)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(group, func() {
				printer.Success("Account group %s now has %d account(s).", group.ID, len(group.AccountIDs))
			})
		},
	}
	cmd.Flags().StringArrayVar(&accounts, "account", nil, "account id the group should contain (repeatable)")
	cmd.Flags().BoolVar(&clear, "clear", false, "remove every account from the group")
	return cmd
}

func newAccountGroupsDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <group-id>",
		Short: "Delete an account group, keeping its accounts connected",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete account group %s? This cannot be undone.", args[0])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.AccountGroups.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[0]}, func() {
				printer.Success("Deleted account group %s.", args[0])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
