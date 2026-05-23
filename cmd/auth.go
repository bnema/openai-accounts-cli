package cmd

import (
	"fmt"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newAuthCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage account authentication",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wantsJSON(cmd) {
				return writeJSONError(cmd, fmt.Errorf("%s: subcommand required", cmd.CommandPath()))
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAuthSetCmd(app), newAuthRemoveCmd(app), newLoginCmd(app), newAuthCheckCmd(app))

	return cmd
}

func newAuthSetCmd(app *app) *cobra.Command {
	var accountID string
	var method string
	var secretKey string
	var secretValue string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set account authentication",
		RunE: func(cmd *cobra.Command, _ []string) error {
			authMethod, err := parseAuthMethod(method)
			if err != nil {
				return writeJSONError(cmd, err)
			}
			resolvedAccountID, err := resolveAccountID(cmd.Context(), app, accountID)
			if err != nil {
				return writeJSONError(cmd, err)
			}

			if err := app.service.SetAuth(
				cmd.Context(),
				resolvedAccountID,
				authMethod,
				secretKey,
				secretValue,
			); err != nil {
				return writeJSONError(cmd, err)
			}

			if wantsJSON(cmd) {
				return writeJSONOutput(cmd, map[string]any{
					"ok":         true,
					"account_id": resolvedAccountID,
					"method":     authMethod,
					"secret_key": secretKey,
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account", "0", "Account ID (0 or empty auto-assigns next: 1,2,...)")
	cmd.Flags().StringVar(&method, "method", "", "Auth method (api_key|chatgpt)")
	cmd.Flags().StringVar(&secretKey, "secret-key", "", "Secret-store key")
	cmd.Flags().StringVar(&secretValue, "secret-value", "", "Secret value")
	_ = cmd.MarkFlagRequired("method")
	_ = cmd.MarkFlagRequired("secret-key")
	_ = cmd.MarkFlagRequired("secret-value")

	return cmd
}

func newAuthRemoveCmd(app *app) *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove account authentication",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := app.service.RemoveAuth(cmd.Context(), domain.AccountID(accountID)); err != nil {
				return writeJSONError(cmd, err)
			}
			if wantsJSON(cmd) {
				return writeJSONOutput(cmd, map[string]any{"ok": true, "account_id": accountID})
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account", "", "Account ID")
	_ = cmd.MarkFlagRequired("account")

	return cmd
}

func parseAuthMethod(raw string) (domain.AuthMethod, error) {
	method := domain.AuthMethod(raw)
	switch method {
	case domain.AuthMethodAPIKey:
		return method, nil
	case domain.AuthMethodChatGPT:
		return method, nil
	default:
		return "", fmt.Errorf("unsupported auth method %q", raw)
	}
}
