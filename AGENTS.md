# AGENTS.md

Short guide for AI coding assistants working on this repository.

## Scope

This is a public Go library module: `github.com/AltSoyuz/lib`.

- Keep it libraries-only: no `cmd/`, no app wiring, no product-specific code.
- Keep packages small, explicit, and dependency-light.
- Prefer godoc over package README files. The root README is the landing page.

## Commands

```sh
go test -race -count=1 ./...
go vet ./...
staticcheck ./...   # if installed
go build ./...
```

## Rules

- Single module only. Do not add nested `go.mod` files.
- New behavior needs tests. Bug fixes need regression tests.
- New dependencies need a clear justification; prefer the standard library.
- Public API changes need a `CHANGELOG.md` entry under `## [Unreleased]`.
- All exported identifiers need godoc starting with the identifier name.
- I/O functions should accept `context.Context` as their first argument.
- Return errors for recoverable failures; do not panic in library code.
- Keep package docs in `doc.go` or package comments, not package READMEs.

## Package map

| Package | Purpose |
|---|---|
| `auth` | Auth subpackages and SQL artefacts. |
| `buildinfo` | Version flag and build metric. |
| `claude` | Anthropic Messages API client. |
| `db` | `database/sql` helpers, migrations, tracing. |
| `envflag` | Flags with env fallback. |
| `httpserver` | HTTP serving, JSON, IP, rate limit, UI helpers. |
| `logger` | Small structured logger. |
| `mailer` | Blackhole, SMTP, AWS SES v2, Resend email. |
| `oauthgoogle` | Placeholder Google OAuth provider API. |
| `telegram` | Telegram Bot API client. |

## Release notes

Move `CHANGELOG.md` entries from `[Unreleased]` to the target version, then tag:

```sh
git tag -s vX.Y.Z -m "vX.Y.Z"
git push origin main vX.Y.Z
```
