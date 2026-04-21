package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &TeamResourceLinkResource{}
var _ resource.ResourceWithImportState = &TeamResourceLinkResource{}

func NewTeamResourceLinkResource() resource.Resource {
	return &TeamResourceLinkResource{}
}

// TeamResourceLinkResource manages the association between a resource and a team.
type TeamResourceLinkResource struct {
	client *client.AikidoClient
}

// TeamResourceLinkResourceModel describes the resource data model.
type TeamResourceLinkResourceModel struct {
	ID           types.String `tfsdk:"id"`
	TeamID       types.String `tfsdk:"team_id"`
	ResourceType types.String `tfsdk:"resource_type"`
	ResourceID   types.String `tfsdk:"resource_id"`
}

func (r *TeamResourceLinkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_resource_link"
}

func (r *TeamResourceLinkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Links a resource (code repository, container repository, cloud, domain, or Zen app) to an Aikido team.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The composite identifier (`team_id:resource_type:resource_id`).",
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
			"resource_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of resource to link. Valid values: `code_repository`, `container_repository`, `cloud`, `domain`, `zen_app`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("code_repository", "container_repository", "cloud", "domain", "zen_app"),
				},
			},
			"resource_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the resource to link to the team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *TeamResourceLinkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TeamResourceLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamResourceLinkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, resourceID, diags := parseResourceLinkIDs(data.TeamID.ValueString(), data.ResourceID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := data.ResourceType.ValueString()

	if err := r.client.LinkResourceToTeam(ctx, teamID, resourceType, resourceID); err != nil {
		resp.Diagnostics.AddError("Error Linking Resource to Team", fmt.Sprintf("Unable to link %s %d to team %d: %s", resourceType, resourceID, teamID, err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d:%s:%d", teamID, resourceType, resourceID))

	tflog.Debug(ctx, "linked resource to team", map[string]interface{}{"team_id": teamID, "resource_type": resourceType, "resource_id": resourceID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResourceLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamResourceLinkResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, resourceID, diags := parseResourceLinkIDs(data.TeamID.ValueString(), data.ResourceID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := data.ResourceType.ValueString()

	linked, err := r.client.IsResourceLinkedToTeam(ctx, teamID, resourceType, resourceID)
	if err != nil {
		resp.State.RemoveResource(ctx)
		tflog.Warn(ctx, "team not found, removing resource link from state", map[string]interface{}{"team_id": teamID})
		return
	}

	if !linked {
		resp.State.RemoveResource(ctx)
		tflog.Warn(ctx, "resource link not found, removing from state", map[string]interface{}{"team_id": teamID, "resource_type": resourceType, "resource_id": resourceID})
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResourceLinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "Team resource link does not support in-place updates. This is a provider bug.")
}

func (r *TeamResourceLinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamResourceLinkResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID, resourceID, diags := parseResourceLinkIDs(data.TeamID.ValueString(), data.ResourceID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := data.ResourceType.ValueString()

	if err := r.client.UnlinkResourceFromTeam(ctx, teamID, resourceType, resourceID); err != nil {
		resp.Diagnostics.AddError("Error Unlinking Resource from Team", fmt.Sprintf("Unable to unlink %s %d from team %d: %s", resourceType, resourceID, teamID, err))
		return
	}

	tflog.Debug(ctx, "unlinked resource from team", map[string]interface{}{"team_id": teamID, "resource_type": resourceType, "resource_id": resourceID})
}

func (r *TeamResourceLinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 3)
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid Import ID", "Import ID must be in the format `team_id:resource_type:resource_id` (e.g., `123:code_repository:456`).")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_type"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_id"), parts[2])...)
}

func parseResourceLinkIDs(teamIDStr, resourceIDStr string) (int, int, diag.Diagnostics) {
	var diags diag.Diagnostics

	teamID, err := strconv.Atoi(teamIDStr)
	if err != nil {
		diags.AddError("Invalid Team ID", fmt.Sprintf("Cannot parse team_id %q as integer: %s", teamIDStr, err))
	}

	resourceID, err := strconv.Atoi(resourceIDStr)
	if err != nil {
		diags.AddError("Invalid Resource ID", fmt.Sprintf("Cannot parse resource_id %q as integer: %s", resourceIDStr, err))
	}

	return teamID, resourceID, diags
}
