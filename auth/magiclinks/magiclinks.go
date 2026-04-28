// Package magiclinks holds shared semantics for one-shot email tokens used by
// the verify-email and password-reset flows: TTL defaults and stable purpose
// strings used as DB enums. No DB dependency.
package magiclinks

import "time"

const (
	// VerifyEmailTTL is the default lifetime for verify-email magic links.
	VerifyEmailTTL = 24 * time.Hour

	// ResetPasswordTTL is the default lifetime for reset-password magic links.
	ResetPasswordTTL = 15 * time.Minute

	// PurposeVerifyEmail is the stable purpose string for verify-email tokens.
	PurposeVerifyEmail = "verify_email"

	// PurposeResetPassword is the stable purpose string for reset-password tokens.
	PurposeResetPassword = "reset_password"
)
