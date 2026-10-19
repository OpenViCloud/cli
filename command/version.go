package command

import (
	"github.com/spf13/cobra"

	"github.com/OpenViCloud/cli/output"
)

type versionInfo struct {
	Version string `json:"version"`
	Api     string `json:"api"`
	Auth    string `json:"auth"`
}

// NewVersion prints the build and the endpoints it targets: the two things support asks first.
func NewVersion(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the CLI version and the endpoints it uses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := versionInfo{Version: deps.Version, Api: deps.Endpoints.Api, Auth: deps.Endpoints.Auth}
			fields := []output.Field{
				{Name: "Version", Value: info.Version},
				{Name: "API", Value: info.Api},
				{Name: "Login server", Value: info.Auth},
			}
			return deps.Printer.Object(fields, info)
		},
	}
}
