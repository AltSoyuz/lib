// Package auth documents the authentication-related subpackages and SQL
// artefacts in this module.
//
// The package itself intentionally contains no runtime helpers. Use the
// focused subpackages directly:
//
//   - passwords for bcrypt hashing, verification, email normalization, and
//     HIBP-compatible password strength checks.
//   - sessions for random session tokens, token hashing, secure cookies, and
//     request base URL helpers.
//   - magiclinks for shared TTL and purpose constants used by one-shot
//     email-token flows.
//
// The schema directory contains copyable SQL migrations and queries for
// consumers that want the shared authentication schema.
package auth
