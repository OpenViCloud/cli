// Package tenant is `openvicloud tenant get`.
package tenant

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/OpenViCloud/cli/api"
	"github.com/OpenViCloud/cli/command"
	"github.com/OpenViCloud/cli/output"
)

func New(deps *command.Deps) *cobra.Command {
	tenant := &cobra.Command{
		Use:   "tenant",
		Short: "The tenant you belong to",
	}
	tenant.AddCommand(newGet(deps))
	return tenant
}

func newGet(deps *command.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show your tenant: status and quota",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := deps.Client(cmd.Context())
			if err != nil {
				return err
			}
			response, err := client.TitanInfrastructureEndpointsTenantGetTenantEndpointWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			if response.JSON200 == nil {
				return output.Explain(api.ProblemOf(response.HTTPResponse, response.Body))
			}
			tenant := response.JSON200
			fields := []output.Field{
				{Name: "ID", Value: text(tenant.Id)},
				{Name: "Name", Value: text(tenant.Name)},
				{Name: "Status", Value: status(tenant.Status)},
				{Name: "Max VMs", Value: number(tenant.MaxVms)},
				{Name: "Max networks", Value: number(tenant.MaxNetworks)},
				{Name: "Created", Value: date(tenant.CreatedAt)},
			}
			return deps.Printer.Object(fields, tenant)
		},
	}
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func status(value *api.TenantStatus) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func number(value *int32) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(int(*value))
}

func date(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
