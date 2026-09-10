package ui

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	profileInnerWidth = 58
	bannerInnerWidth  = 48
)

// padRow builds a card row with exact border alignment regardless of ANSI escape codes in styledVal.
func padRow(w io.Writer, key, plainVal, styledVal string) string {
	borderColor := Dim(w, "│")
	keyWidth := 16
	// profileInnerWidth = 58: 2 (left space) + 16 (key) + 2 (gap) + valWidth + 2 (right space)
	// valWidth = 58 - 2 - 16 - 2 - 2 = 36
	valWidth := profileInnerWidth - 2 - keyWidth - 2 - 2

	keyRunes := utf8.RuneCountInString(key)
	valRunes := utf8.RuneCountInString(plainVal)

	keyPadding := ""
	if keyWidth > keyRunes {
		keyPadding = strings.Repeat(" ", keyWidth-keyRunes)
	}

	valPadding := ""
	if valWidth > valRunes {
		valPadding = strings.Repeat(" ", valWidth-valRunes)
	}

	keyText := Dim(w, key)
	return fmt.Sprintf("%s  %s%s  %s%s  %s\n", borderColor, keyText, keyPadding, styledVal, valPadding, borderColor)
}

func padBannerLine(w io.Writer, plainText, styledText string) string {
	borderColor := Dim(w, "│")
	contentWidth := bannerInnerWidth - 4 // 2 left spaces + 2 right spaces = 44
	plainRunes := utf8.RuneCountInString(plainText)

	padding := ""
	if contentWidth > plainRunes {
		padding = strings.Repeat(" ", contentWidth-plainRunes)
	}
	return fmt.Sprintf("%s  %s%s  %s\n", borderColor, styledText, padding, borderColor)
}

// WelcomeBanner generates the startup application header banner.
func WelcomeBanner(w io.Writer) string {
	borderDim := func(s string) string { return Dim(w, s) }
	title := BrightCyan(w, "osto auth cli")
	version := Dim(w, "v1.0.0")

	titleLinePlain := "osto auth cli  v1.0.0"
	titleLineStyled := fmt.Sprintf("%s  %s", title, version)

	line1 := "Welcome to the CLI Login System."
	line2 := "Type 'help' for commands, 'exit' to quit."

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(borderDim("╭"+strings.Repeat("─", bannerInnerWidth)+"╮") + "\n")
	b.WriteString(padBannerLine(w, titleLinePlain, titleLineStyled))
	b.WriteString(padBannerLine(w, line1, line1))
	b.WriteString(padBannerLine(w, line2, line2))
	b.WriteString(borderDim("╰"+strings.Repeat("─", bannerInnerWidth)+"╯") + "\n\n")
	return b.String()
}

// RenderProfileCard builds a sleek rounded-border session profile card.
func RenderProfileCard(
	w io.Writer,
	username string,
	registeredAt time.Time,
	totpEnabled bool,
	expiresAt *time.Time,
	lastLoginAt *time.Time,
) string {
	borderDim := func(s string) string { return Dim(w, s) }

	titleText := " Session Profile "
	// total inner width = 58. 2 dashes before titleText, then titleText (17 chars), remaining = 58 - 2 - 17 = 39 dashes
	titleRunes := utf8.RuneCountInString(titleText)
	topDashes := profileInnerWidth - 2 - titleRunes
	if topDashes < 2 {
		topDashes = 2
	}
	topBorder := borderDim("╭──") + BrightCyan(w, titleText) + borderDim(strings.Repeat("─", topDashes)+"╮")
	botBorder := borderDim("╰" + strings.Repeat("─", profileInnerWidth) + "╯")

	// 1. User
	userPlain := username
	userStyled := Bold(w, BrightCyan(w, username))

	// 2. Registered
	regPlain := registeredAt.UTC().Format("2006-01-02 15:04:05 UTC")
	regStyled := regPlain

	// 3. 2FA Status
	var mfaPlain, mfaStyled string
	if totpEnabled {
		mfaPlain = "● enabled (TOTP)"
		dot := Green(w, "●")
		txt := Green(w, "enabled")
		tag := Dim(w, "(TOTP)")
		mfaStyled = fmt.Sprintf("%s %s %s", dot, txt, tag)
	} else {
		mfaPlain = "○ disabled"
		dot := Dim(w, "○")
		txt := Dim(w, "disabled")
		mfaStyled = fmt.Sprintf("%s %s", dot, txt)
	}

	// 4. Session Expiration
	var expPlain, expStyled string
	if expiresAt != nil {
		now := time.Now().UTC()
		tStr := expiresAt.UTC().Format("15:04:05 UTC")
		if expiresAt.After(now) {
			rem := expiresAt.Sub(now).Round(time.Minute)
			if rem < time.Minute {
				rem = time.Minute
			}
			expPlain = fmt.Sprintf("%s (%s remaining)", tStr, rem)
			remText := Green(w, fmt.Sprintf("(%s remaining)", rem))
			expStyled = fmt.Sprintf("%s %s", tStr, remText)
		} else {
			expPlain = fmt.Sprintf("%s (expired)", tStr)
			expStyled = fmt.Sprintf("%s %s", tStr, Red(w, "(expired)"))
		}
	} else {
		expPlain = "N/A"
		expStyled = Dim(w, "N/A")
	}

	// 5. Last Login
	var lastPlain, lastStyled string
	if lastLoginAt != nil {
		lastPlain = lastLoginAt.UTC().Format("2006-01-02 15:04:05 UTC")
		lastStyled = lastPlain
	} else {
		lastPlain = "first login"
		lastStyled = Dim(w, "first login")
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(topBorder + "\n")
	b.WriteString(padRow(w, "User", userPlain, userStyled))
	b.WriteString(padRow(w, "Registered", regPlain, regStyled))
	b.WriteString(padRow(w, "2FA Security", mfaPlain, mfaStyled))
	b.WriteString(padRow(w, "Session Expiry", expPlain, expStyled))
	b.WriteString(padRow(w, "Last Login", lastPlain, lastStyled))
	b.WriteString(botBorder + "\n\n")

	return b.String()
}

// RenderHelp formats an aligned command table under a given section title.
func RenderHelp(w io.Writer, sectionTitle string, items [][2]string) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(Bold(w, sectionTitle) + "\n")

	maxCmdLen := 0
	for _, item := range items {
		if l := len(item[0]); l > maxCmdLen {
			maxCmdLen = l
		}
	}

	for _, item := range items {
		cmd := item[0]
		desc := item[1]
		pad := strings.Repeat(" ", maxCmdLen-len(cmd)+4)
		cmdStyled := Cyan(w, cmd)
		descStyled := Dim(w, desc)
		b.WriteString(fmt.Sprintf("  %s%s%s\n", cmdStyled, pad, descStyled))
	}
	b.WriteString("\n")
	return b.String()
}

// Render2FAHeader returns a styled informational box for 2FA enrollment.
func Render2FAHeader(w io.Writer) string {
	borderDim := func(s string) string { return Dim(w, s) }
	title := BrightCyan(w, " Two-Factor Authentication Setup ")

	msg := "Scan this QR code with Google Authenticator or compatible app:"
	innerW := 64
	topDashes := innerW - 2 - utf8.RuneCountInString(" Two-Factor Authentication Setup ")

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(borderDim("╭──") + title + borderDim(strings.Repeat("─", topDashes)+"╮") + "\n")
	b.WriteString(fmt.Sprintf("%s  %s  %s\n", borderDim("│"), msg, borderDim("│")))
	b.WriteString(borderDim("╰"+strings.Repeat("─", innerW)+"╯") + "\n\n")
	return b.String()
}
