// Package session signs the customer in on the web (OAuth 2.0 Device Authorization Grant against Keycloak),
// keeps the tokens on disk, refreshes them and revokes them on logout.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var ErrNotLoggedIn = errors.New("not logged in")

// Session is what one `auth login` leaves on disk. Api and Issuer record where it belongs, so a token issued
// for one environment is never sent to another.
type Session struct {
	Api             string    `json:"api"`
	Issuer          string    `json:"issuer"`
	Username        string    `json:"username"`
	AccessToken     string    `json:"accessToken"`
	AccessExpiresAt time.Time `json:"accessExpiresAt"`
	RefreshToken    string    `json:"refreshToken"`
}

// Store keeps the session in <Dir>/session.json, readable by the owner only: the refresh token is a
// 30-day credential.
type Store struct {
	Dir string
}

func (s Store) path() string {
	return filepath.Join(s.Dir, "session.json")
}

func (s Store) Load() (Session, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, ErrNotLoggedIn
	}
	if err != nil {
		return Session{}, err
	}
	var session Session
	err = json.Unmarshal(data, &session)
	if err != nil {
		return Session{}, fmt.Errorf("read %s: %w", s.path(), err)
	}
	return session, nil
}

func (s Store) Save(session Session) error {
	err := os.MkdirAll(s.Dir, 0o700)
	if err != nil {
		return err
	}
	// MkdirAll leaves an existing directory's mode alone.
	err = os.Chmod(s.Dir, 0o700)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	// Write a new 0600 file and rename it over the old one: a file that already existed with a looser mode
	// is replaced rather than rewritten in place, and a crash never leaves half a session.
	temp, err := os.CreateTemp(s.Dir, "session-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	_, writeErr := temp.Write(data)
	closeErr := temp.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(tempPath)
		return errors.Join(writeErr, closeErr)
	}
	err = os.Chmod(tempPath, 0o600)
	if err != nil {
		os.Remove(tempPath)
		return err
	}
	return os.Rename(tempPath, s.path())
}

func (s Store) Delete() error {
	err := os.Remove(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotLoggedIn
	}
	return err
}

// DefaultDir is $XDG_CONFIG_HOME/openvicloud, else ~/.config/openvicloud on every OS.
func DefaultDir(getenv func(string) string) (string, error) {
	xdg := getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		return filepath.Join(xdg, "openvicloud"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "openvicloud"), nil
}
