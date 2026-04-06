package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRankSelectionCandidatesUsesUrgentRenewalPool(t *testing.T) {
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
	}, SelectionPolicy{
		UrgentRenewalWindow:          7 * 24 * time.Hour,
		UrgentRenewalWeeklyThreshold: 10,
	}, now)

	require.Equal(t, SelectionPoolUrgentRenewal, ranking.Pool)
	require.Len(t, ranking.Candidates, 2)
	assert.Equal(t, AccountID("urgent-sooner"), ranking.Candidates[0].Candidate.AccountID)
	assert.Equal(t, AccountID("urgent-later"), ranking.Candidates[1].Candidate.AccountID)
	assert.Equal(t, []int{1, 2}, []int{ranking.Candidates[0].Rank, ranking.Candidates[1].Rank})
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
	}, SelectionPolicy{
		UrgentRenewalWindow:          7 * 24 * time.Hour,
		UrgentRenewalWeeklyThreshold: 10,
	}, now)

	require.Equal(t, SelectionPoolFallback, ranking.Pool)
	require.Len(t, ranking.Candidates, 4)
	assert.Equal(t, []AccountID{
		"weekly-best-daily-best-earlier",
		"weekly-best-daily-best",
		"weekly-best-daily-worse",
		"weekly-next",
	}, []AccountID{
		ranking.Candidates[0].Candidate.AccountID,
		ranking.Candidates[1].Candidate.AccountID,
		ranking.Candidates[2].Candidate.AccountID,
		ranking.Candidates[3].Candidate.AccountID,
	})
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
}
