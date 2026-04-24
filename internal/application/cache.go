package application

import "time"

const UsageCacheTTL = 5 * time.Minute

func StatusUsageCacheFresh(status Status, now time.Time) bool {
	capturedAt := StatusUsageCacheCapturedAt(status)
	if capturedAt.IsZero() {
		return false
	}

	return now.Sub(capturedAt) < UsageCacheTTL
}

func StatusUsageCacheCapturedAt(status Status) time.Time {
	var capturedAt time.Time
	if status.DailyLimit != nil && !status.DailyLimit.CapturedAt.IsZero() {
		capturedAt = status.DailyLimit.CapturedAt
	}
	if status.WeeklyLimit != nil && !status.WeeklyLimit.CapturedAt.IsZero() {
		if capturedAt.IsZero() || status.WeeklyLimit.CapturedAt.After(capturedAt) {
			capturedAt = status.WeeklyLimit.CapturedAt
		}
	}

	return capturedAt
}
