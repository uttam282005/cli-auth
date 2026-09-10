package auth

import (
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
)

// GenerateTOTPKey generates a new RFC 6238 compliant TOTP key for a user.
func GenerateTOTPKey(username, issuer string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: username,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}
	return key, nil
}

// ValidateTOTP verifies a 6-digit TOTP passcode against a base32 secret.
func ValidateTOTP(passcode, secret string) bool {
	return totp.Validate(passcode, secret)
}

// GenerateTerminalQR renders an ASCII/Unicode QR code representation of the given URI for terminal display.
func GenerateTerminalQR(uri string) (string, error) {
	qr, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}
	// ToSmallString(false) renders compact block characters
	return qr.ToSmallString(false), nil
}
