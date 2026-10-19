package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeKeycloak answers the three OpenID Connect routes the CLI uses. Each test scripts the answers.
type fakeKeycloak struct {
	server         *httptest.Server
	deviceAnswers  []string // one OAuth error code per poll, "" means "issue the tokens"
	refreshRefused bool
	revokeFails    bool
	polls          int
	refreshes      int
	revoked        []string
	scopeAsked     string
	clientIdAsked  string
}

func newFakeKeycloak(t *testing.T) *fakeKeycloak {
	fake := &fakeKeycloak{}
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/titan/protocol/openid-connect/auth/device", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		fake.scopeAsked = r.Form.Get("scope")
		fake.clientIdAsked = r.Form.Get("client_id")
		writeJSON(w, http.StatusOK, map[string]any{
			"device_code":               "device-123",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          "https://auth.example/device",
			"verification_uri_complete": "https://auth.example/device?user_code=ABCD-EFGH",
			"expires_in":                600,
			"interval":                  5,
		})
	})
	mux.HandleFunc("/realms/titan/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		grant := r.Form.Get("grant_type")
		if grant == "urn:ietf:params:oauth:grant-type:device_code" {
			answer := ""
			if fake.polls < len(fake.deviceAnswers) {
				answer = fake.deviceAnswers[fake.polls]
			}
			fake.polls++
			if answer != "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": answer})
				return
			}
			writeJSON(w, http.StatusOK, tokenResponse("alice", "refresh-1"))
			return
		}
		if grant == "refresh_token" {
			fake.refreshes++
			if fake.refreshRefused {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
				return
			}
			writeJSON(w, http.StatusOK, tokenResponse("alice", "refresh-2"))
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
	})
	mux.HandleFunc("/realms/titan/protocol/openid-connect/revoke", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if fake.revokeFails {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fake.revoked = append(fake.revoked, r.Form.Get("token"))
		w.WriteHeader(http.StatusOK)
	})
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeKeycloak) issuer() string {
	return f.server.URL + "/realms/titan"
}

func tokenResponse(username string, refreshToken string) map[string]any {
	return map[string]any{
		"access_token":  unsignedJWT(map[string]any{"preferred_username": username}),
		"refresh_token": refreshToken,
		"expires_in":    300,
		"token_type":    "Bearer",
	}
}

func unsignedJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, _ := json.Marshal(claims)
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

type testClock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *testClock) Now() time.Time {
	return c.now
}

func (c *testClock) Sleep(d time.Duration) {
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
}

func newManager(t *testing.T, fake *fakeKeycloak, clock *testClock) *Manager {
	endpoints := Endpoints{Api: "https://api.example", Auth: fake.issuer()}
	return &Manager{
		Endpoints: endpoints,
		Store:     Store{Dir: filepath.Join(t.TempDir(), "openvicloud")},
		Keycloak: Keycloak{
			Issuer: endpoints.Auth,
			HTTP:   fake.server.Client(),
			Sleep:  clock.Sleep,
			Now:    clock.Now,
		},
	}
}

func TestLogin_PollsUntilTheUserApproves_ThenSavesTheSession(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.deviceAnswers = []string{"authorization_pending", "authorization_pending", ""}
	clock := &testClock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	manager := newManager(t, fake, clock)
	var shown DeviceCode

	saved, err := manager.Login(context.Background(), func(code DeviceCode) { shown = code })

	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if shown.UserCode != "ABCD-EFGH" || shown.VerificationUriComplete != "https://auth.example/device?user_code=ABCD-EFGH" {
		t.Fatalf("the user was not shown the code and the page: %+v", shown)
	}
	if fake.polls != 3 {
		t.Fatalf("polls = %d, want 3", fake.polls)
	}
	if len(clock.sleeps) != 3 || clock.sleeps[0] != 5*time.Second {
		t.Fatalf("sleeps = %v, want three waits of the 5s interval Keycloak asked for", clock.sleeps)
	}
	if fake.clientIdAsked != "openvicloud-cli" || fake.scopeAsked != "openid offline_access" {
		t.Fatalf("asked client %q scope %q", fake.clientIdAsked, fake.scopeAsked)
	}
	loaded, err := manager.Store.Load()
	if err != nil {
		t.Fatalf("session not saved: %v", err)
	}
	if loaded.Username != "alice" || saved.Username != "alice" {
		t.Fatalf("username = %q / %q, want alice", loaded.Username, saved.Username)
	}
	if loaded.Api != "https://api.example" || loaded.Issuer != fake.issuer() {
		t.Fatalf("session does not record where it belongs: %+v", loaded)
	}
	if loaded.RefreshToken != "refresh-1" {
		t.Fatalf("refresh token = %q", loaded.RefreshToken)
	}
	if !loaded.AccessExpiresAt.Equal(clock.now.Add(300 * time.Second)) {
		t.Fatalf("access token expiry = %v, want now + expires_in", loaded.AccessExpiresAt)
	}
}

func TestLogin_SlowDown_WaitsFiveSecondsLonger(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.deviceAnswers = []string{"slow_down", ""}
	clock := &testClock{now: time.Now()}
	manager := newManager(t, fake, clock)

	_, err := manager.Login(context.Background(), func(DeviceCode) {})

	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if len(clock.sleeps) != 2 || clock.sleeps[1] != 10*time.Second {
		t.Fatalf("sleeps = %v, want 5s then 10s", clock.sleeps)
	}
}

func TestLogin_UserDenies_FailsAndSavesNothing(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.deviceAnswers = []string{"authorization_pending", "access_denied"}
	manager := newManager(t, fake, &testClock{now: time.Now()})

	_, err := manager.Login(context.Background(), func(DeviceCode) {})

	if !errors.Is(err, ErrLoginDenied) {
		t.Fatalf("err = %v, want ErrLoginDenied", err)
	}
	_, loadErr := manager.Store.Load()
	if !errors.Is(loadErr, ErrNotLoggedIn) {
		t.Fatalf("a denied login left a session behind: %v", loadErr)
	}
}

func TestLogin_CodeExpires_FailsAndSavesNothing(t *testing.T) {
	fake := newFakeKeycloak(t)
	fake.deviceAnswers = []string{"expired_token"}
	manager := newManager(t, fake, &testClock{now: time.Now()})

	_, err := manager.Login(context.Background(), func(DeviceCode) {})

	if !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("err = %v, want ErrLoginExpired", err)
	}
	_, loadErr := manager.Store.Load()
	if !errors.Is(loadErr, ErrNotLoggedIn) {
		t.Fatalf("an expired login left a session behind: %v", loadErr)
	}
}

func TestLogin_KeycloakNeverAnswers_StopsAtTheCodeLifetime(t *testing.T) {
	fake := newFakeKeycloak(t)
	pending := make([]string, 1000)
	for i := range pending {
		pending[i] = "authorization_pending"
	}
	fake.deviceAnswers = pending
	manager := newManager(t, fake, &testClock{now: time.Now()})

	_, err := manager.Login(context.Background(), func(DeviceCode) {})

	if !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("err = %v, want ErrLoginExpired", err)
	}
	if fake.polls > 121 {
		t.Fatalf("polled %d times for a 600s code at 5s interval", fake.polls)
	}
}

func loggedIn(t *testing.T, fake *fakeKeycloak, clock *testClock) *Manager {
	manager := newManager(t, fake, clock)
	_, err := manager.Login(context.Background(), func(DeviceCode) {})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	return manager
}

func TestAccessToken_FreshToken_IsUsedWithoutAskingKeycloak(t *testing.T) {
	fake := newFakeKeycloak(t)
	clock := &testClock{now: time.Now()}
	manager := loggedIn(t, fake, clock)

	token, err := manager.AccessToken(context.Background())

	if err != nil || token == "" {
		t.Fatalf("token %q err %v", token, err)
	}
	if fake.refreshes != 0 {
		t.Fatalf("refreshed a token that was still fresh")
	}
}

func TestAccessToken_TokenAboutToExpire_IsRefreshedAndTheNewPairSaved(t *testing.T) {
	fake := newFakeKeycloak(t)
	clock := &testClock{now: time.Now()}
	manager := loggedIn(t, fake, clock)
	clock.now = clock.now.Add(280 * time.Second)

	_, err := manager.AccessToken(context.Background())

	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if fake.refreshes != 1 {
		t.Fatalf("refreshes = %d, want 1", fake.refreshes)
	}
	loaded, _ := manager.Store.Load()
	if loaded.RefreshToken != "refresh-2" {
		t.Fatalf("the rotated refresh token was not saved: %q", loaded.RefreshToken)
	}
}

func TestAccessToken_RefreshRefused_IsSessionExpired(t *testing.T) {
	fake := newFakeKeycloak(t)
	clock := &testClock{now: time.Now()}
	manager := loggedIn(t, fake, clock)
	fake.refreshRefused = true
	clock.now = clock.now.Add(time.Hour)

	_, err := manager.AccessToken(context.Background())

	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired", err)
	}
}

func TestAccessToken_NoSession_IsNotLoggedIn(t *testing.T) {
	fake := newFakeKeycloak(t)
	manager := newManager(t, fake, &testClock{now: time.Now()})

	_, err := manager.AccessToken(context.Background())

	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
}

func TestAccessToken_SessionOfAnotherEndpoint_IsNotSent(t *testing.T) {
	fake := newFakeKeycloak(t)
	clock := &testClock{now: time.Now()}
	manager := loggedIn(t, fake, clock)
	manager.Endpoints.Api = "https://api.other.example"

	token, err := manager.AccessToken(context.Background())

	if !errors.Is(err, ErrOtherEndpoint) {
		t.Fatalf("err = %v, want ErrOtherEndpoint", err)
	}
	if token != "" {
		t.Fatalf("handed out a token for another endpoint")
	}
}

func TestLogout_RevokesTheRefreshTokenThenDeletesTheFile(t *testing.T) {
	fake := newFakeKeycloak(t)
	manager := loggedIn(t, fake, &testClock{now: time.Now()})

	err := manager.Logout(context.Background())

	if err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if len(fake.revoked) != 1 || fake.revoked[0] != "refresh-1" {
		t.Fatalf("revoked = %v, want the refresh token", fake.revoked)
	}
	_, loadErr := manager.Store.Load()
	if !errors.Is(loadErr, ErrNotLoggedIn) {
		t.Fatalf("session file still there: %v", loadErr)
	}
}

func TestLogout_RevokeFails_StillDeletesTheFileAndSaysSo(t *testing.T) {
	fake := newFakeKeycloak(t)
	manager := loggedIn(t, fake, &testClock{now: time.Now()})
	fake.revokeFails = true

	err := manager.Logout(context.Background())

	if !errors.Is(err, ErrRevokeFailed) {
		t.Fatalf("err = %v, want ErrRevokeFailed", err)
	}
	_, loadErr := manager.Store.Load()
	if !errors.Is(loadErr, ErrNotLoggedIn) {
		t.Fatalf("session file still there: %v", loadErr)
	}
}

func TestLogout_NotLoggedIn_IsNotLoggedIn(t *testing.T) {
	fake := newFakeKeycloak(t)
	manager := newManager(t, fake, &testClock{now: time.Now()})

	err := manager.Logout(context.Background())

	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
}

func TestStore_OnlyTheOwnerCanReadTheSession(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "openvicloud")}

	err := store.Save(Session{Api: "https://api.example", RefreshToken: "secret"})

	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	dirInfo, _ := os.Stat(store.Dir)
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %v, want 0700", dirInfo.Mode().Perm())
	}
	fileInfo, _ := os.Stat(filepath.Join(store.Dir, "session.json"))
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", fileInfo.Mode().Perm())
	}
}

func TestStore_ExistingLooseFile_IsTightenedOnSave(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "openvicloud")}
	os.MkdirAll(store.Dir, 0o755)
	os.WriteFile(filepath.Join(store.Dir, "session.json"), []byte("{}"), 0o644)

	err := store.Save(Session{RefreshToken: "secret"})

	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	fileInfo, _ := os.Stat(filepath.Join(store.Dir, "session.json"))
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", fileInfo.Mode().Perm())
	}
}
