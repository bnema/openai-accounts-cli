package cmd

import (
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newOpencodeSyncCmd(app *app) *cobra.Command {
	var accountID string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync OpenCode auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := syncAccountIntoOpencode(cmd.Context(), app, domain.AccountID(accountID))
			return err
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Account ID")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}
