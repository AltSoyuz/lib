# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0]

### Added

- `web/`: the UI kit shared by the SvelteKit apps (tokens and base CSS, `Shell`, `TabBar`, `TitleBar`,
  `Sheet`, PWA state, theme, API calls, service worker). Apps copy it into their `src/lib/kit/` from the lib
  version in their `go.mod`; see `web/README.md`.

### Changed

- `httpserver`: `ServeSveltekitStaticFiles` answers `/service-worker.js`, `/_app/version.json` and
  `*.webmanifest` with `Cache-Control: no-cache` (and the manifest content type), so an app always sees
  its new version. Apps that set these headers themselves can drop that code.

## [0.1.4]

### Changed

- `httpserver`: `WriteError` no longer logs a request its client canceled (a reload, a page left
  mid-load) as an error. It writes nothing, since nobody reads the answer, and logs an info line,
  `http request canceled by the client`, with the status, method, path and error. A request whose
  context ran out of time is still logged as an error.

## [0.1.3]

### Added

- `oauthgoogle`: working Google Sign-In implementation (authorization-code
  flow with PKCE, stdlib only): `Config`, `New`, `Configure`, `NewState`,
  `NewVerifier`, `ProviderName`; `Start` and `Callback` now perform the real
  redirect and token exchange with ID token claim validation.

### Changed

- `oauthgoogle`: `Start` now returns an `error` (previously no return value).

## [0.1.2]

### Added

- `auth/sessions`: `SessionPolicy`, `NewSessionPolicy`, `NextSessionExpiry` for session TTL and refresh logic.
- `auth/sessions`: `ParseAllowlistCSV` for parsing comma-separated email allowlists.
- `httpserver`: `WriteData` and `Heartbeat` helpers for Server-Sent Events.
- `provision` package added to module (VM provisioning model, templates, renderers, validation).

### Changed

- Replaced package README documentation with godoc-first package comments.
- Corrected package descriptions in repository and AI assistant documentation.

### Fixed

- `httpserver`: fixed data race in `TestHeartbeat` — body read now happens after goroutine stops.

## [0.1.0]

### Added

- Initial release.
- Packages: `auth` (passwords, sessions, magiclinks, schema), `buildinfo`,
  `claude`, `db`, `envflag`, `httpserver`, `logger`, `mailer`, `oauthgoogle`,
  `telegram`.
