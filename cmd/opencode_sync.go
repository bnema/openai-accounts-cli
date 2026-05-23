package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSyncOpencodeCmd(app *app, flags *syncFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "opencode",
		Short: "Sync OpenCode auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeJSONError(cmd, syncSingleTarget(cmd, app, flags.options(), "opencode", "OpenCode", writeOAuthTokensToOpencode))
		},
	}
}

func newOpencodeSyncCmd(app *app) *cobra.Command {
	flags := syncFlags{}
	cmd := newSyncOpencodeCmd(app, &flags)
	cmd.Use = "sync"
	cmd.Hidden = true
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if wantsJSON(cmd) {
			return nil
		}
		_, err := fmt.Fprintln(cmd.ErrOrStderr(), `Command "sync" is deprecated, use `+"`oa sync opencode`"+` instead`)
		return err
	}
	cmd.Flags().BoolVar(&flags.evenly, "evenly", false, "rebalance among top candidates using recent selection history")
	cmd.Flags().StringVar(&flags.forceAccountID, "force-account-id", "", "sync with this account ID instead of the best ranked account")
	return cmd
}
