package magiclinks

import (
	"testing"
	"time"
)

func TestConstantsNonZero(t *testing.T) {
	if VerifyEmailTTL <= 0 {
		t.Fatalf("VerifyEmailTTL must be positive")
	}
	if ResetPasswordTTL <= 0 {
		t.Fatalf("ResetPasswordTTL must be positive")
	}
	if VerifyEmailTTL <= time.Minute {
		t.Fatalf("VerifyEmailTTL suspiciously short: %v", VerifyEmailTTL)
	}
	if PurposeVerifyEmail == PurposeResetPassword {
		t.Fatalf("purpose constants must be distinct")
	}
}
