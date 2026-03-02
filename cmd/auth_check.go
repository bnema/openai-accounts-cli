package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newAuthCheckCmd(app *app) *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verify authentication for one or all ChatGPT accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuthCheck(cmd, app, accountID)
		},
	}

	cmd.Flags().StringVar(&accountID, "account", "", "Account ID (default: all accounts)")

	return cmd
}

func runAuthCheck(cmd *cobra.Command, app *app, accountID string) error {
	statuses, err := loadStatuses(cmd, app.service, accountID)
	if err != nil {
		return err
	}

	accounts := filterChatGPTAccounts(statuses)
	if len(accounts) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no ChatGPT accounts to check")
		return nil
	}

	fetchFn := func(ctx context.Context) error {
		return fetchAccountsConcurrently(ctx, app, accounts, cmd.ErrOrStderr())
	}

	if err := runUsageFetchSpinner(cmd.Context(), cmd.ErrOrStderr(), "Checking auth...", fetchFn); err != nil {
		return err
	}

	updated, err := loadStatuses(cmd, app.service, accountID)
	if err != nil {
		return err
	}

	now := app.now()
	for _, status := range updated {
		if status.Account.Auth.Method == "" {
			continue
		}
		label := status.Account.Name
		if label == "" {
			label = string(status.Account.ID)
		}

		expiryParts := ""
		if status.DailyLimit != nil && !status.DailyLimit.CapturedAt.IsZero() {
			age := now.Sub(status.DailyLimit.CapturedAt).Round(time.Second)
			expiryParts = fmt.Sprintf("data fetched %s ago", age)
		}

		if expiryParts != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "account %s (%s): ok — %s\n", status.Account.ID, label, expiryParts)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "account %s (%s): ok\n", status.Account.ID, label)
		}
	}

	return nil
}
