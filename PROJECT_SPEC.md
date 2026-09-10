# Spec: Containerized CLI Login System with Optional 2FA

## Context for the agent
This is a take-home assignment being evaluated on: correctness, code quality, security, Docker usage, CLI usability, and documentation. Build it like it's going to be read by a senior Go engineer, not just made to pass a checklist. Prefer boring, correct, idiomatic code over cleverness.

Language: **Go** (stdlib-first, minimal deps).
Database: **SQLite**, file-backed, persisted via a mounted/named volume.
Deployment: **Docker + docker-compose**.

---

## 1. Project layout

```
.
├── cmd/
│   └── cli/
│       └── main.go              # entrypoint, wires everything together
├── internal/
│   ├── auth/
│   │   ├── password.go          # bcrypt hash/verify
│   │   ├── totp.go              # TOTP generate/verify, secret gen
│   │   └── lockout.go           # failed-attempt tracking + lockout logic
│   ├── session/
│   │   └── session.go           # in-memory session store, TTL/expiry
│   ├── store/
│   │   ├── db.go                # sqlite connection setup, pragmas
│   │   ├── user.go               # user CRUD queries
│   │   └── migrations/
│   │       ├── 0001_init.up.sql
│   │       └── 0001_init.down.sql
│   ├── cli/
│   │   ├── repl.go               # interactive loop, history, tab-completion
│   │   ├── commands_pre.go       # register/login/help/exit
│   │   └── commands_post.go      # whoami/enable-2fa/disable-2fa/logout/help
│   └── config/
│       └── config.go             # env var loading
├── data/                          # gitignored, holds the sqlite file when run locally
├── Dockerfile
├── docker-compose.yml
├── go.mod / go.sum
├── .env.example
├── README.md
└── Makefile                      # optional but nice: make up/down/test/migrate
```

---

## 2. Tech choices (be explicit, don't improvise silently)

- **DB driver**: `modernc.org/sqlite` (pure-Go, no CGO — avoids needing gcc in the build image) via `database/sql`. If CGO in the build stage is acceptable, `mattn/go-sqlite3` is the more common alternative — pick one and note the tradeoff (pure-Go = simpler Docker build, cgo driver = more battle-tested/faster). Default to `modernc.org/sqlite` unless there's a reason not to.
- Enable `PRAGMA journal_mode=WAL;` and `PRAGMA foreign_keys=ON;` on connection open — SQLite doesn't enforce FKs by default and WAL mode matters for a process doing concurrent reads/writes to its own session logic.
- **Password hashing**: `golang.org/x/crypto/bcrypt`, cost factor 12.
- **TOTP**: `github.com/pquerna/otp` (supports RFC 6238, generates otpauth:// URLs compatible with Google Authenticator).
- **CLI REPL**: `github.com/chzyer/readline` (gives history + tab completion out of the box) — do not hand-roll a readline implementation.
- **Migrations**: plain `.sql` files run manually via `golang-migrate/migrate` CLI (has a sqlite3 driver) or a small embedded-migration runner using `embed.FS` — pick one and document it in README. Don't use an ORM.
- **UUIDs**: `github.com/google/uuid` for user IDs and session tokens.
- No web framework, no HTTP server — this is a pure CLI app talking directly to the SQLite file.

---

## 3. Database schema

```sql
CREATE TABLE users (
    id              TEXT PRIMARY KEY,          -- UUID string, generated in Go (uuid.NewString())
    username        TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,
    totp_secret     TEXT,                      -- NULL until 2FA enabled
    totp_enabled    INTEGER NOT NULL DEFAULT 0, -- SQLite has no native bool; 0/1
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until    TEXT,                      -- ISO-8601 UTC string, NULL if not locked
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    last_login_at   TEXT
);
```

SQLite has no `UUID`, `BOOLEAN`, or `TIMESTAMPTZ` types — generate UUIDs in Go with `uuid.NewString()` before insert, treat `totp_enabled` as an int (0/1) and convert to bool in Go, and store all timestamps as UTC ISO-8601 strings (`time.Now().UTC().Format(time.RFC3339)`), parsing back on read. Don't rely on SQLite's loose typing to paper over this — keep the Go struct strongly typed and convert explicitly at the store boundary.

Sessions do **not** need a DB table — keep them in-memory in the running process (map keyed by session token, protected by a mutex), since this is a single-process CLI, not a multi-instance service. Document this as a deliberate simplification in the README.

---

## 4. Auth flow requirements

### Registration (`register`)
- Prompt for username, password (masked input — `readline` supports this via `SetPasswordCallback` or reading with `golang.org/x/term.ReadPassword`).
- Validate: username non-empty, unique; password min length 8.
- Hash password with bcrypt, insert row.
- On success: print confirmation, do **not** auto-login.

### Login (`login`)
- Prompt username + masked password.
- If user not found → generic error ("invalid username or password") — never reveal whether the username exists.
- If `locked_until` is set and in the future → reject with remaining lockout time, don't check password at all.
- Verify bcrypt hash.
  - On failure: increment `failed_attempts`. If it crosses the threshold (**5**), set `locked_until = now() + 15 minutes` and reset `failed_attempts` to 0. Persist to DB.
  - On success: reset `failed_attempts` to 0, clear `locked_until`.
- If `totp_enabled` is true, prompt for a 6-digit TOTP code and verify before completing login. Wrong code counts as a failed login attempt (same lockout logic).
- On full success: create session (UUID token, TTL from config, default 30 min), update `last_login_at`, then auto-display user details (see §5).

### 2FA enable/disable
- `enable-2fa`: generate a TOTP secret, display the `otpauth://` URI and the raw secret (for manual entry), ask user to confirm by entering a valid code once before persisting `totp_secret` + setting `totp_enabled = true`. Don't enable on unconfirmed setup — that's how people lock themselves out.
- `disable-2fa`: require current password re-entry before disabling (don't let a hijacked session casually turn off 2FA).

### Session management
- Config-driven timeout (env var, default 30 min).
- Sessions expire passively — check expiry on every post-login command; if expired, force logout and require re-login.
- `logout` invalidates the session token immediately.

### Lockout
- Threshold and duration should be config vars with sensible defaults (5 attempts / 15 min), not magic numbers buried in logic.

---

## 5. CLI behavior

- Runs as a persistent REPL (`cmd/cli/main.go` → `internal/cli/repl.go`), using `chzyer/readline` for history + tab-completion of command names.
- Prompt should reflect state, e.g. `> ` before login, `alice> ` after login.
- `help` output differs pre- vs post-login (only show commands valid in current state).
- Auto-display after successful login:
  ```
  Welcome, alice
  Registered:        2026-01-04 10:22:03 UTC
  MFA:                enabled
  Session expires:    2026-09-10 14:32:00 UTC
  Last login:         2026-09-09 08:15:41 UTC (or "first login")
  ```
- `whoami` prints the exact same block on demand (same fields, re-fetched — session expiry should reflect current remaining time, not the value at login). Don't duplicate the formatting logic between login and `whoami`; share one render function.
- Errors are clear and never leak internals (no raw SQL errors, no stack traces to the user — log those to stderr/log file instead).
- `exit` from pre-login state and `logout` from post-login state should both be graceful (flush nothing needed since sessions are in-memory, but close the DB pool cleanly).

---

## 6. Docker

### Dockerfile
- Multi-stage build: `golang:1.23-alpine` (or current stable) builder stage → minimal final stage (`alpine`, not `scratch` — SQLite still benefits from having a shell/`sqlite3` CLI available for debugging, and if you end up using the cgo driver you need libc anyway).
- If using `modernc.org/sqlite` (pure Go, no cgo), `CGO_ENABLED=0` keeps the build simple and `scratch` becomes viable — call this out as the reason for the driver choice.
- Final image should just run the compiled binary; CLI needs a TTY so document `docker compose run` usage, not `up` (a REPL doesn't make sense as a detached service).

### docker-compose.yml
- Single `app` service — no separate DB container needed since SQLite is embedded. Mount a named volume at e.g. `/data` and point the app at `/data/app.db` via env var, so the DB file survives container recreation.
- `stdin_open: true` + `tty: true` (required for interactive CLI in a container).
- `.env.example` with `DB_PATH` (e.g. `/data/app.db`), `SESSION_TIMEOUT_MINUTES`, `LOCKOUT_THRESHOLD`, `LOCKOUT_DURATION_MINUTES`.

Run instructions to document in README:
```
docker compose run --rm app
```
No separate "start the DB first" step — this is one of the concrete simplicity wins over Postgres, worth a line in the README.

---

## 7. Migrations

Include both up and down SQL files. With SQLite, running migrations automatically on app startup (check-and-apply against a `schema_migrations` table before opening the REPL) is the simplest option and avoids a separate migration step against a file that may not exist yet — recommended default here, but still document it explicitly in README rather than leaving it implicit.

---

## 8. Tests (optional per spec, but do them for these specifically)

- `internal/auth/password_test.go` — hash/verify round trip, wrong password rejected.
- `internal/auth/totp_test.go` — generate secret, verify valid code at current time step, reject invalid code.
- `internal/auth/lockout_test.go` — attempts under threshold don't lock, threshold triggers lock, lock expires correctly.
- Use table-driven tests, Go stdlib `testing`, no assertion library needed (or `testify/require` if you want cleaner failure output — either is fine, pick one).

---

## 9. README.md must include

1. Prerequisites (Docker, Docker Compose).
2. Setup: copy `.env.example` to `.env`, `docker compose run --rm app` (migrations apply automatically on first startup).
3. Usage walkthrough: register → login → enable-2fa → login with 2FA → logout.
4. Architecture note: why sessions are in-memory, why SQLite (single-process CLI, no need for a networked DB, simpler Docker setup, embedded file persisted via a volume), lockout policy numbers.
5. Known limitations (e.g., single-process only, no password reset flow, no rate limiting beyond lockout).

---

## 10. Explicit non-goals (don't gold-plate)

- No HTTP API, no web UI.
- No email/SMS, no password reset flow.
- No RBAC/multi-role system — this is single-role user auth.
- No horizontal scaling concerns — in-memory sessions are fine and should be defended as a decision, not treated as a shortcut to hide.

---

## 11. Order of implementation (suggested, not mandatory)

1. `config` + `store/db.go` (open the sqlite file, set pragmas) + migrations → confirm DB connects and schema applies.
2. `auth/password.go` + basic `register`/`login` CLI commands (no lockout/2FA yet) → confirm core loop works end-to-end.
3. `auth/lockout.go` → wire into login.
4. `session` package → wire session creation/expiry into login/logout/whoami.
5. `auth/totp.go` → wire `enable-2fa`/`disable-2fa`, TOTP check into login.
6. Polish CLI: readline history/tab-completion, help text, error messages, auto-display block.
7. Dockerfile + docker-compose + `.env.example`.
8. Tests.
9. README.

---

## 12. Definition of done

- `docker compose run --rm app` gets you to a working prompt with zero manual steps beyond `.env` setup.
- Registering, logging in, enabling 2FA, logging out, and logging back in with a TOTP code all work end-to-end.
- 5 wrong password attempts locks the account for 15 minutes; a 6th attempt during lockout is rejected without touching the password check.
- Killing and re-running the `app` container reuses the same volume — the SQLite file (and thus users) persists; sessions don't survive a restart, and that's expected and documented.
- `go vet ./...` and `gofmt -l .` are clean.
