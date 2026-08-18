package provider

import (
	"context"
	"fmt"

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

var _ resource.Resource = &AutofixSastResource{}
var _ resource.ResourceWithImportState = &AutofixSastResource{}
var _ resource.ResourceWithConfigValidators = &AutofixSastResource{}

// NewAutofixSastResource creates a new SAST and IaC autofix settings resource.
func NewAutofixSastResource() resource.Resource {
	return &AutofixSastResource{}
}

// AutofixSastResource manages the workspace SAST and IaC autofix settings.
type AutofixSastResource struct {
	client *client.AikidoClient
}

// AutofixSastResourceModel describes the resource data model.
type AutofixSastResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	SeverityFilter types.String `tfsdk:"severity_filter"`
	ReposScope     types.String `tfsdk:"repos_scope"`
	RepoIDs        types.Set    `tfsdk:"repo_ids"`
}

func (r *AutofixSastResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_autofix_sast"
}

func (r *AutofixSastResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages SAST and IaC AutoFix pull request creation settings in Aikido Security.\n\n" +
			"These settings apply to the entire workspace, so only one instance of this resource should exist. " +
			"Destroying it disables SAST and IaC AutoFix for the whole workspace rather than simply ceasing to manage it.\n\n" +
			"Requires the `autofix:read` and `autofix:write` API scopes.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Fixed identifier for these workspace-wide settings. Always `sast`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether automatic SAST and IaC AutoFix pull request creation is enabled.",
			},
			"severity_filter": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Which findings to autofix: `critical_issues_only`, `critical_and_high_only`, or `all`. " +
					"Required when `enabled` is `true`, and ignored by the API otherwise.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						"critical_issues_only",
						"critical_and_high_only",
						"all",
					),
				},
				// Reuse state when omitted so a change to a sibling attribute does not plan
				// this as unknown. The value is written from the plan, never derived by the API.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repos_scope": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Which code repositories SAST and IaC AutoFix applies to: `all` or `selected`. " +
					"Required when `enabled` is `true`, and ignored by the API otherwise. `all` requires a paying account.",
				Validators: []validator.String{
					stringvalidator.OneOf(autofixReposScopeAll, autofixReposScopeSelected),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repo_ids": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Code repository IDs that SAST and IaC AutoFix applies to. Required when `repos_scope` is " +
					"`selected`, and must be omitted or empty when it is `all`. Repository IDs that are inactive or unknown to " +
					"Aikido are silently filtered out by the API and are not reported as configuration drift.",
				Validators: autofixRepoIDValidators(),
				PlanModifiers: []planmodifier.Set{
					repoIDsPlanModifier{},
				},
			},
		},
	}
}

func (r *AutofixSastResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&autofixFieldsValidator{hasReposScope: true},
	}
}

func (r *AutofixSastResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	aikidoClient, ok := req.ProviderData.(*client.AikidoClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.AikidoClient, got: %T.", req.ProviderData))
		return
	}

	r.client = aikidoClient
}

func (r *AutofixSastResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AutofixSastResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applySettings(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "created sast autofix settings", map[string]interface{}{
		"id":      autofixSastID,
		"enabled": data.Enabled.ValueBool(),
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AutofixSastResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AutofixSastResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.client.GetAutofixSastSettings(ctx)
	if err != nil {
		// These settings are workspace-wide and always exist, so an error is never treated
		// as the resource having been deleted.
		if !autofixDisabledDiagnostic(&resp.Diagnostics, err) {
			resp.Diagnostics.AddError("Error Reading SAST AutoFix Settings", fmt.Sprintf("Unable to read settings: %s", err))
		}
		return
	}

	data.ID = types.StringValue(autofixSastID)
	data.Enabled = types.BoolValue(settings.Enabled)

	// When autofix is disabled the remaining settings are meaningless: the API reports
	// severity_filter as "none" and leaves the other values stale. Refreshing them would
	// fight a configuration that still declares them, so prior state is preserved.
	if !settings.Enabled {
		tflog.Debug(ctx, "sast autofix is disabled, preserving subordinate settings from state", map[string]interface{}{
			"id": autofixSastID,
		})
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	priorRepoIDs, diags := autofixRepoIDsFromSet(ctx, data.RepoIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.SeverityFilter = autofixSeverityToModel(settings.SeverityFilter)
	data.ReposScope = types.StringValue(settings.ReposScope)

	repoIDs, diags := autofixRepoIDsToSet(ctx, reconcileRepoIDs(priorRepoIDs, settings.RepoIDs))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.RepoIDs = repoIDs

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AutofixSastResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AutofixSastResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applySettings(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "updated sast autofix settings", map[string]interface{}{
		"id":      autofixSastID,
		"enabled": data.Enabled.ValueBool(),
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AutofixSastResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// There is no delete endpoint, so disable autofix instead.
	if err := r.client.UpdateAutofixSastSettings(ctx, client.AutofixSettingsRequest{Enabled: false}); err != nil {
		// Autofix being disabled workspace-wide already satisfies the intent of the delete.
		if errorsIsAutofixDisabled(err) {
			tflog.Warn(ctx, "autofix is disabled for the workspace, nothing to disable (delete)", map[string]interface{}{
				"id": autofixSastID,
			})
			return
		}
		resp.Diagnostics.AddError("Error Disabling SAST AutoFix", fmt.Sprintf("Unable to disable SAST autofix: %s", err))
		return
	}

	tflog.Debug(ctx, "disabled sast autofix settings (delete)", map[string]interface{}{"id": autofixSastID})
}

func (r *AutofixSastResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// These settings are a workspace-wide singleton, so the import ID is ignored.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), autofixSastID)...)
}

// applySettings writes the planned settings to the API and normalises the model so that
// state matches what the API stores. State is set from the plan rather than from a
// read-back: the API ignores the subordinate fields when disabling, so re-reading them
// would contradict the plan and produce an inconsistent-result-after-apply error. It also
// saves a request against a tightly rate-limited API.
func (r *AutofixSastResource) applySettings(ctx context.Context, data *AutofixSastResourceModel, diags *diag.Diagnostics) {
	settingsReq := client.AutofixSettingsRequest{Enabled: data.Enabled.ValueBool()}

	if data.Enabled.ValueBool() {
		severity := data.SeverityFilter.ValueString()
		scope := data.ReposScope.ValueString()

		repoIDs, d := autofixRepoIDsFromSet(ctx, data.RepoIDs)
		diags.Append(d...)
		if diags.HasError() {
			return
		}

		// The API ignores repo_ids for the "all" scope and expects an empty array. Storing
		// an empty set keeps state aligned with the API and avoids a null-to-empty diff on
		// the next read.
		if scope == autofixReposScopeAll {
			repoIDs = []int64{}
			emptySet, d := autofixRepoIDsToSet(ctx, repoIDs)
			diags.Append(d...)
			if diags.HasError() {
				return
			}
			data.RepoIDs = emptySet
		}

		settingsReq.SeverityFilter = &severity
		settingsReq.ReposScope = &scope
		settingsReq.RepoIDs = &repoIDs
	} else {
		// Optional and computed attributes omitted from the configuration are unknown in
		// the plan. Nothing is sent for them when disabling and nothing is read back, so
		// they are resolved to null to keep state fully known.
		if data.SeverityFilter.IsUnknown() {
			data.SeverityFilter = types.StringNull()
		}
		if data.ReposScope.IsUnknown() {
			data.ReposScope = types.StringNull()
		}
		if data.RepoIDs.IsUnknown() {
			data.RepoIDs = types.SetNull(types.Int64Type)
		}
	}

	if err := r.client.UpdateAutofixSastSettings(ctx, settingsReq); err != nil {
		if !autofixDisabledDiagnostic(diags, err) {
			diags.AddError("Error Updating SAST AutoFix Settings", fmt.Sprintf("Unable to update settings: %s", err))
		}
		return
	}

	data.ID = types.StringValue(autofixSastID)
}
