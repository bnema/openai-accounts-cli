package cmd

import (
	"errors"
	"fmt"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/spf13/cobra"
)

func newOpencodeSyncCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync OpenCode auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ranked, err := app.service.RankOpencodeSyncAccounts(cmd.Context())
			if err != nil {
				return err
			}
			if len(ranked) == 0 {
				return application.ErrNoEligibleOpencodeAccount
			}

			var (
				status  application.Status
				syncErr error
			)
			for _, candidate := range ranked {
				status, syncErr = syncAccountIntoOpencode(cmd.Context(), app, candidate.Account.ID)
				if syncErr == nil {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "synced OpenCode auth for %s (%s)\n", status.Account.Name, status.Account.ID)
					return err
				}
				if !errors.Is(syncErr, errOpencodeCandidateUnavailable) {
					return syncErr
				}
			}

			return syncErr
		},
	}
}
