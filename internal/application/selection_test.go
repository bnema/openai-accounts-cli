package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/bnema/openai-accounts-cli/internal/ports/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRebalanceStatusesEvenlyReordersAwayFromDefaultLeaderWithinTopBand(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	history := mocks.NewMockSelectionHistory(t)
	clock.EXPECT().Now().Return(now).Once()
	history.EXPECT().RecentSelections(mockAnyContext(), now.Add(-selectionFairnessWindow)).Return([]domain.AccountID{
		"acc-most-used",
		"acc-most-used",
		"acc-least-used",
	}, nil).Once()

	service := NewService(repo, store, clock, history)
	renewal := now.Add(14 * 24 * time.Hour)
	rebalanced, err := service.RebalanceStatusesEvenly(context.Background(), []Status{
		selectionStatus("acc-most-used", renewal, 10, 20),
		selectionStatus("acc-least-used", renewal, 11, 20),
		selectionStatus("acc-outside-band", renewal, 50, 20),
	})
	require.NoError(t, err)

	assert.Equal(t, []domain.AccountID{"acc-least-used", "acc-most-used", "acc-outside-band"}, []domain.AccountID{
		rebalanced[0].Account.ID,
		rebalanced[1].Account.ID,
		rebalanced[2].Account.ID,
	})
}

func TestServiceRebalanceStatusesEvenlyExcludesSameWeeklyButWorseDailyCandidateFromTopBand(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	history := mocks.NewMockSelectionHistory(t)
	clock.EXPECT().Now().Return(now).Once()

	service := NewService(repo, store, clock, history)
	rebalanced, err := service.RebalanceStatusesEvenly(context.Background(), []Status{
		selectionStatus("acc-default-leader", now.Add(14*24*time.Hour), 10, 20),
		selectionStatus("acc-same-weekly-worse-daily", now.Add(14*24*time.Hour), 45, 20),
	})
	require.NoError(t, err)

	assert.Equal(t, []domain.AccountID{"acc-default-leader", "acc-same-weekly-worse-daily"}, []domain.AccountID{
		rebalanced[0].Account.ID,
		rebalanced[1].Account.ID,
	})
}

func TestServiceRebalanceStatusesEvenlyRotatesOnlyWithinComparableRiskBand(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	history := mocks.NewMockSelectionHistory(t)
	clock.EXPECT().Now().Return(now).Once()
	history.EXPECT().RecentSelections(mockAnyContext(), now.Add(-selectionFairnessWindow)).Return([]domain.AccountID{
		"top-default",
		"top-default",
		"top-least-used",
	}, nil).Once()

	topRenewal := now.Add(10 * 24 * time.Hour)
	lowRiskRenewal := now.Add(20 * 24 * time.Hour)

	service := NewService(repo, store, clock, history)
	rebalanced, err := service.RebalanceStatusesEvenly(context.Background(), []Status{
		selectionStatus("top-default", topRenewal, 10, 20),
		selectionStatus("top-least-used", topRenewal, 11, 20),
		selectionStatus("lower-risk-never-used", lowRiskRenewal, 0, 20),
	})
	require.NoError(t, err)

	assert.Equal(t, []domain.AccountID{"top-least-used", "top-default", "lower-risk-never-used"}, []domain.AccountID{
		rebalanced[0].Account.ID,
		rebalanced[1].Account.ID,
		rebalanced[2].Account.ID,
	})
}

func TestServiceRebalanceStatusesEvenlyFallsBackToDefaultOrderWhenHistoryUnavailable(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	history := mocks.NewMockSelectionHistory(t)
	clock.EXPECT().Now().Return(now).Once()
	history.EXPECT().RecentSelections(mockAnyContext(), now.Add(-selectionFairnessWindow)).Return(nil, errors.New("history unavailable")).Once()

	service := NewService(repo, store, clock, history)
	renewal := now.Add(14 * 24 * time.Hour)
	rebalanced, err := service.RebalanceStatusesEvenly(context.Background(), []Status{
		selectionStatus("acc-best", renewal, 10, 20),
		selectionStatus("acc-next", renewal, 11, 20),
	})
	require.NoError(t, err)

	assert.Equal(t, []domain.AccountID{"acc-best", "acc-next"}, []domain.AccountID{
		rebalanced[0].Account.ID,
		rebalanced[1].Account.ID,
	})
}

func TestServiceRecommendAccountsPrioritizesSubscriptionDeadlinePressure(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)
	soonest := now.Add(24 * time.Hour)
	later := now.Add(72 * time.Hour)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(now).Once()

	service := NewService(repo, store, clock)
	repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
		selectionAccount("fallback-best", now.Add(14*24*time.Hour), 5, 5),
		selectionAccount("urgent-later", later, 60, 30),
		selectionAccount("urgent-sooner", soonest, 90, 40),
		{ID: "ineligible-without-subscription"},
	}, nil)

	selection, err := service.RecommendAccounts(context.Background())
	require.NoError(t, err)
	var result RecommendationResult
	result = selection
	assert.Equal(t, selection.Pool, result.Pool)

	require.Equal(t, domain.SelectionPoolFallback, selection.Pool)
	require.Len(t, selection.Ordered, 3)
	assert.Equal(t, domain.AccountID("urgent-sooner"), selection.Ordered[0].Status.Account.ID)
	assert.Equal(t, 1, selection.Ordered[0].Rank)
	assert.Equal(t, domain.AccountID("urgent-later"), selection.Ordered[1].Status.Account.ID)
	assert.Equal(t, 2, selection.Ordered[1].Rank)
	assert.Equal(t, domain.AccountID("fallback-best"), selection.Ordered[2].Status.Account.ID)
	assert.Equal(t, 3, selection.Ordered[2].Rank)
	require.NotNil(t, selection.Selected)
	var selected RecommendedAccount
	selected = *selection.Selected
	assert.Equal(t, selection.Ordered[0], selected)
}

func TestServiceRecommendAccountsUsesFallbackOrdering(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)
	earlier := now.Add(10 * 24 * time.Hour)
	later := now.Add(12 * 24 * time.Hour)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(now).Once()

	service := NewService(repo, store, clock)
	repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
		selectionAccount("weekly-best-daily-best", later, 30, 20),
		selectionAccount("weekly-best-daily-worse", earlier, 80, 20),
		selectionAccount("weekly-next", earlier, 1, 25),
		selectionAccount("weekly-best-daily-best-earlier", earlier, 30, 20),
	}, nil)

	selection, err := service.RecommendAccounts(context.Background())
	require.NoError(t, err)

	require.Equal(t, domain.SelectionPoolFallback, selection.Pool)
	require.Len(t, selection.Ordered, 4)
	assert.Equal(t, []domain.AccountID{
		"weekly-best-daily-best-earlier",
		"weekly-best-daily-worse",
		"weekly-next",
		"weekly-best-daily-best",
	}, []domain.AccountID{
		selection.Ordered[0].Status.Account.ID,
		selection.Ordered[1].Status.Account.ID,
		selection.Ordered[2].Status.Account.ID,
		selection.Ordered[3].Status.Account.ID,
	})
	require.NotNil(t, selection.Selected)
	assert.Equal(t, selection.Ordered[0], *selection.Selected)
}

func TestRecommendAccountsFromStatusesTreatsExhaustedDailyWithoutResetAsUnavailable(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)

	activeUntil := now.Add(24 * time.Hour)

	selection := RecommendAccountsFromStatuses([]Status{
		{
			Account: domain.Account{
				ID: "exhausted-daily-no-reset",
				Subscription: &domain.Subscription{
					ActiveUntil: activeUntil,
				},
				Limits: domain.AccountLimitSnapshots{
					Daily: &domain.AccountLimitSnapshot{Percent: 100},
				},
			},
			DailyLimit: &StatusLimit{
				Window:     LimitWindowDaily,
				Percent:    100,
				CapturedAt: now,
			},
			Subscription: &StatusSubscription{
				ActiveUntil: activeUntil,
			},
		},
	}, now)

	assert.Empty(t, selection.Ordered)
	assert.Nil(t, selection.Selected)
	assert.Equal(t, RecommendationUnavailableReset, selection.UnavailableReason)
}

func TestServiceRecommendAccountsSkipsOnePercentDailyRemainingUntilReset(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)
	dailyReset := now.Add(5 * time.Hour)
	weeklyReset := now.Add(48 * time.Hour)
	renewal := now.Add(14 * 24 * time.Hour)

	selection := RecommendAccountsFromStatuses([]Status{
		{
			Account: domain.Account{
				ID: "nearly-empty-5hours",
				Subscription: &domain.Subscription{
					ActiveUntil: renewal,
				},
				Limits: domain.AccountLimitSnapshots{
					Daily:  &domain.AccountLimitSnapshot{Percent: 99, ResetsAt: dailyReset},
					Weekly: &domain.AccountLimitSnapshot{Percent: 68, ResetsAt: weeklyReset},
				},
			},
			DailyLimit: &StatusLimit{
				Window:     LimitWindowDaily,
				Percent:    99,
				ResetsAt:   dailyReset,
				CapturedAt: now,
			},
			WeeklyLimit: &StatusLimit{
				Window:     LimitWindowWeekly,
				Percent:    68,
				ResetsAt:   weeklyReset,
				CapturedAt: now,
			},
			Subscription: &StatusSubscription{
				ActiveUntil: renewal,
			},
		},
		{
			Account: domain.Account{
				ID: "usable-5hours",
				Subscription: &domain.Subscription{
					ActiveUntil: renewal,
				},
				Limits: domain.AccountLimitSnapshots{
					Daily:  &domain.AccountLimitSnapshot{Percent: 0, ResetsAt: dailyReset},
					Weekly: &domain.AccountLimitSnapshot{Percent: 74, ResetsAt: weeklyReset},
				},
			},
			DailyLimit: &StatusLimit{
				Window:     LimitWindowDaily,
				Percent:    0,
				ResetsAt:   dailyReset,
				CapturedAt: now,
			},
			WeeklyLimit: &StatusLimit{
				Window:     LimitWindowWeekly,
				Percent:    74,
				ResetsAt:   weeklyReset,
				CapturedAt: now,
			},
			Subscription: &StatusSubscription{
				ActiveUntil: renewal,
			},
		},
	}, now)

	require.Len(t, selection.Ordered, 1)
	assert.Equal(t, domain.AccountID("usable-5hours"), selection.Ordered[0].Status.Account.ID)
	require.NotNil(t, selection.Selected)
	assert.Equal(t, selection.Ordered[0], *selection.Selected)
}

func TestServiceRecommendAccountsDoesNotRequireSyncSpecificAuth(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)
	soon := now.Add(24 * time.Hour)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(now).Once()

	service := NewService(repo, store, clock)
	repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
		{
			ID:   "usable-no-auth",
			Name: "No Auth",
			Subscription: &domain.Subscription{
				ActiveUntil: soon,
			},
			Limits: domain.AccountLimitSnapshots{
				Daily:  &domain.AccountLimitSnapshot{Percent: 10, ResetsAt: soon},
				Weekly: &domain.AccountLimitSnapshot{Percent: 20, ResetsAt: soon},
			},
		},
	}, nil)

	selection, err := service.RecommendAccounts(context.Background())
	require.NoError(t, err)

	require.Len(t, selection.Ordered, 1)
	assert.Equal(t, domain.AccountID("usable-no-auth"), selection.Ordered[0].Status.Account.ID)
	require.NotNil(t, selection.Selected)
	assert.Equal(t, selection.Ordered[0], *selection.Selected)
}

func TestServiceRecommendAccountsReturnsEmptySelectionWhenNoUsableAccounts(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(now).Once()

	service := NewService(repo, store, clock)
	repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
		{ID: "missing-subscription"},
		{
			ID: "exhausted",
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily: &domain.AccountLimitSnapshot{Percent: 100, ResetsAt: now.Add(30 * time.Minute)},
			},
		},
	}, nil)

	selection, err := service.RecommendAccounts(context.Background())
	require.NoError(t, err)

	assert.Equal(t, domain.SelectionPoolFallback, selection.Pool)
	assert.Empty(t, selection.Ordered)
	assert.Nil(t, selection.Selected)
	assert.Equal(t, RecommendationUnavailableSubscription, selection.UnavailableReason)
	assert.Equal(t, "recommendation: no eligible account now (subscription rules)", selection.UnavailableMessage)
}

func TestServiceRecommendAccountsMarksResetExhaustionWhenSubscriptionIsActive(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(now).Once()

	service := NewService(repo, store, clock)
	repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
		{
			ID: "reset-blocked",
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily: &domain.AccountLimitSnapshot{Percent: 100, ResetsAt: now.Add(30 * time.Minute)},
			},
		},
	}, nil)

	selection, err := service.RecommendAccounts(context.Background())
	require.NoError(t, err)

	assert.Empty(t, selection.Ordered)
	assert.Nil(t, selection.Selected)
	assert.Equal(t, RecommendationUnavailableReset, selection.UnavailableReason)
	assert.Equal(t, "recommendation: no account available now (waiting for reset)", selection.UnavailableMessage)
}

func TestRecommendAccountsFromStatusesUsesProvidedSubsetForUnavailableMessage(t *testing.T) {
	now := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)

	selection := RecommendAccountsFromStatuses([]Status{
		{
			Account: domain.Account{ID: "reset-blocked"},
			DailyLimit: &StatusLimit{
				Window:     LimitWindowDaily,
				Percent:    100,
				ResetsAt:   now.Add(30 * time.Minute),
				CapturedAt: now,
			},
			Subscription: &StatusSubscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
		},
	}, now)

	assert.Empty(t, selection.Ordered)
	assert.Equal(t, RecommendationUnavailableReset, selection.UnavailableReason)
	assert.Equal(t, "recommendation: no account available now (waiting for reset)", selection.UnavailableMessage)
}

func TestServiceRecommendAccountsReturnsListError(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	listErr := errors.New("list failed")
	repo.EXPECT().List(mockAnyContext()).Return(nil, listErr)

	_, err := service.RecommendAccounts(context.Background())
	require.ErrorIs(t, err, listErr)
}

func TestOrderStatusesPreservesExistingMainListBehavior(t *testing.T) {
	now := time.Date(2026, 2, 14, 11, 0, 0, 0, time.UTC)

	ordered := OrderStatuses([]Status{
		{
			Account: domain.Account{ID: "acc-blocked", Name: "blocked@example.com"},
			DailyLimit: &StatusLimit{
				Window:     LimitWindowDaily,
				Percent:    0,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &StatusLimit{
				Window:     LimitWindowWeekly,
				Percent:    100,
				ResetsAt:   now.Add(2 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{ID: "acc-mid", Name: "mid@example.com"},
			DailyLimit: &StatusLimit{
				Window:     LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &StatusLimit{
				Window:     LimitWindowWeekly,
				Percent:    70,
				ResetsAt:   now.Add(2 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
		{
			Account: domain.Account{ID: "acc-best", Name: "best@example.com"},
			DailyLimit: &StatusLimit{
				Window:     LimitWindowDaily,
				Percent:    20,
				ResetsAt:   now.Add(5 * time.Hour),
				CapturedAt: now,
			},
			WeeklyLimit: &StatusLimit{
				Window:     LimitWindowWeekly,
				Percent:    0,
				ResetsAt:   now.Add(5 * 24 * time.Hour),
				CapturedAt: now,
			},
		},
	}, now)

	assert.Equal(t, []domain.AccountID{"acc-best", "acc-mid", "acc-blocked"}, []domain.AccountID{
		ordered[0].Account.ID,
		ordered[1].Account.ID,
		ordered[2].Account.ID,
	})
}

func selectionAccount(id domain.AccountID, activeUntil time.Time, dailyPercent, weeklyPercent float64) domain.Account {
	return domain.Account{
		ID: id,
		Subscription: &domain.Subscription{
			ActiveUntil: activeUntil,
		},
		Limits: domain.AccountLimitSnapshots{
			Daily:  &domain.AccountLimitSnapshot{Percent: dailyPercent, ResetsAt: activeUntil},
			Weekly: &domain.AccountLimitSnapshot{Percent: weeklyPercent, ResetsAt: activeUntil},
		},
	}
}

func selectionStatus(id domain.AccountID, activeUntil time.Time, dailyPercent, weeklyPercent float64) Status {
	account := selectionAccount(id, activeUntil, dailyPercent, weeklyPercent)

	return Status{
		Account: account,
		DailyLimit: &StatusLimit{
			Window:   LimitWindowDaily,
			Percent:  dailyPercent,
			ResetsAt: activeUntil,
		},
		WeeklyLimit: &StatusLimit{
			Window:   LimitWindowWeekly,
			Percent:  weeklyPercent,
			ResetsAt: activeUntil,
		},
		Subscription: &StatusSubscription{
			ActiveUntil: activeUntil,
			WillRenew:   account.Subscription.WillRenew,
		},
	}
}
