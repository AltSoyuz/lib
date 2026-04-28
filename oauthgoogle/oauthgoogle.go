package oauthgoogle

import (
	"context"
	"errors"
	"net/http"
)

type UserInfo struct {
	Provider      string
	Subject       string
	Email         string
	VerifiedEmail bool
}

type Provider struct{}

var current = &Provider{}

var ErrNotAvailable = errors.New("oauth_google_not_available")

func Current() *Provider { return current }

func (p *Provider) Available() bool { return false }

func (p *Provider) Start(w http.ResponseWriter, state, verifier string) {
	// TODO: implement OAuth redirect
}

func (p *Provider) Callback(ctx context.Context, code, verifier string) (*UserInfo, error) {
	// TODO: implement OAuth token exchange
	return nil, ErrNotAvailable
}
