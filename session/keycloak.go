package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClientId is the public Keycloak client of the CLI (gitops/keycloak/realm-import/configmap.yaml).
const ClientId = "openvicloud-cli"

// offline_access makes the refresh token an offline token: it outlives the realm's 30-minute SSO idle
// timeout, so a login lasts about 30 days of not using the CLI.
const loginScope = "openid offline_access"

var (
	ErrLoginDenied    = errors.New("the login was refused on the web page")
	ErrLoginExpired   = errors.New("the login code expired before it was confirmed on the web page")
	ErrSessionExpired = errors.New("the session has expired or was revoked")
	ErrRevokeFailed   = errors.New("the server did not confirm the session was revoked")
)

// DeviceCode is what Keycloak hands out to start a login: the user opens VerificationUriComplete (or
// VerificationUri and types UserCode) while the CLI polls with DeviceCode.
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationUri         string `json:"verification_uri"`
	VerificationUriComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// Keycloak talks to the OpenID Connect endpoints of one realm. Sleep and Now are injected so tests do not wait.
type Keycloak struct {
	Issuer string
	HTTP   *http.Client
	Sleep  func(time.Duration)
	Now    func() time.Time
}

func (k Keycloak) startDeviceLogin(ctx context.Context) (DeviceCode, error) {
	form := url.Values{}
	form.Set("client_id", ClientId)
	form.Set("scope", loginScope)
	status, body, err := k.postForm(ctx, "/protocol/openid-connect/auth/device", form)
	if err != nil {
		return DeviceCode{}, err
	}
	if status != http.StatusOK {
		return DeviceCode{}, describeOAuthFailure("start the login", status, body)
	}
	var code DeviceCode
	err = json.Unmarshal(body, &code)
	if err != nil {
		return DeviceCode{}, fmt.Errorf("start the login: %w", err)
	}
	return code, nil
}

// waitForTokens polls the token endpoint until the user confirms or refuses on the web page, or the code
// expires (RFC 8628 §3.5).
func (k Keycloak) waitForTokens(ctx context.Context, code DeviceCode) (tokens, error) {
	interval := time.Duration(code.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := k.Now().Add(time.Duration(code.ExpiresIn) * time.Second)
	form := url.Values{}
	form.Set("client_id", ClientId)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("device_code", code.DeviceCode)

	for {
		k.Sleep(interval)
		if ctx.Err() != nil {
			return tokens{}, ctx.Err()
		}
		codeStillValid := k.Now().Before(deadline)
		if !codeStillValid {
			return tokens{}, ErrLoginExpired
		}
		status, body, err := k.postForm(ctx, "/protocol/openid-connect/token", form)
		if err != nil {
			return tokens{}, err
		}
		if status == http.StatusOK {
			return parseTokens(body)
		}
		answer := parseOAuthError(body)
		switch answer.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval = interval + 5*time.Second
			continue
		case "access_denied":
			return tokens{}, ErrLoginDenied
		case "expired_token":
			return tokens{}, ErrLoginExpired
		default:
			return tokens{}, describeOAuthFailure("finish the login", status, body)
		}
	}
}

func (k Keycloak) refresh(ctx context.Context, refreshToken string) (tokens, error) {
	form := url.Values{}
	form.Set("client_id", ClientId)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	status, body, err := k.postForm(ctx, "/protocol/openid-connect/token", form)
	if err != nil {
		return tokens{}, err
	}
	if status == http.StatusOK {
		return parseTokens(body)
	}
	answer := parseOAuthError(body)
	if answer.Error == "invalid_grant" {
		return tokens{}, ErrSessionExpired
	}
	return tokens{}, describeOAuthFailure("refresh the session", status, body)
}

func (k Keycloak) revoke(ctx context.Context, refreshToken string) error {
	form := url.Values{}
	form.Set("client_id", ClientId)
	form.Set("token", refreshToken)
	form.Set("token_type_hint", "refresh_token")
	status, body, err := k.postForm(ctx, "/protocol/openid-connect/revoke", form)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRevokeFailed, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: %v", ErrRevokeFailed, describeOAuthFailure("revoke the session", status, body))
	}
	return nil
}

func (k Keycloak) postForm(ctx context.Context, path string, form url.Values) (int, []byte, error) {
	endpoint := strings.TrimRight(k.Issuer, "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := k.HTTP.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("cannot reach the login server %s: %w", k.Issuer, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, body, nil
}

func parseTokens(body []byte) (tokens, error) {
	var parsed tokens
	err := json.Unmarshal(body, &parsed)
	if err != nil {
		return tokens{}, fmt.Errorf("read the tokens: %w", err)
	}
	if parsed.AccessToken == "" || parsed.RefreshToken == "" {
		return tokens{}, errors.New("the login server answered without an access token and a refresh token")
	}
	return parsed, nil
}

func parseOAuthError(body []byte) oauthError {
	var answer oauthError
	json.Unmarshal(body, &answer)
	return answer
}

func describeOAuthFailure(action string, status int, body []byte) error {
	answer := parseOAuthError(body)
	if answer.Error == "" {
		return fmt.Errorf("could not %s: the login server answered HTTP %d", action, status)
	}
	if answer.Description == "" {
		return fmt.Errorf("could not %s: %s", action, answer.Error)
	}
	return fmt.Errorf("could not %s: %s (%s)", action, answer.Error, answer.Description)
}
