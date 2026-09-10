package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI escape codes
const (
	resetCode     = "\033[0m"
	boldCode      = "\033[1m"
	dimCode       = "\033[2m"
	italicCode    = "\033[3m"
	underlineCode = "\033[4m"

	redCode     = "\033[31m"
	greenCode   = "\033[32m"
	yellowCode  = "\033[33m"
	blueCode    = "\033[34m"
	magentaCode = "\033[35m"
	cyanCode    = "\033[36m"
	grayCode    = "\033[90m"

	brightRedCode    = "\033[91m"
	brightGreenCode  = "\033[92m"
	brightYellowCode = "\033[93m"
	brightCyanCode   = "\033[96m"
	brightWhiteCode  = "\033[97m"
)

var forceColor *bool

// SetForcedColor allows tests or flags to explicitly force enable/disable color.
func SetForcedColor(forced *bool) {
	forceColor = forced
}

// IsColorEnabled checks whether ANSI color escape sequences should be used for w.
func IsColorEnabled(w io.Writer) bool {
	if forceColor != nil {
		return *forceColor
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if w == nil {
		w = os.Stdout
	}
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// Colorize wraps text with code if color is enabled on writer w.
func Colorize(w io.Writer, code, text string) string {
	if !IsColorEnabled(w) || text == "" {
		return text
	}
	return code + text + resetCode
}

// Bold returns text in bold style.
func Bold(w io.Writer, text string) string {
	return Colorize(w, boldCode, text)
}

// Dim returns text in dim / faint style.
func Dim(w io.Writer, text string) string {
	return Colorize(w, dimCode, text)
}

// Cyan returns text in cyan.
func Cyan(w io.Writer, text string) string {
	return Colorize(w, cyanCode, text)
}

// BrightCyan returns text in bright cyan.
func BrightCyan(w io.Writer, text string) string {
	return Colorize(w, brightCyanCode, text)
}

// Green returns text in emerald green.
func Green(w io.Writer, text string) string {
	return Colorize(w, greenCode, text)
}

// Red returns text in coral red.
func Red(w io.Writer, text string) string {
	return Colorize(w, redCode, text)
}

// Yellow returns text in amber yellow.
func Yellow(w io.Writer, text string) string {
	return Colorize(w, yellowCode, text)
}

// Gray returns text in subtle dark gray.
func Gray(w io.Writer, text string) string {
	return Colorize(w, grayCode, text)
}

// Success returns a styled success message with an emerald checkmark.
func Success(w io.Writer, msg string) string {
	icon := Green(w, "✔")
	return fmt.Sprintf("%s  %s", icon, msg)
}

// Error returns a styled error message with a coral cross badge.
func Error(w io.Writer, err any) string {
	icon := Red(w, "✖")
	label := Red(w, "Error:")
	return fmt.Sprintf("%s  %s %v", icon, label, err)
}

// Warn returns a styled warning message with an amber warning badge.
func Warn(w io.Writer, msg string) string {
	icon := Yellow(w, "⚠")
	label := Yellow(w, "Warning:")
	return fmt.Sprintf("%s  %s %s", icon, label, msg)
}

// Info returns an informational line with a cyan info badge.
func Info(w io.Writer, msg string) string {
	icon := Cyan(w, "ℹ")
	return fmt.Sprintf("%s  %s", icon, msg)
}

// PromptPrefix formats the interactive readline prompt.
func PromptPrefix(w io.Writer, username string) string {
	chevron := Cyan(w, "›")
	if username == "" {
		host := Dim(w, "osto")
		return fmt.Sprintf("%s %s ", host, chevron)
	}
	user := BrightCyan(w, username)
	at := Dim(w, "@")
	host := Dim(w, "osto")
	return fmt.Sprintf("%s %s %s %s ", user, at, host, chevron)
}

// InputPrompt formats a sub-prompt asking for user input.
func InputPrompt(w io.Writer, label string) string {
	glyph := Cyan(w, "?")
	cleanLabel := strings.TrimSpace(label)
	return fmt.Sprintf("%s %s ", glyph, cleanLabel)
}
