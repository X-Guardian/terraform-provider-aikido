package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &TeamMembershipResource{}
var _ resource.ResourceWithImportState = &TeamMembershipResource{}

func NewTeamMembershipResource() resource.Resource {
	return &TeamMembershipResource{}
}

// TeamMembershipResource manages the association between a user and a team.
type TeamMembershipResource struct {
	client *client.AikidoClient
}

// TeamMembershipResourceModel describes the resource data model.
type TeamMembershipResourceModel struct {
	ID     types.String `tfsdk:"id"`
	TeamID types.String `tfsdk:"team_id"`
	UserID types.String `tfsdk:"user_id"`
}

func (r *TeamMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_membership"
}

func (r *TeamMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a user's membership in an Aikido team.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The composite identifier of the membership (`team_id:user_id`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"team_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the user to add to the team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *TeamMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	aikidoClient, ok := req.ProviderData.(*client.AikidoClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.AikidoClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = aikidoClient
}

func (r *TeamMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamMembershipResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, userID, diags := parseTeamAndUserIDs(data.TeamID.ValueString(), data.UserID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.AddUserToTeam(ctx, teamID, userID); err != nil {
		resp.Diagnostics.AddError("Error Adding User to Team", fmt.Sprintf("Unable to add user %d to team %d: %s", userID, teamID, err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d:%d", teamID, userID))

	tflog.Trace(ctx, "created team membership", map[string]interface{}{"team_id": teamID, "user_id": userID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamMembershipResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, userID, diags := parseTeamAndUserIDs(data.TeamID.ValueString(), data.UserID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	isMember, err := r.client.IsUserInTeam(ctx, teamID, userID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Team Membership", fmt.Sprintf("Unable to check membership: %s", err))
		return
	}

	if !isMember {
		resp.State.RemoveResource(ctx)
		tflog.Warn(ctx, "team membership not found, removing from state", map[string]interface{}{"team_id": teamID, "user_id": userID})
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Both team_id and user_id have RequiresReplace, so Update should never be called.
	resp.Diagnostics.AddError("Unexpected Update", "Team membership does not support in-place updates. This is a provider bug.")
}

func (r *TeamMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamMembershipResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, userID, diags := parseTeamAndUserIDs(data.TeamID.ValueString(), data.UserID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveUserFromTeam(ctx, teamID, userID); err != nil {
		resp.Diagnostics.AddError("Error Removing User from Team", fmt.Sprintf("Unable to remove user %d from team %d: %s", userID, teamID, err))
		return
	}

	tflog.Trace(ctx, "deleted team membership", map[string]interface{}{"team_id": teamID, "user_id": userID})
}

func (r *TeamMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Import ID must be in the format `team_id:user_id` (e.g., `123:456`).")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[1])...)
}

func parseTeamAndUserIDs(teamIDStr, userIDStr string) (int, int, diag.Diagnostics) {
	var diags diag.Diagnostics

	teamID, err := strconv.Atoi(teamIDStr)
	if err != nil {
		diags.AddError("Invalid Team ID", fmt.Sprintf("Cannot parse team_id %q as integer: %s", teamIDStr, err))
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		diags.AddError("Invalid User ID", fmt.Sprintf("Cannot parse user_id %q as integer: %s", userIDStr, err))
	}

	return teamID, userID, diags
}
