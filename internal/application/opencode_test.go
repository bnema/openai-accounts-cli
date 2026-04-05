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

func TestServiceDecideOpencodeRecoveryRefreshesCurrentAccountForAuthInvalid(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	decision, err := service.DecideOpencodeRecovery(context.Background(), "acc-1", errors.New("authentication failed: invalid api key"))
	require.NoError(t, err)
	assert.Equal(t, OpencodeRecoveryDecision{
		Class:     domain.OpencodeFailureAuthInvalid,
		Action:    OpencodeRecoveryActionRefreshCurrent,
		AccountID: "acc-1",
		Retry:     true,
	}, decision)
}

func TestServiceDecideOpencodeRecoveryFailsOverForRecoverableAccountFailures(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		err  error
	}{
		{name: "cooldown", err: errors.New("request failed: rate limit exceeded")},
		{name: "weekly limit", err: errors.New("weekly limit reached")},
		{name: "no subscription", err: errors.New("account has no active subscription")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockAccountRepository(t)
			store := mocks.NewMockSecretStore(t)
			clock := mocks.NewMockClock(t)
			clock.EXPECT().Now().Return(now)

			service := NewService(repo, store, clock)
			repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
				{
					ID:   "acc-1",
					Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-1/oauth"},
					Subscription: &domain.Subscription{
						ActiveUntil: now.Add(24 * time.Hour),
					},
					Limits: domain.AccountLimitSnapshots{
						Daily: &domain.AccountLimitSnapshot{Percent: 100, ResetsAt: now.Add(30 * time.Minute)},
					},
				},
				{
					ID:   "acc-2",
					Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-2/oauth"},
					Subscription: &domain.Subscription{
						ActiveUntil: now.Add(24 * time.Hour),
					},
					Limits: domain.AccountLimitSnapshots{
						Daily:  &domain.AccountLimitSnapshot{Percent: 10, ResetsAt: now.Add(30 * time.Minute)},
						Weekly: &domain.AccountLimitSnapshot{Percent: 20, ResetsAt: now.Add(48 * time.Hour)},
					},
				},
			}, nil)

			decision, err := service.DecideOpencodeRecovery(context.Background(), "acc-1", tt.err)
			require.NoError(t, err)
			assert.Equal(t, OpencodeRecoveryDecision{
				Class:     domain.ClassifyOpencodeFailure(tt.err),
				Action:    OpencodeRecoveryActionFailover,
				AccountID: "acc-2",
				Retry:     true,
			}, decision)
		})
	}
}

func TestServiceDecideOpencodeRecoveryChoosesBestEligibleStatusForFailover(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(now)

	service := NewService(repo, store, clock)
	repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
		{
			ID:   "acc-1",
			Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-1/oauth"},
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily: &domain.AccountLimitSnapshot{Percent: 100, ResetsAt: now.Add(30 * time.Minute)},
			},
		},
		{
			ID:   "acc-2",
			Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-2/oauth"},
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily:  &domain.AccountLimitSnapshot{Percent: 40, ResetsAt: now.Add(30 * time.Minute)},
				Weekly: &domain.AccountLimitSnapshot{Percent: 35, ResetsAt: now.Add(48 * time.Hour)},
			},
		},
		{
			ID:   "acc-3",
			Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-3/oauth"},
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily:  &domain.AccountLimitSnapshot{Percent: 15, ResetsAt: now.Add(30 * time.Minute)},
				Weekly: &domain.AccountLimitSnapshot{Percent: 25, ResetsAt: now.Add(48 * time.Hour)},
			},
		},
	}, nil)

	decision, err := service.DecideOpencodeRecovery(context.Background(), "acc-1", errors.New("request failed: rate limit exceeded"))
	require.NoError(t, err)
	assert.Equal(t, OpencodeRecoveryDecision{
		Class:     domain.OpencodeFailureCooldown,
		Action:    OpencodeRecoveryActionFailover,
		AccountID: "acc-3",
		Retry:     true,
	}, decision)
}

func TestServiceDecideOpencodeRecoverySkipsApiKeyAccountsForFailover(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(now)

	service := NewService(repo, store, clock)
	repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
		{
			ID:   "acc-1",
			Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-1/oauth"},
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily: &domain.AccountLimitSnapshot{Percent: 100, ResetsAt: now.Add(30 * time.Minute)},
			},
		},
		{
			ID:   "acc-2",
			Auth: domain.Auth{Method: domain.AuthMethodAPIKey, SecretRef: "openai://acc-2/api_key"},
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily:  &domain.AccountLimitSnapshot{Percent: 10, ResetsAt: now.Add(30 * time.Minute)},
				Weekly: &domain.AccountLimitSnapshot{Percent: 10, ResetsAt: now.Add(48 * time.Hour)},
			},
		},
	}, nil)

	decision, err := service.DecideOpencodeRecovery(context.Background(), "acc-1", errors.New("weekly limit reached"))
	require.NoError(t, err)
	assert.Equal(t, OpencodeRecoveryDecision{
		Class:  domain.OpencodeFailureWeeklyLimit,
		Action: OpencodeRecoveryActionFallback,
		Retry:  false,
	}, decision)
}

func TestServiceDecideOpencodeRecoveryFallsBackWithoutRetryForUnknownFailures(t *testing.T) {
	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	service := NewService(repo, store, clock)

	decision, err := service.DecideOpencodeRecovery(context.Background(), "acc-1", errors.New("unexpected upstream failure"))
	require.NoError(t, err)
	assert.Equal(t, OpencodeRecoveryDecision{
		Class:  domain.OpencodeFailureUnknown,
		Action: OpencodeRecoveryActionFallback,
		Retry:  false,
	}, decision)
}

func TestServiceSelectOpencodeSyncAccountChoosesBestEligibleStatus(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	repo := mocks.NewMockAccountRepository(t)
	store := mocks.NewMockSecretStore(t)
	clock := mocks.NewMockClock(t)
	clock.EXPECT().Now().Return(now)

	service := NewService(repo, store, clock)
	repo.EXPECT().List(mockAnyContext()).Return([]domain.Account{
		{
			ID:   "acc-1",
			Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-1/oauth"},
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily: &domain.AccountLimitSnapshot{Percent: 100, ResetsAt: now.Add(30 * time.Minute)},
			},
		},
		{
			ID:   "acc-2",
			Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-2/oauth"},
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily:  &domain.AccountLimitSnapshot{Percent: 40, ResetsAt: now.Add(30 * time.Minute)},
				Weekly: &domain.AccountLimitSnapshot{Percent: 35, ResetsAt: now.Add(48 * time.Hour)},
			},
		},
		{
			ID:   "acc-3",
			Auth: domain.Auth{Method: domain.AuthMethodChatGPT, SecretRef: "openai://acc-3/oauth"},
			Subscription: &domain.Subscription{
				ActiveUntil: now.Add(24 * time.Hour),
			},
			Limits: domain.AccountLimitSnapshots{
				Daily:  &domain.AccountLimitSnapshot{Percent: 15, ResetsAt: now.Add(30 * time.Minute)},
				Weekly: &domain.AccountLimitSnapshot{Percent: 25, ResetsAt: now.Add(48 * time.Hour)},
			},
		},
	}, nil)

	status, err := service.SelectOpencodeSyncAccount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, domain.AccountID("acc-3"), status.Account.ID)
}
