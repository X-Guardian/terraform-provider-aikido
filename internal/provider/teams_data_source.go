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

var _ datasource.DataSource = &TeamsDataSource{}

func NewTeamsDataSource() datasource.DataSource {
	return &TeamsDataSource{}
}

// TeamsDataSource defines the data source implementation.
type TeamsDataSource struct {
	client *client.AikidoClient
}

// TeamsDataSourceModel describes the data source data model.
type TeamsDataSourceModel struct {
	Teams []TeamDataSourceModel `tfsdk:"teams"`
}

// TeamDataSourceModel describes a single team in the data source.
type TeamDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	ExternalSource   types.String `tfsdk:"external_source"`
	ExternalSourceID types.String `tfsdk:"external_source_id"`
	Active           types.Bool   `tfsdk:"active"`
}

func (d *TeamsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_teams"
}

func (d *TeamsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all teams in the Aikido workspace.",

		Attributes: map[string]schema.Attribute{
			"teams": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of teams.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the team.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the team.",
						},
						"external_source": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The external source of the team (e.g., github), or null if manually created.",
						},
						"external_source_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The external source identifier for the team.",
						},
						"active": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the team is active.",
						},
					},
				},
			},
		},
	}
}

func (d *TeamsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TeamsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TeamsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teams, err := d.client.ListTeams(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Teams", fmt.Sprintf("Unable to list teams: %s", err))
		return
	}

	data.Teams = make([]TeamDataSourceModel, len(teams))
	for i, team := range teams {
		model := TeamDataSourceModel{
			ID:     types.StringValue(strconv.Itoa(team.ID)),
			Name:   types.StringValue(team.Name),
			Active: types.BoolValue(team.Active),
		}
		if team.ExternalSource != nil {
			model.ExternalSource = types.StringValue(*team.ExternalSource)
		} else {
			model.ExternalSource = types.StringNull()
		}
		if team.ExternalSourceID != nil {
			model.ExternalSourceID = types.StringValue(*team.ExternalSourceID)
		} else {
			model.ExternalSourceID = types.StringNull()
		}
		data.Teams[i] = model
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
