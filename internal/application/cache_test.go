package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatusUsageCacheFresh(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name: "fresh when newest limit capture is within ttl",
			status: Status{
				DailyLimit:  &StatusLimit{CapturedAt: now.Add(-4 * time.Minute)},
				WeeklyLimit: &StatusLimit{CapturedAt: now.Add(-10 * time.Minute)},
			},
			want: true,
		},
		{
			name: "stale when newest limit capture is older than ttl",
			status: Status{
				DailyLimit:  &StatusLimit{CapturedAt: now.Add(-6 * time.Minute)},
				WeeklyLimit: &StatusLimit{CapturedAt: now.Add(-10 * time.Minute)},
			},
			want: false,
		},
		{
			name:   "stale when no limit capture exists",
			status: Status{},
			want:   false,
		},
		{
			name: "fresh when only weekly limit exists and is within ttl",
			status: Status{
				WeeklyLimit: &StatusLimit{CapturedAt: now.Add(-3 * time.Minute)},
			},
			want: true,
		},
		{
			name: "stale when both limits have zero captured at",
			status: Status{
				DailyLimit:  &StatusLimit{CapturedAt: time.Time{}},
				WeeklyLimit: &StatusLimit{CapturedAt: time.Time{}},
			},
			want: false,
		},
		{
			name: "stale at exact ttl boundary",
			status: Status{
				DailyLimit: &StatusLimit{CapturedAt: now.Add(-5 * time.Minute)},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StatusUsageCacheFresh(tt.status, now))
		})
	}
}
