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

var _ resource.Resource = &ContainerConfigResource{}
var _ resource.ResourceWithImportState = &ContainerConfigResource{}

// emptyStringToNullModifier normalizes a configured empty string to null so that an empty tag_filter and an omitted
// tag_filter are equivalent.
type emptyStringToNullModifier struct{}

func (m emptyStringToNullModifier) Description(context.Context) string {
	return "Treats an empty string as null."
}

func (m emptyStringToNullModifier) MarkdownDescription(context.Context) string {
	return "Treats an empty string as null."
}

func (m emptyStringToNullModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() && req.ConfigValue.ValueString() == "" {
		resp.PlanValue = types.StringNull()
	}
}

// NewContainerConfigResource creates a new container config resource.
func NewContainerConfigResource() resource.Resource {
	return &ContainerConfigResource{}
}

// ContainerConfigResource manages the scanning configuration of a container repository.
type ContainerConfigResource struct {
	client *client.AikidoClient
}

// ContainerConfigResourceModel describes the resource data model.
type ContainerConfigResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ContainerRepoID  types.String `tfsdk:"container_repo_id"`
	Active           types.Bool   `tfsdk:"active"`
	Sensitivity      types.String `tfsdk:"sensitivity"`
	InternetExposed  types.String `tfsdk:"internet_exposed"`
	TagFilter        types.String `tfsdk:"tag_filter"`
	LinkedCodeRepoID types.String `tfsdk:"linked_code_repo_id"`
	Name             types.String `tfsdk:"name"`
	ProviderName     types.String `tfsdk:"provider_name"`
	RegistryName     types.String `tfsdk:"registry_name"`
	Tag              types.String `tfsdk:"tag"`
	Distro           types.String `tfsdk:"distro"`
}

func (r *ContainerConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container_config"
}

func (r *ContainerConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the scanning configuration of an existing container repository in Aikido Security.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The container repository ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"container_repo_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the container repository to manage.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"active": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether scanning is active for this container.",
			},
			"sensitivity": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The sensitivity level: `extreme`, `sensitive`, `normal`, `not_sensitive`, or `no_data`.",
				Validators: []validator.String{
					stringvalidator.OneOf("extreme", "sensitive", "normal", "not_sensitive", "no_data"),
				},
			},
			"internet_exposed": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The internet exposure status: `connected`, `not_connected`, or `unknown`.",
				Validators: []validator.String{
					stringvalidator.OneOf("connected", "not_connected", "unknown"),
				},
			},
			"tag_filter": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Tag filter pattern for scanning. Supports wildcards (`*`) and `semver-production`. Set to `null` (or omit) to scan the latest image.",
				PlanModifiers: []planmodifier.String{
					emptyStringToNullModifier{},
				},
			},
			"linked_code_repo_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The ID of a code repository to link to this container.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the container repository.",
			},
			"provider_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The registry provider.",
			},
			"registry_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the registry.",
			},
			"tag": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current tag being scanned.",
			},
			"distro": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The OS distribution.",
			},
		},
	}
}

func (r *ContainerConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	aikidoClient, ok := req.ProviderData.(*client.AikidoClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.AikidoClient, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = aikidoClient
}

func (r *ContainerConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ContainerConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	containerID, err := strconv.Atoi(data.ContainerRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Container ID", fmt.Sprintf("Cannot parse container_repo_id: %s", err))
		return
	}

	// Read the current container first. The tag filter is reflected back as the container's `tag` field.
	current, err := r.client.GetContainer(ctx, containerID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Container", fmt.Sprintf("Unable to read container before create: %s", err))
		return
	}

	// Apply configured settings.
	r.applyConfig(ctx, containerID, &data, current, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read back full state.
	container, err := r.client.GetContainer(ctx, containerID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Container", fmt.Sprintf("Unable to read container after create: %s", err))
		return
	}

	r.mapContainerToModel(container, &data)

	tflog.Debug(ctx, "created container config", map[string]interface{}{"id": containerID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContainerConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ContainerConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	containerID, err := strconv.Atoi(data.ContainerRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Container ID", fmt.Sprintf("Cannot parse container_repo_id: %s", err))
		return
	}

	container, err := r.client.GetContainer(ctx, containerID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			tflog.Warn(ctx, "container not found, removing from state", map[string]interface{}{"id": containerID})
			return
		}
		resp.Diagnostics.AddError("Error Reading Container", fmt.Sprintf("Unable to read container %d: %s", containerID, err))
		return
	}

	r.mapContainerToModel(container, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContainerConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ContainerConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	containerID, err := strconv.Atoi(plan.ContainerRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Container ID", fmt.Sprintf("Cannot parse container_repo_id: %s", err))
		return
	}

	if !plan.Active.IsNull() && !plan.Active.Equal(state.Active) {
		if plan.Active.ValueBool() {
			if err := r.client.ActivateContainer(ctx, containerID); err != nil {
				resp.Diagnostics.AddError("Error Activating Container", err.Error())
				return
			}
		} else {
			if err := r.client.DeactivateContainer(ctx, containerID); err != nil {
				resp.Diagnostics.AddError("Error Deactivating Container", err.Error())
				return
			}
		}
	}

	if !plan.Sensitivity.IsNull() && !plan.Sensitivity.Equal(state.Sensitivity) {
		if err := r.client.UpdateContainerSensitivity(ctx, containerID, plan.Sensitivity.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error Updating Sensitivity", err.Error())
			return
		}
	}

	if !plan.InternetExposed.IsNull() && !plan.InternetExposed.Equal(state.InternetExposed) {
		if err := r.client.UpdateContainerConnectivity(ctx, containerID, plan.InternetExposed.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error Updating Connectivity", err.Error())
			return
		}
	}

	// state.Tag holds the live tag filter from the last read. Skip the update when the desired filter already matches it
	if !plan.TagFilter.IsNull() && !plan.TagFilter.Equal(state.TagFilter) && plan.TagFilter.ValueString() != state.Tag.ValueString() {
		if err := r.client.UpdateContainerTagFilter(ctx, containerID, plan.TagFilter.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error Updating Tag Filter", err.Error())
			return
		}
	}

	if !plan.LinkedCodeRepoID.Equal(state.LinkedCodeRepoID) {
		if plan.LinkedCodeRepoID.IsNull() {
			if err := r.client.UnlinkCodeRepoFromContainer(ctx, containerID); err != nil {
				resp.Diagnostics.AddError("Error Unlinking Code Repo", err.Error())
				return
			}
		} else {
			codeRepoID, err := strconv.Atoi(plan.LinkedCodeRepoID.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("Invalid Linked Code Repo ID", fmt.Sprintf("Cannot parse linked_code_repo_id: %s", err))
				return
			}
			if codeRepoID == 0 {
				resp.Diagnostics.AddError("Invalid Linked Code Repo ID", "linked_code_repo_id must be a valid code repository ID; use null to leave the container unlinked.")
				return
			}
			if err := r.client.LinkCodeRepoToContainer(ctx, containerID, codeRepoID); err != nil {
				resp.Diagnostics.AddError("Error Linking Code Repo", err.Error())
				return
			}
		}
	}

	// Read back full state.
	container, err := r.client.GetContainer(ctx, containerID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Container", fmt.Sprintf("Unable to read container after update: %s", err))
		return
	}

	r.mapContainerToModel(container, &plan)

	tflog.Debug(ctx, "updated container config", map[string]interface{}{"id": containerID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ContainerConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ContainerConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	containerID, err := strconv.Atoi(data.ContainerRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Container ID", fmt.Sprintf("Cannot parse container_repo_id: %s", err))
		return
	}

	if err := r.client.DeactivateContainer(ctx, containerID); err != nil {
		resp.Diagnostics.AddError("Error Deactivating Container", fmt.Sprintf("Unable to deactivate container %d: %s", containerID, err))
		return
	}

	tflog.Debug(ctx, "deactivated container (delete)", map[string]interface{}{"id": containerID})
}

func (r *ContainerConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("container_repo_id"), req.ID)...)
}

// applyConfig applies configured settings during create. Current is the container's state before any changes, used to skip no-op updates.
func (r *ContainerConfigResource) applyConfig(ctx context.Context, containerID int, data *ContainerConfigResourceModel, current *client.ContainerDetail, diags *diag.Diagnostics) {
	if !data.Active.IsNull() {
		if data.Active.ValueBool() {
			if err := r.client.ActivateContainer(ctx, containerID); err != nil {
				diags.AddError("Error Activating Container", err.Error())
				return
			}
		} else {
			if err := r.client.DeactivateContainer(ctx, containerID); err != nil {
				diags.AddError("Error Deactivating Container", err.Error())
				return
			}
		}
	}

	if !data.Sensitivity.IsNull() {
		if err := r.client.UpdateContainerSensitivity(ctx, containerID, data.Sensitivity.ValueString()); err != nil {
			diags.AddError("Error Updating Sensitivity", err.Error())
			return
		}
	}

	if !data.InternetExposed.IsNull() {
		if err := r.client.UpdateContainerConnectivity(ctx, containerID, data.InternetExposed.ValueString()); err != nil {
			diags.AddError("Error Updating Connectivity", err.Error())
			return
		}
	}

	if !data.TagFilter.IsNull() && data.TagFilter.ValueString() != current.Tag {
		if err := r.client.UpdateContainerTagFilter(ctx, containerID, data.TagFilter.ValueString()); err != nil {
			diags.AddError("Error Updating Tag Filter", err.Error())
			return
		}
	}

	if !data.LinkedCodeRepoID.IsNull() {
		codeRepoID, err := strconv.Atoi(data.LinkedCodeRepoID.ValueString())
		if err != nil {
			diags.AddError("Invalid Linked Code Repo ID", fmt.Sprintf("Cannot parse linked_code_repo_id: %s", err))
			return
		}
		if codeRepoID == 0 {
			diags.AddError("Invalid Linked Code Repo ID", "linked_code_repo_id must be a valid code repository ID; use null to leave the container unlinked.")
			return
		}
		if err := r.client.LinkCodeRepoToContainer(ctx, containerID, codeRepoID); err != nil {
			diags.AddError("Error Linking Code Repo", err.Error())
			return
		}
	}
}

// mapContainerToModel populates the Terraform model from an API response.
func (r *ContainerConfigResource) mapContainerToModel(container *client.ContainerDetail, data *ContainerConfigResourceModel) {
	data.ID = types.StringValue(strconv.Itoa(container.ID))
	data.ContainerRepoID = types.StringValue(strconv.Itoa(container.ID))
	data.Name = types.StringValue(container.Name)
	data.ProviderName = types.StringValue(container.Provider)
	data.Tag = types.StringValue(container.Tag)
	data.Distro = types.StringValue(container.Distro)

	if container.Tag != "" {
		data.TagFilter = types.StringValue(container.Tag)
	} else {
		data.TagFilter = types.StringNull()
	}

	if container.RegistryName != nil {
		data.RegistryName = types.StringValue(*container.RegistryName)
	} else {
		data.RegistryName = types.StringNull()
	}

	// The API returns 0 (not null) when no code repo is linked; treat both as unlinked.
	if container.LinkedCodeRepoID != nil && *container.LinkedCodeRepoID != 0 {
		data.LinkedCodeRepoID = types.StringValue(strconv.Itoa(*container.LinkedCodeRepoID))
	} else {
		data.LinkedCodeRepoID = types.StringNull()
	}
}
