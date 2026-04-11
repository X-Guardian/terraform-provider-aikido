package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ datasource.DataSource = &DomainsDataSource{}

// NewDomainsDataSource creates a new domains data source.
func NewDomainsDataSource() datasource.DataSource {
	return &DomainsDataSource{}
}

// DomainsDataSource defines the data source implementation.
type DomainsDataSource struct {
	client *client.AikidoClient
}

// DomainsDataSourceModel describes the data source data model.
type DomainsDataSourceModel struct {
	Domains []DomainDataSourceModel `tfsdk:"domains"`
}

// DomainDataSourceModel describes a single domain.
type DomainDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	Domain types.String `tfsdk:"domain"`
	Kind   types.String `tfsdk:"kind"`
}

func (d *DomainsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *DomainsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all domains in the Aikido workspace.",

		Attributes: map[string]schema.Attribute{
			"domains": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of domains.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the domain.",
						},
						"domain": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The domain name.",
						},
						"kind": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The type of domain (front_end, rest_api, graphql_api, hosted).",
						},
					},
				},
			},
		},
	}
}

func (d *DomainsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	aikidoClient, ok := req.ProviderData.(*client.AikidoClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.AikidoClient, got: %T.", req.ProviderData),
		)
		return
	}

	d.client = aikidoClient
}

func (d *DomainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domains, err := d.client.ListDomains(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Domains", fmt.Sprintf("Unable to list domains: %s", err))
		return
	}

	data.Domains = make([]DomainDataSourceModel, len(domains))
	for i, domain := range domains {
		data.Domains[i] = DomainDataSourceModel{
			ID:     types.StringValue(strconv.Itoa(domain.ID)),
			Domain: types.StringValue(domain.Domain),
			Kind:   types.StringValue(domain.Kind),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
