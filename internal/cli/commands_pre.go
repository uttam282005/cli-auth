package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"osto-cli-auth/internal/auth"
	"osto-cli-auth/internal/cli/ui"
	"osto-cli-auth/internal/store"
)

func (r *REPL) handleRegister(ctx context.Context) error {
	username, err := r.readPrompt(ui.InputPrompt(r.out, "Enter username:"))
	if err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return err
	}

	password, err := r.readPassword(ui.InputPrompt(r.out, "Enter password:"))
	if err != nil {
		return err
	}
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}

	confirmPassword, err := r.readPassword(ui.InputPrompt(r.out, "Confirm password:"))
	if err != nil {
		return err
	}
	if password != confirmPassword {
		return errors.New("passwords do not match")
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to process password: %w", err)
	}

	_, err = r.store.CreateUser(ctx, username, passwordHash)
	if err != nil {
		if errors.Is(err, store.ErrUsernameTaken) {
			return errors.New("username is already taken")
		}
		return fmt.Errorf("registration failed: %w", err)
	}

	r.println(ui.Success(r.out, "User registered successfully. You may now log in."))
	return nil
}

func (r *REPL) handleLogin(ctx context.Context) error {
	now := time.Now().UTC()

	// 0. Check session-level lockout first
	if locked, remaining := auth.IsLocked(r.sessionLockedUntil, now); locked {
		return fmt.Errorf("account is locked due to multiple failed login attempts. Please try again in %v", remaining)
	}

	username, err := r.readPrompt(ui.InputPrompt(r.out, "Enter username:"))
	if err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		justLocked, _ := r.recordLoginFailure(now)
		if justLocked {
			return fmt.Errorf("account has been locked for %v due to %d consecutive failed attempts", r.cfg.LockoutDuration, r.cfg.LockoutThreshold)
		}
		return errors.New("username cannot be empty")
	}

	password, err := r.readPassword(ui.InputPrompt(r.out, "Enter password:"))
	if err != nil {
		return err
	}

	user, err := r.store.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			// Record failure on session level regardless of whether username exists
			justLocked, _ := r.recordLoginFailure(now)
			if justLocked {
				return fmt.Errorf("account has been locked for %v due to %d consecutive failed attempts", r.cfg.LockoutDuration, r.cfg.LockoutThreshold)
			}
			// Constant-time/generic error to avoid username enumeration
			return errors.New("invalid username or password")
		}
		return fmt.Errorf("login failed: %w", err)
	}

	// 1. Check account lockout on user
	if locked, remaining := auth.IsLocked(user.LockedUntil, now); locked {
		return fmt.Errorf("account is locked due to multiple failed login attempts. Please try again in %v", remaining)
	}

	// 2. Verify password hash
	if !auth.VerifyPassword(user.PasswordHash, password) {
		newAttempts, lockedUntil, userJustLocked := auth.CalculateFailure(
			user.FailedAttempts,
			r.cfg.LockoutThreshold,
			r.cfg.LockoutDuration,
			now,
		)
		_ = r.store.UpdateFailedAttempts(ctx, user.ID, newAttempts, lockedUntil)

		sessionJustLocked, _ := r.recordLoginFailure(now)

		if userJustLocked || sessionJustLocked {
			return fmt.Errorf("account has been locked for %v due to %d consecutive failed attempts", r.cfg.LockoutDuration, r.cfg.LockoutThreshold)
		}
		return errors.New("invalid username or password")
	}

	// 3. If TOTP is enabled, prompt and verify passcode
	if user.TOTPEnabled {
		if user.TOTPSecret == nil {
			return errors.New("2FA is marked as enabled but secret is missing; please contact support")
		}

		passcode, err := r.readPrompt(ui.InputPrompt(r.out, "Enter 6-digit 2FA code:"))
		if err != nil {
			return err
		}
		passcode = strings.TrimSpace(passcode)

		if !auth.ValidateTOTP(passcode, *user.TOTPSecret) {
			// Wrong TOTP counts as a failed login attempt
			newAttempts, lockedUntil, userJustLocked := auth.CalculateFailure(
				user.FailedAttempts,
				r.cfg.LockoutThreshold,
				r.cfg.LockoutDuration,
				now,
			)
			_ = r.store.UpdateFailedAttempts(ctx, user.ID, newAttempts, lockedUntil)

			sessionJustLocked, _ := r.recordLoginFailure(now)

			if userJustLocked || sessionJustLocked {
				return fmt.Errorf("account has been locked for %v due to %d consecutive failed attempts", r.cfg.LockoutDuration, r.cfg.LockoutThreshold)
			}
			return errors.New("invalid 2FA code")
		}
	}

	// 4. Successful full authentication: reset failures and clear lockout
	if err := r.store.ResetFailedAttempts(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to reset lockout status: %w", err)
	}
	r.sessionFailedAttempts = 0
	r.sessionLockedUntil = nil

	// Update last login timestamp in DB
	loginTime := time.Now().UTC()
	_ = r.store.UpdateLastLogin(ctx, user.ID, loginTime)

	// Create session
	sess := r.sessions.Create(user.ID, user.Username, r.cfg.SessionTimeoutDuration)
	r.currentSession = sess
	r.currentUser = user

	// Display user details banner per spec §5
	RenderUserDetails(r.out, user, sess)

	return nil
}

func (r *REPL) handlePreLoginHelp() error {
	items := [][2]string{
		{"register", "Create a new user account"},
		{"login", "Authenticate with username and password (+ 2FA)"},
		{"help", "Display available commands"},
		{"exit", "Exit the application"},
	}
	r.print(ui.RenderHelp(r.out, "Authentication Commands:", items))
	return nil
}
