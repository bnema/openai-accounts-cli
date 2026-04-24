package cmd

import "github.com/spf13/cobra"

func newSyncOpencodeCmd(app *app, evenly *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "opencode",
		Short: "Sync OpenCode auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return syncSingleTarget(cmd, app, *evenly, "OpenCode", writeOAuthTokensToOpencode)
		},
	}
}

func newOpencodeSyncCmd(app *app) *cobra.Command {
	evenly := false
	cmd := newSyncOpencodeCmd(app, &evenly)
	cmd.Use = "sync"
	cmd.Hidden = true
	cmd.Deprecated = "use `oa sync opencode` instead"
	cmd.Flags().BoolVar(&evenly, "evenly", false, "rebalance among top candidates using recent selection history")
	return cmd
}
