package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"osto-cli-auth/internal/auth"
)

func (r *REPL) handleWhoami(ctx context.Context) error {
	freshUser, err := r.store.GetUserByUsername(ctx, r.currentUser.Username)
	if err != nil {
		return fmt.Errorf("failed to fetch user details: %w", err)
	}
	r.currentUser = freshUser

	sess, valid := r.sessions.Get(r.currentSession.Token)
	if !valid {
		r.currentSession = nil
		r.currentUser = nil
		return errors.New("session expired")
	}

	RenderUserDetails(r.out, freshUser, sess)
	return nil
}

func (r *REPL) handleEnable2FA(ctx context.Context) error {
	freshUser, err := r.store.GetUserByUsername(ctx, r.currentUser.Username)
	if err != nil {
		return fmt.Errorf("failed to refresh user state: %w", err)
	}
	r.currentUser = freshUser

	if freshUser.TOTPEnabled {
		return errors.New("2FA is already enabled on this account")
	}

	key, err := auth.GenerateTOTPKey(freshUser.Username, "OstoAuth")
	if err != nil {
		return fmt.Errorf("failed to generate 2FA key: %w", err)
	}

	secret := key.Secret()
	uri := key.URL()

	r.println("\n--- Two-Factor Authentication Setup ---")
	qrStr, err := auth.GenerateTerminalQR(uri)
	if err == nil && qrStr != "" {
		r.println("\nScan the QR code below with Google Authenticator or compatible app:")
		r.println(qrStr)
	}

	r.println("Manual entry key:")
	r.printf("  Secret: %s\n", secret)
	r.printf("  URI:    %s\n\n", uri)

	passcode, err := r.readPrompt("Enter the 6-digit code from your authenticator app to confirm: ")
	if err != nil {
		return err
	}
	passcode = strings.TrimSpace(passcode)

	if !auth.ValidateTOTP(passcode, secret) {
		return errors.New("invalid verification code. 2FA setup cancelled")
	}

	if err := r.store.SetTOTP(ctx, freshUser.ID, secret, true); err != nil {
		return fmt.Errorf("failed to persist 2FA status: %w", err)
	}

	r.currentUser.TOTPEnabled = true
	r.currentUser.TOTPSecret = &secret

	r.println("\nSuccess: 2FA has been successfully enabled for your account.")
	return nil
}

func (r *REPL) handleDisable2FA(ctx context.Context) error {
	freshUser, err := r.store.GetUserByUsername(ctx, r.currentUser.Username)
	if err != nil {
		return fmt.Errorf("failed to refresh user state: %w", err)
	}
	r.currentUser = freshUser

	if !freshUser.TOTPEnabled {
		return errors.New("2FA is not enabled on this account")
	}

	// Security requirement: Re-verify current password before disabling 2FA
	password, err := r.readPassword("Enter your current password to confirm disabling 2FA: ")
	if err != nil {
		return err
	}

	if !auth.VerifyPassword(freshUser.PasswordHash, password) {
		return errors.New("incorrect password. 2FA was not disabled")
	}

	if err := r.store.DisableTOTP(ctx, freshUser.ID); err != nil {
		return fmt.Errorf("failed to disable 2FA: %w", err)
	}

	r.currentUser.TOTPEnabled = false
	r.currentUser.TOTPSecret = nil

	r.println("Success: 2FA has been disabled for your account.")
	return nil
}

func (r *REPL) handleLogout(ctx context.Context) error {
	if r.currentSession != nil {
		r.sessions.Delete(r.currentSession.Token)
		r.currentSession = nil
	}
	r.currentUser = nil

	r.println("Logged out successfully.")
	return nil
}

func (r *REPL) handlePostLoginHelp() error {
	r.println("\nAvailable commands:")
	r.println("  whoami       - Display current user profile and session status")
	r.println("  enable-2fa   - Enable TOTP-based two-factor authentication")
	r.println("  disable-2fa  - Disable two-factor authentication")
	r.println("  logout       - End your active session")
	r.println("  help         - Show available commands")
	return nil
}
