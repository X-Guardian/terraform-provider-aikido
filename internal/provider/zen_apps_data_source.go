package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ datasource.DataSource = &ZenAppsDataSource{}

// NewZenAppsDataSource creates a new Zen apps data source.
func NewZenAppsDataSource() datasource.DataSource {
	return &ZenAppsDataSource{}
}

// ZenAppsDataSource defines the data source implementation.
type ZenAppsDataSource struct {
	client *client.AikidoClient
}

// ZenAppsDataSourceModel describes the data source data model.
type ZenAppsDataSourceModel struct {
	Apps []ZenAppDataSourceModel `tfsdk:"apps"`
}

// ZenAppDataSourceModel describes a single Zen app.
type ZenAppDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Environment  types.String `tfsdk:"environment"`
	Blocking     types.Bool   `tfsdk:"blocking"`
	CodeRepoName types.String `tfsdk:"code_repo_name"`
}

func (d *ZenAppsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zen_apps"
}

func (d *ZenAppsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Zen runtime firewall apps in the Aikido workspace.",

		Attributes: map[string]schema.Attribute{
			"apps": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of Zen apps.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the Zen app.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the Zen app.",
						},
						"environment": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The environment (production, staging, development).",
						},
						"blocking": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether blocking mode is enabled.",
						},
						"code_repo_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the linked code repository.",
						},
					},
				},
			},
		},
	}
}

func (d *ZenAppsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ZenAppsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ZenAppsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apps, err := d.client.ListZenApps(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Zen Apps", fmt.Sprintf("Unable to list Zen apps: %s", err))
		return
	}

	data.Apps = make([]ZenAppDataSourceModel, len(apps))
	for i, app := range apps {
		codeRepoName := types.StringNull()
		if app.CodeRepoName != nil {
			codeRepoName = types.StringValue(*app.CodeRepoName)
		}
		data.Apps[i] = ZenAppDataSourceModel{
			ID:           types.StringValue(app.ID),
			Name:         types.StringValue(app.Name),
			Environment:  types.StringValue(app.Environment),
			Blocking:     types.BoolValue(app.Blocking),
			CodeRepoName: codeRepoName,
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
