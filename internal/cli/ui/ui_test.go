package ui_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"osto-cli-auth/internal/cli/ui"
)

func TestUI_ColorDisabledByDefaultOnBuffer(t *testing.T) {
	buf := &bytes.Buffer{}
	if ui.IsColorEnabled(buf) {
		t.Errorf("expected color to be disabled on *bytes.Buffer by default")
	}

	res := ui.Cyan(buf, "hello")
	if res != "hello" {
		t.Errorf("expected plain string 'hello', got: %q", res)
	}

	success := ui.Success(buf, "all good")
	if !strings.Contains(success, "✔  all good") {
		t.Errorf("expected checkmark with plain message, got: %q", success)
	}

	errStr := ui.Error(buf, "failed")
	if !strings.Contains(errStr, "✖  Error: failed") {
		t.Errorf("expected error message with cross, got: %q", errStr)
	}

	warnStr := ui.Warn(buf, "careful")
	if !strings.Contains(warnStr, "⚠  Warning: careful") {
		t.Errorf("expected warning message with symbol, got: %q", warnStr)
	}
}

func TestUI_ForcedColor(t *testing.T) {
	buf := &bytes.Buffer{}
	forced := true
	ui.SetForcedColor(&forced)
	defer func() {
		ui.SetForcedColor(nil)
	}()

	if !ui.IsColorEnabled(buf) {
		t.Errorf("expected color to be enabled when forced")
	}

	res := ui.Cyan(buf, "hello")
	if !strings.Contains(res, "\033[36mhello\033[0m") {
		t.Errorf("expected ANSI cyan code, got: %q", res)
	}
}

func TestUI_WelcomeBanner(t *testing.T) {
	buf := &bytes.Buffer{}
	banner := ui.WelcomeBanner(buf)

	if !strings.Contains(banner, "osto auth cli") {
		t.Errorf("expected app title in banner, got:\n%s", banner)
	}
	if !strings.Contains(banner, "Welcome to the CLI Login System") {
		t.Errorf("expected welcome message in banner, got:\n%s", banner)
	}
}

func TestUI_RenderProfileCard(t *testing.T) {
	buf := &bytes.Buffer{}
	regTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	expTime := time.Now().UTC().Add(25 * time.Minute)
	lastLogin := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	// Test with 2FA enabled
	card := ui.RenderProfileCard(buf, "alice", regTime, true, &expTime, &lastLogin)
	if !strings.Contains(card, "alice") {
		t.Errorf("expected username in profile card, got:\n%s", card)
	}
	if !strings.Contains(card, "2FA Security") || !strings.Contains(card, "enabled") {
		t.Errorf("expected 2FA status in profile card, got:\n%s", card)
	}
	if !strings.Contains(card, "2026-01-15 10:00:00 UTC") {
		t.Errorf("expected registered timestamp, got:\n%s", card)
	}

	// Test with 2FA disabled & first login
	card2 := ui.RenderProfileCard(buf, "bob", regTime, false, nil, nil)
	if !strings.Contains(card2, "bob") {
		t.Errorf("expected bob in card, got:\n%s", card2)
	}
	if !strings.Contains(card2, "disabled") {
		t.Errorf("expected disabled 2FA status, got:\n%s", card2)
	}
	if !strings.Contains(card2, "first login") {
		t.Errorf("expected first login in card, got:\n%s", card2)
	}
}

func TestUI_RenderHelp(t *testing.T) {
	buf := &bytes.Buffer{}
	items := [][2]string{
		{"register", "Create account"},
		{"login", "Log in"},
	}
	help := ui.RenderHelp(buf, "Commands:", items)

	if !strings.Contains(help, "Commands:") {
		t.Errorf("expected section title in help, got:\n%s", help)
	}
	if !strings.Contains(help, "register") || !strings.Contains(help, "Create account") {
		t.Errorf("expected register command, got:\n%s", help)
	}
}

func TestUI_PromptPrefix(t *testing.T) {
	buf := &bytes.Buffer{}
	unauth := ui.PromptPrefix(buf, "")
	if !strings.Contains(unauth, "osto › ") {
		t.Errorf("expected 'osto › ', got: %q", unauth)
	}

	auth := ui.PromptPrefix(buf, "charlie")
	if !strings.Contains(auth, "charlie @ osto › ") {
		t.Errorf("expected 'charlie @ osto › ', got: %q", auth)
	}

	input := ui.InputPrompt(buf, "Enter password:")
	if !strings.Contains(input, "? Enter password:") {
		t.Errorf("expected '? Enter password:', got: %q", input)
	}
}
