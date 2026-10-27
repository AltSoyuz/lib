# AGENTS.md

Short guide for AI coding assistants working on this repository.

## Scope

This is a public Go library module: `github.com/AltSoyuz/lib`, plus `web/`, the UI kit the
SvelteKit apps copy from the module (see `web/README.md`).

- Keep it libraries-only: no `cmd/`, no app wiring, no product-specific code.
- Supplier regime: something enters lib at its second real use in an app, and a break ships with its migration (below).
- Keep packages small, explicit, and dependency-light.
- Prefer godoc over package README files. The root README is the landing page.

## How lib evolves

lib's consumers are a few apps with one owner, edited by the same agents. That allows what a public framework cannot
do, and asks for what it never needed.

- **The app owns the wiring.** The app calls lib; inversion of control stops at a `Mount(mux)` and a `Config`, as in
  `backoffice`. No hooks, no behavior triggered by a name.
- **A break ships with its migration.** An API change that breaks a consumer lands with the migration of every app
  that follows lib (compass, exploreassistant), in the same pass, on clean trees. No deprecation layer: the API keeps
  no trace of its past mistakes. Tags stay, so each app deploys at its own pace; an app pinned to an older version
  (homeapp) keeps it until someone works on it.
- **Few releases.** What still moves (the kit's layout under iOS) is tuned in one app and enters lib once validated
  on an iPhone: one release per settled change, not one per tweak. Each release costs a bump, `make kit` and the
  checks in every app.
- **Conventions are checks.** A rule the apps must follow fails a check with what to do (`make kit-check`), rather
  than living in prose an agent may not read.
- **Properties, not code, from mature frameworks.** For security-sensitive code (auth, sessions, CSRF, links sent by
  email), list what the closest reference guarantees (`phx.gen.auth` for auth), build it from the standard library
  or `golang.org/x/crypto`, and make each property a test.
- **The godoc is the contract.** Agents in the apps read lib one symbol at a time
  (`go doc github.com/AltSoyuz/lib/<pkg> <Symbol>`), never the whole module, and the kit through `web/README.md`.
  A package's first sentence says what it owns and what it calls in the app.

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
| `provision` | VM provisioning model, templates, renderers, and validation. |
| `telegram` | Telegram Bot API client. |
| `web/` | UI kit (CSS, Svelte, TS) copied by the apps, not a Go package. |

## Release notes

Move `CHANGELOG.md` entries from `[Unreleased]` to the target version, then tag:

```sh
git tag -s vX.Y.Z -m "vX.Y.Z"
git push origin main vX.Y.Z
```
