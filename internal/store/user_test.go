package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"osto-cli-auth/internal/store"
)

func setupTestDB(t *testing.T) (*store.SQLiteUserStore, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "osto-auth-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := store.OpenDB(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open test db: %v", err)
	}

	userStore := store.NewSQLiteUserStore(db)
	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return userStore, cleanup
}

func TestUserStore_CreateAndGetUser(t *testing.T) {
	s, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Create user with mixed case
	u, err := s.CreateUser(ctx, "Alice_Dev", "$2a$12$dummyHash")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if u.Username != "alice_dev" {
		t.Errorf("expected lowercase username alice_dev, got %s", u.Username)
	}

	// 2. Duplicate username check (case-insensitive)
	_, err = s.CreateUser(ctx, "ALICE_DEV", "$2a$12$anotherHash")
	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}

	// 3. Fetch user
	fetched, err := s.GetUserByUsername(ctx, "Alice_Dev")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if fetched.ID != u.ID {
		t.Errorf("expected ID %s, got %s", u.ID, fetched.ID)
	}
	if fetched.Username != "alice_dev" {
		t.Errorf("expected username alice_dev, got %s", fetched.Username)
	}
	if fetched.TOTPEnabled {
		t.Errorf("expected TOTPEnabled to be false initially")
	}
	if fetched.FailedAttempts != 0 {
		t.Errorf("expected FailedAttempts to be 0, got %d", fetched.FailedAttempts)
	}

	// 4. Non-existent user
	_, err = s.GetUserByUsername(ctx, "nonexistent")
	if !errors.Is(err, store.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_LockoutCycle(t *testing.T) {
	s, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	u, err := s.CreateUser(ctx, "bob", "$2a$12$dummy")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Increment failed attempts
	lockUntil := time.Now().UTC().Add(15 * time.Minute)
	if err := s.UpdateFailedAttempts(ctx, u.ID, 5, &lockUntil); err != nil {
		t.Fatalf("failed to update failed attempts: %v", err)
	}

	fetched, err := s.GetUserByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("failed to fetch user: %v", err)
	}
	if fetched.FailedAttempts != 5 {
		t.Errorf("expected 5 failed attempts, got %d", fetched.FailedAttempts)
	}
	if fetched.LockedUntil == nil {
		t.Fatalf("expected LockedUntil to be set")
	}

	// Reset failed attempts
	if err := s.ResetFailedAttempts(ctx, u.ID); err != nil {
		t.Fatalf("failed to reset failed attempts: %v", err)
	}

	cleared, err := s.GetUserByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("failed to fetch user: %v", err)
	}
	if cleared.FailedAttempts != 0 {
		t.Errorf("expected 0 failed attempts, got %d", cleared.FailedAttempts)
	}
	if cleared.LockedUntil != nil {
		t.Errorf("expected LockedUntil to be nil after reset")
	}
}

func TestUserStore_TOTPAndLastLogin(t *testing.T) {
	s, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	u, err := s.CreateUser(ctx, "charlie", "$2a$12$dummy")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Enable TOTP
	secret := "JBSWY3DPEHPK3PXP"
	if err := s.SetTOTP(ctx, u.ID, secret, true); err != nil {
		t.Fatalf("failed to set TOTP: %v", err)
	}

	withTOTP, err := s.GetUserByUsername(ctx, "charlie")
	if err != nil {
		t.Fatalf("failed to fetch user: %v", err)
	}
	if !withTOTP.TOTPEnabled {
		t.Errorf("expected TOTPEnabled = true")
	}
	if withTOTP.TOTPSecret == nil || *withTOTP.TOTPSecret != secret {
		t.Errorf("expected secret %s, got %v", secret, withTOTP.TOTPSecret)
	}

	// Update last login
	loginTime := time.Now().UTC()
	if err := s.UpdateLastLogin(ctx, u.ID, loginTime); err != nil {
		t.Fatalf("failed to update last login: %v", err)
	}

	withLogin, err := s.GetUserByUsername(ctx, "charlie")
	if err != nil {
		t.Fatalf("failed to fetch user: %v", err)
	}
	if withLogin.LastLoginAt == nil {
		t.Fatalf("expected LastLoginAt to be set")
	}

	// Disable TOTP
	if err := s.DisableTOTP(ctx, u.ID); err != nil {
		t.Fatalf("failed to disable TOTP: %v", err)
	}

	withoutTOTP, err := s.GetUserByUsername(ctx, "charlie")
	if err != nil {
		t.Fatalf("failed to fetch user: %v", err)
	}
	if withoutTOTP.TOTPEnabled {
		t.Errorf("expected TOTPEnabled = false after disable")
	}
	if withoutTOTP.TOTPSecret != nil {
		t.Errorf("expected TOTPSecret to be nil after disable")
	}
}
