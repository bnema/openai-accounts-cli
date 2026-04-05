package domain

import (
	"strings"
	"time"
)

type OpencodeFailureClass string

const (
	OpencodeFailureCooldown       OpencodeFailureClass = "cooldown"
	OpencodeFailureWeeklyLimit    OpencodeFailureClass = "weekly_limit"
	OpencodeFailureNoSubscription OpencodeFailureClass = "no_subscription"
	OpencodeFailureAuthInvalid    OpencodeFailureClass = "auth_invalid"
	OpencodeFailureUnknown        OpencodeFailureClass = "unknown"
)

// ClassifyOpencodeFailure matches the current upstream/OpenCode error strings.
// The inputs are free-form errors, so the classifier intentionally uses
// case-insensitive substring checks for stable phrases such as "weekly limit",
// "no active subscription", "no subscription", "invalid api key",
// "authentication failed", "unauthorized", "rate limit", and "cooldown".
func ClassifyOpencodeFailure(err error) OpencodeFailureClass {
	if err == nil {
		return OpencodeFailureUnknown
	}

	message := strings.ToLower(err.Error())

	switch {
	case strings.Contains(message, "weekly limit"):
		return OpencodeFailureWeeklyLimit
	case strings.Contains(message, "no active subscription"), strings.Contains(message, "no subscription"):
		return OpencodeFailureNoSubscription
	case strings.Contains(message, "invalid api key"), strings.Contains(message, "invalid auth"), strings.Contains(message, "authentication failed"), strings.Contains(message, "unauthorized"):
		return OpencodeFailureAuthInvalid
	case strings.Contains(message, "rate limit"), strings.Contains(message, "cooldown"):
		return OpencodeFailureCooldown
	default:
		return OpencodeFailureUnknown
	}
}

func AccountEligibleForOpencodeFailover(account Account, now time.Time) bool {
	if !hasOpencodeAuth(account) {
		return false
	}

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

func hasOpencodeAuth(account Account) bool {
	return account.Auth.Method == AuthMethodChatGPT && account.Auth.SecretRef != ""
}

func hasActiveSubscription(sub *Subscription, now time.Time) bool {
	if sub == nil || sub.IsDelinquent {
		return false
	}

	// Zero ActiveUntil means the subscription has no known end date yet.
	if sub.ActiveUntil.IsZero() {
		return true
	}

	return sub.ActiveUntil.After(now)
}

func limitExhaustedUntilReset(snapshot *AccountLimitSnapshot, now time.Time) bool {
	if snapshot == nil {
		return false
	}

	if snapshot.Percent < 100 {
		return false
	}

	if snapshot.ResetsAt.IsZero() {
		return true
	}

	return snapshot.ResetsAt.After(now)
}
