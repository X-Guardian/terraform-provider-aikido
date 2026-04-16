package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &TeamResource{}
var _ resource.ResourceWithImportState = &TeamResource{}

func NewTeamResource() resource.Resource {
	return &TeamResource{}
}

// TeamResource defines the resource implementation.
type TeamResource struct {
	client *client.AikidoClient
}

// TeamResourceModel describes the resource data model.
type TeamResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	ExternalSource   types.String `tfsdk:"external_source"`
	ExternalSourceID types.String `tfsdk:"external_source_id"`
	Active           types.Bool   `tfsdk:"active"`
}

func (r *TeamResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (r *TeamResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a team in Aikido Security.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the team.",
			},
			"external_source": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The external source of the team (e.g., github), or null if manually created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"external_source_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The external source identifier for the team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the team is active.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TeamResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	team, err := r.client.CreateTeam(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Team", fmt.Sprintf("Unable to create team: %s", err))
		return
	}

	mapTeamToModel(team, &data)

	tflog.Trace(ctx, "created team", map[string]interface{}{"id": team.ID, "name": team.Name})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Team ID", fmt.Sprintf("Cannot parse team ID %q as integer: %s", data.ID.ValueString(), err))
		return
	}

	team, err := r.client.GetTeam(ctx, teamID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			tflog.Warn(ctx, "team not found, removing from state", map[string]interface{}{"id": teamID})
			return
		}
		resp.Diagnostics.AddError("Error Reading Team", fmt.Sprintf("Unable to read team %d: %s", teamID, err))
		return
	}

	mapTeamToModel(team, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TeamResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Team ID", fmt.Sprintf("Cannot parse team ID %q as integer: %s", data.ID.ValueString(), err))
		return
	}

	updateReq := client.UpdateTeamRequest{
		Name: data.Name.ValueString(),
	}

	if err := r.client.UpdateTeam(ctx, teamID, updateReq); err != nil {
		resp.Diagnostics.AddError("Error Updating Team", fmt.Sprintf("Unable to update team: %s", err))
		return
	}

	team, err := r.client.GetTeam(ctx, teamID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Team", fmt.Sprintf("Unable to read team after update: %s", err))
		return
	}

	mapTeamToModel(team, &data)

	tflog.Trace(ctx, "updated team", map[string]interface{}{"id": teamID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Team ID", fmt.Sprintf("Cannot parse team ID %q as integer: %s", data.ID.ValueString(), err))
		return
	}

	if err := r.client.DeleteTeam(ctx, teamID); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Team",
			fmt.Sprintf("Unable to delete team %d: %s. If this is an imported team, remove it from state with `terraform state rm`.", teamID, err),
		)
		return
	}

	tflog.Trace(ctx, "deleted team", map[string]interface{}{"id": teamID})
}

func (r *TeamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapTeamToModel maps an API Team to the Terraform resource model.
func mapTeamToModel(team *client.Team, data *TeamResourceModel) {
	data.ID = types.StringValue(strconv.Itoa(team.ID))
	data.Name = types.StringValue(team.Name)
	if team.ExternalSource != nil {
		data.ExternalSource = types.StringValue(*team.ExternalSource)
	} else {
		data.ExternalSource = types.StringNull()
	}
	if team.ExternalSourceID != nil {
		data.ExternalSourceID = types.StringValue(*team.ExternalSourceID)
	} else {
		data.ExternalSourceID = types.StringNull()
	}
	data.Active = types.BoolValue(team.Active)
}
