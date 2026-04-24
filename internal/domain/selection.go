package domain

import (
	"sort"
	"time"
)

type SelectionPool string

const (
	SelectionPoolFallback SelectionPool = "fallback"
)

const selectionPressureRelativeTolerance = 0.05

type SelectionCandidate struct {
	AccountID       AccountID
	Eligible        bool
	WeeklyRemaining float64
	DailyRemaining  float64
	RenewalAt       *time.Time
	WeeklyResetsAt  *time.Time
	DailyResetsAt   *time.Time
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
		WeeklyResetsAt:  selectionLimitResetsAt(account.Limits.Weekly, now),
		DailyResetsAt:   selectionLimitResetsAt(account.Limits.Daily, now),
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

func RankSelectionCandidates(candidates []SelectionCandidate, now time.Time) SelectionRanking {
	eligible := filterEligibleSelectionCandidates(candidates)
	pool := SelectionPoolFallback
	ranked := eligible

	sort.Slice(ranked, func(i, j int) bool {
		return compareSelectionCandidates(ranked[i], ranked[j], now) < 0
	})

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

func compareSelectionCandidates(left, right SelectionCandidate, now time.Time) int {
	if cmp := compareSelectionPressure(SelectionSubscriptionWeeklyPressure(left, now), SelectionSubscriptionWeeklyPressure(right, now)); cmp != 0 {
		return cmp
	}
	if cmp := compareSelectionPressure(SelectionWeeklyResetPressure(left, now), SelectionWeeklyResetPressure(right, now)); cmp != 0 {
		return cmp
	}
	if cmp := compareSelectionPressure(SelectionDailyResetPressure(left, now), SelectionDailyResetPressure(right, now)); cmp != 0 {
		return cmp
	}
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

func compareSelectionPressure(left, right float64) int {
	if selectionPressureWithinRelativeTolerance(left, right) {
		return 0
	}
	if left > right {
		return -1
	}
	return 1
}

func selectionPressureWithinRelativeTolerance(left, right float64) bool {
	diff := left - right
	if diff < 0 {
		diff = -diff
	}
	if diff == 0 {
		return true
	}

	largest := left
	if right > largest {
		largest = right
	}
	if largest <= 0 {
		return true
	}

	return diff/largest <= selectionPressureRelativeTolerance
}

// SelectionSubscriptionWeeklyPressure estimates how much weekly capacity must be used per hour
// to avoid wasting it before the subscription period ends.
func SelectionSubscriptionWeeklyPressure(candidate SelectionCandidate, now time.Time) float64 {
	return candidate.WeeklyRemaining / selectionHoursUntil(candidate.RenewalAt, now, 30*24)
}

// SelectionWeeklyResetPressure estimates how much weekly capacity must be used per hour
// to avoid wasting it before the current weekly window resets.
func SelectionWeeklyResetPressure(candidate SelectionCandidate, now time.Time) float64 {
	return candidate.WeeklyRemaining / selectionHoursUntil(candidate.WeeklyResetsAt, now, 7*24)
}

// SelectionDailyResetPressure estimates how much 5-hour capacity must be used per hour
// to avoid wasting it before the short window resets.
func SelectionDailyResetPressure(candidate SelectionCandidate, now time.Time) float64 {
	return candidate.DailyRemaining / selectionHoursUntil(candidate.DailyResetsAt, now, 5)
}

func selectionHoursUntil(deadline *time.Time, now time.Time, fallbackHours float64) float64 {
	if deadline == nil || deadline.IsZero() || now.IsZero() {
		return fallbackHours
	}

	remaining := deadline.Sub(now).Hours()
	if remaining < 1 {
		return 1
	}

	return remaining
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

func selectionLimitResetsAt(limit *AccountLimitSnapshot, now time.Time) *time.Time {
	if limit == nil || limit.ResetsAt.IsZero() || !limit.ResetsAt.After(now) {
		return nil
	}

	resetsAt := limit.ResetsAt
	return &resetsAt
}
