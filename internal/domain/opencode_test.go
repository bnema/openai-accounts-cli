package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClassifyOpencodeFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want OpencodeFailureClass
	}{
		{name: "cooldown from rate limit", err: errors.New("request failed: rate limit exceeded"), want: OpencodeFailureCooldown},
		{name: "weekly limit", err: errors.New("weekly limit reached"), want: OpencodeFailureWeeklyLimit},
		{name: "no subscription", err: errors.New("account has no active subscription"), want: OpencodeFailureNoSubscription},
		{name: "auth invalid", err: errors.New("authentication failed: invalid api key"), want: OpencodeFailureAuthInvalid},
		{name: "unknown", err: errors.New("unexpected upstream failure"), want: OpencodeFailureUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ClassifyOpencodeFailure(tt.err))
		})
	}
}

func TestAccountEligibleForOpencodeFailover(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		account Account
		want    bool
	}{
		{
			name: "eligible account",
			account: Account{
				ID:   "acc-2",
				Auth: Auth{Method: AuthMethodChatGPT, SecretRef: "openai://acc-2/oauth"},
				Subscription: &Subscription{
					ActiveUntil: now.Add(24 * time.Hour),
				},
				Limits: AccountLimitSnapshots{
					Daily:  &AccountLimitSnapshot{Percent: 20, ResetsAt: now.Add(2 * time.Hour)},
					Weekly: &AccountLimitSnapshot{Percent: 40, ResetsAt: now.Add(48 * time.Hour)},
				},
			},
			want: true,
		},
		{
			name: "missing auth secret",
			account: Account{
				ID: "acc-2",
				Subscription: &Subscription{
					ActiveUntil: now.Add(24 * time.Hour),
				},
			},
			want: false,
		},
		{
			name: "api key auth is not eligible for oauth sync failover",
			account: Account{
				ID:   "acc-2",
				Auth: Auth{Method: AuthMethodAPIKey, SecretRef: "openai://acc-2/api_key"},
				Subscription: &Subscription{
					ActiveUntil: now.Add(24 * time.Hour),
				},
			},
			want: false,
		},
		{
			name: "metadata secret ref does not qualify",
			account: Account{
				ID:       "acc-2",
				Metadata: AccountMetadata{SecretRef: "openai://acc-2/oauth"},
				Subscription: &Subscription{
					ActiveUntil: now.Add(24 * time.Hour),
				},
			},
			want: false,
		},
		{
			name: "missing subscription",
			account: Account{
				ID:   "acc-2",
				Auth: Auth{Method: AuthMethodChatGPT, SecretRef: "openai://acc-2/oauth"},
			},
			want: false,
		},
		{
			name: "weekly limit exhausted until reset",
			account: Account{
				ID:   "acc-2",
				Auth: Auth{Method: AuthMethodChatGPT, SecretRef: "openai://acc-2/oauth"},
				Subscription: &Subscription{
					ActiveUntil: now.Add(24 * time.Hour),
				},
				Limits: AccountLimitSnapshots{
					Weekly: &AccountLimitSnapshot{Percent: 100, ResetsAt: now.Add(48 * time.Hour)},
				},
			},
			want: false,
		},
		{
			name: "daily cooldown already reset",
			account: Account{
				ID:   "acc-2",
				Auth: Auth{Method: AuthMethodChatGPT, SecretRef: "openai://acc-2/oauth"},
				Subscription: &Subscription{
					ActiveUntil: now.Add(24 * time.Hour),
				},
				Limits: AccountLimitSnapshots{
					Daily: &AccountLimitSnapshot{Percent: 100, ResetsAt: now.Add(-1 * time.Minute)},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AccountEligibleForOpencodeFailover(tt.account, now))
		})
	}
}
