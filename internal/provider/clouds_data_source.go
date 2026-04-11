// Copyright IBM Corp. 2025
// SPDX-License-Identifier: MPL-2.0

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

var _ datasource.DataSource = &CloudsDataSource{}

// NewCloudsDataSource creates a new clouds data source.
func NewCloudsDataSource() datasource.DataSource {
	return &CloudsDataSource{}
}

// CloudsDataSource defines the data source implementation.
type CloudsDataSource struct {
	client *client.AikidoClient
}

// CloudsDataSourceModel describes the data source data model.
type CloudsDataSourceModel struct {
	Clouds []CloudDataSourceModel `tfsdk:"clouds"`
}

// CloudDataSourceModel describes a single cloud environment.
type CloudDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Provider    types.String `tfsdk:"provider_name"`
	Environment types.String `tfsdk:"environment"`
	ExternalID  types.String `tfsdk:"external_id"`
}

func (d *CloudsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_clouds"
}

func (d *CloudsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all connected cloud environments in the Aikido workspace.",

		Attributes: map[string]schema.Attribute{
			"clouds": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of cloud environments.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the cloud environment.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the cloud environment.",
						},
						"provider_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The cloud provider (e.g., aws, gcp, azure).",
						},
						"environment": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The deployment tier (e.g., production, staging, development, mixed).",
						},
						"external_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The native identifier from the cloud provider.",
						},
					},
				},
			},
		},
	}
}

func (d *CloudsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	aikidoClient, ok := req.ProviderData.(*client.AikidoClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.AikidoClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = aikidoClient
}

func (d *CloudsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CloudsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clouds, err := d.client.ListClouds(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Clouds", fmt.Sprintf("Unable to list clouds: %s", err))
		return
	}

	data.Clouds = make([]CloudDataSourceModel, len(clouds))
	for i, cloud := range clouds {
		data.Clouds[i] = CloudDataSourceModel{
			ID:          types.StringValue(strconv.Itoa(cloud.ID)),
			Name:        types.StringValue(cloud.Name),
			Provider:    types.StringValue(cloud.Provider),
			Environment: types.StringValue(cloud.Environment),
			ExternalID:  types.StringValue(cloud.ExternalID),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
