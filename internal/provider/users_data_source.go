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

var _ datasource.DataSource = &UsersDataSource{}

func NewUsersDataSource() datasource.DataSource {
	return &UsersDataSource{}
}

// UsersDataSource defines the data source implementation.
type UsersDataSource struct {
	client *client.AikidoClient
}

// UsersDataSourceModel describes the data source data model.
type UsersDataSourceModel struct {
	TeamID          types.String          `tfsdk:"team_id"`
	IncludeInactive types.Bool            `tfsdk:"include_inactive"`
	Users           []UserDataSourceModel `tfsdk:"users"`
}

// UserDataSourceModel describes a single user in the data source.
type UserDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	FullName types.String `tfsdk:"full_name"`
	Email    types.String `tfsdk:"email"`
	Active   types.Bool   `tfsdk:"active"`
	Role     types.String `tfsdk:"role"`
	AuthType types.String `tfsdk:"auth_type"`
}

func (d *UsersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *UsersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists users in the Aikido workspace, with optional filters.",

		Attributes: map[string]schema.Attribute{
			"team_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter users by team ID. Only returns users that are members of this team.",
			},
			"include_inactive": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Include inactive users in the results. Defaults to `false`.",
			},
			"users": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of users.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the user.",
						},
						"full_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The full name of the user.",
						},
						"email": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The email address of the user.",
						},
						"active": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the user is active.",
						},
						"role": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The role of the user (e.g., admin, default, team_only).",
						},
						"auth_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The authentication type of the user (e.g., saml).",
						},
					},
				},
			},
		},
	}
}

func (d *UsersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UsersDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &client.ListUsersOptions{}

	if !data.TeamID.IsNull() {
		teamID, err := strconv.Atoi(data.TeamID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Team ID", fmt.Sprintf("Cannot parse team_id %q as integer: %s", data.TeamID.ValueString(), err))
			return
		}
		opts.TeamID = &teamID
	}

	if !data.IncludeInactive.IsNull() && data.IncludeInactive.ValueBool() {
		opts.IncludeInactive = true
	}

	users, err := d.client.ListUsers(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Users", fmt.Sprintf("Unable to list users: %s", err))
		return
	}

	data.Users = make([]UserDataSourceModel, len(users))
	for i, user := range users {
		data.Users[i] = UserDataSourceModel{
			ID:       types.StringValue(strconv.Itoa(user.ID)),
			FullName: types.StringValue(user.FullName),
			Email:    types.StringValue(user.Email),
			Active:   types.BoolValue(user.Active == 1),
			Role:     types.StringValue(user.Role),
			AuthType: types.StringValue(user.AuthType),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
