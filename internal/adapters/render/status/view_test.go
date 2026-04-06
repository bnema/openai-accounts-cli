package status

import (
	"strings"
	"testing"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSingleAccountStatus(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{
				ID:   "acc-1",
				Name: "Primary",
				Auth: domain.Auth{Method: domain.AuthMethodAPIKey},
			},
			Usage: domain.Usage{InputTokens: 1200, OutputTokens: 800, CachedInputTokens: 500},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    73.2,
				ResetsAt:   now.Add(13 * time.Hour),
				CapturedAt: now.Add(-15 * time.Minute),
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "accounts: 1")
	assert.Contains(t, output, "Primary")
	assert.Contains(t, output, "5hours limit:")
	assert.Contains(t, output, "27% left")
	assert.Contains(t, output, "resets in 13 hours (00:00)")
	assert.Contains(t, output, "[")
	assert.Contains(t, output, "]")
	assert.NotContains(t, output, "stale")
}

func TestRenderMultiAccountStatus(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{
				ID:   "acc-1",
				Name: "Primary",
				Auth: domain.Auth{Method: domain.AuthMethodAPIKey},
			},
			Usage: domain.Usage{InputTokens: 1000, OutputTokens: 500, CachedInputTokens: 100},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    52.5,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{
				ID:   "acc-2",
				Name: "Backup",
				Auth: domain.Auth{Method: domain.AuthMethodAPIKey},
			},
			Usage: domain.Usage{InputTokens: 400, OutputTokens: 200, CachedInputTokens: 0},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    12.3,
				ResetsAt:   now.Add(4 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 24 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "accounts: 2")
	assert.Contains(t, output, "Primary")
	assert.Contains(t, output, "Backup")
	assert.Contains(t, output, "5hours limit:")
	assert.Contains(t, output, "weekly limit:")
	assert.Contains(t, output, "48% left")
	assert.Contains(t, output, "88% left")
	assert.Contains(t, output, "resets in 5 hours (16:00)")
	assert.Contains(t, output, "resets in 4 days (11:00 on 18 Feb)")
}

func TestRenderMarksStaleLimitSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{
				ID:   "acc-1",
				Name: "Primary",
				Auth: domain.Auth{Method: domain.AuthMethodAPIKey},
			},
			Usage: domain.Usage{InputTokens: 300, OutputTokens: 200, CachedInputTokens: 50},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    80,
				ResetsAt:   now.Add(8 * time.Hour),
				CapturedAt: now.Add(-48 * time.Hour),
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 12 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "5hours limit:")
	assert.Contains(t, output, "20% left")
	assert.Contains(t, output, "[stale]")
}

func TestFormatResetRelativeRendersClockTimeInNowLocation(t *testing.T) {
	loc := time.FixedZone("CEST", 2*60*60)
	now := time.Date(2026, 4, 5, 19, 37, 0, 0, loc)
	resetAt := time.Date(2026, 4, 5, 17, 48, 0, 0, time.UTC)

	assert.Equal(t, "resets in 1 hour (19:48)", formatResetRelative(resetAt, now))
}

func TestRenderShowsDailyAndWeeklyLimitsWhenBothAvailable(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "Primary", Auth: domain.Auth{Method: domain.AuthMethodAPIKey}},
			Usage:   domain.Usage{InputTokens: 100, OutputTokens: 50, CachedInputTokens: 25},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    80,
				ResetsAt:   now.Add(12 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    45,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "5hours limit:")
	assert.Contains(t, output, "weekly limit:")
	assert.Contains(t, output, "20% left")
	assert.Contains(t, output, "55% left")
}

func TestRenderDoesNotMarkStaleWhenNowNotProvided(t *testing.T) {
	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "Primary", Auth: domain.Auth{Method: domain.AuthMethodAPIKey}},
			Usage:   domain.Usage{InputTokens: 300, OutputTokens: 200, CachedInputTokens: 50},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    80,
				ResetsAt:   time.Date(2026, 2, 15, 11, 0, 0, 0, time.UTC),
				CapturedAt: time.Date(2026, 2, 10, 11, 0, 0, 0, time.UTC),
			},
		},
	}, RenderOptions{StaleAfter: 12 * time.Hour})

	require.NoError(t, err)
	assert.NotContains(t, output, "[stale]")
}

func TestRenderShowsUnavailableUsageHintForChatGPTWithoutTokenSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "Primary", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			Usage:   domain.Usage{},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    80,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "Primary")
	assert.Contains(t, output, "5hours limit:")
}

func TestRenderRecommendationUsesProvidedResultWithoutRecomputingRanking(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)
	statuses := []application.Status{
		{
			Account: domain.Account{ID: "acc-display-first", Name: "display-first@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    10,
				ResetsAt:   now.Add(24 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{ID: "acc-provided", Name: "provided@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    20,
				ResetsAt:   now.Add(7 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}

	recommendation := application.RecommendationResult{
		Pool: domain.SelectionPoolFallback,
		Ordered: []application.RecommendedAccount{
			{Status: statuses[1], Pool: domain.SelectionPoolFallback, Rank: 1},
			{Status: statuses[0], Pool: domain.SelectionPoolFallback, Rank: 2},
		},
	}
	recommendation.Selected = &recommendation.Ordered[0]

	output, err := Render(statuses, RenderOptions{
		Now:                    now,
		StaleAfter:             6 * time.Hour,
		Recommendation:         recommendation,
		RecommendationProvided: true,
	})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: use provided@example.com")
	assert.Contains(t, output, "next: display-first@example.com")

	firstDisplayIndex := strings.Index(output, "Account #acc-display-first: display-first@example.com")
	providedIndex := strings.Index(output, "Account #acc-provided: provided@example.com")

	require.NotEqual(t, -1, firstDisplayIndex)
	require.NotEqual(t, -1, providedIndex)
	assert.Less(t, firstDisplayIndex, providedIndex)
}

func TestRenderOmitsRecommendationSectionWhenNotSupplied(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "available@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    15,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.NotContains(t, output, "recommendation:")
	assert.NotContains(t, output, "details:")
	assert.NotContains(t, output, "next:")
}

func TestRenderRecommendationUsesProvidedResetExhaustionReason(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "available@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    15,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{
		Now:                    now,
		StaleAfter:             6 * time.Hour,
		Recommendation:         application.RecommendationResult{UnavailableMessage: "recommendation: no account available now (waiting for reset)"},
		RecommendationProvided: true,
	})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: no account available now (waiting for reset)")
	assert.NotContains(t, output, "recommendation: use available@example.com")
}

func TestRenderRecommendationUsesProvidedSubscriptionReason(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1", Name: "available@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    15,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{
		Now:        now,
		StaleAfter: 6 * time.Hour,
		Recommendation: application.RecommendationResult{
			UnavailableMessage: "recommendation: no eligible account now (subscription rules)",
		},
		RecommendationProvided: true,
	})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: no eligible account now (subscription rules)")
	assert.NotContains(t, output, "recommendation: no account available now (waiting for reset)")
}

func TestRenderPreservesSuppliedAccountOrder(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)
	statuses := []application.Status{
		{
			Account: domain.Account{ID: "acc-blocked", Name: "blocked@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    0,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    100,
				ResetsAt:   now.Add(2 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{ID: "acc-mid", Name: "mid@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    70,
				ResetsAt:   now.Add(2 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{ID: "acc-best", Name: "best@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    0,
				ResetsAt:   now.Add(5 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}

	recommendation := application.RecommendationResult{
		Pool: domain.SelectionPoolFallback,
		Ordered: []application.RecommendedAccount{
			{Status: statuses[2], Pool: domain.SelectionPoolFallback, Rank: 1},
			{Status: statuses[1], Pool: domain.SelectionPoolFallback, Rank: 2},
		},
	}
	recommendation.Selected = &recommendation.Ordered[0]

	output, err := Render(statuses, RenderOptions{
		Now:                    now,
		StaleAfter:             6 * time.Hour,
		Recommendation:         recommendation,
		RecommendationProvided: true,
	})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation:")
	assert.Contains(t, output, "best@example.com")
	assert.Contains(t, output, "details:")
	assert.Contains(t, output, "next:")
	assert.NotContains(t, output, "recommendation: use Account:")

	bestIndex := strings.Index(output, "Account #acc-best: best@example.com")
	midIndex := strings.Index(output, "Account #acc-mid: mid@example.com")
	blockedIndex := strings.Index(output, "Account #acc-blocked: blocked@example.com")

	require.NotEqual(t, -1, bestIndex)
	require.NotEqual(t, -1, midIndex)
	require.NotEqual(t, -1, blockedIndex)
	assert.Less(t, blockedIndex, midIndex)
	assert.Less(t, midIndex, bestIndex)
}
