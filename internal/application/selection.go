package application

import (
	"context"
	"fmt"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

const selectionFairnessWindow = 30 * 24 * time.Hour
const selectionFairnessPressureTolerance = 0.05

type RecommendedAccount struct {
	Status Status
	Pool   domain.SelectionPool
	Rank   int
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
			Status: statusesByID[ranked.Candidate.AccountID],
			Pool:   ranked.Pool,
			Rank:   ranked.Rank,
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
	return !statusLimitExhaustedUntilReset(status.DailyLimit, now) && !statusLimitExhaustedUntilReset(status.WeeklyLimit, now)
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
	bestCandidate := domain.SelectionCandidateFromAccount(best.Status.Account, now)
	candidateSelection := domain.SelectionCandidateFromAccount(candidate.Status.Account, now)

	return selectionPressureWithinTolerance(domain.SelectionSubscriptionWeeklyPressure(bestCandidate, now), domain.SelectionSubscriptionWeeklyPressure(candidateSelection, now)) &&
		selectionPressureWithinTolerance(domain.SelectionWeeklyResetPressure(bestCandidate, now), domain.SelectionWeeklyResetPressure(candidateSelection, now)) &&
		selectionPressureWithinTolerance(domain.SelectionDailyResetPressure(bestCandidate, now), domain.SelectionDailyResetPressure(candidateSelection, now))
}

func selectionPressureWithinTolerance(left, right float64) bool {
	diff := left - right
	if diff < 0 {
		diff = -diff
	}

	largest := left
	if right > largest {
		largest = right
	}
	if largest <= 0 {
		return true
	}

	return diff/largest <= selectionFairnessPressureTolerance
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
