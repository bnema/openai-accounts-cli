package domain

import (
	"sort"
	"time"
)

type SelectionPool string

const (
	SelectionPoolFallback      SelectionPool = "fallback"
	SelectionPoolUrgentRenewal SelectionPool = "urgent_renewal"
)

type SelectionCandidate struct {
	AccountID       AccountID
	Eligible        bool
	WeeklyRemaining float64
	DailyRemaining  float64
	RenewalAt       *time.Time
}

type SelectionPolicy struct {
	UrgentRenewalWindow          time.Duration
	UrgentRenewalWeeklyThreshold float64
}

type RankedSelectionCandidate struct {
	Candidate SelectionCandidate
	Pool      SelectionPool
	Rank      int
}

type SelectionRanking struct {
	Pool       SelectionPool
	Candidates []RankedSelectionCandidate
}

func SelectionCandidateFromAccount(account Account, now time.Time) SelectionCandidate {
	return SelectionCandidate{
		AccountID:       account.ID,
		Eligible:        AccountUsableForSelection(account, now),
		WeeklyRemaining: selectionRemainingPercent(account.Limits.Weekly, now),
		DailyRemaining:  selectionRemainingPercent(account.Limits.Daily, now),
		RenewalAt:       selectionRenewalAt(account.Subscription),
	}
}

func AccountUsableForSelection(account Account, now time.Time) bool {
	if !hasActiveSubscription(account.Subscription, now) {
		return false
	}

	if limitExhaustedUntilReset(account.Limits.Daily, now) {
		return false
	}
	if limitExhaustedUntilReset(account.Limits.Weekly, now) {
		return false
	}

	return true
}

func RankSelectionCandidates(candidates []SelectionCandidate, policy SelectionPolicy, now time.Time) SelectionRanking {
	eligible := filterEligibleSelectionCandidates(candidates)
	pool := SelectionPoolFallback
	ranked := eligible

	urgent := filterUrgentRenewalSelectionCandidates(eligible, policy, now)
	if len(urgent) > 0 {
		pool = SelectionPoolUrgentRenewal
		ranked = urgent
		sort.Slice(ranked, func(i, j int) bool {
			return compareUrgentSelectionCandidates(ranked[i], ranked[j]) < 0
		})
	} else {
		sort.Slice(ranked, func(i, j int) bool {
			return compareFallbackSelectionCandidates(ranked[i], ranked[j]) < 0
		})
	}

	result := SelectionRanking{
		Pool:       pool,
		Candidates: make([]RankedSelectionCandidate, 0, len(ranked)),
	}

	for i, candidate := range ranked {
		result.Candidates = append(result.Candidates, RankedSelectionCandidate{
			Candidate: candidate,
			Pool:      pool,
			Rank:      i + 1,
		})
	}

	return result
}

func filterEligibleSelectionCandidates(candidates []SelectionCandidate) []SelectionCandidate {
	eligible := make([]SelectionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Eligible {
			continue
		}
		eligible = append(eligible, candidate)
	}

	return eligible
}

func filterUrgentRenewalSelectionCandidates(candidates []SelectionCandidate, policy SelectionPolicy, now time.Time) []SelectionCandidate {
	urgent := make([]SelectionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.WeeklyRemaining <= policy.UrgentRenewalWeeklyThreshold {
			continue
		}
		if !selectionRenewsWithin(candidate.RenewalAt, now, policy.UrgentRenewalWindow) {
			continue
		}
		urgent = append(urgent, candidate)
	}

	return urgent
}

func selectionRenewsWithin(renewalAt *time.Time, now time.Time, window time.Duration) bool {
	if renewalAt == nil || renewalAt.IsZero() || renewalAt.Before(now) {
		return false
	}

	if window <= 0 {
		return renewalAt.Equal(now)
	}

	return !renewalAt.After(now.Add(window))
}

func compareUrgentSelectionCandidates(left, right SelectionCandidate) int {
	if cmp := compareSelectionRenewal(left.RenewalAt, right.RenewalAt); cmp != 0 {
		return cmp
	}
	if cmp := compareSelectionRemaining(left.WeeklyRemaining, right.WeeklyRemaining); cmp != 0 {
		return cmp
	}
	if cmp := compareSelectionRemaining(left.DailyRemaining, right.DailyRemaining); cmp != 0 {
		return cmp
	}

	return compareSelectionAccountID(left.AccountID, right.AccountID)
}

func compareFallbackSelectionCandidates(left, right SelectionCandidate) int {
	if cmp := compareSelectionRemaining(left.WeeklyRemaining, right.WeeklyRemaining); cmp != 0 {
		return cmp
	}
	if cmp := compareSelectionRemaining(left.DailyRemaining, right.DailyRemaining); cmp != 0 {
		return cmp
	}
	if cmp := compareSelectionRenewal(left.RenewalAt, right.RenewalAt); cmp != 0 {
		return cmp
	}

	return compareSelectionAccountID(left.AccountID, right.AccountID)
}

func compareSelectionRemaining(left, right float64) int {
	if left > right {
		return -1
	}
	if left < right {
		return 1
	}

	return 0
}

func compareSelectionRenewal(left, right *time.Time) int {
	leftKnown := left != nil && !left.IsZero()
	rightKnown := right != nil && !right.IsZero()

	if leftKnown != rightKnown {
		if leftKnown {
			return -1
		}
		return 1
	}
	if !leftKnown {
		return 0
	}
	if left.Before(*right) {
		return -1
	}
	if left.After(*right) {
		return 1
	}

	return 0
}

func compareSelectionAccountID(left, right AccountID) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}

	return 0
}

func selectionRemainingPercent(limit *AccountLimitSnapshot, now time.Time) float64 {
	if limit == nil {
		return 100
	}
	if !limit.ResetsAt.IsZero() && !limit.ResetsAt.After(now) {
		return 100
	}

	remaining := 100 - limit.Percent
	if remaining < 0 {
		return 0
	}
	if remaining > 100 {
		return 100
	}

	return remaining
}

func selectionRenewalAt(sub *Subscription) *time.Time {
	if sub == nil || sub.ActiveUntil.IsZero() {
		return nil
	}

	renewalAt := sub.ActiveUntil
	return &renewalAt
}
