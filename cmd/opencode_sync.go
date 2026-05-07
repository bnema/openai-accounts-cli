package cmd

import "github.com/spf13/cobra"

func newSyncOpencodeCmd(app *app, flags *syncFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "opencode",
		Short: "Sync OpenCode auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return syncSingleTarget(cmd, app, flags.options(), "OpenCode", writeOAuthTokensToOpencode)
		},
	}
}

func newOpencodeSyncCmd(app *app) *cobra.Command {
	flags := syncFlags{}
	cmd := newSyncOpencodeCmd(app, &flags)
	cmd.Use = "sync"
	cmd.Hidden = true
	cmd.Deprecated = "use `oa sync opencode` instead"
	cmd.Flags().BoolVar(&flags.evenly, "evenly", false, "rebalance among top candidates using recent selection history")
	cmd.Flags().StringVar(&flags.forceAccountID, "force-account-id", "", "sync with this account ID instead of the best ranked account")
	return cmd
}
