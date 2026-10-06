package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

// ciChecksDefaultID is the fixed identifier of the workspace-wide default CI checks configuration.
const ciChecksDefaultID = "default"

// ciChecksDefaultCodeQualitySeverity is the code quality inline comment severity the API applies when the code quality
// scan is enabled and none is given.
const ciChecksDefaultCodeQualitySeverity = "low"

var _ resource.Resource = &CodeRepoCIChecksDefaultResource{}
var _ resource.ResourceWithImportState = &CodeRepoCIChecksDefaultResource{}
var _ resource.ResourceWithConfigValidators = &CodeRepoCIChecksDefaultResource{}

// NewCodeRepoCIChecksDefaultResource creates a new default CI checks configuration resource.
func NewCodeRepoCIChecksDefaultResource() resource.Resource {
	return &CodeRepoCIChecksDefaultResource{}
}

// CodeRepoCIChecksDefaultResource manages the workspace default CI checks configuration.
type CodeRepoCIChecksDefaultResource struct {
	client *client.AikidoClient
}

// CodeRepoCIChecksDefaultResourceModel describes the resource data model.
type CodeRepoCIChecksDefaultResourceModel struct {
	ID                                       types.String `tfsdk:"id"`
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
}

func (r *CodeRepoCIChecksDefaultResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_code_repo_ci_checks_default"
}

func (r *CodeRepoCIChecksDefaultResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the workspace default CI checks configuration in Aikido Security.\n\n" +
			"The default is applied to newly activated code repositories that have no repository-specific " +
			"configuration. It does not change existing repositories; use `aikido_code_repo_ci_checks` for those. " +
			"Aikido's dashboard and documentation refer to this feature as PR gating or PR Check settings.\n\n" +
			"These settings apply to the entire workspace, so only one instance of this resource should exist. " +
			"Destroying it removes the workspace default rather than simply ceasing to manage it.\n\n" +
			"Enabling a default configuration requires a paying Aikido account.\n\n" +
			"Requires the `repositories:read` and `repositories:write` API scopes.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Fixed identifier for this workspace-wide configuration. Always `default`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
					"`none`, `high`, or `critical`. `none` means the check never fails for license issues. The API " +
					"only applies this when the CI license scan feature is enabled for the workspace.",
				Validators: []validator.String{
					stringvalidator.OneOf("none", "high", "critical"),
				},
			},
			"enable_code_quality_scan": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the code quality scan runs as part of the CI check.",
			},
			"fail_on_code_quality_scan": schema.BoolAttribute{
				Required: true,
				MarkdownDescription: "Whether the CI check should fail for new code quality issues. Must be `false` " +
					"when `enable_code_quality_scan` is `false`.",
			},
			"post_code_quality_inline_comments_min_severity": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Minimum severity of new code quality issues for which inline comments are " +
					"posted: `low`, `medium`, `high`, or `critical`. Defaults to `low` when " +
					"`enable_code_quality_scan` is `true`. Must not be set when `enable_code_quality_scan` is `false`.",
				PlanModifiers: []planmodifier.String{
					ciChecksDefaultCodeQualitySeverityModifier{},
				},
				Validators: []validator.String{
					stringvalidator.OneOf("low", "medium", "high", "critical"),
				},
			},
			"post_inline_comments_min_severity": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("none"),
				MarkdownDescription: "Minimum severity of new issues for which inline comments are posted: `none`, " +
					"`low`, `medium`, `high`, or `critical`. Defaults to `none`, which disables inline comments.",
				Validators: []validator.String{
					stringvalidator.OneOf("none", "low", "medium", "high", "critical"),
				},
			},
			"run_deep_audit_pr_scan": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "Whether the Deep Review scan runs on pull requests. Defaults to `false`. The " +
					"API only allows it in the EU and US regions, and only with at least one `fail_on_*` " +
					"vulnerability scan enabled.",
			},
		},
	}
}

func (r *CodeRepoCIChecksDefaultResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&ciChecksCodeQualityValidator{},
		&ciChecksDefaultValidator{},
	}
}

func (r *CodeRepoCIChecksDefaultResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CodeRepoCIChecksDefaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CodeRepoCIChecksDefaultResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyConfiguration(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "created default ci checks configuration", map[string]interface{}{"id": ciChecksDefaultID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CodeRepoCIChecksDefaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CodeRepoCIChecksDefaultResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.GetCIChecksDefaultConfiguration(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Default CI Checks Configuration",
			fmt.Sprintf("Unable to read the default CI checks configuration: %s", err),
		)
		return
	}

	// With no default configured the API returns fallback values rather than a 404. Those describe nothing that
	// exists, so the resource is removed and Terraform plans a re-create.
	if !config.IsEnabled {
		tflog.Warn(ctx, "default ci checks configuration not found, removing from state", map[string]interface{}{
			"id": ciChecksDefaultID,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	mapCIChecksDefaultToModel(config, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CodeRepoCIChecksDefaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CodeRepoCIChecksDefaultResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyConfiguration(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "updated default ci checks configuration", map[string]interface{}{"id": ciChecksDefaultID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CodeRepoCIChecksDefaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CodeRepoCIChecksDefaultResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no delete endpoint. The API documents that saving with every scan flag false removes the default, and
	// the bool fields of the request are left at their false zero values to do exactly that. The severities are still
	// required by the request schema, so the prior values are sent.
	err := r.client.SaveCIChecksDefaultConfiguration(ctx, client.SaveCIChecksDefaultConfigurationRequest{
		MinimumSeverity:               data.MinimumSeverity.ValueString(),
		MinimumLicenseSeverity:        data.MinimumLicenseSeverity.ValueString(),
		PostInlineCommentsMinSeverity: "none",
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Removing Default CI Checks Configuration",
			fmt.Sprintf("Unable to remove the default CI checks configuration: %s", err),
		)
		return
	}

	tflog.Debug(ctx, "removed default ci checks configuration (delete)", map[string]interface{}{"id": ciChecksDefaultID})
}

func (r *CodeRepoCIChecksDefaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// The default configuration is a workspace-wide singleton, so the import ID is ignored.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), ciChecksDefaultID)...)
}

// applyConfiguration writes the planned configuration to the API.
//
// State is set from the plan rather than from a read-back: the write is a full replace and every attribute is known
// in the plan, so re-reading would spend a second request against a tightly rate-limited API for no new information.
func (r *CodeRepoCIChecksDefaultResource) applyConfiguration(ctx context.Context, data *CodeRepoCIChecksDefaultResourceModel, diags *diag.Diagnostics) {
	// The plan modifier resolves the code quality severity unless enable_code_quality_scan was itself unknown at plan
	// time. It is known now, so resolve the same way here.
	if data.PostCodeQualityInlineCommentsMinSeverity.IsUnknown() {
		data.PostCodeQualityInlineCommentsMinSeverity = ciChecksDefaultCodeQualitySeverityFor(data.EnableCodeQualityScan)
	}

	saveReq := client.SaveCIChecksDefaultConfigurationRequest{
		MinimumSeverity:               data.MinimumSeverity.ValueString(),
		FailOnDependencyScan:          data.FailOnDependencyScan.ValueBool(),
		FailOnSastScan:                data.FailOnSastScan.ValueBool(),
		FailOnIacScan:                 data.FailOnIacScan.ValueBool(),
		FailOnSecretsScan:             data.FailOnSecretsScan.ValueBool(),
		FailOnMalwareScan:             data.FailOnMalwareScan.ValueBool(),
		MinimumLicenseSeverity:        data.MinimumLicenseSeverity.ValueString(),
		PostInlineCommentsMinSeverity: data.PostInlineCommentsMinSeverity.ValueString(),
		EnableCodeQualityScan:         data.EnableCodeQualityScan.ValueBool(),
		FailOnCodeQualityScan:         data.FailOnCodeQualityScan.ValueBool(),
		RunDeepAuditPRScan:            data.RunDeepAuditPRScan.ValueBool(),
	}

	if isKnown(data.PostCodeQualityInlineCommentsMinSeverity) {
		severity := data.PostCodeQualityInlineCommentsMinSeverity.ValueString()
		saveReq.PostCodeQualityInlineCommentsMinSeverity = &severity
	}

	if err := r.client.SaveCIChecksDefaultConfiguration(ctx, saveReq); err != nil {
		diags.AddError(
			"Error Saving Default CI Checks Configuration",
			fmt.Sprintf("Unable to save the default CI checks configuration: %s", err),
		)
		return
	}

	data.ID = types.StringValue(ciChecksDefaultID)
}

// mapCIChecksDefaultToModel populates the Terraform model from an API response, translating the API's read
// representation of "off" back to the values the configuration uses for it.
func mapCIChecksDefaultToModel(config *client.CIChecksDefaultConfiguration, data *CodeRepoCIChecksDefaultResourceModel) {
	data.ID = types.StringValue(ciChecksDefaultID)
	data.MinimumSeverity = types.StringValue(config.MinimumSeverity)
	data.FailOnDependencyScan = types.BoolValue(config.FailOnDependencyScan)
	data.FailOnSastScan = types.BoolValue(config.FailOnSastScan)
	data.FailOnIacScan = types.BoolValue(config.FailOnIacScan)
	data.FailOnSecretsScan = types.BoolValue(config.FailOnSecretsScan)
	data.FailOnMalwareScan = types.BoolValue(config.FailOnMalwareScan)
	data.MinimumLicenseSeverity = types.StringValue(config.MinimumLicenseSeverity)
	data.EnableCodeQualityScan = types.BoolValue(config.EnableCodeQualityScan)
	data.FailOnCodeQualityScan = types.BoolValue(config.FailOnCodeQualityScan)
	data.RunDeepAuditPRScan = types.BoolValue(config.RunDeepAuditPRScan)

	// The API returns null for disabled inline comments, but accepts and defaults to "none" on write.
	if config.PostInlineCommentsMinSeverity != nil {
		data.PostInlineCommentsMinSeverity = types.StringValue(*config.PostInlineCommentsMinSeverity)
	} else {
		data.PostInlineCommentsMinSeverity = types.StringValue("none")
	}

	// The API returns "none" when the code quality scan is disabled, which the write enum does not accept.
	if config.EnableCodeQualityScan && config.PostCodeQualityInlineCommentsMinSeverity != "none" {
		data.PostCodeQualityInlineCommentsMinSeverity = types.StringValue(config.PostCodeQualityInlineCommentsMinSeverity)
	} else {
		data.PostCodeQualityInlineCommentsMinSeverity = types.StringNull()
	}
}

// ciChecksDefaultCodeQualitySeverityFor returns the code quality inline comment severity the API stores when none is
// configured: its documented default while the scan is enabled, and nothing while it is disabled.
func ciChecksDefaultCodeQualitySeverityFor(enableCodeQuality types.Bool) types.String {
	if enableCodeQuality.IsUnknown() {
		return types.StringUnknown()
	}
	if enableCodeQuality.ValueBool() {
		return types.StringValue(ciChecksDefaultCodeQualitySeverity)
	}
	return types.StringNull()
}

// ciChecksDefaultCodeQualitySeverityModifier plans an omitted post_code_quality_inline_comments_min_severity as the
// value the API will store, which depends on enable_code_quality_scan.
//
// UseStateForUnknown would be wrong here: after the scan is turned off it would carry the old severity forward, while
// the API reports none, producing a perpetual diff.
type ciChecksDefaultCodeQualitySeverityModifier struct{}

var _ planmodifier.String = ciChecksDefaultCodeQualitySeverityModifier{}

func (m ciChecksDefaultCodeQualitySeverityModifier) Description(ctx context.Context) string {
	return m.MarkdownDescription(ctx)
}

func (m ciChecksDefaultCodeQualitySeverityModifier) MarkdownDescription(context.Context) string {
	return "When not configured, plans `low` if `enable_code_quality_scan` is `true` and null otherwise."
}

func (m ciChecksDefaultCodeQualitySeverityModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}

	var enableCodeQuality types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("enable_code_quality_scan"), &enableCodeQuality)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.PlanValue = ciChecksDefaultCodeQualitySeverityFor(enableCodeQuality)
}

// ciChecksDefaultValidator enforces the rules the default configuration endpoint documents beyond those it shares with
// the per-repository endpoint:
//
//   - At least one scan flag must be enabled. The API removes the default when every flag is false, which would leave
//     the resource reading back as absent and planning a re-create forever. Destroying the resource is the way to
//     remove the default.
//   - fail_on_code_quality_scan must be false while the code quality scan is disabled, because the API forces it to
//     false and the configured true would otherwise be reported as drift on every plan.
type ciChecksDefaultValidator struct{}

var _ resource.ConfigValidator = &ciChecksDefaultValidator{}

func (v *ciChecksDefaultValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

func (v *ciChecksDefaultValidator) MarkdownDescription(context.Context) string {
	return "Ensures at least one scan is enabled, and that `fail_on_code_quality_scan` is only `true` when " +
		"`enable_code_quality_scan` is `true`."
}

func (v *ciChecksDefaultValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data CodeRepoCIChecksDefaultResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateCIChecksDefault(&data)...)
}

// validateCIChecksDefault holds the default configuration rules, separated from attribute plumbing so that they can be
// unit tested directly.
func validateCIChecksDefault(data *CodeRepoCIChecksDefaultResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if !data.EnableCodeQualityScan.IsNull() && !data.EnableCodeQualityScan.IsUnknown() &&
		!data.EnableCodeQualityScan.ValueBool() && data.FailOnCodeQualityScan.ValueBool() {
		diags.AddAttributeError(
			path.Root("fail_on_code_quality_scan"),
			"Conflicting Attribute Configuration",
			"`fail_on_code_quality_scan` must be `false` when `enable_code_quality_scan` is `false`, because the "+
				"API forces it to `false`.",
		)
	}

	// run_deep_audit_pr_scan is optional, so null counts as false, its default.
	scanFlags := []types.Bool{
		data.FailOnDependencyScan,
		data.FailOnSastScan,
		data.FailOnIacScan,
		data.FailOnSecretsScan,
		data.FailOnMalwareScan,
		data.EnableCodeQualityScan,
		data.RunDeepAuditPRScan,
	}
	for _, flag := range scanFlags {
		// A value that is not yet known might turn out to be true, so the configuration cannot be rejected.
		if flag.IsUnknown() || flag.ValueBool() {
			return diags
		}
	}

	diags.AddError(
		"No Scans Enabled",
		"At least one of the `fail_on_*` scans, `enable_code_quality_scan` or `run_deep_audit_pr_scan` must be "+
			"`true`. The API removes the default configuration when they are all `false`; destroy this resource "+
			"to remove it instead.",
	)

	return diags
}
