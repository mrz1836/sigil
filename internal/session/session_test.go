package session

import (
	"testing"
	"time"
)

func TestSession_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "valid session - future expiry",
			expiresAt: time.Now().Add(10 * time.Minute),
			want:      true,
		},
		{
			name:      "expired session - past expiry",
			expiresAt: time.Now().Add(-10 * time.Minute),
			want:      false,
		},
		{
			name:      "expired session - just expired",
			expiresAt: time.Now().Add(-1 * time.Millisecond),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				WalletName: "test",
				CreatedAt:  time.Now().Add(-5 * time.Minute),
				ExpiresAt:  tt.expiresAt,
			}
			if got := s.IsValid(); got != tt.want {
				t.Errorf("Session.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

//nolint:gocognit // Test function with multiple test cases
func TestSession_TTL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		expiresAt time.Time
		wantZero  bool
		wantMin   time.Duration
		wantMax   time.Duration
	}{
		{
			name:      "valid session - returns remaining time",
			expiresAt: time.Now().Add(10 * time.Minute),
			wantZero:  false,
			wantMin:   9 * time.Minute,
			wantMax:   11 * time.Minute,
		},
		{
			name:      "expired session - returns zero",
			expiresAt: time.Now().Add(-10 * time.Minute),
			wantZero:  true,
		},
		{
			name:      "about to expire - small TTL",
			expiresAt: time.Now().Add(1 * time.Second),
			wantZero:  false,
			wantMin:   0,
			wantMax:   2 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				WalletName: "test",
				CreatedAt:  time.Now().Add(-5 * time.Minute),
				ExpiresAt:  tt.expiresAt,
			}
			got := s.TTL()

			if tt.wantZero {
				if got != 0 {
					t.Errorf("Session.TTL() = %v, want 0", got)
				}
			} else {
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("Session.TTL() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

func TestSession_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	expiry := now.Add(15 * time.Minute)

	s := &Session{
		WalletName: "main",
		CreatedAt:  now,
		ExpiresAt:  expiry,
	}

	if s.WalletName != "main" {
		t.Errorf("WalletName = %v, want main", s.WalletName)
	}
	if !s.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", s.CreatedAt, now)
	}
	if !s.ExpiresAt.Equal(expiry) {
		t.Errorf("ExpiresAt = %v, want %v", s.ExpiresAt, expiry)
	}
}
