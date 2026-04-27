package application

import (
	"context"
	"fmt"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

const selectionFairnessWindow = 30 * 24 * time.Hour

type RecommendedAccount struct {
	Status             Status
	Pool               domain.SelectionPool
	Rank               int
	selectionCandidate domain.SelectionCandidate
}

type RecommendationUnavailableReason string

const (
	RecommendationUnavailableNone         RecommendationUnavailableReason = ""
	RecommendationUnavailableReset        RecommendationUnavailableReason = "reset"
	RecommendationUnavailableSubscription RecommendationUnavailableReason = "subscription"
)

type RecommendationResult struct {
	Pool               domain.SelectionPool
	Ordered            []RecommendedAccount
	Selected           *RecommendedAccount
	UnavailableReason  RecommendationUnavailableReason
	UnavailableMessage string
}

func (s *Service) RecommendAccounts(ctx context.Context) (RecommendationResult, error) {
	if err := ctx.Err(); err != nil {
		return RecommendationResult{}, err
	}

	statuses, err := s.GetStatusAll(ctx)
	if err != nil {
		return RecommendationResult{}, fmt.Errorf("get statuses: %w", err)
	}

	now := s.clock.Now()

	return RecommendAccountsFromStatuses(statuses, now), nil
}

func (s *Service) RebalanceStatusesEvenly(ctx context.Context, statuses []Status) ([]Status, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(statuses) < 2 {
		return statuses, nil
	}

	now := s.clock.Now()
	recommendation := RecommendAccountsFromStatuses(statuses, now)
	if len(recommendation.Ordered) < 2 {
		return reorderRecommendedStatuses(recommendation.Ordered, ""), nil
	}

	topBand := recommendationTopBand(recommendation, now)
	if len(topBand) < 2 {
		return reorderRecommendedStatuses(recommendation.Ordered, ""), nil
	}

	if s.selectionHistory == nil {
		return reorderRecommendedStatuses(recommendation.Ordered, ""), nil
	}

	recent, err := s.selectionHistory.RecentSelections(ctx, now.Add(-selectionFairnessWindow))
	if err != nil {
		return reorderRecommendedStatuses(recommendation.Ordered, ""), nil
	}

	selectedID := leastUsedRecommendedAccountID(topBand, recent)
	if selectedID == "" || selectedID == recommendation.Ordered[0].Status.Account.ID {
		return reorderRecommendedStatuses(recommendation.Ordered, ""), nil
	}

	return reorderRecommendedStatuses(recommendation.Ordered, selectedID), nil
}

func RecommendAccountsFromStatuses(statuses []Status, now time.Time) RecommendationResult {
	ranking := domain.RankSelectionCandidates(selectionCandidatesFromStatuses(statuses, now), now)

	statusesByID := make(map[domain.AccountID]Status, len(statuses))
	for _, status := range statuses {
		statusesByID[status.Account.ID] = status
	}

	result := RecommendationResult{
		Pool:    ranking.Pool,
		Ordered: make([]RecommendedAccount, 0, len(ranking.Candidates)),
	}

	for _, ranked := range ranking.Candidates {
		result.Ordered = append(result.Ordered, RecommendedAccount{
			Status:             statusesByID[ranked.Candidate.AccountID],
			Pool:               ranked.Pool,
			Rank:               ranked.Rank,
			selectionCandidate: ranked.Candidate,
		})
	}

	if len(result.Ordered) > 0 {
		selected := result.Ordered[0]
		result.Selected = &selected
	} else {
		result.UnavailableReason = recommendationUnavailableReason(statuses, now)
		result.UnavailableMessage = recommendationUnavailableMessage(result.UnavailableReason)
	}

	return result
}

func recommendationUnavailableMessage(reason RecommendationUnavailableReason) string {
	if reason == RecommendationUnavailableSubscription {
		return "recommendation: no eligible account now (subscription rules)"
	}

	return "recommendation: no account available now (waiting for reset)"
}

func recommendationUnavailableReason(statuses []Status, now time.Time) RecommendationUnavailableReason {
	for _, status := range statuses {
		if !statusAvailableByLimits(status, now) {
			continue
		}
		if !statusHasEligibleSubscription(status, now) {
			return RecommendationUnavailableSubscription
		}
	}

	return RecommendationUnavailableReset
}

func statusAvailableByLimits(status Status, now time.Time) bool {
	return !statusDailyLimitUnavailableUntilReset(status.DailyLimit, now) && !statusLimitExhaustedUntilReset(status.WeeklyLimit, now)
}

func statusDailyLimitUnavailableUntilReset(limit *StatusLimit, now time.Time) bool {
	if limit == nil {
		return false
	}

	if limit.ResetsAt.IsZero() {
		return limit.Percent >= 100
	}
	if !limit.ResetsAt.After(now) {
		return false
	}

	return statusLimitRemainingPercent(limit) <= domain.SelectionMinimumUsableDailyRemainingPercent
}

func statusHasEligibleSubscription(status Status, now time.Time) bool {
	sub := status.Subscription
	if sub == nil || sub.ActiveUntil.IsZero() {
		return false
	}

	if !sub.ActiveUntil.After(now) && !sub.WillRenew {
		return false
	}

	return true
}

func statusLimitRemainingPercent(limit *StatusLimit) float64 {
	if limit == nil {
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

func statusLimitExhaustedUntilReset(limit *StatusLimit, now time.Time) bool {
	if limit == nil {
		return false
	}

	if limit.ResetsAt.IsZero() || !limit.ResetsAt.After(now) {
		return false
	}

	return limit.Percent >= 100
}

func selectionCandidatesFromStatuses(statuses []Status, now time.Time) []domain.SelectionCandidate {
	candidates := make([]domain.SelectionCandidate, 0, len(statuses))
	for _, status := range statuses {
		candidates = append(candidates, domain.SelectionCandidateFromAccount(status.Account, now))
	}

	return candidates
}

func recommendationTopBand(recommendation RecommendationResult, now time.Time) []RecommendedAccount {
	if len(recommendation.Ordered) == 0 {
		return nil
	}

	best := recommendation.Ordered[0]
	topBand := []RecommendedAccount{best}
	for _, candidate := range recommendation.Ordered[1:] {
		if !recommendedAccountInTopBand(best, candidate, now) {
			break
		}
		topBand = append(topBand, candidate)
	}

	return topBand
}

func recommendedAccountInTopBand(best, candidate RecommendedAccount, now time.Time) bool {
	return domain.SelectionPressureWithinRelativeTolerance(domain.SelectionSubscriptionWeeklyPressure(best.selectionCandidate, now), domain.SelectionSubscriptionWeeklyPressure(candidate.selectionCandidate, now)) &&
		domain.SelectionPressureWithinRelativeTolerance(domain.SelectionWeeklyResetPressure(best.selectionCandidate, now), domain.SelectionWeeklyResetPressure(candidate.selectionCandidate, now)) &&
		domain.SelectionPressureWithinRelativeTolerance(domain.SelectionDailyResetPressure(best.selectionCandidate, now), domain.SelectionDailyResetPressure(candidate.selectionCandidate, now))
}

func leastUsedRecommendedAccountID(candidates []RecommendedAccount, recent []domain.AccountID) domain.AccountID {
	counts := make(map[domain.AccountID]int, len(candidates))
	allowed := make(map[domain.AccountID]struct{}, len(candidates))
	for _, candidate := range candidates {
		allowed[candidate.Status.Account.ID] = struct{}{}
	}

	for _, id := range recent {
		if _, ok := allowed[id]; !ok {
			continue
		}
		counts[id]++
	}

	selectedID := candidates[0].Status.Account.ID
	selectedCount := counts[selectedID]
	for _, candidate := range candidates[1:] {
		count := counts[candidate.Status.Account.ID]
		if count < selectedCount {
			selectedID = candidate.Status.Account.ID
			selectedCount = count
		}
	}

	return selectedID
}

func reorderRecommendedStatuses(ordered []RecommendedAccount, selectedID domain.AccountID) []Status {
	rebalanced := make([]Status, 0, len(ordered))
	if selectedID != "" {
		for _, candidate := range ordered {
			if candidate.Status.Account.ID != selectedID {
				continue
			}
			rebalanced = append(rebalanced, candidate.Status)
			break
		}
	}
	for _, candidate := range ordered {
		if candidate.Status.Account.ID == selectedID {
			continue
		}
		rebalanced = append(rebalanced, candidate.Status)
	}

	return rebalanced
}
