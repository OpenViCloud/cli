// Package catalog is what a customer chooses from when creating a VM: `zone list`, `template list`, `size list`.
package catalog

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/OpenViCloud/cli/api"
	"github.com/OpenViCloud/cli/command"
	"github.com/OpenViCloud/cli/output"
)

func NewZone(deps *command.Deps) *cobra.Command {
	zone := &cobra.Command{Use: "zone", Short: "Zones you can create resources in"}
	zone.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the zones",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := deps.Client(cmd.Context())
			if err != nil {
				return err
			}
			response, err := client.TitanInfrastructureEndpointsCatalogListZonesEndpointWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			if response.JSON200 == nil {
				return output.Explain(api.ProblemOf(response.HTTPResponse, response.Body))
			}
			rows := [][]string{}
			for _, zone := range *response.JSON200 {
				rows = append(rows, []string{output.Text(zone.Id), output.Text(zone.Name)})
			}
			return deps.Printer.Table([]string{"ID", "NAME"}, rows, response.JSON200)
		},
	})
	return zone
}

func NewTemplate(deps *command.Deps) *cobra.Command {
	var zone string
	list := &cobra.Command{
		Use:   "list",
		Short: "List the operating systems a VM can start from",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := deps.Client(cmd.Context())
			if err != nil {
				return err
			}
			templates, err := Templates(cmd.Context(), client, zone)
			if err != nil {
				return err
			}
			rows := [][]string{}
			for _, template := range templates {
				rows = append(rows, []string{output.Text(template.Id), output.Text(template.Name), output.Text(template.Os), output.Int(template.DiskGb)})
			}
			return deps.Printer.Table([]string{"ID", "NAME", "OS", "DISK GB"}, rows, templates)
		},
	}
	list.Flags().StringVar(&zone, "zone", "", "only the templates ready in this zone")
	template := &cobra.Command{Use: "template", Short: "Operating systems for VMs"}
	template.AddCommand(list)
	return template
}

// Templates lists the templates a customer can pick, in one zone when zone is set.
func Templates(ctx context.Context, client *api.ClientWithResponses, zone string) ([]api.TemplateQueryResult, error) {
	params := &api.TitanInfrastructureEndpointsCatalogListTemplatesEndpointParams{}
	if zone != "" {
		params.ZoneId = &zone
	}
	response, err := client.TitanInfrastructureEndpointsCatalogListTemplatesEndpointWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if response.JSON200 == nil {
		return nil, output.Explain(api.ProblemOf(response.HTTPResponse, response.Body))
	}
	return *response.JSON200, nil
}

func NewSize(deps *command.Deps) *cobra.Command {
	size := &cobra.Command{Use: "size", Short: "How big a VM is, and its price"}
	size.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the VM sizes and their price per hour (VND)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := deps.Client(cmd.Context())
			if err != nil {
				return err
			}
			sizes, err := Sizes(cmd.Context(), client)
			if err != nil {
				return err
			}
			rows := [][]string{}
			for _, size := range sizes {
				rows = append(rows, []string{output.Text(size.Id), output.Text(size.Name), output.Int(size.Vcpu),
					output.Number(size.Cpu), output.Int(size.MemoryMb), output.Number(size.PricePerHour)})
			}
			return deps.Printer.Table([]string{"ID", "NAME", "VCPU", "CPU", "MEMORY MB", "PRICE/HOUR"}, rows, sizes)
		},
	})
	return size
}

func Sizes(ctx context.Context, client *api.ClientWithResponses) ([]api.VmOfferingQueryResult, error) {
	response, err := client.TitanInfrastructureEndpointsCatalogListVmOfferingsEndpointWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if response.JSON200 == nil {
		return nil, output.Explain(api.ProblemOf(response.HTTPResponse, response.Body))
	}
	return *response.JSON200, nil
}
