package application

import (
	"math"
	"slices"
	"strings"
	"time"
)

type statusOrderPriority struct {
	availableNow      bool
	hasWeekly         bool
	weeklyPressure    float64
	weeklyLeftPercent float64
	dailyLeftPercent  float64
	weeklyResetHours  float64
	sortKey           string
}

// OrderStatuses returns a copy of statuses sorted for display.
func OrderStatuses(statuses []Status, now time.Time) []Status {
	ordered := append([]Status(nil), statuses...)

	slices.SortStableFunc(ordered, func(a, b Status) int {
		left := buildStatusOrderPriority(a, now)
		right := buildStatusOrderPriority(b, now)

		if cmp := compareBoolDesc(left.availableNow, right.availableNow); cmp != 0 {
			return cmp
		}
		if cmp := compareFloatDesc(left.weeklyPressure, right.weeklyPressure); cmp != 0 {
			return cmp
		}
		if cmp := compareBoolDesc(left.hasWeekly, right.hasWeekly); cmp != 0 {
			return cmp
		}
		if cmp := compareFloatDesc(left.weeklyLeftPercent, right.weeklyLeftPercent); cmp != 0 {
			return cmp
		}
		if cmp := compareFloatDesc(left.dailyLeftPercent, right.dailyLeftPercent); cmp != 0 {
			return cmp
		}
		if cmp := compareFloatAsc(left.weeklyResetHours, right.weeklyResetHours); cmp != 0 {
			return cmp
		}

		return strings.Compare(left.sortKey, right.sortKey)
	})

	return ordered
}

func buildStatusOrderPriority(status Status, now time.Time) statusOrderPriority {
	weeklyLeft := statusLimitLeftPercent(status.WeeklyLimit)
	dailyLeft := statusLimitLeftPercent(status.DailyLimit)
	hasWeekly := status.WeeklyLimit != nil
	weeklyHours := statusWeeklyResetHours(status.WeeklyLimit, now)
	weeklyPressure := 0.0

	if hasWeekly && weeklyLeft > 0 {
		weeklyPressure = weeklyLeft / math.Max(weeklyHours, 1)
	}

	return statusOrderPriority{
		availableNow:      statusCanUseNow(status, now),
		hasWeekly:         hasWeekly,
		weeklyPressure:    weeklyPressure,
		weeklyLeftPercent: weeklyLeft,
		dailyLeftPercent:  dailyLeft,
		weeklyResetHours:  weeklyHours,
		sortKey:           strings.ToLower(strings.TrimSpace(string(status.Account.ID) + "|" + status.Account.Name)),
	}
}

func statusCanUseNow(status Status, now time.Time) bool {
	if statusLimitBlocksNow(status.WeeklyLimit, now) {
		return false
	}

	if statusLimitBlocksNow(status.DailyLimit, now) {
		return false
	}

	return true
}

func statusLimitBlocksNow(limit *StatusLimit, now time.Time) bool {
	if limit == nil {
		return false
	}

	if statusLimitLeftPercent(limit) > 0 {
		return false
	}

	if now.IsZero() || limit.ResetsAt.IsZero() {
		return true
	}

	return limit.ResetsAt.After(now)
}

func statusLimitLeftPercent(limit *StatusLimit) float64 {
	if limit == nil {
		return 0
	}

	return clampPercent(100 - limit.Percent)
}

func statusWeeklyResetHours(limit *StatusLimit, now time.Time) float64 {
	const weeklyWindowHours = 7.0 * 24.0

	if limit == nil {
		return weeklyWindowHours
	}

	if now.IsZero() || limit.ResetsAt.IsZero() {
		return weeklyWindowHours
	}

	remaining := limit.ResetsAt.Sub(now)
	if remaining <= 0 {
		return 1
	}

	hours := remaining.Hours()
	if hours < 1 {
		return 1
	}

	return hours
}

func compareBoolDesc(left, right bool) int {
	if left == right {
		return 0
	}
	if left {
		return -1
	}
	return 1
}

func compareFloatDesc(left, right float64) int {
	if math.Abs(left-right) < 1e-9 {
		return 0
	}
	if left > right {
		return -1
	}
	return 1
}

func compareFloatAsc(left, right float64) int {
	if math.Abs(left-right) < 1e-9 {
		return 0
	}
	if left < right {
		return -1
	}
	return 1
}

func clampPercent(value float64) float64 {
	return math.Max(0, math.Min(100, value))
}
