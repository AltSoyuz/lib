# lib

The shared base of a few web apps with one owner (compass, exploreassistant): Go packages, a UI kit each app copies,
and a back office with its Go and UI sides.

This repository is a single Go module (`github.com/AltSoyuz/lib`). Each top-level directory is an importable
package unless documented otherwise.

## Philosophy

lib's consumers are a few apps with one owner, edited by the same agents. That allows what a public framework
cannot do, and asks for what it never needed.

- **The app owns the wiring.** An app calls lib from its own code; inversion of control stops at a `Mount(mux)` and
  a `Config`, as in `backoffice`. Plain `net/http`, `database/sql`, `log/slog`; no hooks, no behavior triggered by
  a name.
- **Something enters lib at its second real use** in an app. Until then it stays in the app.
- **A break ships with its migration.** A change that breaks an app lands with the migration of every app that
  follows lib, in the same pass, on clean trees. No deprecation layer: the API keeps no trace of its past mistakes.
  Tags stay, so each app deploys at its own pace, and an app pinned to an older version keeps it until someone
  works on it.
- **Few releases.** What still moves (the kit's layout under iOS) is tuned in one app and enters lib once validated
  on an iPhone: one release per settled change, not one per tweak. Each release costs a bump, `make kit` and the
  checks in every app.
- **Conventions are checks.** A rule the apps must follow fails a check with what to do (`make kit-check`), rather
  than living in prose nobody reads.
- **Properties, not code, from mature frameworks.** Security-sensitive code (auth, sessions, CSRF, links sent by
  email) lists what the closest reference guarantees (`phx.gen.auth` for auth), builds it from the standard library
  or `golang.org/x/crypto`, and makes each property a test.
- **Small, tested, few dependencies.** Each package fits in your head; its godoc is the contract, and its first
  sentence says what it owns and what it calls in the app. Every package ships with `*_test.go`; CI runs
  `go test -race ./...` and `go vet ./...`. A new dependency needs a justification.

## How an app uses lib

An app pins a lib tag in its `go.mod` (never a committed `replace`) and calls the Go packages from its own code.
The UI kit, `web/`, is copied into the app by `make kit` from that same tag, so `go.sum` checks the copy, and
`make kit-check`, part of `make check`, refuses a hand-edited one (see [`web/README.md`](./web/README.md)). The back
office has both sides: the `backoffice` package mounts its routes on the app's mux, and `web/admin/` holds its
screens.

A change to lib reaches the apps in five steps:

1. Change lib with its tests and a `CHANGELOG.md` entry under `[Unreleased]`. Try it in the apps through a
   `go.work` kept outside the repositories.
2. Release: the entries move under `## [X.Y.Z]` in a `release: vX.Y.Z` commit, tagged `vX.Y.Z` and pushed with
   `main`.
3. In each app that follows lib: `go get github.com/AltSoyuz/lib@vX.Y.Z`, then `make kit`.
4. Migrate the app if the release breaks it, then `make check`.
5. Ship each app through its own release process.

## Packages

| Package | Purpose |
|---------|---------|
| [`audit`](./audit) | Audit log of an app: who changed what, written in the transaction of the change, paged for a back office. |
| [`auth`](./auth) | Authentication subpackages and SQL artefacts: passwords, sessions, magic links, schema. |
| [`backoffice`](./backoffice) | The back office the apps share: roles and access, the session check, admin routes, View as, announcement, account stats. |
| [`buildinfo`](./buildinfo) | Version flag handling and build info metric registration through `-ldflags`. |
| [`claude`](./claude) | Minimal Anthropic Messages API client with normal and streaming completions. |
| [`db`](./db) | `database/sql` helpers: null conversion, SQLite uniqueness checks, migrations, transactions, and tracing. |
| [`envflag`](./envflag) | `flag` parsing with environment-variable fallback and optional env prefix. |
| [`httpserver`](./httpserver) | HTTP helpers for serving, graceful shutdown, JSON, IP detection, rate limiting, and embedded UI assets. |
| [`logger`](./logger) | Small structured logger with flag-based level/output/timezone settings and stdlib bridge. |
| [`mailer`](./mailer) | Email delivery through blackhole, SMTP, AWS SES v2, or Resend, plus verification URL helper. |
| [`oauthgoogle`](./oauthgoogle) | Google Sign-In through the OAuth 2.0 authorization-code flow with PKCE, standard library only. |
| [`telegram`](./telegram) | Telegram Bot API client for allowed-user checks, sending messages, polling updates, and command parsing. |
| [`web`](./web) | UI kit for the SvelteKit apps, copied from the module into each app (not a Go package). |

## Install

```sh
go get github.com/AltSoyuz/lib@latest
```

```go
import (
    "github.com/AltSoyuz/lib/httpserver"
    "github.com/AltSoyuz/lib/logger"
)
```

## Versioning

Standard Go module semver. Tags are `vMAJOR.MINOR.PATCH`. While the module is `v0.x.y` the API may change between
minor versions; breaking changes are called out in `CHANGELOG.md` and ship with the migration of the apps.

## Documentation

Read lib through godoc, one package or one symbol at a time:

```sh
go doc github.com/AltSoyuz/lib/mailer
go doc github.com/AltSoyuz/lib/backoffice Config
```

## Contributing

See [`CONTRIBUTING.md`](./CONTRIBUTING.md) and [`AGENTS.md`](./AGENTS.md).

## License

[MIT](./LICENSE)
