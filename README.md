# lib

A small set of focused, dependency-light Go libraries.

Single Go module (`github.com/AltSoyuz/lib`); each top-level directory is one
importable package.

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
| [`auth`](./auth) | Authentication primitives: passwords (bcrypt + HIBP), sessions, magic links. Pure, no DB. |
| [`buildinfo`](./buildinfo) | Build version / commit injection via `-ldflags`. |
| [`claude`](./claude) | Anthropic Claude API client. |
| [`db`](./db) | `database/sql` helpers: migrations, query tracing, transactions. |
| [`envflag`](./envflag) | `flag` package wrapper that also reads from environment variables. |
| [`httpserver`](./httpserver) | HTTP server utilities: graceful shutdown, IP extraction, rate limiting, embedded UI helpers. |
| [`logger`](./logger) | `log/slog` configuration with sane defaults. |
| [`mailer`](./mailer) | Email sender (AWS SES v2). |
| [`oauthgoogle`](./oauthgoogle) | Google OAuth2 client. |
| [`telegram`](./telegram) | Telegram bot API client. |

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

## Contributing

See [`CONTRIBUTING.md`](./CONTRIBUTING.md) and [`AGENTS.md`](./AGENTS.md).

## License

[MIT](./LICENSE)

