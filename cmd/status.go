package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	statusadapter "github.com/bnema/openai-accounts-cli/internal/adapters/render/status"
	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

func writeStatusesOutput(cmd *cobra.Command, app *app, statuses []application.Status, recommendation application.RecommendationResult, recommendationProvided bool, now time.Time, staleAfter time.Duration, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(statuses)
	}

	ordered := application.OrderStatuses(statuses, now)

	rendered, err := app.statusRenderer(ordered, statusadapter.RenderOptions{
		Now:                    now,
		StaleAfter:             staleAfter,
		Recommendation:         recommendation,
		RecommendationProvided: recommendationProvided,
	})
	if err != nil {
		return fmt.Errorf("render status: %w", err)
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), rendered)
	return err
}

func loadStatuses(cmd *cobra.Command, svc *application.Service, accountID string) ([]application.Status, error) {
	if accountID == "" {
		statuses, err := svc.GetStatusAll(cmd.Context())
		if err != nil {
			return nil, err
		}
		return statuses, nil
	}

	status, err := svc.GetStatus(cmd.Context(), domain.AccountID(accountID))
	if err != nil {
		return nil, err
	}

	return []application.Status{status}, nil
}
