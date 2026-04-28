# AGENTS.md

Tool-agnostic guide for AI coding assistants (Claude Code, GitHub Copilot,
Cursor, Aider, …) working on this repository.

## Repository

A small set of focused Go libraries packaged as a single module
(`github.com/AltSoyuz/lib`). Each top-level directory is one importable
package. No `cmd/`, no application code.

## Layout

```
.
├── auth/         # passwords, sessions, magiclinks, schema (no DB code)
├── buildinfo/    # -ldflags Version/BuildInfo injection
├── claude/       # Anthropic Claude API client
├── db/           # database/sql helpers (migrations, tracing)
├── envflag/      # flag package + env fallback
├── httpserver/   # graceful shutdown, IP, rate limit, embedded UI helpers
├── logger/       # log/slog configuration
├── mailer/       # AWS SES v2 email sender
├── oauthgoogle/  # Google OAuth2 client
└── telegram/     # Telegram bot API client
```

## Quick commands

```sh
go test -race -count=1 ./...
go vet ./...
staticcheck ./...           # if installed
go build ./...
```

CI runs all of the above plus a gitleaks scan.

## Non-negotiables

- **Single module.** Do not add nested `go.mod` files.
- **No application code.** This repo is libraries only — no `main` packages,
  no `cmd/`, no flags wired to application behaviour.
- **Minimal external dependencies.** New deps need a justification in the PR
  description. Prefer the standard library.
- **Stable public API.** Renames, signature changes, or removed exports
  require a `CHANGELOG.md` entry under `## [Unreleased]` and trigger a
  minor bump (pre-1.0) or major bump (post-1.0). Post-1.0 majors require a
  `/v2` module path.
- **Tests are mandatory.** New behaviour ships with `*_test.go`. Bug fixes
  ship with a regression test. CI must be green before merge.
- **No frameworks, no magic.** Plain `net/http`, plain `database/sql`,
  `log/slog`. Composition over inheritance.
- **Decoupled packages.** `auth/passwords`, `auth/sessions`, `auth/magiclinks`
  do not import each other. New packages should follow the same discipline.
- **No globals beyond a clear reason.** If you export a package-level mutable
  global, document why.

## Adding a package

1. Create `<pkg>/<pkg>.go` with godoc on every exported identifier.
2. Create `<pkg>/<pkg>_test.go`. Use `t.Parallel()` when the test does not
   touch shared global state.
3. Run `go mod tidy`, `go vet ./...`, `go test -race ./...`.
4. Add the package to the table in `README.md`.
5. Add a `<pkg>/README.md` only if the package needs more than godoc to be
   used correctly (e.g. `auth/`).
6. Add an entry under `## [Unreleased]` in `CHANGELOG.md`.

## Public API discipline

- All exported identifiers carry a godoc starting with the identifier name.
- Functions that do I/O accept `context.Context` as their first argument.
- Errors are wrapped with `%w` when returned across package boundaries.
- No `panic` on a recoverable condition; return an error.
- No package-level `init()` that performs I/O or registers flags.
- Logging uses `log/slog`. Libraries log at `Debug` / `Info` / `Warn` /
  `Error`; they never call `slog.SetDefault` themselves.

## Style

- Pointer receivers when the value is large or mutable; value receivers
  otherwise. Be consistent within a type.
- Return concrete types; accept interfaces.
- Keep zero values useful when feasible.
- Wrap third-party SDKs behind small, focused interfaces only when there is a
  concrete second implementation in sight (e.g. tests). Otherwise call the
  SDK directly.

## Release flow

```sh
# 1. Move CHANGELOG.md entries from [Unreleased] to [vX.Y.Z]
# 2. Commit
git tag -s vX.Y.Z -m "vX.Y.Z"
git push origin main vX.Y.Z
```

The `release.yml` workflow generates GitHub release notes from the tag.

## Per-package guides

| Package | Reference |
|---------|-----------|
| `auth` | [`auth/README.md`](./auth/README.md) |
| Other  | godoc — `go doc github.com/AltSoyuz/lib/<pkg>` |

## What this repo is **not**

- Not a framework.
- Not a curated standard library replacement.
- Not a place to dump utility code that has only one caller — that code
  belongs in the caller.
