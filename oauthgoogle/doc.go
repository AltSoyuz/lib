// Package oauthgoogle implements Google Sign-In via the OAuth 2.0
// authorization-code flow with PKCE (RFC 7636), using only the standard
// library.
//
// Configure the process-wide provider once at startup:
//
//	err := oauthgoogle.Configure(oauthgoogle.Config{
//		ClientID:     clientID,
//		ClientSecret: clientSecret,
//		RedirectURL:  "https://example.com/auth/google/callback",
//	})
//
// On the login route, generate a state and PKCE verifier, persist both in
// transient cookies (see auth/sessions.SetTransientCookie), then redirect:
//
//	state, _ := oauthgoogle.NewState()
//	verifier, _ := oauthgoogle.NewVerifier()
//	// set transient cookies for state and verifier ...
//	oauthgoogle.Current().Start(w, state, verifier)
//
// On the callback route, compare the state query parameter against the
// cookie, then exchange the code:
//
//	info, err := oauthgoogle.Current().Callback(ctx, code, verifier)
//
// The returned UserInfo carries the stable Google subject and the account
// email, ready to upsert into the auth/schema oauth_accounts table.
//
// The ID token signature is not verified: the token is received directly
// from Google's token endpoint over TLS, so per OpenID Connect Core
// §3.1.3.7 the TLS server validation suffices. Issuer, audience and expiry
// claims are validated.
package oauthgoogle
