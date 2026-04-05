package application

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/domain"
)

type OpencodeRecoveryAction string

const (
	OpencodeRecoveryActionRefreshCurrent OpencodeRecoveryAction = "refresh_current"
	OpencodeRecoveryActionFailover       OpencodeRecoveryAction = "failover"
	OpencodeRecoveryActionFallback       OpencodeRecoveryAction = "fallback"
)

type OpencodeRecoveryDecision struct {
	Class     domain.OpencodeFailureClass
	Action    OpencodeRecoveryAction
	AccountID domain.AccountID
	Retry     bool
}

func (s *Service) DecideOpencodeRecovery(ctx context.Context, currentID domain.AccountID, cause error) (OpencodeRecoveryDecision, error) {
	if err := ctx.Err(); err != nil {
		return OpencodeRecoveryDecision{}, err
	}

	class := domain.ClassifyOpencodeFailure(cause)

	switch class {
	case domain.OpencodeFailureAuthInvalid:
		return OpencodeRecoveryDecision{
			Class:     class,
			Action:    OpencodeRecoveryActionRefreshCurrent,
			AccountID: currentID,
			Retry:     true,
		}, nil
	case domain.OpencodeFailureCooldown, domain.OpencodeFailureWeeklyLimit, domain.OpencodeFailureNoSubscription:
		statuses, err := s.GetStatusAll(ctx)
		if err != nil {
			return OpencodeRecoveryDecision{}, fmt.Errorf("get statuses: %w", err)
		}

		now := s.clock.Now()
		if account := bestOpencodeFailoverAccount(statuses, currentID, class, now); account != nil {
			return OpencodeRecoveryDecision{
				Class:     class,
				Action:    OpencodeRecoveryActionFailover,
				AccountID: account.ID,
				Retry:     true,
			}, nil
		}
	}

	return OpencodeRecoveryDecision{
		Class:  class,
		Action: OpencodeRecoveryActionFallback,
		Retry:  false,
	}, nil
}

func bestOpencodeFailoverAccount(statuses []Status, currentID domain.AccountID, class domain.OpencodeFailureClass, now time.Time) *domain.Account {
	var best *Status

	for i := range statuses {
		status := &statuses[i]
		if status.Account.ID == currentID {
			continue
		}
		if !domain.AccountEligibleForOpencodeFailover(status.Account, now) {
			continue
		}
		if best == nil || compareOpencodeFailoverStatus(status, best, class) < 0 {
			best = status
		}
	}

	if best == nil {
		return nil
	}

	return &best.Account
}

func compareOpencodeFailoverStatus(left, right *Status, class domain.OpencodeFailureClass) int {
	leftPrimary, leftSecondary := opencodeFailoverRank(left, class)
	rightPrimary, rightSecondary := opencodeFailoverRank(right, class)

	if leftPrimary != rightPrimary {
		if leftPrimary < rightPrimary {
			return -1
		}
		return 1
	}

	if leftSecondary != rightSecondary {
		if leftSecondary < rightSecondary {
			return -1
		}
		return 1
	}

	if left.Account.ID < right.Account.ID {
		return -1
	}
	if left.Account.ID > right.Account.ID {
		return 1
	}

	return 0
}

func opencodeFailoverRank(status *Status, class domain.OpencodeFailureClass) (float64, float64) {
	daily := opencodeLimitPercent(status.DailyLimit)
	weekly := opencodeLimitPercent(status.WeeklyLimit)

	switch class {
	case domain.OpencodeFailureWeeklyLimit:
		return weekly, daily
	case domain.OpencodeFailureCooldown:
		return daily, weekly
	default:
		return daily, weekly
	}
}

func opencodeLimitPercent(limit *StatusLimit) float64 {
	if limit == nil {
		return math.MaxFloat64
	}

	return limit.Percent
}
