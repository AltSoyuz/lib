# auth

Pure utility packages for authentication primitives. No Go code at the root —
only sub-packages and shared SQL artefacts.

**Principle:** `auth/*` packages never touch a database. Tests are unit-only.
Persistence, HTTP handlers, and flow orchestration are the caller's
responsibility.

## Packages

| Package | Role |
|---------|------|
| [`passwords`](./passwords) | bcrypt `Hash` / `Verify`, `NormalizeEmail`, `CheckStrength` (HIBP-compatible). |
| [`sessions`](./sessions) | Random tokens + hashing, cookies, host/proto detection, `RequestBaseURL`. |
| [`magiclinks`](./magiclinks) | TTL and `Purpose` constants for verify-email and reset-password flows. |
| [`schema`](./schema) | Shared SQL artefacts (queries + migrations). No Go. |

Each sub-package is independent: `passwords`, `sessions`, and `magiclinks` do
not import each other.

## Usage

```go
import (
    "github.com/AltSoyuz/lib/auth/passwords"
    "github.com/AltSoyuz/lib/auth/sessions"
)

email, err := passwords.NormalizeEmail("User@Example.COM ")
hash, err := passwords.Hash("correct horse battery staple")
err = passwords.Verify(hash, "correct horse battery staple")

token, hashed, err := sessions.NewToken()
sessions.SetCookie(w, r, token, time.Now().Add(sessions.TTL))
```

## Shared SQL (`schema/`)

The SQL files are **not** embedded via `//go:embed` from this module. They are
templates that consumers copy into their own migrations + queries directories.

```sh
# Copy schema artefacts from the module cache into your project
cp $(go env GOMODCACHE)/github.com/!alt!soyuz/lib@<version>/auth/schema/migrations/*.sql \
   ./internal/store/migrations/
cp $(go env GOMODCACHE)/github.com/!alt!soyuz/lib@<version>/auth/schema/queries/auth.sql \
   ./internal/auth/queries/
```

Example `sqlc` config:

```yaml
- engine: "sqlite"
  queries: "internal/auth/queries"
  schema: "internal/store/migrations"
  gen:
    go:
      package: "dal"
      out: "internal/auth/dal"
```

## Rules

- ❌ No DB drivers, no SQLite, no integration tests in `auth/*`.
- ❌ No coupling between `passwords`, `sessions`, `magiclinks`.
- ❌ No `//go:embed` of `schema/` from this module.
- ✅ Pure logic only (crypto, parsing, normalisation).
- ✅ Composition (HTTP, DB, transactions, flows) lives in the caller.
