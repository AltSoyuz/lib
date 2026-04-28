package oauthgoogle

import (
	"context"
	"errors"
	"net/http"
)

// UserInfo describes the authenticated Google account.
type UserInfo struct {
	Provider      string
	Subject       string
	Email         string
	VerifiedEmail bool
}

// Provider starts and completes Google OAuth flows.
type Provider struct{}

var current = &Provider{}

// ErrNotAvailable is returned while the provider implementation is disabled.
var ErrNotAvailable = errors.New("oauth_google_not_available")

// Current returns the process-wide Google OAuth provider.
func Current() *Provider { return current }

// Available reports whether the Google OAuth provider is configured.
func (p *Provider) Available() bool { return false }

// Start writes the OAuth redirect response.
func (p *Provider) Start(w http.ResponseWriter, state, verifier string) {
	// TODO: implement OAuth redirect
}

// Callback exchanges an OAuth authorization code for user information.
func (p *Provider) Callback(ctx context.Context, code, verifier string) (*UserInfo, error) {
	// TODO: implement OAuth token exchange
	return nil, ErrNotAvailable
}
