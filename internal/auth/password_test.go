package auth_test

import (
	"testing"

	"osto-cli-auth/internal/auth"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"empty password", "", true},
		{"too short (7 chars)", "1234567", true},
		{"exact min length (8 chars)", "12345678", false},
		{"long password", "correct-horse-battery-staple", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePassword(%q) error = %v, wantErr = %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	password := "superSecret123!"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}

	if hash == password {
		t.Fatalf("hashed password should not match plain text")
	}

	// Verify with correct password
	if !auth.VerifyPassword(hash, password) {
		t.Errorf("VerifyPassword() failed for valid password")
	}

	// Verify with wrong password
	if auth.VerifyPassword(hash, "wrongPassword") {
		t.Errorf("VerifyPassword() returned true for invalid password")
	}
}
