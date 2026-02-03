package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/session"
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

func TestOutputSessionStatusJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sessions     []*session.Session
		wantContains []string
	}{
		{
			name:     "no sessions",
			sessions: []*session.Session{},
			wantContains: []string{
				`"available": true`,
				`"sessions": []`,
			},
		},
		{
			name: "single session",
			sessions: []*session.Session{
				{
					WalletName: "test-wallet",
					CreatedAt:  time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC),
					ExpiresAt:  time.Now().Add(15 * time.Minute),
				},
			},
			wantContains: []string{
				`"available": true`,
				`"sessions": [`,
				`"wallet": "test-wallet"`,
				`"expires_in":`,
				`"created_at": "2026-01-31T12:00:00Z"`,
			},
		},
		{
			name: "multiple sessions",
			sessions: []*session.Session{
				{
					WalletName: "wallet-1",
					CreatedAt:  time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC),
					ExpiresAt:  time.Now().Add(10 * time.Minute),
				},
				{
					WalletName: "wallet-2",
					CreatedAt:  time.Date(2026, 1, 31, 13, 0, 0, 0, time.UTC),
					ExpiresAt:  time.Now().Add(20 * time.Minute),
				},
			},
			wantContains: []string{
				`"available": true`,
				`"wallet": "wallet-1"`,
				`"wallet": "wallet-2"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			outputSessionStatusJSON(cmd, tc.sessions)

			output := buf.String()
			for _, s := range tc.wantContains {
				assert.Contains(t, output, s)
			}
		})
	}
}

func TestOutputSessionStatusText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sessions     []*session.Session
		wantContains []string
	}{
		{
			name:     "no sessions",
			sessions: []*session.Session{},
			wantContains: []string{
				"No active sessions",
			},
		},
		{
			name: "single session",
			sessions: []*session.Session{
				{
					WalletName: "my-wallet",
					CreatedAt:  time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC),
					ExpiresAt:  time.Now().Add(5 * time.Minute),
				},
			},
			wantContains: []string{
				"Active Sessions:",
				"my-wallet:",
				"expires in",
			},
		},
		{
			name: "multiple sessions",
			sessions: []*session.Session{
				{
					WalletName: "wallet-a",
					CreatedAt:  time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC),
					ExpiresAt:  time.Now().Add(3 * time.Minute),
				},
				{
					WalletName: "wallet-b",
					CreatedAt:  time.Date(2026, 1, 31, 13, 0, 0, 0, time.UTC),
					ExpiresAt:  time.Now().Add(7 * time.Minute),
				},
			},
			wantContains: []string{
				"Active Sessions:",
				"wallet-a:",
				"wallet-b:",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			outputSessionStatusText(cmd, tc.sessions)

			output := buf.String()
			for _, s := range tc.wantContains {
				assert.Contains(t, output, s)
			}
		})
	}
}
