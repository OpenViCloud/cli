// Package auth is `openvicloud auth login|logout|status`.
package auth

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/OpenViCloud/titan-cli/command"
	"github.com/OpenViCloud/titan-cli/output"
	"github.com/OpenViCloud/titan-cli/session"
)

func New(deps *command.Deps) *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Log in, log out and see who you are logged in as",
	}
	auth.AddCommand(newLogin(deps), newLogout(deps), newStatus(deps))
	return auth
}

func newLogin(deps *command.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in on the web; the CLI waits until you confirm",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			show := func(code session.DeviceCode) {
				deps.Printer.Message("To log in, open this page and check that it shows the code %s:", code.UserCode)
				deps.Printer.Message("  %s", code.VerificationUriComplete)
				// The link is printed first, so a machine without a browser (an SSH session) still logs in.
				deps.OpenBrowser(code.VerificationUriComplete)
				deps.Printer.Message("Waiting for you to confirm in the browser...")
			}
			saved, err := deps.Sessions.Login(cmd.Context(), show)
			if errors.Is(err, session.ErrLoginDenied) {
				return fmt.Errorf("the login was refused in the browser; nothing was saved")
			}
			if errors.Is(err, session.ErrLoginExpired) {
				return fmt.Errorf("the login code expired before it was confirmed; run `openvicloud auth login` again")
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(deps.Printer.Out, "Logged in to %s as %s\n", saved.Api, saved.Username)
			deps.TermsReminder(cmd.Context())
			return nil
		},
	}
}

func newLogout(deps *command.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the session and forget it on this computer",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := deps.Sessions.Logout(cmd.Context())
			if errors.Is(err, session.ErrNotLoggedIn) {
				fmt.Fprintln(deps.Printer.Out, "You were not logged in.")
				return nil
			}
			if errors.Is(err, session.ErrRevokeFailed) {
				return fmt.Errorf("logged out on this computer, but the server did not confirm the session was revoked, so it may still be valid there until it expires: %w", err)
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(deps.Printer.Out, "Logged out.")
			return nil
		},
	}
}

type statusInfo struct {
	Username string `json:"username"`
	Api      string `json:"api"`
	Auth     string `json:"auth"`
}

func newStatus(deps *command.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show who you are logged in as, and where",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			current, err := deps.Sessions.Current()
			if errors.Is(err, session.ErrNotLoggedIn) {
				return fmt.Errorf("you are not logged in to %s: run `openvicloud auth login`", deps.Endpoints.Api)
			}
			if err != nil {
				return err
			}
			belongsHere := current.Api == deps.Endpoints.Api && current.Issuer == deps.Endpoints.Auth
			if !belongsHere {
				return fmt.Errorf("you are logged in to %s, not %s: run `openvicloud auth login`", current.Api, deps.Endpoints.Api)
			}
			info := statusInfo{Username: current.Username, Api: current.Api, Auth: current.Issuer}
			fields := []output.Field{
				{Name: "User", Value: info.Username},
				{Name: "API", Value: info.Api},
				{Name: "Login server", Value: info.Auth},
			}
			return deps.Printer.Object(fields, info)
		},
	}
}
