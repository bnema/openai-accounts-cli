package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRankSelectionCandidatesPrioritizesSubscriptionDeadlinePressure(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	soonest := now.Add(24 * time.Hour)
	later := now.Add(72 * time.Hour)

	ranking := RankSelectionCandidates([]SelectionCandidate{
		{
			AccountID:       "fallback-best",
			Eligible:        true,
			WeeklyRemaining: 95,
			DailyRemaining:  95,
			RenewalAt:       nil,
		},
		{
			AccountID:       "urgent-later",
			Eligible:        true,
			WeeklyRemaining: 70,
			DailyRemaining:  40,
			RenewalAt:       &later,
		},
		{
			AccountID:       "urgent-sooner",
			Eligible:        true,
			WeeklyRemaining: 60,
			DailyRemaining:  10,
			RenewalAt:       &soonest,
		},
		{
			AccountID:       "urgent-low-weekly",
			Eligible:        true,
			WeeklyRemaining: 5,
			DailyRemaining:  99,
			RenewalAt:       &soonest,
		},
		{
			AccountID:       "ineligible",
			Eligible:        false,
			WeeklyRemaining: 80,
			DailyRemaining:  80,
			RenewalAt:       &soonest,
		},
	}, now)

	require.Equal(t, SelectionPoolFallback, ranking.Pool)
	require.Len(t, ranking.Candidates, 4)
	assert.Equal(t, []AccountID{
		"urgent-sooner",
		"urgent-later",
		"urgent-low-weekly",
		"fallback-best",
	}, []AccountID{
		ranking.Candidates[0].Candidate.AccountID,
		ranking.Candidates[1].Candidate.AccountID,
		ranking.Candidates[2].Candidate.AccountID,
		ranking.Candidates[3].Candidate.AccountID,
	})
	assert.Equal(t, []int{1, 2, 3, 4}, []int{
		ranking.Candidates[0].Rank,
		ranking.Candidates[1].Rank,
		ranking.Candidates[2].Rank,
		ranking.Candidates[3].Rank,
	})
}

func TestRankSelectionCandidatesUsesFallbackWeeklyDailyOrdering(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	earlier := now.Add(10 * 24 * time.Hour)
	later := now.Add(12 * 24 * time.Hour)

	ranking := RankSelectionCandidates([]SelectionCandidate{
		{
			AccountID:       "weekly-best-daily-best",
			Eligible:        true,
			WeeklyRemaining: 80,
			DailyRemaining:  70,
			RenewalAt:       &later,
		},
		{
			AccountID:       "weekly-best-daily-worse",
			Eligible:        true,
			WeeklyRemaining: 80,
			DailyRemaining:  20,
			RenewalAt:       &earlier,
		},
		{
			AccountID:       "weekly-next",
			Eligible:        true,
			WeeklyRemaining: 75,
			DailyRemaining:  99,
			RenewalAt:       &earlier,
		},
		{
			AccountID:       "weekly-best-daily-best-earlier",
			Eligible:        true,
			WeeklyRemaining: 80,
			DailyRemaining:  70,
			RenewalAt:       &earlier,
		},
	}, now)

	require.Equal(t, SelectionPoolFallback, ranking.Pool)
	require.Len(t, ranking.Candidates, 4)
	assert.Equal(t, []AccountID{
		"weekly-best-daily-best-earlier",
		"weekly-best-daily-worse",
		"weekly-next",
		"weekly-best-daily-best",
	}, []AccountID{
		ranking.Candidates[0].Candidate.AccountID,
		ranking.Candidates[1].Candidate.AccountID,
		ranking.Candidates[2].Candidate.AccountID,
		ranking.Candidates[3].Candidate.AccountID,
	})
}

func TestRankSelectionCandidatesPrioritizesSubscriptionWeeklyPressure(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	soonRenewal := now.Add(10 * 24 * time.Hour)
	laterRenewal := now.Add(25 * 24 * time.Hour)
	weeklyReset := now.Add(6 * 24 * time.Hour)
	dailyReset := now.Add(5 * time.Hour)

	ranking := RankSelectionCandidates([]SelectionCandidate{
		{
			AccountID:       "less-remaining-but-subscription-sooner",
			Eligible:        true,
			WeeklyRemaining: 60,
			DailyRemaining:  20,
			RenewalAt:       &soonRenewal,
			WeeklyResetsAt:  &weeklyReset,
			DailyResetsAt:   &dailyReset,
		},
		{
			AccountID:       "more-remaining-but-subscription-later",
			Eligible:        true,
			WeeklyRemaining: 90,
			DailyRemaining:  100,
			RenewalAt:       &laterRenewal,
			WeeklyResetsAt:  &weeklyReset,
			DailyResetsAt:   &dailyReset,
		},
	}, now)

	require.Equal(t, SelectionPoolFallback, ranking.Pool)
	require.Len(t, ranking.Candidates, 2)
	assert.Equal(t, AccountID("less-remaining-but-subscription-sooner"), ranking.Candidates[0].Candidate.AccountID)
	assert.Equal(t, AccountID("more-remaining-but-subscription-later"), ranking.Candidates[1].Candidate.AccountID)
}

func TestRankSelectionCandidatesPrioritizesWeeklyResetPressureBeforeDaily(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	renewal := now.Add(20 * 24 * time.Hour)
	soonWeeklyReset := now.Add(24 * time.Hour)
	laterWeeklyReset := now.Add(6 * 24 * time.Hour)
	lowDailyReset := now.Add(5 * time.Hour)
	highDailyReset := now.Add(1 * time.Hour)

	ranking := RankSelectionCandidates([]SelectionCandidate{
		{
			AccountID:       "weekly-reset-sooner-daily-worse",
			Eligible:        true,
			WeeklyRemaining: 70,
			DailyRemaining:  10,
			RenewalAt:       &renewal,
			WeeklyResetsAt:  &soonWeeklyReset,
			DailyResetsAt:   &lowDailyReset,
		},
		{
			AccountID:       "weekly-reset-later-daily-better",
			Eligible:        true,
			WeeklyRemaining: 70,
			DailyRemaining:  100,
			RenewalAt:       &renewal,
			WeeklyResetsAt:  &laterWeeklyReset,
			DailyResetsAt:   &highDailyReset,
		},
	}, now)

	require.Equal(t, SelectionPoolFallback, ranking.Pool)
	require.Len(t, ranking.Candidates, 2)
	assert.Equal(t, AccountID("weekly-reset-sooner-daily-worse"), ranking.Candidates[0].Candidate.AccountID)
	assert.Equal(t, AccountID("weekly-reset-later-daily-better"), ranking.Candidates[1].Candidate.AccountID)
}

func TestRankSelectionCandidatesUsesDailyPressureAfterComparableWeeklyRisk(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	renewal := now.Add(11 * 24 * time.Hour)
	weeklyReset := now.Add(7 * 24 * time.Hour)
	shortDailyReset := now.Add(3 * time.Hour)
	fullDailyReset := now.Add(5 * time.Hour)

	ranking := RankSelectionCandidates([]SelectionCandidate{
		{
			AccountID:       "weekly-slightly-better-daily-worse",
			Eligible:        true,
			WeeklyRemaining: 85,
			DailyRemaining:  51,
			RenewalAt:       &renewal,
			WeeklyResetsAt:  &weeklyReset,
			DailyResetsAt:   &shortDailyReset,
		},
		{
			AccountID:       "weekly-comparable-daily-pressure",
			Eligible:        true,
			WeeklyRemaining: 84,
			DailyRemaining:  100,
			RenewalAt:       &renewal,
			WeeklyResetsAt:  &weeklyReset,
			DailyResetsAt:   &fullDailyReset,
		},
	}, now)

	require.Equal(t, SelectionPoolFallback, ranking.Pool)
	require.Len(t, ranking.Candidates, 2)
	assert.Equal(t, AccountID("weekly-comparable-daily-pressure"), ranking.Candidates[0].Candidate.AccountID)
	assert.Equal(t, AccountID("weekly-slightly-better-daily-worse"), ranking.Candidates[1].Candidate.AccountID)
}

func TestSelectionCandidateFromAccountTreatsExhaustedDailyWithoutResetAsUnavailable(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	candidate := SelectionCandidateFromAccount(Account{
		ID: "exhausted-daily-no-reset",
		Subscription: &Subscription{
			ActiveUntil: now.Add(24 * time.Hour),
		},
		Limits: AccountLimitSnapshots{
			Daily: &AccountLimitSnapshot{
				Percent: 100,
			},
			Weekly: &AccountLimitSnapshot{
				Percent:  25,
				ResetsAt: now.Add(48 * time.Hour),
			},
		},
	}, now)

	assert.False(t, candidate.Eligible)
	assert.Equal(t, 0.0, candidate.DailyRemaining)
}

func TestSelectionCandidateFromAccountTreatsOnePercentDailyRemainingAsUnavailableUntilReset(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	candidate := SelectionCandidateFromAccount(Account{
		ID: "nearly-empty-daily",
		Subscription: &Subscription{
			ActiveUntil: now.Add(24 * time.Hour),
		},
		Limits: AccountLimitSnapshots{
			Daily: &AccountLimitSnapshot{
				Percent:  99,
				ResetsAt: now.Add(5 * time.Hour),
			},
			Weekly: &AccountLimitSnapshot{
				Percent:  25,
				ResetsAt: now.Add(48 * time.Hour),
			},
		},
	}, now)

	assert.False(t, candidate.Eligible)
	assert.Equal(t, 1.0, candidate.DailyRemaining)
}

func TestSelectionCandidateFromAccountTreatsPassedResetPartialUsageAsFullyAvailable(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	candidate := SelectionCandidateFromAccount(Account{
		ID: "reset-account",
		Subscription: &Subscription{
			ActiveUntil: now.Add(24 * time.Hour),
		},
		Limits: AccountLimitSnapshots{
			Daily: &AccountLimitSnapshot{
				Percent:  40,
				ResetsAt: now.Add(-1 * time.Minute),
			},
			Weekly: &AccountLimitSnapshot{
				Percent:  25,
				ResetsAt: now.Add(-1 * time.Hour),
			},
		},
	}, now)

	assert.Equal(t, 100.0, candidate.DailyRemaining)
	assert.Equal(t, 100.0, candidate.WeeklyRemaining)
	assert.Nil(t, candidate.DailyResetsAt)
	assert.Nil(t, candidate.WeeklyResetsAt)
}
