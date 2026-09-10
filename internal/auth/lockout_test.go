package auth_test

import (
	"testing"
	"time"

	"osto-cli-auth/internal/auth"
)

func TestIsLocked(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name        string
		lockedUntil *time.Time
		wantLocked  bool
	}{
		{"nil lockedUntil", nil, false},
		{"past lockout", ptr(now.Add(-5 * time.Minute)), false},
		{"future lockout", ptr(now.Add(10 * time.Minute)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locked, rem := auth.IsLocked(tt.lockedUntil, now)
			if locked != tt.wantLocked {
				t.Fatalf("IsLocked() = %v, want %v", locked, tt.wantLocked)
			}
			if tt.wantLocked && rem <= 0 {
				t.Errorf("expected positive remaining duration, got %v", rem)
			}
		})
	}
}

func TestCalculateFailure(t *testing.T) {
	now := time.Now().UTC()
	threshold := 5
	duration := 15 * time.Minute

	// Attempts 1 to 4 should increment without locking
	for attempt := 0; attempt < threshold-1; attempt++ {
		newAttempts, lockedUntil, justLocked := auth.CalculateFailure(attempt, threshold, duration, now)
		if justLocked {
			t.Fatalf("attempt %d triggered lockout unexpectedly", attempt+1)
		}
		if newAttempts != attempt+1 {
			t.Fatalf("attempt %d: expected newAttempts = %d, got %d", attempt, attempt+1, newAttempts)
		}
		if lockedUntil != nil {
			t.Fatalf("attempt %d: lockedUntil should be nil", attempt)
		}
	}

	// 5th attempt (threshold) triggers lockout and resets attempts to 0
	newAttempts, lockedUntil, justLocked := auth.CalculateFailure(4, threshold, duration, now)
	if !justLocked {
		t.Fatalf("expected 5th attempt to trigger lockout")
	}
	if newAttempts != 0 {
		t.Fatalf("expected failed attempts to reset to 0 on lockout, got %d", newAttempts)
	}
	if lockedUntil == nil {
		t.Fatalf("expected lockedUntil to be set on lockout")
	}
	expectedLockTime := now.Add(duration).UTC()
	if !lockedUntil.Equal(expectedLockTime) {
		t.Fatalf("expected lockedUntil = %v, got %v", expectedLockTime, *lockedUntil)
	}
}

func ptr[T any](v T) *T {
	return &v
}
