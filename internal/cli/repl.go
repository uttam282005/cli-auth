package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"golang.org/x/term"

	"osto-cli-auth/internal/auth"
	"osto-cli-auth/internal/cli/ui"
	"osto-cli-auth/internal/config"
	"osto-cli-auth/internal/session"
	"osto-cli-auth/internal/store"
)

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)
)

// REPL manages the interactive command-line session.
type REPL struct {
	cfg                   *config.Config
	store                 store.UserStore
	sessions              *session.Store
	currentSession        *session.Session
	currentUser           *store.User
	sessionFailedAttempts int
	sessionLockedUntil    *time.Time
	rl                    *readline.Instance
	stdinReader           *bufio.Reader
	in                    io.ReadCloser
	out                   io.Writer
}

// NewREPL initializes a new REPL with standard os.Stdin and os.Stdout.
func NewREPL(cfg *config.Config, userStore store.UserStore, sessionStore *session.Store) *REPL {
	return NewREPLWithIO(cfg, userStore, sessionStore, os.Stdin, os.Stdout)
}

// NewREPLWithIO initializes a REPL with custom input and output streams.
func NewREPLWithIO(cfg *config.Config, userStore store.UserStore, sessionStore *session.Store, in io.ReadCloser, out io.Writer) *REPL {
	return &REPL{
		cfg:         cfg,
		store:       userStore,
		sessions:    sessionStore,
		stdinReader: bufio.NewReader(in),
		in:          in,
		out:         out,
	}
}

// IsAuthenticated checks if there is an active valid session.
func (r *REPL) IsAuthenticated() bool {
	if r.currentSession == nil {
		return false
	}
	sess, valid := r.sessions.Get(r.currentSession.Token)
	if !valid {
		r.currentSession = nil
		r.currentUser = nil
		return false
	}
	r.currentSession = sess
	return true
}

func (r *REPL) getPrompt() string {
	if r.IsAuthenticated() {
		return ui.PromptPrefix(r.out, r.currentUser.Username)
	}
	return ui.PromptPrefix(r.out, "")
}

// dynamicCompleter provides tab completion depending on authentication state.
type dynamicCompleter struct {
	repl *REPL
}

func (d *dynamicCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	var candidates []string
	if d.repl.IsAuthenticated() {
		candidates = []string{"whoami", "enable-2fa", "disable-2fa", "logout", "help"}
	} else {
		candidates = []string{"register", "login", "help", "exit"}
	}

	prefix := strings.TrimSpace(string(line[:pos]))
	var matches [][]rune
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			matches = append(matches, []rune(c[len(prefix):]))
		}
	}
	return matches, len(prefix)
}

// Run starts the REPL loop until exit or EOF.
func (r *REPL) Run() error {
	rlConfig := &readline.Config{
		Prompt:            r.getPrompt(),
		AutoComplete:      &dynamicCompleter{repl: r},
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	}
	if r.in != nil {
		rlConfig.Stdin = r.in
	}
	if r.out != nil {
		rlConfig.Stdout = r.out
	}

	rl, err := readline.NewEx(rlConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()
	r.rl = rl

	r.print(ui.WelcomeBanner(r.out))

	for {
		rl.SetPrompt(r.getPrompt())
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if len(line) == 0 {
					return nil
				}
				continue
			} else if err == io.EOF {
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]

		if err := r.dispatch(cmd, parts[1:]); err != nil {
			if err == errExit {
				return nil
			}
			r.println(ui.Error(r.out, err))
		}
	}
}

var errExit = fmt.Errorf("exit requested")

func (r *REPL) dispatch(cmd string, args []string) error {
	ctx := context.Background()

	// If authenticated, check session validity before executing any command
	if r.currentSession != nil {
		if !r.IsAuthenticated() {
			r.println(ui.Warn(r.out, "Session expired. Please log in again."))
			r.rl.SetPrompt(r.getPrompt())
			return nil
		}
	}

	if r.IsAuthenticated() {
		switch cmd {
		case "whoami":
			return r.handleWhoami(ctx)
		case "enable-2fa":
			return r.handleEnable2FA(ctx)
		case "disable-2fa":
			return r.handleDisable2FA(ctx)
		case "logout":
			return r.handleLogout(ctx)
		case "help":
			return r.handlePostLoginHelp()
		case "exit":
			// Graceful exit from authenticated state
			r.handleLogout(ctx)
			return errExit
		default:
			r.println(ui.Error(r.out, fmt.Sprintf("Unknown command '%s'. Type 'help' to see available commands.", cmd)))
			return nil
		}
	}

	// Pre-login state
	switch cmd {
	case "register":
		return r.handleRegister(ctx)
	case "login":
		return r.handleLogin(ctx)
	case "help":
		return r.handlePreLoginHelp()
	case "exit":
		return errExit
	default:
		r.println(ui.Error(r.out, fmt.Sprintf("Unknown command '%s'. Type 'help' to see available commands.", cmd)))
		return nil
	}
}

// readPrompt reads a line with a prompt message using readline.
func (r *REPL) readPrompt(prompt string) (string, error) {
	if r.rl != nil {
		origPrompt := r.getPrompt()
		r.rl.SetPrompt(prompt)
		defer r.rl.SetPrompt(origPrompt)
		line, err := r.rl.Readline()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}

	fmt.Print(prompt)
	line, err := r.stdinReader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// readPassword securely reads a masked password using term.ReadPassword on a terminal,
// or falls back to prompt reading for automated tests and non-interactive pipes.
func (r *REPL) readPassword(prompt string) (string, error) {
	if r.in == os.Stdin && term.IsTerminal(int(os.Stdin.Fd())) {
		r.printf("%s", prompt)
		bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
		r.println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytePassword)), nil
	}

	return r.readPrompt(prompt)
}

func (r *REPL) print(a ...any) {
	if r.out != nil {
		fmt.Fprint(r.out, a...)
		return
	}
	fmt.Print(a...)
}

func (r *REPL) println(a ...any) {
	if r.out != nil {
		fmt.Fprintln(r.out, a...)
		return
	}
	fmt.Println(a...)
}

func (r *REPL) printf(format string, a ...any) {
	if r.out != nil {
		fmt.Fprintf(r.out, format, a...)
		return
	}
	fmt.Printf(format, a...)
}

func (r *REPL) recordLoginFailure(now time.Time) (justLocked bool, remaining time.Duration) {
	newAttempts, lockedUntil, locked := auth.CalculateFailure(
		r.sessionFailedAttempts,
		r.cfg.LockoutThreshold,
		r.cfg.LockoutDuration,
		now,
	)
	r.sessionFailedAttempts = newAttempts
	if locked {
		r.sessionLockedUntil = lockedUntil
		return true, r.cfg.LockoutDuration
	}
	return false, 0
}

// RenderUserDetails prints user information block per spec §5.
func RenderUserDetails(out io.Writer, u *store.User, sess *session.Session) {
	if out == nil {
		out = os.Stdout
	}
	var expiresAt *time.Time
	if sess != nil {
		expiresAt = &sess.ExpiresAt
	}
	card := ui.RenderProfileCard(out, u.Username, u.CreatedAt, u.TOTPEnabled, expiresAt, u.LastLoginAt)
	fmt.Fprint(out, card)
}

func validateUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("username must be 3-32 characters long and contain only letters, numbers, underscores, and hyphens")
	}
	return nil
}
