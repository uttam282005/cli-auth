package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"osto-cli-auth/internal/auth"
)

func TestGenerateTOTPKey(t *testing.T) {
	username := "bob"
	issuer := "OstoAuth"

	key, err := auth.GenerateTOTPKey(username, issuer)
	if err != nil {
		t.Fatalf("unexpected error generating TOTP key: %v", err)
	}

	if key == nil {
		t.Fatal("expected non-nil TOTP key")
	}

	secret := key.Secret()
	if len(secret) == 0 {
		t.Errorf("expected non-empty secret")
	}

	url := key.URL()
	if !strings.HasPrefix(url, "otpauth://totp/") {
		t.Errorf("expected otpauth URI prefix, got: %s", url)
	}
}

func TestValidateTOTP(t *testing.T) {
	key, err := auth.GenerateTOTPKey("testuser", "OstoAuth")
	if err != nil {
		t.Fatalf("failed to generate TOTP key: %v", err)
	}

	secret := key.Secret()

	// Generate valid passcode at current time
	validCode, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate valid code: %v", err)
	}

	if !auth.ValidateTOTP(validCode, secret) {
		t.Errorf("ValidateTOTP failed for valid code: %s", validCode)
	}

	// Invalid passcode
	if auth.ValidateTOTP("000000", secret) && validCode != "000000" {
		t.Errorf("ValidateTOTP succeeded for invalid passcode 000000")
	}
}

func TestGenerateTerminalQR(t *testing.T) {
	uri := "otpauth://totp/OstoAuth:bob?issuer=OstoAuth&secret=JBSWY3DPEHPK3PXP"
	qrStr, err := auth.GenerateTerminalQR(uri)
	if err != nil {
		t.Fatalf("unexpected error generating terminal QR: %v", err)
	}
	if len(qrStr) == 0 {
		t.Errorf("expected non-empty terminal QR string")
	}
}
