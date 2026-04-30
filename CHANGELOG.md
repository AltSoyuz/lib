# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
