package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrUserNotFound is returned when a user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrUsernameTaken is returned when a username already exists.
	ErrUsernameTaken = errors.New("username already taken")
)

// User represents a user account record.
type User struct {
	ID             string
	Username       string
	PasswordHash   string
	TOTPSecret     *string
	TOTPEnabled    bool
	FailedAttempts int
	LockedUntil    *time.Time
	CreatedAt      time.Time
	LastLoginAt    *time.Time
}

// UserStore defines operations for managing users in the database.
type UserStore interface {
	CreateUser(ctx context.Context, username, passwordHash string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateFailedAttempts(ctx context.Context, userID string, attempts int, lockedUntil *time.Time) error
	ResetFailedAttempts(ctx context.Context, userID string) error
	UpdateLastLogin(ctx context.Context, userID string, loginTime time.Time) error
	SetTOTP(ctx context.Context, userID string, secret string, enabled bool) error
	DisableTOTP(ctx context.Context, userID string) error
}

// SQLiteUserStore implements UserStore using database/sql.
type SQLiteUserStore struct {
	db *sql.DB
}

// NewSQLiteUserStore creates a new SQLiteUserStore.
func NewSQLiteUserStore(db *sql.DB) *SQLiteUserStore {
	return &SQLiteUserStore{db: db}
}

// CreateUser inserts a new user record. Username is normalized to lowercase.
func (s *SQLiteUserStore) CreateUser(ctx context.Context, username, passwordHash string) (*User, error) {
	normalizedUsername := strings.ToLower(strings.TrimSpace(username))
	id := uuid.NewString()
	now := time.Now().UTC()
	createdAtStr := now.Format(time.RFC3339Nano)

	query := `
	INSERT INTO users (id, username, password_hash, totp_enabled, failed_attempts, created_at)
	VALUES (?, ?, ?, 0, 0, ?);`

	_, err := s.db.ExecContext(ctx, query, id, normalizedUsername, passwordHash, createdAtStr)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "constraint failed") {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{
		ID:             id,
		Username:       normalizedUsername,
		PasswordHash:   passwordHash,
		TOTPSecret:     nil,
		TOTPEnabled:    false,
		FailedAttempts: 0,
		LockedUntil:    nil,
		CreatedAt:      now,
		LastLoginAt:    nil,
	}, nil
}

// GetUserByUsername fetches a user record by username (case-insensitive).
func (s *SQLiteUserStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	normalizedUsername := strings.ToLower(strings.TrimSpace(username))
	query := `
	SELECT id, username, password_hash, totp_secret, totp_enabled, failed_attempts, locked_until, created_at, last_login_at
	FROM users
	WHERE username = ?;`

	var (
		u              User
		totpSecret     sql.NullString
		totpEnabledInt int
		lockedUntilStr sql.NullString
		createdAtStr   string
		lastLoginAtStr sql.NullString
	)

	err := s.db.QueryRowContext(ctx, query, normalizedUsername).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&totpSecret,
		&totpEnabledInt,
		&u.FailedAttempts,
		&lockedUntilStr,
		&createdAtStr,
		&lastLoginAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	u.TOTPEnabled = totpEnabledInt == 1
	if totpSecret.Valid {
		secret := totpSecret.String
		u.TOTPSecret = &secret
	}

	if lockedUntilStr.Valid {
		parsed, err := parseTimestamp(lockedUntilStr.String)
		if err == nil {
			u.LockedUntil = &parsed
		}
	}

	parsedCreatedAt, err := parseTimestamp(createdAtStr)
	if err != nil {
		u.CreatedAt = time.Now().UTC()
	} else {
		u.CreatedAt = parsedCreatedAt
	}

	if lastLoginAtStr.Valid {
		parsed, err := parseTimestamp(lastLoginAtStr.String)
		if err == nil {
			u.LastLoginAt = &parsed
		}
	}

	return &u, nil
}

// UpdateFailedAttempts updates the failed attempt counter and optional lockout timestamp.
func (s *SQLiteUserStore) UpdateFailedAttempts(ctx context.Context, userID string, attempts int, lockedUntil *time.Time) error {
	var lockedUntilStr *string
	if lockedUntil != nil {
		formatted := lockedUntil.UTC().Format(time.RFC3339Nano)
		lockedUntilStr = &formatted
	}

	query := `UPDATE users SET failed_attempts = ?, locked_until = ? WHERE id = ?;`
	_, err := s.db.ExecContext(ctx, query, attempts, lockedUntilStr, userID)
	if err != nil {
		return fmt.Errorf("failed to update failed attempts: %w", err)
	}
	return nil
}

// ResetFailedAttempts clears the failed attempts counter and unlocks the account.
func (s *SQLiteUserStore) ResetFailedAttempts(ctx context.Context, userID string) error {
	query := `UPDATE users SET failed_attempts = 0, locked_until = NULL WHERE id = ?;`
	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to reset failed attempts: %w", err)
	}
	return nil
}

// UpdateLastLogin updates the last login timestamp for a user.
func (s *SQLiteUserStore) UpdateLastLogin(ctx context.Context, userID string, loginTime time.Time) error {
	loginTimeStr := loginTime.UTC().Format(time.RFC3339Nano)
	query := `UPDATE users SET last_login_at = ? WHERE id = ?;`
	_, err := s.db.ExecContext(ctx, query, loginTimeStr, userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

// SetTOTP updates the user's TOTP secret and enabled status.
func (s *SQLiteUserStore) SetTOTP(ctx context.Context, userID string, secret string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	query := `UPDATE users SET totp_secret = ?, totp_enabled = ? WHERE id = ?;`
	_, err := s.db.ExecContext(ctx, query, secret, enabledInt, userID)
	if err != nil {
		return fmt.Errorf("failed to set totp: %w", err)
	}
	return nil
}

// DisableTOTP removes TOTP configuration for a user.
func (s *SQLiteUserStore) DisableTOTP(ctx context.Context, userID string) error {
	query := `UPDATE users SET totp_secret = NULL, totp_enabled = 0 WHERE id = ?;`
	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to disable totp: %w", err)
	}
	return nil
}

func parseTimestamp(ts string) (time.Time, error) {
	// Try RFC3339Nano first, then RFC3339
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.UTC(), nil
	}
	// Fallback to SQLite standard datetime format "2006-01-02 15:04:05"
	return time.Parse("2006-01-02 15:04:05", ts)
}
