package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

const noctaliaSnapshotSchemaVersion = 1

type noctaliaSnapshotOutput struct {
	SchemaVersion  int                       `json:"schema_version"`
	GeneratedAt    time.Time                 `json:"generated_at"`
	Refreshed      bool                      `json:"refreshed"`
	RefreshCommand []string                  `json:"refresh_command"`
	Recommendation noctaliaRecommendation    `json:"recommendation"`
	Accounts       []noctaliaAccount         `json:"accounts"`
	SyncTargets    []noctaliaSyncTarget      `json:"sync_targets"`
	Warnings       []noctaliaSnapshotWarning `json:"warnings,omitempty"`
}

type noctaliaRecommendation struct {
	Available   bool   `json:"available"`
	AccountID   string `json:"account_id,omitempty"`
	AccountName string `json:"account_name,omitempty"`
	Rank        int    `json:"rank,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Message     string `json:"message,omitempty"`
}

type noctaliaAccount struct {
	ID             string                        `json:"id"`
	Name           string                        `json:"name"`
	Provider       string                        `json:"provider,omitempty"`
	Model          string                        `json:"model,omitempty"`
	PlanType       string                        `json:"plan_type,omitempty"`
	AuthMethod     string                        `json:"auth_method,omitempty"`
	AuthConfigured bool                          `json:"auth_configured"`
	Recommendation noctaliaAccountRecommendation `json:"recommendation"`
	Daily          *noctaliaLimit                `json:"daily,omitempty"`
	Weekly         *noctaliaLimit                `json:"weekly,omitempty"`
	Subscription   *noctaliaSubscription         `json:"subscription,omitempty"`
}

type noctaliaAccountRecommendation struct {
	Rank     int  `json:"rank,omitempty"`
	Selected bool `json:"selected"`
	Eligible bool `json:"eligible"`
}

type noctaliaLimit struct {
	PercentUsed      float64   `json:"percent_used"`
	PercentRemaining float64   `json:"percent_remaining"`
	ResetsAt         time.Time `json:"resets_at,omitempty"`
	CapturedAt       time.Time `json:"captured_at,omitempty"`
}

type noctaliaSubscription struct {
	ActiveStart     time.Time `json:"active_start,omitempty"`
	ActiveUntil     time.Time `json:"active_until,omitempty"`
	WillRenew       bool      `json:"will_renew"`
	BillingPeriod   string    `json:"billing_period,omitempty"`
	BillingCurrency string    `json:"billing_currency,omitempty"`
	IsDelinquent    bool      `json:"is_delinquent"`
	CapturedAt      time.Time `json:"captured_at,omitempty"`
}

type noctaliaSyncTarget struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Command []string `json:"command"`
}

type noctaliaSnapshotWarning struct {
	AccountID string `json:"account_id"`
	Message   string `json:"message"`
}

func newNoctaliaCmd(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "noctalia",
		Short: "Machine-friendly endpoints for the Noctalia plugin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wantsJSON(cmd) {
				return writeJSONError(cmd, fmt.Errorf("%s: subcommand required", cmd.CommandPath()))
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(newNoctaliaSnapshotCmd(app))

	return cmd
}

func newNoctaliaSnapshotCmd(app *app) *cobra.Command {
	var refresh bool

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Return a stable snapshot for the Noctalia plugin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !wantsJSON(cmd) {
				return writeJSONError(cmd, fmt.Errorf("%s: --json is required", cmd.CommandPath()))
			}

			payload, err := buildNoctaliaSnapshot(cmd, app, refresh)
			if err != nil {
				return writeJSONError(cmd, err)
			}
			return writeJSONOutput(cmd, payload)
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh usage data before building the snapshot")

	return cmd
}

func buildNoctaliaSnapshot(cmd *cobra.Command, app *app, refresh bool) (noctaliaSnapshotOutput, error) {
	statuses, err := loadStatuses(cmd, app.service, "")
	if err != nil {
		return noctaliaSnapshotOutput{}, err
	}

	summary := fetchSummary{}
	if refresh {
		summary, err = refreshNoctaliaStatuses(cmd.Context(), app, statuses)
		if err != nil {
			return noctaliaSnapshotOutput{}, err
		}

		statuses, err = loadStatuses(cmd, app.service, "")
		if err != nil {
			return noctaliaSnapshotOutput{}, err
		}
	}

	now := app.now()
	recommendation := application.RecommendAccountsFromStatuses(statuses, now)

	payload := noctaliaSnapshotOutput{
		SchemaVersion:  noctaliaSnapshotSchemaVersion,
		GeneratedAt:    now,
		Refreshed:      refresh,
		RefreshCommand: []string{"noctalia", "snapshot", "--json", "--refresh"},
		Recommendation: buildNoctaliaRecommendation(recommendation),
		Accounts:       buildNoctaliaAccounts(statuses, recommendation, now),
		SyncTargets: []noctaliaSyncTarget{
			{ID: "opencode", Label: "OpenCode", Command: []string{"sync", "opencode", "--json"}},
			{ID: "codex", Label: "Codex", Command: []string{"sync", "codex", "--json"}},
			{ID: "pi", Label: "Pi", Command: []string{"sync", "pi", "--json"}},
			{ID: "all", Label: "All", Command: []string{"sync", "--all", "--json"}},
		},
		Warnings: buildNoctaliaWarnings(summary),
	}

	return payload, nil
}

func refreshNoctaliaStatuses(ctx context.Context, app *app, statuses []application.Status) (fetchSummary, error) {
	accounts := filterChatGPTAccounts(statuses)
	if len(accounts) == 0 {
		return fetchSummary{}, nil
	}

	return fetchAccountsConcurrently(ctx, app, accounts)
}

func buildNoctaliaRecommendation(recommendation application.RecommendationResult) noctaliaRecommendation {
	output := noctaliaRecommendation{}
	if recommendation.Selected != nil {
		output.Available = true
		output.AccountID = string(recommendation.Selected.Status.Account.ID)
		output.AccountName = recommendation.Selected.Status.Account.Name
		output.Rank = recommendation.Selected.Rank
		output.Message = fmt.Sprintf("recommended account: %s", recommendation.Selected.Status.Account.Name)
		return output
	}

	output.Reason = string(recommendation.UnavailableReason)
	output.Message = recommendation.UnavailableMessage
	return output
}

func buildNoctaliaAccounts(statuses []application.Status, recommendation application.RecommendationResult, now time.Time) []noctaliaAccount {
	ranks := make(map[domain.AccountID]int, len(recommendation.Ordered))
	selected := make(map[domain.AccountID]bool, len(recommendation.Ordered))
	for _, candidate := range recommendation.Ordered {
		ranks[candidate.Status.Account.ID] = candidate.Rank
	}
	if recommendation.Selected != nil {
		selected[recommendation.Selected.Status.Account.ID] = true
	}

	accounts := make([]noctaliaAccount, 0, len(statuses))
	for _, status := range statuses {
		account := noctaliaAccount{
			ID:             string(status.Account.ID),
			Name:           status.Account.Name,
			Provider:       status.Account.Metadata.Provider,
			Model:          status.Account.Metadata.Model,
			PlanType:       status.Account.Metadata.PlanType,
			AuthMethod:     string(status.Account.Auth.Method),
			AuthConfigured: status.Account.Auth.Method != "" && status.Account.Auth.SecretRef != "",
			Recommendation: noctaliaAccountRecommendation{
				Rank:     ranks[status.Account.ID],
				Selected: selected[status.Account.ID],
				Eligible: domain.AccountUsableForSelection(status.Account, now),
			},
			Daily:        buildNoctaliaLimit(status.Account.ID, "daily", status.DailyLimit),
			Weekly:       buildNoctaliaLimit(status.Account.ID, "weekly", status.WeeklyLimit),
			Subscription: buildNoctaliaSubscription(status.Subscription),
		}
		accounts = append(accounts, account)
	}

	return accounts
}

func buildNoctaliaLimit(accountID domain.AccountID, window string, limit *application.StatusLimit) *noctaliaLimit {
	if limit == nil {
		return nil
	}

	if limit.Percent < 0 || limit.Percent > 100 {
		log.Printf("warning: noctalia snapshot %s limit percent out of range for account %s: %.2f", window, accountID, limit.Percent)
	}

	remaining := 100 - limit.Percent
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 100 {
		remaining = 100
	}

	return &noctaliaLimit{
		PercentUsed:      limit.Percent,
		PercentRemaining: remaining,
		ResetsAt:         limit.ResetsAt,
		CapturedAt:       limit.CapturedAt,
	}
}

func buildNoctaliaSubscription(sub *application.StatusSubscription) *noctaliaSubscription {
	if sub == nil {
		return nil
	}

	return &noctaliaSubscription{
		ActiveStart:     sub.ActiveStart,
		ActiveUntil:     sub.ActiveUntil,
		WillRenew:       sub.WillRenew,
		BillingPeriod:   sub.BillingPeriod,
		BillingCurrency: sub.BillingCurrency,
		IsDelinquent:    sub.IsDelinquent,
		CapturedAt:      sub.CapturedAt,
	}
}

func buildNoctaliaWarnings(summary fetchSummary) []noctaliaSnapshotWarning {
	if !summary.hasFailures() {
		return nil
	}

	warnings := make([]noctaliaSnapshotWarning, 0, len(summary.failures))
	for _, failure := range summary.failures {
		warnings = append(warnings, noctaliaSnapshotWarning{
			AccountID: string(failure.accountID),
			Message:   failure.err.Error(),
		})
	}
	return warnings
}
