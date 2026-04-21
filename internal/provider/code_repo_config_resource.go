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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &CodeRepoConfigResource{}
var _ resource.ResourceWithImportState = &CodeRepoConfigResource{}

// NewCodeRepoConfigResource creates a new code repo config resource.
func NewCodeRepoConfigResource() resource.Resource {
	return &CodeRepoConfigResource{}
}

// CodeRepoConfigResource manages the scanning configuration of a code repository.
type CodeRepoConfigResource struct {
	client *client.AikidoClient
}

// CodeRepoConfigResourceModel describes the resource data model.
type CodeRepoConfigResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	CodeRepoID            types.String `tfsdk:"code_repo_id"`
	Active                types.Bool   `tfsdk:"active"`
	Sensitivity           types.String `tfsdk:"sensitivity"`
	Connectivity          types.String `tfsdk:"connectivity"`
	DevDepScanningEnabled types.Bool   `tfsdk:"dev_dep_scanning_enabled"`
	ExcludedPaths         types.Set    `tfsdk:"excluded_paths"`
	Name                  types.String `tfsdk:"name"`
	ProviderName          types.String `tfsdk:"provider_name"`
	Branch                types.String `tfsdk:"branch"`
	URL                   types.String `tfsdk:"url"`
}

func (r *CodeRepoConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_code_repo_config"
}

func (r *CodeRepoConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the scanning configuration of an existing code repository in Aikido Security. The repository itself is imported from your Git provider — this resource controls its scanning settings.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The code repository ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"code_repo_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the code repository to manage.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"active": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether scanning is active for this repository.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"sensitivity": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The sensitivity level: `extreme`, `sensitive`, `normal`, `not_sensitive`, or `no_data`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("extreme", "sensitive", "normal", "not_sensitive", "no_data"),
				},
			},
			"connectivity": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The connectivity status: `connected`, `not_connected`, or `unknown`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("connected", "not_connected", "unknown"),
				},
			},
			"dev_dep_scanning_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether development dependency scanning is enabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"excluded_paths": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Paths excluded from scanning.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the code repository.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Git provider (e.g., github, gitlab, bitbucket).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"branch": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The branch being scanned.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The URL of the repository.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *CodeRepoConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CodeRepoConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CodeRepoConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repoID, err := strconv.Atoi(data.CodeRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Code Repo ID", fmt.Sprintf("Cannot parse code_repo_id: %s", err))
		return
	}

	// Apply configured settings.
	r.applyConfig(ctx, repoID, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read back the full state from the API.
	repo, err := r.client.GetCodeRepo(ctx, repoID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Code Repo", fmt.Sprintf("Unable to read code repo after create: %s", err))
		return
	}

	r.mapRepoToModel(ctx, repo, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "created code repo config", map[string]interface{}{"id": repoID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CodeRepoConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CodeRepoConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repoID, err := strconv.Atoi(data.CodeRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Code Repo ID", fmt.Sprintf("Cannot parse code_repo_id: %s", err))
		return
	}

	repo, err := r.client.GetCodeRepo(ctx, repoID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			tflog.Warn(ctx, "code repo not found, removing from state", map[string]interface{}{"id": repoID})
			return
		}
		resp.Diagnostics.AddError("Error Reading Code Repo", fmt.Sprintf("Unable to read code repo %d: %s", repoID, err))
		return
	}

	r.mapRepoToModel(ctx, repo, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CodeRepoConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state CodeRepoConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repoID, err := strconv.Atoi(plan.CodeRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Code Repo ID", fmt.Sprintf("Cannot parse code_repo_id: %s", err))
		return
	}

	// Apply active state change.
	if !plan.Active.IsNull() && !plan.Active.Equal(state.Active) {
		if plan.Active.ValueBool() {
			if err := r.client.ActivateCodeRepo(ctx, repoID); err != nil {
				resp.Diagnostics.AddError("Error Activating Code Repo", err.Error())
				return
			}
		} else {
			if err := r.client.DeactivateCodeRepo(ctx, repoID); err != nil {
				resp.Diagnostics.AddError("Error Deactivating Code Repo", err.Error())
				return
			}
		}
	}

	// Apply sensitivity change.
	if !plan.Sensitivity.IsNull() && !plan.Sensitivity.Equal(state.Sensitivity) {
		if err := r.client.UpdateCodeRepoSensitivity(ctx, repoID, plan.Sensitivity.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error Updating Sensitivity", err.Error())
			return
		}
	}

	// Apply connectivity change.
	if !plan.Connectivity.IsNull() && !plan.Connectivity.Equal(state.Connectivity) {
		if err := r.client.UpdateCodeRepoConnectivity(ctx, repoID, plan.Connectivity.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error Updating Connectivity", err.Error())
			return
		}
	}

	// Apply dev dep scanning change.
	if !plan.DevDepScanningEnabled.IsNull() && !plan.DevDepScanningEnabled.Equal(state.DevDepScanningEnabled) {
		if err := r.client.UpdateCodeRepoDevDepScanning(ctx, repoID, plan.DevDepScanningEnabled.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Error Updating Dev Dep Scanning", err.Error())
			return
		}
	}

	// Apply excluded paths changes.
	if !plan.ExcludedPaths.IsNull() {
		var planPaths, statePaths []string
		resp.Diagnostics.Append(plan.ExcludedPaths.ElementsAs(ctx, &planPaths, false)...)
		resp.Diagnostics.Append(state.ExcludedPaths.ElementsAs(ctx, &statePaths, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		r.syncExcludedPaths(ctx, repoID, statePaths, planPaths, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Read back the full state.
	repo, err := r.client.GetCodeRepo(ctx, repoID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Code Repo", fmt.Sprintf("Unable to read code repo after update: %s", err))
		return
	}

	r.mapRepoToModel(ctx, repo, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "updated code repo config", map[string]interface{}{"id": repoID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CodeRepoConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CodeRepoConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repoID, err := strconv.Atoi(data.CodeRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Code Repo ID", fmt.Sprintf("Cannot parse code_repo_id: %s", err))
		return
	}

	if err := r.client.DeactivateCodeRepo(ctx, repoID); err != nil {
		resp.Diagnostics.AddError("Error Deactivating Code Repo", fmt.Sprintf("Unable to deactivate code repo %d: %s", repoID, err))
		return
	}

	tflog.Debug(ctx, "deactivated code repo (delete)", map[string]interface{}{"id": repoID})
}

func (r *CodeRepoConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("code_repo_id"), req.ID)...)
}

// applyConfig applies the user's configured settings to the repo during create.
func (r *CodeRepoConfigResource) applyConfig(ctx context.Context, repoID int, data *CodeRepoConfigResourceModel, diags *diag.Diagnostics) {
	if !data.Active.IsNull() {
		if data.Active.ValueBool() {
			if err := r.client.ActivateCodeRepo(ctx, repoID); err != nil {
				diags.AddError("Error Activating Code Repo", err.Error())
				return
			}
		} else {
			if err := r.client.DeactivateCodeRepo(ctx, repoID); err != nil {
				diags.AddError("Error Deactivating Code Repo", err.Error())
				return
			}
		}
	}

	if !data.Sensitivity.IsNull() {
		if err := r.client.UpdateCodeRepoSensitivity(ctx, repoID, data.Sensitivity.ValueString()); err != nil {
			diags.AddError("Error Updating Sensitivity", err.Error())
			return
		}
	}

	if !data.Connectivity.IsNull() {
		if err := r.client.UpdateCodeRepoConnectivity(ctx, repoID, data.Connectivity.ValueString()); err != nil {
			diags.AddError("Error Updating Connectivity", err.Error())
			return
		}
	}

	if !data.DevDepScanningEnabled.IsNull() {
		if err := r.client.UpdateCodeRepoDevDepScanning(ctx, repoID, data.DevDepScanningEnabled.ValueBool()); err != nil {
			diags.AddError("Error Updating Dev Dep Scanning", err.Error())
			return
		}
	}

	if !data.ExcludedPaths.IsNull() {
		var paths []string
		diags.Append(data.ExcludedPaths.ElementsAs(ctx, &paths, false)...)
		if diags.HasError() {
			return
		}
		for _, p := range paths {
			if err := r.client.AddCodeRepoExcludePath(ctx, repoID, p); err != nil {
				diags.AddError("Error Adding Exclude Path", err.Error())
				return
			}
		}
	}
}

// syncExcludedPaths diffs the old and new excluded path sets and applies changes.
func (r *CodeRepoConfigResource) syncExcludedPaths(ctx context.Context, repoID int, oldPaths, newPaths []string, diags *diag.Diagnostics) {
	oldSet := make(map[string]bool, len(oldPaths))
	for _, p := range oldPaths {
		oldSet[p] = true
	}
	newSet := make(map[string]bool, len(newPaths))
	for _, p := range newPaths {
		newSet[p] = true
	}

	// Add new paths.
	for _, p := range newPaths {
		if !oldSet[p] {
			if err := r.client.AddCodeRepoExcludePath(ctx, repoID, p); err != nil {
				diags.AddError("Error Adding Exclude Path", err.Error())
				return
			}
		}
	}

	// Remove old paths.
	for _, p := range oldPaths {
		if !newSet[p] {
			if err := r.client.RemoveCodeRepoExcludePath(ctx, repoID, p); err != nil {
				diags.AddError("Error Removing Exclude Path", err.Error())
				return
			}
		}
	}
}

// mapRepoToModel populates the Terraform model from an API response.
func (r *CodeRepoConfigResource) mapRepoToModel(ctx context.Context, repo *client.CodeRepoDetail, data *CodeRepoConfigResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(strconv.Itoa(repo.ID))
	data.CodeRepoID = types.StringValue(strconv.Itoa(repo.ID))
	data.Active = types.BoolValue(repo.Active)
	data.Sensitivity = types.StringValue(repo.Sensitivity)
	data.Connectivity = types.StringValue(repo.Connectivity)
	data.Name = types.StringValue(repo.Name)
	data.ProviderName = types.StringValue(repo.Provider)
	data.Branch = types.StringValue(repo.Branch)
	data.URL = types.StringValue(repo.URL)

	paths := make([]string, len(repo.ExcludedPaths))
	for i, ep := range repo.ExcludedPaths {
		paths[i] = ep.Path
	}
	pathSet, d := types.SetValueFrom(ctx, types.StringType, paths)
	diags.Append(d...)
	data.ExcludedPaths = pathSet
}
