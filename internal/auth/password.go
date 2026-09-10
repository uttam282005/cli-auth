package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the work factor used for password hashing (cost 12 per spec).
	BcryptCost = 12

	// MinPasswordLength defines the minimum allowed password length.
	MinPasswordLength = 8
)

var (
	// ErrPasswordTooShort is returned when a password has fewer than 8 characters.
	ErrPasswordTooShort = fmt.Errorf("password must be at least %d characters long", MinPasswordLength)
)

// ValidatePassword ensures password satisfies minimum complexity requirements.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// HashPassword hashes a plain text password using bcrypt with cost 12.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword compares a bcrypt hashed password with its plain text candidate.
func VerifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}
