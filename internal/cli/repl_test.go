package cli_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"osto-cli-auth/internal/auth"
	"osto-cli-auth/internal/cli"
	"osto-cli-auth/internal/config"
	"osto-cli-auth/internal/session"
	"osto-cli-auth/internal/store"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func setupTestCLI(t *testing.T, input string) (*cli.REPL, *bytes.Buffer, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "osto-cli-test-*")
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
	sessionStore := session.NewStore()

	cfg := &config.Config{
		DBPath:                 dbPath,
		SessionTimeoutDuration: 30 * time.Minute,
		LockoutThreshold:       5,
		LockoutDuration:        15 * time.Minute,
	}

	out := &bytes.Buffer{}
	in := nopCloser{Reader: strings.NewReader(input)}

	repl := cli.NewREPLWithIO(cfg, userStore, sessionStore, in, out)

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	}

	return repl, out, cleanup
}

func TestREPL_RegisterLoginWhoamiLogout(t *testing.T) {
	// Sequence:
	// 1. help
	// 2. register user dave with password "password123"
	// 3. login dave
	// 4. whoami
	// 5. logout
	// 6. exit
	input := "help\nregister\ndave\npassword123\npassword123\nlogin\ndave\npassword123\nwhoami\nlogout\nexit\n"

	repl, out, cleanup := setupTestCLI(t, input)
	defer cleanup()

	err := repl.Run()
	if err != nil {
		t.Fatalf("unexpected error running REPL: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Welcome to the CLI Login System") {
		t.Errorf("expected welcome message in output, got:\n%s", output)
	}
}

func TestREPL_EnableAndDisable2FA(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "osto-cli-2fa-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := store.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	userStore := store.NewSQLiteUserStore(db)
	sessionStore := session.NewStore()
	cfg := &config.Config{
		DBPath:                 dbPath,
		SessionTimeoutDuration: 30 * time.Minute,
		LockoutThreshold:       5,
		LockoutDuration:        15 * time.Minute,
	}

	// Pre-create user and session
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	u, err := userStore.CreateUser(context.Background(), "eve", hash)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Verify user starts without 2FA
	if u.TOTPEnabled {
		t.Fatal("user should not have 2FA enabled initially")
	}

	// Manually configure 2FA on store and test disable flow via CLI
	key, err := auth.GenerateTOTPKey("eve", "OstoAuth")
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	if err := userStore.SetTOTP(context.Background(), u.ID, key.Secret(), true); err != nil {
		t.Fatalf("failed to enable TOTP on store: %v", err)
	}

	// Test disabling 2FA via REPL (requires current password)
	input := "login\neve\npassword123\n" + authMockCode(t, key.Secret()) + "\ndisable-2fa\npassword123\nlogout\nexit\n"
	out := &bytes.Buffer{}
	repl := cli.NewREPLWithIO(cfg, userStore, sessionStore, nopCloser{Reader: strings.NewReader(input)}, out)

	if err := repl.Run(); err != nil {
		t.Fatalf("unexpected error running REPL: %v", err)
	}

	// Verify in DB that TOTP is now disabled
	refreshed, err := userStore.GetUserByUsername(context.Background(), "eve")
	if err != nil {
		t.Fatalf("failed to fetch user: %v", err)
	}
	if refreshed.TOTPEnabled {
		t.Errorf("expected TOTP to be disabled, but was still enabled")
	}
}

func authMockCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}
	return code
}

func TestREPL_SessionLockoutOnUnknownUser(t *testing.T) {
	// 5 failed attempts on non-existent users followed by 6th attempt
	input := "login\nunknown1\npass1\nlogin\nunknown2\npass2\nlogin\nunknown3\npass3\nlogin\nunknown4\npass4\nlogin\nunknown5\npass5\nlogin\nexit\n"

	repl, out, cleanup := setupTestCLI(t, input)
	defer cleanup()

	err := repl.Run()
	if err != nil {
		t.Fatalf("unexpected error running REPL: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "account has been locked for 15m0s due to 5 consecutive failed attempts") {
		t.Errorf("expected 5th attempt to trigger lockout in output, got:\n%s", output)
	}
	if !strings.Contains(output, "account is locked due to multiple failed login attempts") {
		t.Errorf("expected 6th attempt to be rejected due to lockout in output, got:\n%s", output)
	}
}
