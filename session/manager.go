package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrOtherEndpoint = errors.New("the saved session belongs to another endpoint")

// Endpoints are where the CLI talks to: the /v1 API, the Keycloak realm (issuer URL) and the customer portal,
// where a customer accepts the terms.
type Endpoints struct {
	Api    string
	Auth   string
	Portal string
}

// refreshMargin: a token this close to expiry is refreshed before use, so it does not expire on the way.
const refreshMargin = 30 * time.Second

type Manager struct {
	Endpoints Endpoints
	Store     Store
	Keycloak  Keycloak
}

// Login runs the Device Authorization Grant: show tells the user where to confirm, then Login waits for
// the confirmation and saves the session. Nothing is saved when the login fails.
func (m *Manager) Login(ctx context.Context, show func(DeviceCode)) (Session, error) {
	code, err := m.Keycloak.startDeviceLogin(ctx)
	if err != nil {
		return Session{}, err
	}
	show(code)
	issued, err := m.Keycloak.waitForTokens(ctx, code)
	if err != nil {
		return Session{}, err
	}
	session := Session{
		Api:             m.Endpoints.Api,
		Issuer:          m.Endpoints.Auth,
		Username:        usernameOf(issued.AccessToken),
		AccessToken:     issued.AccessToken,
		AccessExpiresAt: m.Keycloak.Now().Add(time.Duration(issued.ExpiresIn) * time.Second),
		RefreshToken:    issued.RefreshToken,
	}
	err = m.Store.Save(session)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

// AccessToken returns a token valid for the configured API, refreshing it first when it is about to expire.
func (m *Manager) AccessToken(ctx context.Context) (string, error) {
	session, err := m.Store.Load()
	if err != nil {
		return "", err
	}
	belongsHere := session.Api == m.Endpoints.Api && session.Issuer == m.Endpoints.Auth
	if !belongsHere {
		return "", ErrOtherEndpoint
	}
	stillFresh := m.Keycloak.Now().Add(refreshMargin).Before(session.AccessExpiresAt)
	if stillFresh {
		return session.AccessToken, nil
	}
	refreshed, err := m.Keycloak.refresh(ctx, session.RefreshToken)
	if err != nil {
		return "", err
	}
	session.AccessToken = refreshed.AccessToken
	session.RefreshToken = refreshed.RefreshToken
	session.AccessExpiresAt = m.Keycloak.Now().Add(time.Duration(refreshed.ExpiresIn) * time.Second)
	err = m.Store.Save(session)
	if err != nil {
		return "", err
	}
	return session.AccessToken, nil
}

// Logout revokes the session at Keycloak, then deletes it from disk. The file goes even when the revoke
// fails, and the error says so: the refresh token may still be valid on the server until it expires.
func (m *Manager) Logout(ctx context.Context) error {
	session, err := m.Store.Load()
	if err != nil {
		return err
	}
	revokeErr := m.Keycloak.revoke(ctx, session.RefreshToken)
	deleteErr := m.Store.Delete()
	if deleteErr != nil && !errors.Is(deleteErr, ErrNotLoggedIn) {
		return errors.Join(revokeErr, deleteErr)
	}
	return revokeErr
}

// Current is the saved session as it is on disk, whichever endpoint it belongs to.
func (m *Manager) Current() (Session, error) {
	return m.Store.Load()
}

// usernameOf reads the display name from the access token. The token is not verified here: the name is only
// shown to the user, and the API verifies the token on every request.
func usernameOf(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
	}
	err = json.Unmarshal(payload, &claims)
	if err != nil {
		return ""
	}
	if claims.PreferredUsername != "" {
		return claims.PreferredUsername
	}
	return claims.Email
}
