package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	authadapter "github.com/bnema/openai-accounts-cli/internal/adapters/auth"
	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func newAuthCheckCmd(app *app) *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check authentication validity for one or all ChatGPT accounts",
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

	var hasError bool
	for _, status := range statuses {
		if err := checkAccountAuth(cmd, app, status); err != nil {
			hasError = true
		}
	}

	if hasError {
		return fmt.Errorf("one or more accounts have invalid authentication")
	}
	return nil
}

func checkAccountAuth(cmd *cobra.Command, app *app, status application.Status) error {
	account := status.Account
	out := cmd.OutOrStdout()

	if account.Auth.Method != domain.AuthMethodChatGPT {
		fmt.Fprintf(out, "account %s (%s): skip (auth method: %s)\n", account.ID, account.Name, account.Auth.Method)
		return nil
	}

	secretRef := strings.TrimSpace(account.Auth.SecretRef)
	if secretRef == "" {
		err := fmt.Errorf("account %s: auth secret reference is empty", account.ID)
		fmt.Fprintf(out, "account %s (%s): FAIL — %v\n", account.ID, account.Name, err)
		return err
	}

	secretValue, err := app.secretStore.Get(cmd.Context(), secretRef)
	if err != nil {
		fmt.Fprintf(out, "account %s (%s): FAIL — load secret: %v\n", account.ID, account.Name, err)
		return err
	}

	tokens, err := decodeOAuthTokens(secretValue)
	if err != nil {
		fmt.Fprintf(out, "account %s (%s): FAIL — decode tokens: %v\n", account.ID, account.Name, err)
		return err
	}

	// Token expiry info
	expiryInfo := formatTokenExpiry(tokens, app.now())

	// Attempt refresh if needed
	tokens, err = ensureFreshTokens(cmd.Context(), app, account, tokens, false)
	if err != nil {
		if errors.Is(err, authadapter.ErrRefreshTokenInvalid) {
			fmt.Fprintf(out, "account %s (%s): FAIL — session expired, re-login with `oa auth login browser --account %s`\n", account.ID, account.Name, account.ID)
			return err
		}
		fmt.Fprintf(out, "account %s (%s): FAIL — refresh tokens: %v\n", account.ID, account.Name, err)
		return err
	}

	// Ping the usage endpoint
	_, err = fetchUsagePayload(cmd.Context(), app.httpClient, app.usageBaseURL, tokens)
	if err != nil {
		if errors.Is(err, errUsageSessionExpired) {
			fmt.Fprintf(out, "account %s (%s): FAIL — session expired, re-login with `oa auth login browser --account %s`\n", account.ID, account.Name, account.ID)
			return fmt.Errorf("session expired")
		}
		fmt.Fprintf(out, "account %s (%s): FAIL — ping failed: %v\n", account.ID, account.Name, err)
		return err
	}

	claims := parseTokenClaims(tokens.IDToken)
	email := strings.TrimSpace(claims.Email)
	if email == "" {
		email = strings.TrimSpace(account.Name)
	}

	fmt.Fprintf(out, "account %s (%s): ok — session valid, %s\n", account.ID, email, expiryInfo)
	return nil
}

func formatTokenExpiry(tokens oauthTokens, now time.Time) string {
	if tokens.ExpiresAt <= 0 {
		return "expires: unknown"
	}
	expiresAt := time.Unix(tokens.ExpiresAt, 0)
	if expiresAt.Before(now) {
		return fmt.Sprintf("expires: expired %s ago", now.Sub(expiresAt).Round(time.Minute))
	}
	return fmt.Sprintf("expires in %s", expiresAt.Sub(now).Round(time.Minute))
}
