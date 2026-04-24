package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/spf13/cobra"
)

type syncTokenWriter func(app *app, tokens oauthTokens) error

type syncTargetTokenWriter struct {
	name  string
	write syncTokenWriter
}

func newSyncCmd(app *app) *cobra.Command {
	var evenly bool
	var all bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync ChatGPT OAuth auth into local tools",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !all {
				return cmd.Help()
			}
			return syncAllTargets(cmd, app, evenly)
		},
	}

	cmd.PersistentFlags().BoolVar(&evenly, "evenly", false, "rebalance among top candidates using recent selection history")
	cmd.Flags().BoolVar(&all, "all", false, "sync all supported targets")

	cmd.AddCommand(
		newSyncOpencodeCmd(app, &evenly),
		newSyncCodexCmd(app, &evenly),
		newSyncPICmd(app, &evenly),
	)

	return cmd
}

func newSyncCodexCmd(app *app, evenly *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "codex",
		Short: "Sync Codex auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return syncSingleTarget(cmd, app, *evenly, "Codex", writeOAuthTokensToCodex)
		},
	}
}

func newSyncPICmd(app *app, evenly *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "pi",
		Aliases: []string{"pi-mono"},
		Short:   "Sync Pi auth",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return syncSingleTarget(cmd, app, *evenly, "Pi", writeOAuthTokensToPI)
		},
	}
}

func syncSingleTarget(cmd *cobra.Command, app *app, evenly bool, targetName string, writer syncTokenWriter) error {
	tokens, status, err := loadTokensForBestCandidate(cmd, app, evenly)
	if err != nil {
		return err
	}
	if err := writer(app, tokens); err != nil {
		return err
	}
	if evenly {
		if err := app.service.RecordSelection(cmd.Context(), status.Account.ID); err != nil {
			return fmt.Errorf("record selection history: %w", err)
		}
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "synced %s auth for %s (%s)\n", targetName, status.Account.Name, status.Account.ID)
	return err
}

func loadTokensForBestCandidate(cmd *cobra.Command, app *app, evenly bool) (oauthTokens, application.Status, error) {
	ranked, err := freshRankedSyncAccounts(cmd, app, evenly)
	if err != nil {
		return oauthTokens{}, application.Status{}, err
	}

	return loadTokensForAvailableAccount(cmd, app, ranked)
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

func freshRankedSyncAccounts(cmd *cobra.Command, app *app, evenly bool) ([]application.Status, error) {
	if err := refreshSyncUsageCaches(cmd, app); err != nil {
		return nil, err
	}

	ranked, err := app.service.RankSyncAccounts(cmd.Context())
	if err != nil {
		return nil, err
	}
	if evenly {
		ranked, err = app.service.RebalanceStatusesEvenly(cmd.Context(), ranked)
		if err != nil {
			return nil, err
		}
	}

	return ranked, nil
}

func refreshSyncUsageCaches(cmd *cobra.Command, app *app) error {
	statuses, err := app.service.GetStatusAll(cmd.Context())
	if err != nil {
		return err
	}

	return fetchAccountsConcurrently(cmd.Context(), app, filterChatGPTAccounts(statuses), cmd.ErrOrStderr())
}

func syncAllTargets(cmd *cobra.Command, app *app, evenly bool) error {
	ranked, err := freshRankedSyncAccounts(cmd, app, evenly)
	if err != nil {
		return err
	}
	if len(ranked) == 0 {
		return application.ErrNoEligibleSyncAccount
	}

	tokens, status, err := loadTokensForAvailableAccount(cmd, app, ranked)
	if err != nil {
		return err
	}

	syncedTargets, err := writeOAuthTokensToAllTargets(app, tokens)
	if err != nil {
		return err
	}

	if evenly {
		if err := app.service.RecordSelection(cmd.Context(), status.Account.ID); err != nil {
			return fmt.Errorf("record selection history: %w", err)
		}
	}

	for _, targetName := range syncedTargets {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "synced %s auth for %s (%s)\n", targetName, status.Account.Name, status.Account.ID); err != nil {
			return err
		}
	}
	return nil
}

func writeOAuthTokensToAllTargets(app *app, tokens oauthTokens) ([]string, error) {
	targets := []syncTargetTokenWriter{
		{name: "OpenCode", write: writeOAuthTokensToOpencode},
		{name: "Codex", write: writeOAuthTokensToCodex},
		{name: "Pi", write: writeOAuthTokensToPI},
	}

	synced := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := target.write(app, tokens); err != nil {
			if len(synced) > 0 {
				return synced, fmt.Errorf("partial sync failure: synced %s; %s failed: %w", strings.Join(synced, ", "), target.name, err)
			}
			return synced, fmt.Errorf("%s failed: %w", target.name, err)
		}
		synced = append(synced, target.name)
	}

	return synced, nil
}
