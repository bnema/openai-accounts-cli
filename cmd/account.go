package cmd

import (
	"fmt"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newAccountCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wantsJSON(cmd) {
				return writeJSONError(cmd, fmt.Errorf("%s: subcommand required", cmd.CommandPath()))
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newAccountListCmd(app),
		newAccountRemoveCmd(app),
	)

	return cmd
}

func newAccountListCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			statuses, err := app.service.GetStatusAll(cmd.Context())
			if err != nil {
				return writeJSONError(cmd, err)
			}

			if wantsJSON(cmd) {
				accounts := make([]map[string]string, 0, len(statuses))
				for _, status := range statuses {
					accounts = append(accounts, map[string]string{
						"id":   string(status.Account.ID),
						"name": status.Account.Name,
					})
				}
				return writeJSONOutput(cmd, map[string]any{"accounts": accounts})
			}

			for _, status := range statuses {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", status.Account.ID, status.Account.Name)
			}

			return nil
		},
	}
}

func newAccountRemoveCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an account and its credentials",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := domain.AccountID(args[0])
			if err := app.service.RemoveAccount(cmd.Context(), id); err != nil {
				return writeJSONError(cmd, err)
			}
			if wantsJSON(cmd) {
				return writeJSONOutput(cmd, map[string]any{"ok": true, "account_id": id})
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "account %s removed\n", id)
			return nil
		},
	}
}

func newNotImplementedCmd(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("%s: %w", cmd.CommandPath(), errNotImplementedYet)
		},
	}
}
