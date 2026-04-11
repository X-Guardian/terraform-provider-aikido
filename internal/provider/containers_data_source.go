package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ datasource.DataSource = &ContainersDataSource{}

// NewContainersDataSource creates a new containers data source.
func NewContainersDataSource() datasource.DataSource {
	return &ContainersDataSource{}
}

// ContainersDataSource defines the data source implementation.
type ContainersDataSource struct {
	client *client.AikidoClient
}

// ContainersDataSourceModel describes the data source data model.
type ContainersDataSourceModel struct {
	FilterName   types.String               `tfsdk:"filter_name"`
	FilterTag    types.String               `tfsdk:"filter_tag"`
	FilterTeamID types.String               `tfsdk:"filter_team_id"`
	FilterStatus types.String               `tfsdk:"filter_status"`
	Containers   []ContainerDataSourceModel `tfsdk:"containers"`
}

// ContainerDataSourceModel describes a single container.
type ContainerDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	ProviderName types.String `tfsdk:"provider_name"`
	RegistryName types.String `tfsdk:"registry_name"`
	Tag          types.String `tfsdk:"tag"`
	Distro       types.String `tfsdk:"distro"`
}

func (d *ContainersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_containers"
}

func (d *ContainersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists container repositories in the Aikido workspace.",

		Attributes: map[string]schema.Attribute{
			"filter_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter containers by name.",
			},
			"filter_tag": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter containers by tag.",
			},
			"filter_team_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter containers by team ID.",
			},
			"filter_status": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by status: `active` (default), `inactive`, or `all`.",
				Validators: []validator.String{
					stringvalidator.OneOf("active", "inactive", "all"),
				},
			},
			"containers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of container repositories.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the container.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the container repository.",
						},
						"provider_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The registry provider (e.g., aws, gcp-artifact-registry, docker-hub).",
						},
						"registry_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the registry.",
						},
						"tag": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The tag filter for image selection.",
						},
						"distro": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The OS distribution.",
						},
					},
				},
			},
		},
	}
}

func (d *ContainersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContainersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ContainersDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &client.ListContainersOptions{}

	if !data.FilterName.IsNull() {
		opts.FilterName = data.FilterName.ValueString()
	}
	if !data.FilterTag.IsNull() {
		opts.FilterTag = data.FilterTag.ValueString()
	}
	if !data.FilterTeamID.IsNull() {
		teamID, err := strconv.Atoi(data.FilterTeamID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Team ID", fmt.Sprintf("Cannot parse filter_team_id: %s", err))
			return
		}
		opts.FilterTeamID = &teamID
	}
	if !data.FilterStatus.IsNull() {
		opts.FilterStatus = data.FilterStatus.ValueString()
	}

	containers, err := d.client.ListContainers(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Containers", fmt.Sprintf("Unable to list containers: %s", err))
		return
	}

	data.Containers = make([]ContainerDataSourceModel, len(containers))
	for i, c := range containers {
		registryName := ""
		if c.RegistryName != nil {
			registryName = *c.RegistryName
		}
		data.Containers[i] = ContainerDataSourceModel{
			ID:           types.StringValue(strconv.Itoa(c.ID)),
			Name:         types.StringValue(c.Name),
			ProviderName: types.StringValue(c.Provider),
			RegistryName: types.StringValue(registryName),
			Tag:          types.StringValue(c.Tag),
			Distro:       types.StringValue(c.Distro),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
