package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"osto-cli-auth/internal/auth"
	"osto-cli-auth/internal/cli/ui"
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

	r.print(ui.Render2FAHeader(r.out))
	qrStr, err := auth.GenerateTerminalQR(uri)
	if err == nil && qrStr != "" {
		r.println(qrStr)
	}

	r.println(ui.Bold(r.out, "Manual Entry Details:"))
	r.printf("  %s %s\n", ui.Dim(r.out, "Secret Key:"), ui.BrightCyan(r.out, secret))
	r.printf("  %s %s\n\n", ui.Dim(r.out, "URI:       "), ui.Dim(r.out, uri))

	passcode, err := r.readPrompt(ui.InputPrompt(r.out, "Enter 6-digit code from authenticator:"))
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

	r.println(ui.Success(r.out, "Two-factor authentication has been successfully enabled."))
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
	password, err := r.readPassword(ui.InputPrompt(r.out, "Enter current password to confirm disabling 2FA:"))
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

	r.println(ui.Success(r.out, "Two-factor authentication has been disabled."))
	return nil
}

func (r *REPL) handleLogout(ctx context.Context) error {
	if r.currentSession != nil {
		r.sessions.Delete(r.currentSession.Token)
		r.currentSession = nil
	}
	r.currentUser = nil

	r.println(ui.Success(r.out, "Logged out successfully."))
	return nil
}

func (r *REPL) handlePostLoginHelp() error {
	items := [][2]string{
		{"whoami", "Display current user profile and session status"},
		{"enable-2fa", "Setup TOTP two-factor authentication with QR code"},
		{"disable-2fa", "Disable two-factor authentication"},
		{"logout", "End your active session"},
		{"help", "Display available commands"},
		{"exit", "Log out and exit the application"},
	}
	r.print(ui.RenderHelp(r.out, "Session Commands:", items))
	return nil
}
