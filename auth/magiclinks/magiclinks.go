// Package magiclinks holds shared semantics for one-shot email tokens used by
// the verify-email and password-reset flows: TTL defaults and stable purpose
// strings used as DB enums. No DB dependency.
package magiclinks

import "time"

const (
	VerifyEmailTTL   = 24 * time.Hour
	ResetPasswordTTL = 15 * time.Minute

	PurposeVerifyEmail   = "verify_email"
	PurposeResetPassword = "reset_password"
)
