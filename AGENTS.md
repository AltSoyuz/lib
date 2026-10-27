# AGENTS.md

Short guide for AI coding assistants working on this repository.

## Scope

This is a public Go library module: `github.com/AltSoyuz/lib`, plus `web/`, the UI kit the
SvelteKit apps copy from the module (see `web/README.md`).

- Keep it libraries-only: no `cmd/`, no app wiring, no product-specific code.
- Supplier regime: something enters lib at its second real use in an app, and a break ships with its migration (below).
- `homeapp` is pinned to an older version and is not migrated with the others until someone works on it.
- Keep packages small, explicit, and dependency-light.
- Prefer godoc over package README files. The root README is the landing page.

## How lib evolves

The rules and the five steps from a lib change to the apps are in [`README.md`](./README.md) (Philosophy, How an
app uses lib): read them before changing an exported API or the kit. Agents in the apps read lib one symbol at a
time (`go doc github.com/AltSoyuz/lib/<pkg> <Symbol>`), never the whole module, and the kit through
`web/README.md`.

## Commands

```sh
go test -race -count=1 ./...
go vet ./...
staticcheck ./...   # if installed
go build ./...
node --test web/*.test.ts
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
| `audit` | Audit log: write in the change's transaction, list for a back office. |
| `auth` | Auth subpackages and SQL artefacts. |
| `backoffice` | Accounts, access, session check, admin routes, View as, announcement, account stats. |
| `buildinfo` | Version flag and build metric. |
| `claude` | Anthropic Messages API client. |
| `db` | `database/sql` helpers, migrations, tracing. |
| `envflag` | Flags with env fallback. |
| `httpserver` | HTTP serving, JSON, IP, rate limit, UI helpers. |
| `logger` | Small structured logger. |
| `mailer` | Blackhole, SMTP, AWS SES v2, Resend email. |
| `oauthgoogle` | Google Sign-In OAuth provider (code flow + PKCE). |
| `telegram` | Telegram Bot API client. |
| `web/` | UI kit (CSS, Svelte, TS) copied by the apps, not a Go package. |

## Release notes

Put the `[Unreleased]` entries under a new `## [X.Y.Z]` heading, commit it as `release: vX.Y.Z`, then tag that
commit with a lightweight tag, like every lib tag so far:

```sh
git tag vX.Y.Z
git push origin main vX.Y.Z
```
