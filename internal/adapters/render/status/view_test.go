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

func TestRenderPrioritizesAccountsForWeeklyUsage(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
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
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

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
	assert.Less(t, bestIndex, midIndex)
	assert.Less(t, midIndex, blockedIndex)
}

func TestRenderRecommendationSkipsIneligibleSubscriptionStates(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-na", Name: "na@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    0,
				ResetsAt:   now.Add(7 * 24 * time.Hour),
				CapturedAt: now,
			},
			Subscription: &application.StatusSubscription{
				WillRenew: true,
			},
		},
		{
			Account: domain.Account{ID: "acc-expired", Name: "expired@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    5,
				ResetsAt:   now.Add(7 * 24 * time.Hour),
				CapturedAt: now,
			},
			Subscription: &application.StatusSubscription{
				ActiveUntil: now.Add(-2 * time.Hour),
				WillRenew:   false,
			},
		},
		{
			Account: domain.Account{ID: "acc-recommended", Name: "recommended@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    30,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    20,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
			Subscription: &application.StatusSubscription{
				ActiveUntil: now.Add(24 * time.Hour),
				WillRenew:   true,
			},
		},
		{
			Account: domain.Account{ID: "acc-next", Name: "next@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    25,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    25,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{ID: "acc-renewed", Name: "renewed@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    30,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
			Subscription: &application.StatusSubscription{
				ActiveUntil: now.Add(-2 * time.Hour),
				WillRenew:   true,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: use recommended@example.com")
	assert.Contains(t, output, "next: next@example.com")
	assert.NotContains(t, output, "recommendation: use na@example.com")
	assert.NotContains(t, output, "recommendation: use expired@example.com")
	assert.NotContains(t, output, "next: na@example.com")
	assert.NotContains(t, output, "next: expired@example.com")
}

func TestRenderRecommendationExplainsWhenOnlySubscriptionRulesExcludeAvailableAccounts(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-na", Name: "na@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
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
			Subscription: &application.StatusSubscription{
				WillRenew: true,
			},
		},
		{
			Account: domain.Account{ID: "acc-expired", Name: "expired@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    25,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    10,
				ResetsAt:   now.Add(6 * 24 * time.Hour),
				CapturedAt: now,
			},
			Subscription: &application.StatusSubscription{
				ActiveUntil: now.Add(-2 * time.Hour),
				WillRenew:   false,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: no eligible account now (subscription rules)")
	assert.NotContains(t, output, "recommendation: no account available now (waiting for reset)")
}

func TestRenderRecommendationUsesExpiryAwareRankingWithoutChangingDisplayOrder(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
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
			Account: domain.Account{ID: "acc-recommended", Name: "recommended@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
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
			Subscription: &application.StatusSubscription{
				ActiveUntil: now.Add(24 * time.Hour),
				WillRenew:   true,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: use recommended@example.com")

	firstDisplayIndex := strings.Index(output, "Account #acc-display-first: display-first@example.com")
	recommendedIndex := strings.Index(output, "Account #acc-recommended: recommended@example.com")

	require.NotEqual(t, -1, firstDisplayIndex)
	require.NotEqual(t, -1, recommendedIndex)
	assert.Less(t, firstDisplayIndex, recommendedIndex)
}

func TestRenderRecommendationKeepsPastDueAutoRenewingSubscriptionEligible(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-past-due-renewing", Name: "past-due-renewing@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    30,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    15,
				ResetsAt:   now.Add(5 * 24 * time.Hour),
				CapturedAt: now,
			},
			Subscription: &application.StatusSubscription{
				ActiveUntil: now.Add(-48 * time.Hour),
				WillRenew:   true,
			},
		},
		{
			Account: domain.Account{ID: "acc-secondary", Name: "secondary@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    35,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    25,
				ResetsAt:   now.Add(5 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: use past-due-renewing@example.com")
	assert.Contains(t, output, "next: secondary@example.com")
}

func TestRenderRecommendationTreatsImmediateWeeklyResetAsMaxUrgency(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-1hour", Name: "one-hour-reset@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    30,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    20,
				ResetsAt:   now.Add(1 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{ID: "acc-immediate", Name: "immediate-reset@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    30,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    20,
				ResetsAt:   now,
				CapturedAt: now,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: use immediate-reset@example.com")
	assert.Contains(t, output, "next: one-hour-reset@example.com")
}

func TestRenderRecommendationKeepsDailyRemainingSecondaryToWeeklyAndExpiry(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	output, err := Render([]application.Status{
		{
			Account: domain.Account{ID: "acc-high-daily", Name: "high-daily@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    5,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    35,
				ResetsAt:   now.Add(7 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{ID: "acc-expiring", Name: "expiring@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    85,
				ResetsAt:   now.Add(4 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &application.StatusLimit{
				Window:     application.LimitWindowWeekly,
				Percent:    20,
				ResetsAt:   now.Add(24 * time.Hour),
				CapturedAt: now,
			},
			Subscription: &application.StatusSubscription{
				ActiveUntil: now.Add(24 * time.Hour),
				WillRenew:   true,
			},
		},
	}, RenderOptions{Now: now, StaleAfter: 6 * time.Hour})

	require.NoError(t, err)
	assert.Contains(t, output, "recommendation: use expiring@example.com")
	assert.Contains(t, output, "next: high-daily@example.com")
}

func TestRecommendationEligibilityScoreTreatsBoundaryExpiryAsIneligible(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	eligible, score := recommendationEligibilityScore(application.Status{
		Account: domain.Account{ID: "acc-boundary", Name: "boundary@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
		WeeklyLimit: &application.StatusLimit{
			Window:     application.LimitWindowWeekly,
			Percent:    20,
			ResetsAt:   now.Add(24 * time.Hour),
			CapturedAt: now,
		},
		Subscription: &application.StatusSubscription{
			ActiveUntil: now,
			WillRenew:   false,
		},
	}, now)

	assert.False(t, eligible)
	assert.Zero(t, score)
}

func TestRecommendedStatusesPreservesVisibleOrderOnEqualScores(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)
	statuses := []application.Status{
		{
			Account: domain.Account{ID: "acc-zeta", Name: "zeta@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    30,
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
		{
			Account: domain.Account{ID: "acc-alpha", Name: "alpha@example.com", Auth: domain.Auth{Method: domain.AuthMethodChatGPT}},
			DailyLimit: &application.StatusLimit{
				Window:     application.LimitWindowDaily,
				Percent:    30,
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

	recommended := recommendedStatuses(statuses, now)

	require.Len(t, recommended, 2)
	assert.Equal(t, domain.AccountID("acc-zeta"), recommended[0].Account.ID)
	assert.Equal(t, domain.AccountID("acc-alpha"), recommended[1].Account.ID)
}
