# lib

Small, focused, dependency-light Go libraries.

This repository is a single Go module (`github.com/AltSoyuz/lib`). Each
top-level directory is an importable package unless documented otherwise.

## Philosophy

- **Minimal external dependencies.** Adding a dep requires justification.
- **Stable APIs.** Breaking changes bump the minor version (pre-1.0) or
  require a `/v2` module path (post-1.0). No silent breakage.
- **Tests are mandatory.** Every package ships with `*_test.go`. CI runs
  `go test -race ./...` and `go vet ./...` on every push and PR.
- **No frameworks, no magic.** Plain `net/http`, plain `database/sql`,
  `log/slog`. Composition over inheritance.
- **Read the code.** Each package is small enough to fit in your head.

## Packages

| Package | Purpose |
|---------|---------|
| [`auth`](./auth) | Authentication subpackages and SQL artefacts: passwords, sessions, magic links, schema. |
| [`buildinfo`](./buildinfo) | Version flag handling and build info metric registration through `-ldflags`. |
| [`claude`](./claude) | Minimal Anthropic Messages API client with normal and streaming completions. |
| [`db`](./db) | `database/sql` helpers: null conversion, SQLite uniqueness checks, migrations, transactions, and tracing. |
| [`envflag`](./envflag) | `flag` parsing with environment-variable fallback and optional env prefix. |
| [`httpserver`](./httpserver) | HTTP helpers for serving, graceful shutdown, JSON, IP detection, rate limiting, and embedded UI assets. |
| [`logger`](./logger) | Small structured logger with flag-based level/output/timezone settings and stdlib bridge. |
| [`mailer`](./mailer) | Email delivery through blackhole, SMTP, AWS SES v2, or Resend, plus verification URL helper. |
| [`oauthgoogle`](./oauthgoogle) | Placeholder Google OAuth provider API that currently reports not available. |
| [`telegram`](./telegram) | Telegram Bot API client for allowed-user checks, sending messages, polling updates, and command parsing. |

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

Standard Go module semver. Tags are `vMAJOR.MINOR.PATCH`. While the module is
`v0.x.y` the API may change between minor versions; breaking changes are
called out in `CHANGELOG.md`.

## Documentation

Use godoc for package-level details:

```sh
go doc github.com/AltSoyuz/lib/mailer
go doc github.com/AltSoyuz/lib/auth/passwords
```

## Contributing

See [`CONTRIBUTING.md`](./CONTRIBUTING.md) and [`AGENTS.md`](./AGENTS.md).

## License

[MIT](./LICENSE)
