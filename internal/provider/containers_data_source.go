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
	FilterName         types.String               `tfsdk:"filter_name"`
	FilterTag          types.String               `tfsdk:"filter_tag"`
	FilterTeamID       types.String               `tfsdk:"filter_team_id"`
	FilterStatus       types.String               `tfsdk:"filter_status"`
	FilterReachability types.String               `tfsdk:"filter_reachability"`
	Containers         []ContainerDataSourceModel `tfsdk:"containers"`
}

// ContainerDataSourceModel describes a single container.
type ContainerDataSourceModel struct {
	ID               types.String                    `tfsdk:"id"`
	Name             types.String                    `tfsdk:"name"`
	ProviderName     types.String                    `tfsdk:"provider_name"`
	RegistryName     types.String                    `tfsdk:"registry_name"`
	Tag              types.String                    `tfsdk:"tag"`
	Distro           types.String                    `tfsdk:"distro"`
	LinkedCodeRepoID types.String                    `tfsdk:"linked_code_repo_id"`
	Active           types.Bool                      `tfsdk:"active"`
	Sensitivity      types.String                    `tfsdk:"sensitivity"`
	InternetExposed  types.String                    `tfsdk:"internet_exposed"`
	IsRunning        types.Bool                      `tfsdk:"is_running"`
	IsEmpty          types.Bool                      `tfsdk:"is_empty"`
	ExposedVia       types.String                    `tfsdk:"exposed_via"`
	Labels           []ContainerLabelDataSourceModel `tfsdk:"labels"`
}

// ContainerLabelDataSourceModel describes a single label on a container.
type ContainerLabelDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	IsImported types.Bool   `tfsdk:"is_imported"`
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
			"filter_reachability": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter containers by how their running images are reachable from the internet: `unknown`, `direct`, `lb`, `limited_ips`, or `none`.",
				Validators: []validator.String{
					stringvalidator.OneOf("unknown", "direct", "lb", "limited_ips", "none"),
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
						"linked_code_repo_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the code repository linked to this container, or null if none is linked.",
						},
						"active": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the container is being scanned by Aikido.",
						},
						"sensitivity": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The sensitivity level: `extreme`, `sensitive`, `normal`, `not_sensitive`, or `no_data`.",
						},
						"internet_exposed": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The internet exposure status: `connected`, `not_connected`, or `unknown`.",
						},
						"is_running": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the container is currently running anywhere.",
						},
						"is_empty": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the container repository has no image pushed to it.",
						},
						"exposed_via": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "How the running container image is reachable from the internet: `direct`, `lb` and `limited_ips` mean reachable, `none` means not reachable, and `unknown` means reachability could not be determined.",
						},
						"labels": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "The labels attached to this container.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The unique identifier of the label.",
									},
									"name": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The name of the label.",
									},
									"is_imported": schema.BoolAttribute{
										Computed:            true,
										MarkdownDescription: "Whether the label was imported from the container's registry.",
									},
								},
							},
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

	// The include options are always set so that every advertised attribute is populated; the API
	// omits these fields unless asked for them.
	opts := &client.ListContainersOptions{
		IncludeIsRunning:    true,
		IncludeSensitivity:  true,
		IncludeConnectivity: true,
		IncludeLabels:       true,
	}

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
	if !data.FilterReachability.IsNull() {
		opts.FilterReachability = data.FilterReachability.ValueString()
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
		// The API returns 0 (not null) when no code repo is linked; treat both as unlinked.
		linkedCodeRepoID := types.StringNull()
		if c.LinkedCodeRepoID != nil && *c.LinkedCodeRepoID != 0 {
			linkedCodeRepoID = types.StringValue(strconv.Itoa(*c.LinkedCodeRepoID))
		}
		labels := make([]ContainerLabelDataSourceModel, len(c.Labels))
		for j, l := range c.Labels {
			labels[j] = ContainerLabelDataSourceModel{
				ID:         types.StringValue(strconv.Itoa(l.ID)),
				Name:       types.StringValue(l.Name),
				IsImported: types.BoolValue(l.IsImported),
			}
		}

		// These fields are requested unconditionally, but a container the API declines to report
		// them for still has to yield a null rather than a misleading zero value.
		sensitivity := types.StringNull()
		if c.Sensitivity != nil {
			sensitivity = types.StringValue(*c.Sensitivity)
		}
		internetExposed := types.StringNull()
		if c.Connectivity != nil {
			internetExposed = types.StringValue(*c.Connectivity)
		}
		isRunning := types.BoolNull()
		if c.IsRunning != nil {
			isRunning = types.BoolValue(*c.IsRunning)
		}

		data.Containers[i] = ContainerDataSourceModel{
			ID:               types.StringValue(strconv.Itoa(c.ID)),
			Name:             types.StringValue(c.Name),
			ProviderName:     types.StringValue(c.Provider),
			RegistryName:     types.StringValue(registryName),
			Tag:              types.StringValue(c.Tag),
			Distro:           types.StringValue(c.Distro),
			LinkedCodeRepoID: linkedCodeRepoID,
			Active:           types.BoolValue(c.IsActive),
			Sensitivity:      sensitivity,
			InternetExposed:  internetExposed,
			IsRunning:        isRunning,
			IsEmpty:          types.BoolValue(c.IsEmpty),
			ExposedVia:       types.StringValue(c.ExposedVia),
			Labels:           labels,
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
