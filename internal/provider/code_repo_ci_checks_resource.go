package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &CodeRepoCIChecksResource{}
var _ resource.ResourceWithImportState = &CodeRepoCIChecksResource{}
var _ resource.ResourceWithConfigValidators = &CodeRepoCIChecksResource{}

// NewCodeRepoCIChecksResource creates a new code repository CI checks resource.
func NewCodeRepoCIChecksResource() resource.Resource {
	return &CodeRepoCIChecksResource{}
}

// CodeRepoCIChecksResource manages a code repository's CI checks configuration.
type CodeRepoCIChecksResource struct {
	client *client.AikidoClient
}

// CodeRepoCIChecksResourceModel describes the resource data model.
type CodeRepoCIChecksResourceModel struct {
	ID                                       types.String `tfsdk:"id"`
	CodeRepoID                               types.String `tfsdk:"code_repo_id"`
	MinimumSeverity                          types.String `tfsdk:"minimum_severity"`
	FailOnDependencyScan                     types.Bool   `tfsdk:"fail_on_dependency_scan"`
	FailOnSastScan                           types.Bool   `tfsdk:"fail_on_sast_scan"`
	FailOnIacScan                            types.Bool   `tfsdk:"fail_on_iac_scan"`
	FailOnSecretsScan                        types.Bool   `tfsdk:"fail_on_secrets_scan"`
	FailOnMalwareScan                        types.Bool   `tfsdk:"fail_on_malware_scan"`
	MinimumLicenseSeverity                   types.String `tfsdk:"minimum_license_severity"`
	EnableCodeQualityScan                    types.Bool   `tfsdk:"enable_code_quality_scan"`
	FailOnCodeQualityScan                    types.Bool   `tfsdk:"fail_on_code_quality_scan"`
	PostCodeQualityInlineCommentsMinSeverity types.String `tfsdk:"post_code_quality_inline_comments_min_severity"`
	PostInlineCommentsMinSeverity            types.String `tfsdk:"post_inline_comments_min_severity"`
	RunDeepAuditPRScan                       types.Bool   `tfsdk:"run_deep_audit_pr_scan"`
	PostDeepAuditInlineCommentsMinSeverity   types.String `tfsdk:"post_deep_audit_inline_comments_min_severity"`
}

func (r *CodeRepoCIChecksResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_code_repo_ci_checks"
}

func (r *CodeRepoCIChecksResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the CI checks configuration of a code repository in Aikido Security.\n\n" +
			"These are the checks that run on a pull request (or a merge request on GitLab), controlling when the " +
			"Aikido check fails and when inline review comments are posted. Aikido's dashboard and documentation " +
			"refer to this feature as PR gating or PR Check settings.\n\n" +
			"The API has no delete endpoint for this configuration, so destroying this resource stops Terraform " +
			"managing it but leaves the settings in place in Aikido. Re-creating the resource re-asserts them.\n\n" +
			"Requires the `repositories:read` and `repositories:write` API scopes.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "The code repository ID. The CI checks configuration is one-to-one with the " +
					"repository, so this matches `code_repo_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"code_repo_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the code repository whose CI checks to manage.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"minimum_severity": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Minimum severity of new issues for which the CI check should fail: `low`, " +
					"`medium`, `high`, `critical`, or `always_pass_check`. Use `always_pass_check` to run the " +
					"enabled scans without ever failing the check.",
				Validators: []validator.String{
					stringvalidator.OneOf("low", "medium", "high", "critical", "always_pass_check"),
				},
			},
			"fail_on_dependency_scan": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the CI check should fail for new open source dependency issues.",
			},
			"fail_on_sast_scan": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the CI check should fail for new SAST issues.",
			},
			"fail_on_iac_scan": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the CI check should fail for new IaC issues.",
			},
			"fail_on_secrets_scan": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the CI check should fail for new secrets issues.",
			},
			"fail_on_malware_scan": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the CI check should fail for new malware issues.",
			},
			"minimum_license_severity": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Minimum severity of new license issues for which the CI check should fail: " +
					"`none`, `high`, or `critical`. `none` means the check never fails for license issues.",
				Validators: []validator.String{
					stringvalidator.OneOf("none", "high", "critical"),
				},
			},
			"enable_code_quality_scan": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the code quality scan runs as part of the CI check.",
			},
			"fail_on_code_quality_scan": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the CI check should fail for new code quality issues.",
			},
			"post_code_quality_inline_comments_min_severity": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Minimum severity of new code quality issues for which inline comments are " +
					"posted: `low`, `medium`, `high`, or `critical`. Must not be set when " +
					"`enable_code_quality_scan` is `false`.",
				// Optional and computed, so omitting it plans as unknown as soon as a sibling changes. Reusing prior
				// state avoids a phantom "(known after apply)" diff; the value is written from the plan, never
				// derived by the API.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("low", "medium", "high", "critical"),
				},
			},
			"post_inline_comments_min_severity": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Minimum severity of new issues for which inline comments are posted: `none`, " +
					"`low`, `medium`, `high`, or `critical`. The API defaults this to `none`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("none", "low", "medium", "high", "critical"),
				},
			},
			"run_deep_audit_pr_scan": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the Deep Review scan runs on pull requests.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"post_deep_audit_inline_comments_min_severity": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Minimum severity of new Deep Review issues for which inline comments are " +
					"posted: `none`, `low`, `medium`, `high`, or `critical`.\n\n" +
					"The API accepts this value but does not return it, so Terraform cannot detect drift in it " +
					"and it is not populated on import.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("none", "low", "medium", "high", "critical"),
				},
			},
		},
	}
}

func (r *CodeRepoCIChecksResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&ciChecksCodeQualityValidator{},
	}
}

func (r *CodeRepoCIChecksResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CodeRepoCIChecksResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CodeRepoCIChecksResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyConfiguration(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "created ci checks configuration", map[string]interface{}{
		"code_repo_id": data.CodeRepoID.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CodeRepoCIChecksResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CodeRepoCIChecksResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repoID, err := strconv.Atoi(data.CodeRepoID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Code Repo ID", fmt.Sprintf("Cannot parse code_repo_id: %s", err))
		return
	}

	config, err := r.client.GetCIChecksConfiguration(ctx, repoID)
	if err != nil {
		// Unlike the workspace-wide autofix settings, a per-repository CI checks configuration may genuinely not exist:
		// it is created by the first write. Removing the resource lets Terraform plan a re-create, which the upsert
		// endpoint satisfies.
		if errors.Is(err, client.ErrCIChecksNotFound) {
			tflog.Warn(ctx, "ci checks configuration not found, removing from state", map[string]interface{}{
				"code_repo_id": repoID,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading CI Checks Configuration",
			fmt.Sprintf("Unable to read CI checks configuration for code repo %d: %s", repoID, err),
		)
		return
	}

	mapCIChecksToModel(config, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CodeRepoCIChecksResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CodeRepoCIChecksResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyConfiguration(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "updated ci checks configuration", map[string]interface{}{
		"code_repo_id": data.CodeRepoID.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CodeRepoCIChecksResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CodeRepoCIChecksResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API has no delete endpoint for CI checks, and no documented "off" state: minimum_severity =
	// always_pass_check with every fail_on_* false is only a guess at neutral, it would discard the practitioner's
	// other settings, and it could not be undone on re-create. Removing the resource from state therefore leaves the
	// configuration in place in Aikido, which the resource description states.
	tflog.Warn(ctx, "ci checks configuration left in place in Aikido; the API has no delete endpoint (delete)",
		map[string]interface{}{"code_repo_id": data.CodeRepoID.ValueString()})
}

func (r *CodeRepoCIChecksResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("code_repo_id"), req.ID)...)
}

// applyConfiguration writes the planned configuration to the API and normalises the model so that state matches what
// the API stores.
//
// State is set from the plan rather than from a read-back: the write is a full replace, so the plan already describes
// exactly what the API stores, and re-reading would spend a second request per apply against a tightly rate-limited
// API for no new information.
func (r *CodeRepoCIChecksResource) applyConfiguration(ctx context.Context, data *CodeRepoCIChecksResourceModel, diags *diag.Diagnostics) {
	repoID, err := strconv.Atoi(data.CodeRepoID.ValueString())
	if err != nil {
		diags.AddError("Invalid Code Repo ID", fmt.Sprintf("Cannot parse code_repo_id: %s", err))
		return
	}

	saveReq := client.SaveCIChecksConfigurationRequest{
		CodeRepoID:             repoID,
		MinimumSeverity:        data.MinimumSeverity.ValueString(),
		FailOnDependencyScan:   data.FailOnDependencyScan.ValueBool(),
		FailOnSastScan:         data.FailOnSastScan.ValueBool(),
		FailOnIacScan:          data.FailOnIacScan.ValueBool(),
		FailOnSecretsScan:      data.FailOnSecretsScan.ValueBool(),
		FailOnMalwareScan:      data.FailOnMalwareScan.ValueBool(),
		MinimumLicenseSeverity: data.MinimumLicenseSeverity.ValueString(),
		EnableCodeQualityScan:  data.EnableCodeQualityScan.ValueBool(),
		FailOnCodeQualityScan:  data.FailOnCodeQualityScan.ValueBool(),
	}

	// The API requires this key to be present but accepts null, so a nil pointer is exactly right when the
	// practitioner omitted it or the code quality scan is off.
	if isKnown(data.PostCodeQualityInlineCommentsMinSeverity) {
		severity := data.PostCodeQualityInlineCommentsMinSeverity.ValueString()
		saveReq.PostCodeQualityInlineCommentsMinSeverity = &severity
	}

	// The remaining optional fields are omitted entirely when not configured, so the API applies its own defaults.
	if isKnown(data.PostInlineCommentsMinSeverity) {
		severity := data.PostInlineCommentsMinSeverity.ValueString()
		saveReq.PostInlineCommentsMinSeverity = &severity
	}
	if !data.RunDeepAuditPRScan.IsNull() && !data.RunDeepAuditPRScan.IsUnknown() {
		runDeepAudit := data.RunDeepAuditPRScan.ValueBool()
		saveReq.RunDeepAuditPRScan = &runDeepAudit
	}
	if isKnown(data.PostDeepAuditInlineCommentsMinSeverity) {
		severity := data.PostDeepAuditInlineCommentsMinSeverity.ValueString()
		saveReq.PostDeepAuditInlineCommentsMinSeverity = &severity
	}

	if err := r.client.SaveCIChecksConfiguration(ctx, saveReq); err != nil {
		diags.AddError(
			"Error Saving CI Checks Configuration",
			fmt.Sprintf("Unable to save CI checks configuration for code repo %d: %s", repoID, err),
		)
		return
	}

	// Optional and computed attributes omitted from the configuration are unknown in the plan. Nothing is read back,
	// so they are resolved to null to keep state fully known: an unknown reaching state fails the apply with
	// "invalid result object after apply".
	if data.PostCodeQualityInlineCommentsMinSeverity.IsUnknown() {
		data.PostCodeQualityInlineCommentsMinSeverity = types.StringNull()
	}
	if data.PostInlineCommentsMinSeverity.IsUnknown() {
		data.PostInlineCommentsMinSeverity = types.StringNull()
	}
	if data.RunDeepAuditPRScan.IsUnknown() {
		data.RunDeepAuditPRScan = types.BoolNull()
	}
	if data.PostDeepAuditInlineCommentsMinSeverity.IsUnknown() {
		data.PostDeepAuditInlineCommentsMinSeverity = types.StringNull()
	}

	data.ID = types.StringValue(strconv.Itoa(repoID))
}

// isKnown reports whether a string attribute holds a usable value, meaning it is neither null nor still unknown.
func isKnown(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// mapCIChecksToModel populates the Terraform model from an API response.
//
// post_deep_audit_inline_comments_min_severity is deliberately not touched: the API does not return it, so the prior
// value is left in place rather than being reported as a change to null. This mirrors the handling of the container
// fields the container API omits.
func mapCIChecksToModel(config *client.CIChecksConfiguration, data *CodeRepoCIChecksResourceModel) {
	data.ID = types.StringValue(strconv.Itoa(config.CodeRepoID))
	data.CodeRepoID = types.StringValue(strconv.Itoa(config.CodeRepoID))
	data.MinimumSeverity = types.StringValue(config.MinimumSeverity)
	data.FailOnDependencyScan = types.BoolValue(config.FailOnDependencyScan)
	data.FailOnSastScan = types.BoolValue(config.FailOnSastScan)
	data.FailOnIacScan = types.BoolValue(config.FailOnIacScan)
	data.FailOnSecretsScan = types.BoolValue(config.FailOnSecretsScan)
	data.FailOnMalwareScan = types.BoolValue(config.FailOnMalwareScan)
	data.MinimumLicenseSeverity = types.StringValue(config.MinimumLicenseSeverity)
	data.EnableCodeQualityScan = types.BoolValue(config.EnableCodeQualityScan)
	data.FailOnCodeQualityScan = types.BoolValue(config.FailOnCodeQualityScan)
	data.PostInlineCommentsMinSeverity = types.StringValue(config.PostInlineCommentsMinSeverity)
	data.RunDeepAuditPRScan = types.BoolValue(config.RunDeepAuditPRScan)

	// The API returns null when the code quality scan is disabled.
	if config.PostCodeQualityInlineCommentsMinSeverity != nil {
		data.PostCodeQualityInlineCommentsMinSeverity = types.StringValue(*config.PostCodeQualityInlineCommentsMinSeverity)
	} else {
		data.PostCodeQualityInlineCommentsMinSeverity = types.StringNull()
	}
}

// ciChecksCodeQualityValidator enforces the one interaction the API documents: the code quality inline comment
// severity is only meaningful when the code quality scan is enabled, and the API accepts null for it when the scan is
// off.
//
// This is a resource-level ConfigValidator rather than resourcevalidator.ConflictsWith because the rule depends on the
// value of enable_code_quality_scan, not merely on whether it is set.
//
// Deliberately not enforced: that the severity is required when the scan is enabled, any tie between
// fail_on_code_quality_scan and enable_code_quality_scan, and any precondition for the Deep Review scan. The API
// documents none of those, and a live apply confirmed it accepts a null severity alongside an enabled scan, so
// rejecting these here would fail configurations the API accepts.
type ciChecksCodeQualityValidator struct{}

var _ resource.ConfigValidator = &ciChecksCodeQualityValidator{}

func (v *ciChecksCodeQualityValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

func (v *ciChecksCodeQualityValidator) MarkdownDescription(context.Context) string {
	return "Ensures `post_code_quality_inline_comments_min_severity` is only set when " +
		"`enable_code_quality_scan` is `true`."
}

func (v *ciChecksCodeQualityValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var enableCodeQuality types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("enable_code_quality_scan"), &enableCodeQuality)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var severity types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("post_code_quality_inline_comments_min_severity"), &severity)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateCIChecksCodeQuality(enableCodeQuality, severity)...)
}

// validateCIChecksCodeQuality holds the code quality rule itself, separated from attribute plumbing so that it can be
// unit tested directly.
func validateCIChecksCodeQuality(enableCodeQuality types.Bool, severity types.String) diag.Diagnostics {
	var diags diag.Diagnostics

	// A value that is not yet known cannot be validated. This is common when it is derived from another resource or a
	// data source. The scan being enabled needs no check: the API documents no requirement to supply a severity then.
	if enableCodeQuality.IsNull() || enableCodeQuality.IsUnknown() || enableCodeQuality.ValueBool() ||
		severity.IsUnknown() {
		return diags
	}

	if !severity.IsNull() {
		diags.AddAttributeError(
			path.Root("post_code_quality_inline_comments_min_severity"),
			"Conflicting Attribute Configuration",
			"`post_code_quality_inline_comments_min_severity` must not be set when `enable_code_quality_scan` is "+
				"`false`, because no code quality issues are found to comment on.",
		)
	}

	return diags
}
