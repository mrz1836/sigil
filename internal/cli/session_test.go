package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		// Seconds only (less than 1 minute)
		{
			name:     "0 seconds",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "1 second",
			duration: time.Second,
			want:     "1s",
		},
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
			want:     "30s",
		},
		{
			name:     "59 seconds",
			duration: 59 * time.Second,
			want:     "59s",
		},

		// Minutes only (no remaining seconds)
		{
			name:     "1 minute exactly",
			duration: time.Minute,
			want:     "1m",
		},
		{
			name:     "5 minutes exactly",
			duration: 5 * time.Minute,
			want:     "5m",
		},
		{
			name:     "15 minutes exactly",
			duration: 15 * time.Minute,
			want:     "15m",
		},
		{
			name:     "60 minutes exactly",
			duration: 60 * time.Minute,
			want:     "60m",
		},

		// Minutes and seconds (mixed)
		{
			name:     "1 minute 1 second",
			duration: time.Minute + time.Second,
			want:     "1m1s",
		},
		{
			name:     "1 minute 30 seconds",
			duration: time.Minute + 30*time.Second,
			want:     "1m30s",
		},
		{
			name:     "2 minutes 45 seconds",
			duration: 2*time.Minute + 45*time.Second,
			want:     "2m45s",
		},
		{
			name:     "14 minutes 59 seconds",
			duration: 14*time.Minute + 59*time.Second,
			want:     "14m59s",
		},
		{
			name:     "59 minutes 59 seconds",
			duration: 59*time.Minute + 59*time.Second,
			want:     "59m59s",
		},

		// Edge cases with milliseconds (should be truncated)
		{
			name:     "30.5 seconds shows 30s",
			duration: 30*time.Second + 500*time.Millisecond,
			want:     "30s",
		},
		{
			name:     "1 minute 30.999 seconds shows 1m30s",
			duration: time.Minute + 30*time.Second + 999*time.Millisecond,
			want:     "1m30s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatDuration(tc.duration)
			assert.Equal(t, tc.want, got)
		})
	}
}
