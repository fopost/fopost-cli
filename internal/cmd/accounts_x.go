package cmd

import (
	"github.com/spf13/cobra"
)

// Follow actions from a connected X account. Each one is an X API call
// metered to the workspace, and an account connected before the follow grant
// was added answers 409 reconnect_required until it is reconnected.
func newAccountsXCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "x",
		Aliases: []string{"twitter"},
		Short:   "Follow and unfollow X users as a connected account",
	}
	cmd.AddCommand(newXFollowCmd(state), newXUnfollowCmd(state))
	return cmd
}

func newXFollowCmd(state *State) *cobra.Command {
	var userID string
	cmd := &cobra.Command{
		Use:   "follow <account-id>",
		Short: "Follow an X user as this account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.FollowXUser(cmd.Context(), args[0], userID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Table([]string{"account", "user_id", "following"},
					[][]string{{result.AccountID, result.UserID, boolLabel(result.Following, "yes", "no")}})
			})
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "Numeric X user id to follow")
	_ = cmd.MarkFlagRequired("user-id")
	return cmd
}

func newXUnfollowCmd(state *State) *cobra.Command {
	var userID string
	cmd := &cobra.Command{
		Use:   "unfollow <account-id>",
		Short: "Unfollow an X user as this account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.UnfollowXUser(cmd.Context(), args[0], userID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(result, func() {
				printer.Table([]string{"account", "user_id", "following"},
					[][]string{{result.AccountID, result.UserID, boolLabel(result.Following, "yes", "no")}})
			})
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "Numeric X user id to unfollow")
	_ = cmd.MarkFlagRequired("user-id")
	return cmd
}
