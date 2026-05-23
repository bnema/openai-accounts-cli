package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"

	"github.com/spf13/cobra"
)

type syncTokenWriter func(app *app, tokens oauthTokens) error

type syncTargetTokenWriter struct {
	id    string
	name  string
	write syncTokenWriter
}

type syncFlags struct {
	evenly         bool
	forceAccountID string
}

func (f syncFlags) options() syncOptions {
	return syncOptions{
		evenly:         f.evenly,
		forceAccountID: domain.AccountID(f.forceAccountID),
	}
}

type syncOptions struct {
	evenly         bool
	forceAccountID domain.AccountID
}

func newSyncCmd(app *app) *cobra.Command {
	flags := syncFlags{}
	var all bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync ChatGPT OAuth auth into local tools",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !all {
				if wantsJSON(cmd) {
					return writeJSONError(cmd, fmt.Errorf("%s: --all or a target subcommand is required", cmd.CommandPath()))
				}
				return cmd.Help()
			}
			if err := syncAllTargets(cmd, app, flags.options()); err != nil {
				return writeJSONError(cmd, err)
			}
			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&flags.evenly, "evenly", false, "rebalance among top candidates using recent selection history")
	cmd.PersistentFlags().StringVar(&flags.forceAccountID, "force-account-id", "", "sync with this account ID instead of the best ranked account")
	cmd.Flags().BoolVar(&all, "all", false, "sync all supported targets")

	cmd.AddCommand(
		newSyncOpencodeCmd(app, &flags),
		newSyncCodexCmd(app, &flags),
		newSyncPICmd(app, &flags),
	)

	return cmd
}

func newSyncCodexCmd(app *app, flags *syncFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "codex",
		Short: "Sync Codex auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeJSONError(cmd, syncSingleTarget(cmd, app, flags.options(), "codex", "Codex", writeOAuthTokensToCodex))
		},
	}
}

func newSyncPICmd(app *app, flags *syncFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "pi",
		Aliases: []string{"pi-mono"},
		Short:   "Sync Pi auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeJSONError(cmd, syncSingleTarget(cmd, app, flags.options(), "pi", "Pi", writeOAuthTokensToPI))
		},
	}
}

func syncSingleTarget(cmd *cobra.Command, app *app, options syncOptions, targetID, targetName string, writer syncTokenWriter) error {
	tokens, status, summary, err := loadTokensForSync(cmd, app, options)
	if err != nil {
		return err
	}
	if err := writer(app, tokens); err != nil {
		return err
	}
	if options.evenly {
		if err := app.service.RecordSelection(cmd.Context(), status.Account.ID); err != nil {
			return fmt.Errorf("record selection history: %w", err)
		}
	}

	if wantsJSON(cmd) {
		payload := map[string]any{
			"ok":           true,
			"target":       targetID,
			"account_id":   status.Account.ID,
			"account_name": status.Account.Name,
		}
		if summary.hasFailures() {
			payload["warnings"] = summary.failurePayload()
		}
		return writeJSONOutput(cmd, payload)
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "synced %s auth for %s (%s)\n", targetName, status.Account.Name, status.Account.ID)
	return err
}

func loadTokensForSync(cmd *cobra.Command, app *app, options syncOptions) (oauthTokens, application.Status, fetchSummary, error) {
	if options.forceAccountID != "" {
		tokens, status, err := loadOAuthTokensForAccount(cmd.Context(), app, options.forceAccountID)
		return tokens, status, fetchSummary{}, err
	}

	ranked, summary, err := freshRankedSyncAccounts(cmd, app, options.evenly)
	if err != nil {
		return oauthTokens{}, application.Status{}, summary, err
	}

	tokens, status, err := loadTokensForAvailableAccount(cmd, app, ranked)
	return tokens, status, summary, err
}

func loadTokensForAvailableAccount(cmd *cobra.Command, app *app, ranked []application.Status) (oauthTokens, application.Status, error) {
	if len(ranked) == 0 {
		return oauthTokens{}, application.Status{}, application.ErrNoEligibleSyncAccount
	}

	var syncErr error
	for _, candidate := range ranked {
		tokens, status, err := loadOAuthTokensForAccount(cmd.Context(), app, candidate.Account.ID)
		syncErr = err
		if syncErr == nil {
			return tokens, status, nil
		}
		if !errors.Is(syncErr, errSyncCandidateUnavailable) {
			return oauthTokens{}, application.Status{}, syncErr
		}
	}

	return oauthTokens{}, application.Status{}, noUsableSyncAccountError(syncErr)
}

func noUsableSyncAccountError(syncErr error) error {
	if syncErr == nil {
		return application.ErrNoEligibleSyncAccount
	}
	return fmt.Errorf("%w: %s", application.ErrNoEligibleSyncAccount, syncErr.Error())
}

func freshRankedSyncAccounts(cmd *cobra.Command, app *app, evenly bool) ([]application.Status, fetchSummary, error) {
	summary, err := refreshSyncUsageCaches(cmd, app)
	if err != nil {
		return nil, summary, err
	}

	ranked, err := app.service.RankSyncAccounts(cmd.Context())
	if err != nil {
		return nil, summary, err
	}
	if evenly {
		ranked, err = app.service.RebalanceStatusesEvenly(cmd.Context(), ranked)
		if err != nil {
			return nil, summary, err
		}
	}

	return ranked, summary, nil
}

func refreshSyncUsageCaches(cmd *cobra.Command, app *app) (fetchSummary, error) {
	statuses, err := app.service.GetStatusAll(cmd.Context())
	if err != nil {
		return fetchSummary{}, err
	}

	summary, err := fetchAccountsConcurrently(cmd.Context(), app, filterChatGPTAccounts(statuses))
	if !wantsJSON(cmd) {
		writeFetchSummaryPlain(cmd.ErrOrStderr(), summary)
	}

	return summary, err
}

func syncAllTargets(cmd *cobra.Command, app *app, options syncOptions) error {
	tokens, status, summary, err := loadTokensForSync(cmd, app, options)
	if err != nil {
		return err
	}

	syncedTargets, err := writeOAuthTokensToAllTargets(app, tokens)
	if err != nil {
		return err
	}

	if options.evenly {
		if err := app.service.RecordSelection(cmd.Context(), status.Account.ID); err != nil {
			return fmt.Errorf("record selection history: %w", err)
		}
	}

	if wantsJSON(cmd) {
		targets := make([]string, 0, len(syncedTargets))
		for _, target := range syncedTargets {
			targets = append(targets, target.id)
		}
		payload := map[string]any{
			"ok":           true,
			"targets":      targets,
			"account_id":   status.Account.ID,
			"account_name": status.Account.Name,
		}
		if summary.hasFailures() {
			payload["warnings"] = summary.failurePayload()
		}
		return writeJSONOutput(cmd, payload)
	}

	for _, target := range syncedTargets {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "synced %s auth for %s (%s)\n", target.name, status.Account.Name, status.Account.ID); err != nil {
			return err
		}
	}
	return nil
}

func writeOAuthTokensToAllTargets(app *app, tokens oauthTokens) ([]syncTargetTokenWriter, error) {
	targets := []syncTargetTokenWriter{
		{id: "opencode", name: "OpenCode", write: writeOAuthTokensToOpencode},
		{id: "codex", name: "Codex", write: writeOAuthTokensToCodex},
		{id: "pi", name: "Pi", write: writeOAuthTokensToPI},
	}

	synced := make([]syncTargetTokenWriter, 0, len(targets))
	syncedNames := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := target.write(app, tokens); err != nil {
			if len(synced) > 0 {
				return synced, fmt.Errorf("partial sync failure: synced %s; %s failed: %w", strings.Join(syncedNames, ", "), target.name, err)
			}
			return synced, fmt.Errorf("%s failed: %w", target.name, err)
		}
		synced = append(synced, target)
		syncedNames = append(syncedNames, target.name)
	}

	return synced, nil
}
