# Containerized CLI Login System with Optional 2FA

A secure, idiomatic, containerized command-line authentication system written in Go. Features user registration, bcrypt password hashing, optional TOTP-based two-factor authentication (Google Authenticator compatible) with terminal QR code display, account lockout protection, in-memory session management, and persistent SQLite storage.

---

## Features

- **Interactive REPL**: Interactive prompt powered by `readline` featuring command history, dynamic context-aware tab completion, and masked password inputs.
- **Secure Password Storage**: Passwords hashed using `bcrypt` with cost factor 12.
- **Optional TOTP-based 2FA**: RFC 6238 compliant two-factor authentication. Automatically generates and prints an ASCII/Unicode QR code in the terminal alongside the raw Base32 secret and `otpauth://` URI. Confirmation is required prior to activation.
- **2FA Disablement Protection**: Re-authentication with the account password is required before 2FA can be disabled.
- **Account Lockout Policy**: Tracks failed authentication attempts (both password and TOTP failures). Locks the account for 15 minutes after 5 consecutive failed attempts, rejecting further attempts during lockout without evaluating passwords.
- **In-Memory Session Management**: High-performance, thread-safe in-process session manager with configurable TTL (default 30 minutes) and passive expiration on commands.
- **Embedded Schema Migrations**: SQL migrations (`0001_init.up.sql`) embedded directly into the binary via `embed.FS` and applied idempotently on startup.
- **Containerized & Persistent**: Runs in Docker with Compose; data persists across container restarts via a named volume.

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (v20.10+)
- [Docker Compose](https://docs.docker.com/compose/) (v2.0+)
- *(Optional for local development without Docker)*: [Go 1.24+](https://golang.org/dl/)

---

## Quickstart (Docker)

### 1. Configure Environment
Copy the example environment file:
```bash
cp .env.example .env
```

### 2. Run the Application
Start the interactive CLI in Docker:
```bash
docker compose run --rm app
```
*Note: Schema migrations apply automatically on startup.*

To stop the CLI, type `exit` at the prompt or press `Ctrl+C`.

---

## Configuration Reference

The application is configured through environment variables (defined in `.env` or passed to Docker):

| Variable | Default | Description |
| :--- | :--- | :--- |
| `DB_PATH` | `/data/app.db` | File path to the SQLite database. In Docker, `/data` is mounted to the named volume `app-data`. |
| `SESSION_TIMEOUT_MINUTES` | `30` | Duration in minutes before an active session expires. |
| `LOCKOUT_THRESHOLD` | `5` | Number of consecutive failed login attempts before an account is locked. |
| `LOCKOUT_DURATION_MINUTES` | `15` | Duration in minutes an account remains locked after crossing the threshold. |

---

## Usage Walkthrough

### 1. Register a New Account
At the `> ` prompt:
```text
> register
Enter username: alice
Enter password: [masked]
Confirm password: [masked]
User registered successfully. You may now log in.
```
*Validation: Usernames must be 3–32 alphanumeric characters (letters, numbers, hyphens, underscores). Passwords must be at least 8 characters long.*

### 2. Log In
```text
> login
Enter username: alice
Enter password: [masked]

Welcome, alice
Registered:        2026-09-10 17:07:57 UTC
MFA:               disabled
Session expires:   2026-09-10 17:37:57 UTC
Last login:        first login
```
Upon login, the prompt updates to reflect the active user:
```text
alice> 
```

### 3. Check Status (`whoami`)
```text
alice> whoami

Welcome, alice
Registered:        2026-09-10 17:07:57 UTC
MFA:               disabled
Session expires:   2026-09-10 17:37:57 UTC
Last login:        2026-09-10 17:07:57 UTC
```

### 4. Enable Two-Factor Authentication (`enable-2fa`)
```text
alice> enable-2fa

--- Two-Factor Authentication Setup ---

Scan the QR code below with Google Authenticator or compatible app:
[ASCII QR Code displayed here]

Manual entry key:
  Secret: JBSWY3DPEHPK3PXP
  URI:    otpauth://totp/OstoAuth:alice?issuer=OstoAuth&secret=JBSWY3DPEHPK3PXP

Enter the 6-digit code from your authenticator app to confirm: 123456

Success: 2FA has been successfully enabled for your account.
```
*Notice: 2FA is not persisted until a valid code is verified, preventing users from being locked out due to misconfiguration.*

### 5. Log Out & Log In with 2FA
```text
alice> logout
Logged out successfully.

> login
Enter username: alice
Enter password: [masked]
Enter 6-digit 2FA code: 123456

Welcome, alice
Registered:        2026-09-10 17:07:57 UTC
MFA:               enabled
Session expires:   2026-09-10 17:42:15 UTC
Last login:        2026-09-10 17:07:57 UTC
```

### 6. Disable 2FA (`disable-2fa`)
To ensure sessions cannot casually toggle off security protections, disabling 2FA requires password confirmation:
```text
alice> disable-2fa
Enter your current password to confirm disabling 2FA: [masked]
Success: 2FA has been disabled for your account.
```

### 7. Account Lockout Demonstration
If 5 consecutive incorrect passwords or invalid TOTP codes are entered:
```text
> login
Enter username: alice
Enter password: [masked]
Error: account has been locked for 15m0s due to 5 consecutive failed attempts

> login
Enter username: alice
Enter password: [masked]
Error: account is locked due to multiple failed login attempts. Please try again in 14m58s
```
*Notice: Subsequent attempts during the lockout duration fail fast without evaluating credentials or exposing username existence.*

---

## Available Commands

### Unauthenticated State
- `register`: Create a new user account.
- `login`: Authenticate with username and password (+ TOTP if enabled).
- `help`: Display pre-login command list.
- `exit`: Terminate the program.

### Authenticated State
- `whoami`: Display current user profile, registration date, MFA status, and session countdown.
- `enable-2fa`: Begin TOTP 2FA enrollment with QR code and verification.
- `disable-2fa`: Disable TOTP 2FA (requires current password).
- `logout`: End active session and return to unauthenticated prompt.
- `help`: Display post-login command list.
- `exit`: Gracefully log out and exit.

---

## Architectural Decisions & Trade-Offs

### 1. SQLite Storage & Pure-Go Driver (`modernc.org/sqlite`)
- **Single-Process CLI**: A CLI application does not require a networked database engine like PostgreSQL or MySQL. An embedded SQLite database eliminates external service dependencies and network latency.
- **Pure-Go Driver**: Using `modernc.org/sqlite` allows building with `CGO_ENABLED=0`, producing a statically linked binary. This avoids requiring a C compiler (`gcc`/`musl`) in the builder stage and enables minimal, secure runtime containers.
- **Concurrency & Pragmas**: Connections are configured with `PRAGMA journal_mode = WAL;`, `PRAGMA foreign_keys = ON;`, and `PRAGMA busy_timeout = 5000;` to ensure transactional safety and crash resilience.
- **Persistence**: In Docker Compose, the database resides at `/data/app.db` backed by the named volume `app-data`. Destroying or rebuilding the container retains all registered users.

### 2. In-Memory Session Management
- **Deliberate Simplification**: Per specification §3 & §10, sessions are maintained in-memory using a thread-safe map protected by `sync.RWMutex`.
- **Stateless CLI Restarts**: Because this is an interactive CLI session, active sessions naturally terminate when the process exits. Persisting sessions across container restarts is unnecessary and contrary to CLI session semantics.
- **Passive Expiration**: Sessions are checked against `SESSION_TIMEOUT_MINUTES` prior to every post-login command execution. Expired sessions are automatically deleted and force the user to re-authenticate.

### 3. Security Posture
- **Password Hashing**: `golang.org/x/crypto/bcrypt` work factor 12.
- **Timing & Enumeration Resistance**: Login attempts for non-existent users return generic `"invalid username or password"` errors to prevent username enumeration.
- **2FA Key Verification**: 2FA secret generation requires a successful one-time code check before persisting `totp_enabled = 1`.
- **Lockout Reset Timing**: The failure counter increments on either wrong password or wrong TOTP code, and only resets to 0 upon full successful authentication.

---

## Local Development & Testing

A `Makefile` is included for common workflows:

```bash
# Run all unit and integration tests with data race detector
make test

# Run linter and formatting checks
make lint

# Compile local binary to bin/cli
make build

# Run application locally
make run

# Build Docker image
make docker-build

# Run interactive Docker CLI
make docker-run

# Clean build artifacts and volumes
make clean
```

---

## Known Limitations & Non-Goals

- **Single-Process Architecture**: In-memory session tracking is designed for single-process CLI usage and is not intended for distributed multi-instance deployments.
- **No Password Reset Flow**: By design (§10), there is no email/SMS recovery flow or administrative password reset.
- **Single-Role Access**: The system implements user authentication without multi-role or RBAC permission trees.
