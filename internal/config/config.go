package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds runtime configuration parameters.
type Config struct {
	DBPath                 string
	SessionTimeoutDuration time.Duration
	LockoutThreshold       int
	LockoutDuration        time.Duration
}

// Load loads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/app.db"
	}

	sessionTimeoutMinutes := getEnvAsInt("SESSION_TIMEOUT_MINUTES", 30)
	lockoutThreshold := getEnvAsInt("LOCKOUT_THRESHOLD", 5)
	lockoutDurationMinutes := getEnvAsInt("LOCKOUT_DURATION_MINUTES", 15)

	// Ensure parent directory for SQLite database exists if specified
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}

	return &Config{
		DBPath:                 dbPath,
		SessionTimeoutDuration: time.Duration(sessionTimeoutMinutes) * time.Minute,
		LockoutThreshold:       lockoutThreshold,
		LockoutDuration:        time.Duration(lockoutDurationMinutes) * time.Minute,
	}, nil
}

func getEnvAsInt(name string, defaultValue int) int {
	valStr := os.Getenv(name)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil || val <= 0 {
		return defaultValue
	}
	return val
}
