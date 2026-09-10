package auth

import (
	"time"
)

// IsLocked checks if an account is locked based on lockedUntil and current time.
// Returns true and the remaining duration if locked.
func IsLocked(lockedUntil *time.Time, now time.Time) (bool, time.Duration) {
	if lockedUntil == nil {
		return false, 0
	}
	if lockedUntil.After(now) {
		return true, lockedUntil.Sub(now).Round(time.Second)
	}
	return false, 0
}

// CalculateFailure updates the failed attempt count and determines whether a lockout should trigger.
// When newAttempts >= threshold, lockedUntil is set to now + duration, and attempts reset to 0.
func CalculateFailure(currentAttempts int, threshold int, duration time.Duration, now time.Time) (newAttempts int, lockedUntil *time.Time, justLocked bool) {
	attempts := currentAttempts + 1
	if attempts >= threshold {
		lockTime := now.Add(duration).UTC()
		return 0, &lockTime, true
	}
	return attempts, nil, false
}
