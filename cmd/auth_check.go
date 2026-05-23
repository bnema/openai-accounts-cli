package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
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
	if wantsJSON(cmd) {
		cmd.Root().SilenceErrors = true
	}

	statuses, err := loadStatuses(cmd, app.service, accountID)
	if err != nil {
		return writeJSONError(cmd, err)
	}

	accounts := filterChatGPTAccounts(statuses)
	if len(accounts) == 0 {
		if wantsJSON(cmd) {
			return writeJSONOutput(cmd, map[string]any{"ok": true, "accounts": []any{}})
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "no ChatGPT accounts to check"); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}

	// Snapshot CapturedAt before fetch to detect which accounts actually updated.
	beforeFetch := capturedAtSnapshot(statuses)

	var summary fetchSummary
	fetchFn := func(ctx context.Context) error {
		result, err := fetchAccountsConcurrently(ctx, app, accounts)
		summary = result
		if !wantsJSON(cmd) {
			if writeErr := writeFetchSummaryPlain(cmd.ErrOrStderr(), result); writeErr != nil {
				if err != nil {
					return errors.Join(err, fmt.Errorf("write fetch summary: %w", writeErr))
				}
				return fmt.Errorf("write fetch summary: %w", writeErr)
			}
		}
		return err
	}

	if wantsJSON(cmd) {
		if err := fetchFn(cmd.Context()); err != nil {
			return writeJSONError(cmd, err)
		}
	} else {
		if err := runUsageFetchSpinner(cmd.Context(), cmd.ErrOrStderr(), "Checking auth...", fetchFn); err != nil {
			return err
		}
	}

	updated, err := loadStatuses(cmd, app.service, accountID)
	if err != nil {
		return writeJSONError(cmd, err)
	}

	now := app.now()
	var hasFailure bool
	failureMessages := summary.errorByAccountID()
	jsonAccounts := make([]map[string]any, 0, len(updated))
	for _, status := range updated {
		if status.Account.Auth.Method != domain.AuthMethodChatGPT {
			continue
		}

		label := status.Account.Name
		if label == "" {
			label = string(status.Account.ID)
		}

		if !didFetchSucceed(status, beforeFetch) {
			hasFailure = true
			if wantsJSON(cmd) {
				message := failureMessages[status.Account.ID]
				if message == "" {
					message = "authentication check failed"
				}
				jsonAccounts = append(jsonAccounts, map[string]any{
					"account_id": status.Account.ID,
					"name":       label,
					"ok":         false,
					"message":    message,
				})
				continue
			}
			// Error details already printed to stderr by fetchAccountsConcurrently.
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "account %s (%s): FAIL — see error above\n", status.Account.ID, label); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			continue
		}

		age := formatFetchAge(status, now)
		if wantsJSON(cmd) {
			jsonAccounts = append(jsonAccounts, map[string]any{
				"account_id": status.Account.ID,
				"name":       label,
				"ok":         true,
				"message":    age,
			})
			continue
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "account %s (%s): ok — %s\n", status.Account.ID, label, age); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}

	if wantsJSON(cmd) {
		if err := writeJSONOutput(cmd, map[string]any{"ok": !hasFailure, "accounts": jsonAccounts}); err != nil {
			return err
		}
		if hasFailure {
			return renderedCommandError{err: fmt.Errorf("one or more accounts failed authentication check")}
		}
		return nil
	}

	if hasFailure {
		return fmt.Errorf("one or more accounts failed authentication check")
	}
	return nil
}

// capturedAtSnapshot records the most recent CapturedAt for each account before a fetch.
func capturedAtSnapshot(statuses []application.Status) map[domain.AccountID]time.Time {
	m := make(map[domain.AccountID]time.Time, len(statuses))
	for _, s := range statuses {
		m[s.Account.ID] = mostRecentCapturedAt(s)
	}
	return m
}

// didFetchSucceed returns true if the account's CapturedAt advanced after the fetch.
func didFetchSucceed(status application.Status, before map[domain.AccountID]time.Time) bool {
	after := mostRecentCapturedAt(status)
	if after.IsZero() {
		return false
	}
	prev, ok := before[status.Account.ID]
	if !ok {
		return !after.IsZero()
	}
	return after.After(prev)
}

// mostRecentCapturedAt returns the most recent CapturedAt across daily and weekly limits.
func mostRecentCapturedAt(status application.Status) time.Time {
	var t time.Time
	if status.DailyLimit != nil && status.DailyLimit.CapturedAt.After(t) {
		t = status.DailyLimit.CapturedAt
	}
	if status.WeeklyLimit != nil && status.WeeklyLimit.CapturedAt.After(t) {
		t = status.WeeklyLimit.CapturedAt
	}
	return t
}

// formatFetchAge returns a human-readable description of how recently the data was fetched.
func formatFetchAge(status application.Status, now time.Time) string {
	t := mostRecentCapturedAt(status)
	if t.IsZero() {
		return "data fetched"
	}
	return fmt.Sprintf("data fetched %s ago", now.Sub(t).Round(time.Second))
}
